// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models_test

import (
	"context"
	"testing"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/testutil/testdb"
	"github.com/stretchr/testify/require"
)

func TestTrackerTrafficStoreRecordsPositiveDeltasAndKeepsDeletedHistory(t *testing.T) {
	db := testdb.NewMigratedSQLite(t, "tracker-traffic")
	ctx := context.Background()
	instances, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	instance, err := instances.Create(ctx, "Traffic", "http://localhost:8080", "user", "pass", nil, nil, false, nil)
	require.NoError(t, err)
	store := models.NewTrackerTrafficStore(db)

	// Existing counters establish a baseline and are not assigned to today.
	require.NoError(t, store.RecordSamples(ctx, instance.ID, "2026-09-21", []models.TrackerTrafficSample{
		{Hash: "abc", TrackerKey: "group:1", Uploaded: 1000, Downloaded: 500},
	}))
	rows, err := store.List(ctx, "2026-09-21")
	require.NoError(t, err)
	require.Empty(t, rows)

	// Later samples add only positive counter changes.
	require.NoError(t, store.RecordSamples(ctx, instance.ID, "2026-09-21", []models.TrackerTrafficSample{
		{Hash: "abc", TrackerKey: "group:1", Uploaded: 1300, Downloaded: 550},
	}))
	rows, err = store.List(ctx, "2026-09-21")
	require.NoError(t, err)
	require.Equal(t, []models.TrackerTrafficRow{{TrackerKey: "group:1", Date: "2026-09-21", Uploaded: 300, Downloaded: 50}}, rows)

	// A reset re-baselines without subtracting traffic. The saved daily row is
	// independent of the checkpoint, so removing the torrent cannot erase it.
	require.NoError(t, store.RecordSamples(ctx, instance.ID, "2026-09-21", []models.TrackerTrafficSample{
		{Hash: "abc", TrackerKey: "group:1", Uploaded: 10, Downloaded: 5},
	}))
	totals, err := store.Totals(ctx)
	require.NoError(t, err)
	require.Equal(t, []models.TrackerTrafficRow{{TrackerKey: "group:1", Uploaded: 300, Downloaded: 50}}, totals)

	require.NoError(t, store.DeleteCheckpoints(ctx, instance.ID, []string{"abc"}))
	require.NoError(t, store.RecordSamples(ctx, instance.ID, "2026-09-21", []models.TrackerTrafficSample{
		{Hash: "abc", TrackerKey: "group:1", Uploaded: 9000, Downloaded: 4000},
	}))
	totals, err = store.Totals(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(300), totals[0].Uploaded, "re-adding a removed hash must establish a fresh baseline")
}

func TestTrackerTrafficStoreSeparatesDatesAndAggregatesTotals(t *testing.T) {
	db := testdb.NewMigratedSQLite(t, "tracker-traffic-dates")
	ctx := context.Background()
	instances, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	instance, err := instances.Create(ctx, "Traffic", "http://localhost:8080", "user", "pass", nil, nil, false, nil)
	require.NoError(t, err)
	store := models.NewTrackerTrafficStore(db)

	require.NoError(t, store.RecordSamples(ctx, instance.ID, "2026-09-20", []models.TrackerTrafficSample{{Hash: "abc", TrackerKey: "domain:tracker.example", Uploaded: 10}}))
	require.NoError(t, store.RecordSamples(ctx, instance.ID, "2026-09-20", []models.TrackerTrafficSample{{Hash: "abc", TrackerKey: "domain:tracker.example", Uploaded: 20}}))
	require.NoError(t, store.RecordSamples(ctx, instance.ID, "2026-09-21", []models.TrackerTrafficSample{{Hash: "abc", TrackerKey: "domain:tracker.example", Uploaded: 35}}))

	day, err := store.List(ctx, "2026-09-21")
	require.NoError(t, err)
	require.Equal(t, int64(15), day[0].Uploaded)
	totals, err := store.Totals(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(25), totals[0].Uploaded)
}
