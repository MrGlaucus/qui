// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/pkg/timeutil"
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

type TrackerTrafficHandler struct {
	store    *models.TrackerTrafficStore
	timezone *timeutil.Provider
}

func NewTrackerTrafficHandler(store *models.TrackerTrafficStore, timezone *timeutil.Provider) *TrackerTrafficHandler {
	return &TrackerTrafficHandler{store: store, timezone: timezone}
}

type TrackerTrafficResponse struct {
	Date   string                     `json:"date"`
	Items  []models.TrackerTrafficRow `json:"items"`
	Totals []models.TrackerTrafficRow `json:"totals"`
}

// Get returns the selected calendar day's tracker traffic and all recorded totals.
func (h *TrackerTrafficHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		RespondError(w, http.StatusServiceUnavailable, "Tracker traffic is unavailable")
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
		if h.timezone != nil {
			date = h.timezone.Today()
		}
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid date")
		return
	}
	items, err := h.store.List(r.Context(), date)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to load tracker traffic")
		return
	}
	totals, err := h.store.Totals(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to load tracker traffic totals")
		return
	}
	RespondJSON(w, http.StatusOK, TrackerTrafficResponse{Date: date, Items: items, Totals: totals})
}
