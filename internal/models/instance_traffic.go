// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"context"
	"database/sql"

	"github.com/autobrr/qui/internal/dbinterface"
)

// InstanceDailyTraffic records cumulative traffic for a UI-timezone calendar day.
// uploaded/downloaded hold the deltas accumulated for that day; peak_ul/dl_speed
// hold the highest instantaneous speed sampled during the day.
//
// Baseline* fields capture the qBittorrent cumulative counters at the first
// sample of the day (the day-boundary baseline). Which pair is meaningful
// depends on DataSource: session rows use BaselineSession*, while alltime or
// restart fallback rows use BaselineAlltime*.
type InstanceDailyTraffic struct {
	ID           int64  `json:"id"`
	InstanceID   int    `json:"instanceId"`
	Date         string `json:"date"` // UI-timezone day, format 2006-01-02
	Uploaded     int64  `json:"uploaded"`
	Downloaded   int64  `json:"downloaded"`
	PeakUlSpeed  int64  `json:"peakUlSpeed"`
	PeakDlSpeed  int64  `json:"peakDlSpeed"`
	DataSource   string `json:"dataSource"` // session | alltime | restart | reinstall
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`

	BaselineSessionUploaded   int64  `json:"baselineSessionUploaded"`
	BaselineSessionDownloaded int64  `json:"baselineSessionDownloaded"`
	BaselineAlltimeUploaded   int64  `json:"baselineAlltimeUploaded"`
	BaselineAlltimeDownloaded int64  `json:"baselineAlltimeDownloaded"`
	BaselineAt                string `json:"baselineAt"` // RFC3339, first sample time of the day
}

// HasBaseline reports whether the row carries a day-boundary baseline snapshot.
func (t *InstanceDailyTraffic) HasBaseline() bool {
	return t.BaselineAt != ""
}

// DataSource values recorded on a row.
const (
	DataSourceSession   = "session"   // normal day, session-level deltas
	DataSourceAlltime   = "alltime"   // fell back to alltime after a qBt restart
	DataSourceRestart   = "restart"   // qBittorrent restarted mid-session
	DataSourceReinstall = "reinstall" // qBittorrent was reinstalled, baseline reset
)

type InstanceDailyTrafficStore struct {
	db dbinterface.Querier
}

func NewInstanceDailyTrafficStore(db dbinterface.Querier) *InstanceDailyTrafficStore {
	return &InstanceDailyTrafficStore{db: db}
}

// Upsert overwrites uploaded/downloaded with the caller-computed day totals
// (current qBittorrent counters minus the day-baseline snapshot) and keeps the
// highest instantaneous peak speeds atomically on conflict.
// The baseline snapshot is written when the row does not yet carry one (first
// sample of the day) or when the sample is classified as a reinstall, which
// re-baselines to the current counters. Other samples leave it untouched.
// Returns the resulting row.
func (s *InstanceDailyTrafficStore) Upsert(
	ctx context.Context,
	instanceID int,
	date string,
	uploaded,
	downloaded,
	peakUlSpeed,
	peakDlSpeed int64,
	dataSource string,
	baseline *TrafficBaseline,
) (*InstanceDailyTraffic, error) {
	if dataSource == "" {
		dataSource = DataSourceSession
	}
	if baseline == nil {
		baseline = &TrafficBaseline{}
	}
	var baselineAt any
	if baseline.At != "" {
		baselineAt = baseline.At
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO instance_daily_traffic
			(instance_id, date, uploaded, downloaded, peak_ul_speed, peak_dl_speed, data_source,
			 baseline_session_uploaded, baseline_session_downloaded, baseline_alltime_uploaded, baseline_alltime_downloaded, baseline_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(instance_id, date) DO UPDATE SET
			uploaded      = excluded.uploaded,
			downloaded    = excluded.downloaded,
			peak_ul_speed = MAX(instance_daily_traffic.peak_ul_speed, excluded.peak_ul_speed),
			peak_dl_speed = MAX(instance_daily_traffic.peak_dl_speed, excluded.peak_dl_speed),
			data_source   = CASE
				WHEN excluded.data_source = 'reinstall' THEN 'reinstall'
				WHEN excluded.data_source = 'restart' AND instance_daily_traffic.data_source IN ('reinstall') THEN instance_daily_traffic.data_source
				WHEN excluded.data_source = 'restart' THEN 'restart'
				ELSE instance_daily_traffic.data_source
			END,
			baseline_session_uploaded   = CASE WHEN instance_daily_traffic.baseline_at IS NULL OR excluded.data_source = 'reinstall' THEN excluded.baseline_session_uploaded ELSE instance_daily_traffic.baseline_session_uploaded END,
			baseline_session_downloaded = CASE WHEN instance_daily_traffic.baseline_at IS NULL OR excluded.data_source = 'reinstall' THEN excluded.baseline_session_downloaded ELSE instance_daily_traffic.baseline_session_downloaded END,
			baseline_alltime_uploaded   = CASE WHEN instance_daily_traffic.baseline_at IS NULL OR excluded.data_source = 'reinstall' THEN excluded.baseline_alltime_uploaded ELSE instance_daily_traffic.baseline_alltime_uploaded END,
			baseline_alltime_downloaded = CASE WHEN instance_daily_traffic.baseline_at IS NULL OR excluded.data_source = 'reinstall' THEN excluded.baseline_alltime_downloaded ELSE instance_daily_traffic.baseline_alltime_downloaded END,
			baseline_at                 = CASE WHEN instance_daily_traffic.baseline_at IS NULL OR excluded.data_source = 'reinstall' THEN excluded.baseline_at ELSE instance_daily_traffic.baseline_at END,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, instance_id, date, uploaded, downloaded, peak_ul_speed, peak_dl_speed, data_source, created_at, updated_at,
			baseline_session_uploaded, baseline_session_downloaded, baseline_alltime_uploaded, baseline_alltime_downloaded, baseline_at
	`, instanceID, date, uploaded, downloaded, peakUlSpeed, peakDlSpeed, dataSource,
		baseline.SessionUploaded, baseline.SessionDownloaded, baseline.AlltimeUploaded, baseline.AlltimeDownloaded, baselineAt)

	return scanDailyTraffic(row)
}

