// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package qbittorrent

import (
	"context"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/pkg/timeutil"
	"github.com/rs/zerolog/log"
)

// dailyTrafficStore is the persistence surface the recorder needs. Keeping it
// a minimal interface lets the recorder be unit-tested without a database.
type dailyTrafficStore interface {
	GetByInstanceAndDate(ctx context.Context, instanceID int, date string) (*models.InstanceDailyTraffic, error)
	Upsert(ctx context.Context, instanceID int, date string, uploaded, downloaded, peakUlSpeed, peakDlSpeed int64, dataSource string, baseline *models.TrafficBaseline) (*models.InstanceDailyTraffic, error)
	DeleteOlderThan(ctx context.Context, instanceID int, beforeDate string) (int64, error)
}

// DailyTrafficRecorder attributes per-instance daily traffic from the live
// qBittorrent MainData stream and persists the day totals to
// instance_daily_traffic.
//
// qBittorrent exposes two cumulative counters:
//   - session (DlInfoData/UpInfoData): exact byte counts for the current process;
//     resets to 0 when qBittorrent restarts.
//   - alltime (AlltimeDl/AlltimeUl): persists across restarts but can drift and
//     resets on reinstall.
//
// Day boundaries use the server-local timezone. The day-baseline snapshot (the
// counters captured at the first sample of the day) lives in the database row,
// so the recorder holds no in-memory counters: each sample reads the row's
// baseline and attributes current - baseline. Session deltas are used by default
// (most accurate against PT-tracker increments) and fall back to alltime deltas
// when a session reset (restart) is detected. A reinstall (both counters
// regress) re-baselines the row without emitting a delta.
type DailyTrafficRecorder struct {
	store dailyTrafficStore
	now   func() time.Time

	// retentionDays keeps history rolling; rows older than this many days are
	// pruned opportunistically on day rollover.
	retentionDays int
}

// NewDailyTrafficRecorder builds a recorder using the given store and server-local time.
func NewDailyTrafficRecorder(store *models.InstanceDailyTrafficStore, retentionDays int) *DailyTrafficRecorder {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	return &DailyTrafficRecorder{
		store:         store,
		now:           func() time.Time { return time.Now() },
		retentionDays: retentionDays,
	}
}

// NewDailyTrafficRecorderWithTimezone builds a recorder whose day boundaries
// follow the given timezone provider (updated at runtime from frontend
// settings), instead of the server-local timezone.
func NewDailyTrafficRecorderWithTimezone(store *models.InstanceDailyTrafficStore, retentionDays int, provider *timeutil.Provider) *DailyTrafficRecorder {
	if retentionDays <= 0 {
		retentionDays = 7
	}
	r := NewDailyTrafficRecorder(store, retentionDays)
	if provider != nil {
		r.now = provider.Now
	}
	return r
}

// trafficDelta is a single sample's computed attribution.
type trafficDelta struct {
	uploaded   int64
	downloaded int64
	source     string
}

// classifyTraffic is pure and unit-testable. It decides how to attribute the
// current sample given the counter deltas since the day-baseline snapshot.
//
//   - session counters increased normally: attribute session deltas.
//   - session counters regressed (restart) but alltime still advanced:
//     attribute alltime deltas and note the restart.
//   - both regressed (reinstall): no traffic attributed; caller re-baselines.
func classifyTraffic(sessionUlDelta, sessionDlDelta, allUlDelta, allDlDelta int64) trafficDelta {
	sessionOK := sessionUlDelta >= 0 && sessionDlDelta >= 0
	allOK := allUlDelta >= 0 && allDlDelta >= 0

	switch {
	case sessionOK && allOK:
		return trafficDelta{uploaded: sessionUlDelta, downloaded: sessionDlDelta, source: models.DataSourceSession}
	case sessionOK && !allOK:
		// Alltime reset while session keeps running — reinstall baseline.
		return trafficDelta{uploaded: 0, downloaded: 0, source: models.DataSourceReinstall}
	case !sessionOK && allOK:
		// qBittorrent restarted mid-day: fall back to alltime for the gap.
		return trafficDelta{uploaded: allUlDelta, downloaded: allDlDelta, source: models.DataSourceRestart}
	default:
		// Both reset — reinstall.
		return trafficDelta{uploaded: 0, downloaded: 0, source: models.DataSourceReinstall}
	}
}

// Record ingests a server_state sample for an instance and persists the
// resulting day total. The day-baseline snapshot is read from the row that was
// persisted at the first sample of the day, so the recorder keeps no state
// across restarts. Safe to call from the sync loop.
func (r *DailyTrafficRecorder) Record(ctx context.Context, instanceID int, serverState *qbt.ServerState) {
	if r == nil || serverState == nil || r.store == nil {
		return
	}

	now := r.now()
	day := now.Format("2006-01-02")

	sessionUl := serverState.UpInfoData
	sessionDl := serverState.DlInfoData
	allUl := serverState.AlltimeUl
	allDl := serverState.AlltimeDl

	row, err := r.store.GetByInstanceAndDate(ctx, instanceID, day)
	if err != nil {
		log.Error().Err(err).Int("instanceID", instanceID).Msg("Failed to read daily traffic row")
		return
	}

	// First sample of the day: persist the day-boundary baseline snapshot with
	// no delta, then prune old history opportunistically.
	if row == nil || row.BaselineAt == "" {
		if _, err := r.store.Upsert(ctx, instanceID, day, 0, 0, serverState.UpInfoSpeed, serverState.DlInfoSpeed, models.DataSourceSession, &models.TrafficBaseline{
			SessionUploaded:   sessionUl,
			SessionDownloaded: sessionDl,
			AlltimeUploaded:   allUl,
			AlltimeDownloaded: allDl,
			At:                now.UTC().Format(time.RFC3339),
		}); err != nil {
			log.Error().Err(err).Int("instanceID", instanceID).Msg("Failed to record daily traffic baseline")
		}

		pruneDate := now.AddDate(0, 0, -(r.retentionDays - 1)).Format("2006-01-02")
		if _, err := r.store.DeleteOlderThan(ctx, instanceID, pruneDate); err != nil {
			log.Error().Err(err).Int("instanceID", instanceID).Msg("Failed to prune old daily traffic rows")
		}
		return
	}

	// Subsequent samples: attribute current - baseline.
	delta := classifyTraffic(
		sessionUl-row.BaselineSessionUploaded,
		sessionDl-row.BaselineSessionDownloaded,
		allUl-row.BaselineAlltimeUploaded,
		allDl-row.BaselineAlltimeDownloaded,
	)

	// A reinstall re-baselines the row so the next sample measures from the
	// fresh counters. Other sources leave the baseline untouched.
	var baseline *models.TrafficBaseline
	if delta.source == models.DataSourceReinstall {
		baseline = &models.TrafficBaseline{
			SessionUploaded:   sessionUl,
			SessionDownloaded: sessionDl,
			AlltimeUploaded:   allUl,
			AlltimeDownloaded: allDl,
			At:                now.UTC().Format(time.RFC3339),
		}
	}

	if _, err := r.store.Upsert(
		ctx,
		instanceID,
		day,
		delta.uploaded,
		delta.downloaded,
		serverState.UpInfoSpeed,
		serverState.DlInfoSpeed,
		delta.source,
		baseline,
	); err != nil {
		log.Error().Err(err).Int("instanceID", instanceID).Msg("Failed to record daily traffic")
	}
}
