// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/autobrr/qui/internal/dbinterface"
)

// TrackerTrafficSample is the latest cumulative counter snapshot for one torrent.
type TrackerTrafficSample struct {
	Hash       string
	TrackerKey string
	Uploaded   int64
	Downloaded int64
}

// TrackerTrafficRow contains traffic attributed to one tracker for a calendar day.
type TrackerTrafficRow struct {
	TrackerKey string `json:"trackerKey"`
	Date       string `json:"date"`
	Uploaded   int64  `json:"uploaded"`
	Downloaded int64  `json:"downloaded"`
}

// TrackerTrafficStore persists torrent checkpoints and their positive counter deltas.
type TrackerTrafficStore struct{ db dbinterface.Querier }

func NewTrackerTrafficStore(db dbinterface.Querier) *TrackerTrafficStore {
	return &TrackerTrafficStore{db: db}
}

// RecordSamples atomically advances checkpoints and attributes positive deltas.
// A new or regressed counter establishes a baseline and contributes no traffic.
func (s *TrackerTrafficStore) RecordSamples(ctx context.Context, instanceID int, date string, samples []TrackerTrafficSample) error {
	if s == nil || s.db == nil || len(samples) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	type delta struct{ uploaded, downloaded int64 }
	deltas := make(map[string]delta)
	for _, sample := range samples {
		hash := strings.ToUpper(strings.TrimSpace(sample.Hash))
		key := strings.TrimSpace(sample.TrackerKey)
		if hash == "" {
			continue
		}

		var previousKey string
		var previousUploaded, previousDownloaded int64
		err := tx.QueryRowContext(ctx, `
			SELECT tracker_key, uploaded, downloaded
			FROM tracker_torrent_checkpoints
			WHERE instance_id = ? AND torrent_hash = ?
		`, instanceID, hash).Scan(&previousKey, &previousUploaded, &previousDownloaded)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if key == "" && err == nil {
			key = previousKey
		}
		if key == "" {
			continue
		}

		if err == nil && sample.Uploaded >= previousUploaded && sample.Downloaded >= previousDownloaded {
			d := deltas[key]
			d.uploaded += sample.Uploaded - previousUploaded
			d.downloaded += sample.Downloaded - previousDownloaded
			deltas[key] = d
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO tracker_torrent_checkpoints (instance_id, torrent_hash, tracker_key, uploaded, downloaded)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(instance_id, torrent_hash) DO UPDATE SET
				tracker_key = excluded.tracker_key,
				uploaded = excluded.uploaded,
				downloaded = excluded.downloaded,
				updated_at = CURRENT_TIMESTAMP
		`, instanceID, hash, key, sample.Uploaded, sample.Downloaded); err != nil {
			return err
		}
	}

	for key, d := range deltas {
		if d.uploaded == 0 && d.downloaded == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO tracker_daily_traffic (instance_id, tracker_key, date, uploaded, downloaded)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(instance_id, tracker_key, date) DO UPDATE SET
				uploaded = tracker_daily_traffic.uploaded + excluded.uploaded,
				downloaded = tracker_daily_traffic.downloaded + excluded.downloaded,
				updated_at = CURRENT_TIMESTAMP
		`, instanceID, key, date, d.uploaded, d.downloaded); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *TrackerTrafficStore) DeleteCheckpoints(ctx context.Context, instanceID int, hashes []string) error {
	if s == nil || s.db == nil || len(hashes) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, hash := range hashes {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM tracker_torrent_checkpoints
			WHERE instance_id = ? AND torrent_hash = ?
		`, instanceID, strings.ToUpper(strings.TrimSpace(hash))); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *TrackerTrafficStore) List(ctx context.Context, date string) ([]TrackerTrafficRow, error) {
	where := ""
	args := []any{}
	if date != "" {
		where = "WHERE date = ?"
		args = append(args, date)
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT tracker_key, date, SUM(uploaded), SUM(downloaded)
		FROM tracker_daily_traffic
		%s
		GROUP BY tracker_key, date
		ORDER BY date DESC, tracker_key ASC
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []TrackerTrafficRow
	for rows.Next() {
		var row TrackerTrafficRow
		if err := rows.Scan(&row.TrackerKey, &row.Date, &row.Uploaded, &row.Downloaded); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *TrackerTrafficStore) Totals(ctx context.Context) ([]TrackerTrafficRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tracker_key, '', SUM(uploaded), SUM(downloaded)
		FROM tracker_daily_traffic
		GROUP BY tracker_key
		ORDER BY tracker_key ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []TrackerTrafficRow
	for rows.Next() {
		var row TrackerTrafficRow
		if err := rows.Scan(&row.TrackerKey, &row.Date, &row.Uploaded, &row.Downloaded); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
