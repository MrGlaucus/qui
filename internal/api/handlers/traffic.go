// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/autobrr/qui/internal/models"
)

type TrafficHandler struct {
	trafficStore *models.InstanceDailyTrafficStore
}

func NewTrafficHandler(trafficStore *models.InstanceDailyTrafficStore) *TrafficHandler {
	return &TrafficHandler{trafficStore: trafficStore}
}

// InstanceDailyTrafficResponse is a page of daily traffic rows for an instance.
type InstanceDailyTrafficResponse struct {
	Items []*models.InstanceDailyTraffic `json:"items"`
	Total int                            `json:"total"`
}

// GetDailyTraffic returns recent daily traffic rows for an instance.
// Query param `days` (default 7) limits how many latest rows are returned.
func (h *TrafficHandler) GetDailyTraffic(w http.ResponseWriter, r *http.Request) {
	instanceID, err := strconv.Atoi(chi.URLParam(r, "instanceID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid instance ID")
		return
	}

	days := 7
	if raw := r.URL.Query().Get("days"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 365 {
			days = v
		}
	}

	items, err := h.trafficStore.ListHistory(r.Context(), instanceID, days)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to load daily traffic")
		return
	}

	RespondJSON(w, http.StatusOK, InstanceDailyTrafficResponse{
		Items: items,
		Total: len(items),
	})
}