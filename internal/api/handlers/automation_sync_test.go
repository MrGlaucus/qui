// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/testutil/testdb"
)

func TestAutomationSyncHTTP(t *testing.T) {
	ctx := context.Background()
	db := testdb.NewMigratedSQLite(t, "sync-handler")
	instances, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	a, err := instances.Create(ctx, "Source", "http://localhost:8080", "u", "p", nil, nil, false, nil)
	require.NoError(t, err)
	b, err := instances.Create(ctx, "Target", "http://localhost:8080", "u", "p", nil, nil, false, nil)
	require.NoError(t, err)
	store := models.NewAutomationStore(db)
	h := NewAutomationHandler(store, nil, instances, nil, nil)
	router := chi.NewRouter()
	router.Route("/instances/{instanceID}/automations", func(r chi.Router) {
		r.Route("/{ruleID}", func(r chi.Router) {
			r.Put("/", h.Update)
			r.Delete("/", h.Delete)
			r.Get("/sync", h.GetSync)
			r.Put("/sync", h.ConfigureSync)
			r.Delete("/sync", h.DetachSync)
		})
	})
	request := func(method, path string, body any, status int) *httptest.ResponseRecorder {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewReader(data)))
		require.Equal(t, status, w.Code, w.Body.String())
		return w
	}
	source, err := store.Create(ctx, &models.Automation{InstanceID: a.ID, Name: "Rule", TrackerPattern: "*", Enabled: true, Conditions: &models.ActionConditions{SchemaVersion: "1", Pause: &models.PauseAction{Enabled: true}}})
	require.NoError(t, err)
	path := fmt.Sprintf("/instances/%d/automations/%d", a.ID, source.ID)
	options := models.AutomationSyncOptions{Automatic: true, Targets: []models.AutomationSyncTarget{{InstanceID: b.ID, PreserveEnabled: true}}}
	request(http.MethodPut, path+"/sync", options, http.StatusOK)
	var followers []models.Automation
	require.NoError(t, json.Unmarshal(request(http.MethodGet, path+"/sync", nil, http.StatusOK).Body.Bytes(), &followers))
	require.Len(t, followers, 1)
	source.Name = "Updated from API"
	request(http.MethodPut, path, source, http.StatusOK)
	target, err := store.Get(ctx, b.ID, followers[0].ID)
	require.NoError(t, err)
	require.Equal(t, source.Name, target.Name)
	targetPath := fmt.Sprintf("/instances/%d/automations/%d", b.ID, target.ID)
	target.Enabled = false
	request(http.MethodPut, targetPath, target, http.StatusOK)
	target.Name = "Local edit"
	request(http.MethodPut, targetPath, target, http.StatusConflict)
	request(http.MethodDelete, path, nil, http.StatusConflict)
	// Target-specific validation rejects exporting a rule to its own destination.
	source.Conditions = &models.ActionConditions{SchemaVersion: "1", ExportToInstance: &models.ExportToInstanceAction{Enabled: true, TargetInstanceID: b.ID}}
	request(http.MethodPut, path, source, http.StatusBadRequest)
	unchanged, err := store.Get(ctx, a.ID, source.ID)
	require.NoError(t, err)
	require.NotNil(t, unchanged.Conditions.Pause)
	request(http.MethodDelete, targetPath+"/sync", nil, http.StatusNoContent)
	request(http.MethodPut, targetPath, target, http.StatusOK)
	request(http.MethodPut, path+"/sync", models.AutomationSyncOptions{Targets: []models.AutomationSyncTarget{{InstanceID: 9999}}}, http.StatusBadRequest)
	request(http.MethodDelete, path+"?followers=keep", nil, http.StatusNoContent)
}
