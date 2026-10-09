// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/qui/internal/models"
)

func syncRuleIDs(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	instanceID, err := parseInstanceID(w, r)
	if err != nil {
		return 0, 0, false
	}
	ruleID, err := strconv.Atoi(chi.URLParam(r, "ruleID"))
	if err != nil || ruleID <= 0 {
		RespondError(w, http.StatusBadRequest, "Invalid automation ID")
		return 0, 0, false
	}
	return instanceID, ruleID, true
}

func (h *AutomationHandler) respondSyncError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		RespondError(w, http.StatusNotFound, "Automation not found")
	case errors.Is(err, models.ErrAutomationSyncConflict), errors.Is(err, models.ErrAutomationSyncReadOnly):
		RespondError(w, http.StatusConflict, err.Error())
	default:
		log.Error().Err(err).Msg("automation sync failed")
		RespondError(w, http.StatusInternalServerError, "Failed to synchronize automations")
	}
}

func (h *AutomationHandler) validateSyncTarget(ctx context.Context, rule *models.Automation, target models.AutomationSyncTarget) error {
	if _, err := h.instanceStore.Get(ctx, target.InstanceID); err != nil {
		return fmt.Errorf("target instance %d not found: %w", target.InstanceID, err)
	}
	candidate := *rule
	if target.PreserveEnabled {
		existing, err := h.store.ResolveSyncTarget(ctx, rule, target)
		if err != nil {
			return err
		}
		if existing != nil {
			candidate.Enabled = existing.Enabled
		}
	}
	data, err := json.Marshal(&candidate)
	if err != nil {
		return err
	}
	var payload AutomationPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if _, message, err := h.validatePayload(ctx, target.InstanceID, &payload); err != nil {
		return fmt.Errorf("target instance %d: %s: %w", target.InstanceID, message, err)
	}
	return nil
}

func (h *AutomationHandler) GetSync(w http.ResponseWriter, r *http.Request) {
	instanceID, ruleID, ok := syncRuleIDs(w, r)
	if !ok {
		return
	}
	targets, err := h.store.SyncTargets(r.Context(), instanceID, ruleID)
	if err != nil {
		h.respondSyncError(w, err)
		return
	}
	RespondJSON(w, http.StatusOK, targets)
}

func (h *AutomationHandler) ConfigureSync(w http.ResponseWriter, r *http.Request) {
	instanceID, ruleID, ok := syncRuleIDs(w, r)
	if !ok {
		return
	}
	var options models.AutomationSyncOptions
	if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid sync settings")
		return
	}
	source, err := h.store.Get(r.Context(), instanceID, ruleID)
	if err != nil {
		h.respondSyncError(w, err)
		return
	}
	for _, target := range options.Targets {
		if err := h.validateSyncTarget(r.Context(), source, target); err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	result, err := h.store.ConfigureSync(r.Context(), instanceID, ruleID, options)
	if err != nil {
		h.respondSyncError(w, err)
		return
	}
	RespondJSON(w, http.StatusOK, result)
}

func (h *AutomationHandler) DetachSync(w http.ResponseWriter, r *http.Request) {
	instanceID, ruleID, ok := syncRuleIDs(w, r)
	if !ok {
		return
	}
	if err := h.store.DetachSync(r.Context(), instanceID, ruleID); err != nil {
		h.respondSyncError(w, err)
		return
	}
	RespondJSON(w, http.StatusNoContent, nil)
}