// TrafficBaseline is the day-boundary snapshot of qBittorrent cumulative
// counters captured at the first sample of a calendar day.
type TrafficBaseline struct {
	SessionUploaded   int64
	SessionDownloaded int64
	AlltimeUploaded   int64
	AlltimeDownloaded int64
	At                string // RFC3339 timestamp of the first sample
}

// GetByInstanceAndDate returns the row for an instance on a given date, or nil
// when no row exists yet.
func (s *InstanceDailyTrafficStore) GetByInstanceAndDate(ctx context.Context, instanceID int, date string) (*InstanceDailyTraffic, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, instance_id, date, uploaded, downloaded, peak_ul_speed, peak_dl_speed, data_source, created_at, updated_at,
			baseline_session_uploaded, baseline_session_downloaded, baseline_alltime_uploaded, baseline_alltime_downloaded, baseline_at
		FROM instance_daily_traffic
		WHERE instance_id = ? AND date = ?
	`, instanceID, date)

	t, err := scanDailyTraffic(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// ListHistory returns up to `limit` rows for the instance, newest first.
func (s *InstanceDailyTrafficStore) ListHistory(ctx context.Context, instanceID, limit int) ([]*InstanceDailyTraffic, error) {
	if limit <= 0 {
		limit = 7
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, instance_id, date, uploaded, downloaded, peak_ul_speed, peak_dl_speed, data_source, created_at, updated_at,
			baseline_session_uploaded, baseline_session_downloaded, baseline_alltime_uploaded, baseline_alltime_downloaded, baseline_at
		FROM instance_daily_traffic
		WHERE instance_id = ?
		ORDER BY date DESC
		LIMIT ?
	`, instanceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*InstanceDailyTraffic
	for rows.Next() {
		t := &InstanceDailyTraffic{}
		var baselineAt sql.NullString
		if err := rows.Scan(&t.ID, &t.InstanceID, &t.Date, &t.Uploaded, &t.Downloaded, &t.PeakUlSpeed, &t.PeakDlSpeed, &t.DataSource, &t.CreatedAt, &t.UpdatedAt,
			&t.BaselineSessionUploaded, &t.BaselineSessionDownloaded, &t.BaselineAlltimeUploaded, &t.BaselineAlltimeDownloaded, &baselineAt); err != nil {
			return nil, err
		}
		t.BaselineAt = baselineAt.String
		items = append(items, t)
	}
	return items, rows.Err()
}

// ListByDate returns the daily traffic rows for every instance on a given date,
// ordered by instance id. Used by the daily traffic report to summarize all
// instances for a settled day.
func (s *InstanceDailyTrafficStore) ListByDate(ctx context.Context, date string) ([]*InstanceDailyTraffic, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, instance_id, date, uploaded, downloaded, peak_ul_speed, peak_dl_speed, data_source, created_at, updated_at,
			baseline_session_uploaded, baseline_session_downloaded, baseline_alltime_uploaded, baseline_alltime_downloaded, baseline_at
		FROM instance_daily_traffic
		WHERE date = ?
		ORDER BY instance_id ASC
	`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*InstanceDailyTraffic
	for rows.Next() {
		t := &InstanceDailyTraffic{}
		var baselineAt sql.NullString
		if err := rows.Scan(&t.ID, &t.InstanceID, &t.Date, &t.Uploaded, &t.Downloaded, &t.PeakUlSpeed, &t.PeakDlSpeed, &t.DataSource, &t.CreatedAt, &t.UpdatedAt,
			&t.BaselineSessionUploaded, &t.BaselineSessionDownloaded, &t.BaselineAlltimeUploaded, &t.BaselineAlltimeDownloaded, &baselineAt); err != nil {
			return nil, err
		}
		t.BaselineAt = baselineAt.String
		items = append(items, t)
	}
	return items, rows.Err()
}

// DeleteOlderThan removes rows for the instance whose date string is strictly
// before the given beforeDate (format 2006-01-02). Used to enforce a rolling 7-day window.
func (s *InstanceDailyTrafficStore) DeleteOlderThan(ctx context.Context, instanceID int, beforeDate string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM instance_daily_traffic
		WHERE instance_id = ? AND date < ?
	`, instanceID, beforeDate)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func scanDailyTraffic(row interface{ Scan(...any) error }) (*InstanceDailyTraffic, error) {
	t := &InstanceDailyTraffic{}
	var baselineAt sql.NullString
	if err := row.Scan(&t.ID, &t.InstanceID, &t.Date, &t.Uploaded, &t.Downloaded, &t.PeakUlSpeed, &t.PeakDlSpeed, &t.DataSource, &t.CreatedAt, &t.UpdatedAt,
		&t.BaselineSessionUploaded, &t.BaselineSessionDownloaded, &t.BaselineAlltimeUploaded, &t.BaselineAlltimeDownloaded, &baselineAt); err != nil {
		return nil, err
	}
	t.BaselineAt = baselineAt.String
	return t, nil
}