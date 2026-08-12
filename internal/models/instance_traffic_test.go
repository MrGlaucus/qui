// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/testutil/testdb"
)

func TestInstanceDailyTrafficStore_UpsertMergesAndMaxes(t *testing.T) {
	db := testdb.NewMigratedSQLite(t, "instance-traffic")
	ctx := context.Background()

	instanceStore, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	instance, err := instanceStore.Create(ctx, "Traffic Test", "http://localhost:8080", "user", "pass", nil, nil, false, nil)
	require.NoError(t, err)

	store := models.NewInstanceDailyTrafficStore(db)

	day := "2026-08-04"

	// First sample of the day.
	first, err := store.Upsert(ctx, instance.ID, day, 1000, 2000, 5, 7, models.DataSourceSession, &models.TrafficBaseline{
		SessionUploaded:   10,
		SessionDownloaded: 20,
		AlltimeUploaded:   100,
		AlltimeDownloaded: 200,
		At:                "2026-08-04T00:00:00Z",
	})
	require.NoError(t, err)
	assert.Equal(t, day, first.Date)
	assert.Equal(t, int64(1000), first.Uploaded)
	assert.Equal(t, int64(2000), first.Downloaded)
	assert.Equal(t, int64(5), first.PeakUlSpeed)
	assert.Equal(t, int64(7), first.PeakDlSpeed)
	assert.Equal(t, models.DataSourceSession, first.DataSource)
	assert.Equal(t, int64(10), first.BaselineSessionUploaded)
	assert.Equal(t, int64(20), first.BaselineSessionDownloaded)
	assert.Equal(t, int64(100), first.BaselineAlltimeUploaded)
	assert.Equal(t, int64(200), first.BaselineAlltimeDownloaded)
	assert.Equal(t, "2026-08-04T00:00:00Z", first.BaselineAt)

	// Second sample: totals are overwritten (day total = current - baseline),
	// peaks take the max. The baseline is preserved.
	second, err := store.Upsert(ctx, instance.ID, day, 500, 300, 3, 12, models.DataSourceSession, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(500), second.Uploaded)
	assert.Equal(t, int64(300), second.Downloaded)
	assert.Equal(t, int64(5), second.PeakUlSpeed)
	assert.Equal(t, int64(12), second.PeakDlSpeed)
	assert.Equal(t, models.DataSourceSession, second.DataSource)
	assert.Equal(t, int64(10), second.BaselineSessionUploaded)
	assert.Equal(t, "2026-08-04T00:00:00Z", second.BaselineAt)

	// A restart tag on a later sample flags the whole day as a restart.
	third, err := store.Upsert(ctx, instance.ID, day, 200, 100, 0, 0, models.DataSourceRestart, nil)
	require.NoError(t, err)
	assert.Equal(t, models.DataSourceRestart, third.DataSource)
	assert.Equal(t, int64(200), third.Uploaded)

	// A subsequent session sample cannot downgrade the restart marker.
	fourth, err := store.Upsert(ctx, instance.ID, day, 100, 100, 0, 0, models.DataSourceSession, nil)
	require.NoError(t, err)
	assert.Equal(t, models.DataSourceRestart, fourth.DataSource)

	// Reinstall wins over everything, including a restart marker, and
	// re-baselines the row.
	fifth, err := store.Upsert(ctx, instance.ID, day, 0, 0, 0, 0, models.DataSourceReinstall, &models.TrafficBaseline{
		SessionUploaded:   15,
		SessionDownloaded: 25,
		AlltimeUploaded:   150,
		AlltimeDownloaded: 250,
		At:                "2026-08-04T12:00:00Z",
	})
	require.NoError(t, err)
	assert.Equal(t, models.DataSourceReinstall, fifth.DataSource)
	assert.Equal(t, int64(15), fifth.BaselineSessionUploaded)
	assert.Equal(t, "2026-08-04T12:00:00Z", fifth.BaselineAt)

	// A normal sample cannot overwrite the reinstall marker either.
	sixth, err := store.Upsert(ctx, instance.ID, day, 100, 100, 0, 0, models.DataSourceSession, nil)
	require.NoError(t, err)
	assert.Equal(t, models.DataSourceReinstall, sixth.DataSource)

	got, err := store.GetByInstanceAndDate(ctx, instance.ID, day)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int64(100), got.Uploaded)
	assert.Equal(t, int64(100), got.Downloaded)
	assert.Equal(t, int64(15), got.BaselineSessionUploaded)
	assert.Equal(t, "2026-08-04T12:00:00Z", got.BaselineAt)
}

func TestInstanceDailyTrafficStore_ListHistoryAndPrune(t *testing.T) {
	db := testdb.NewMigratedSQLite(t, "instance-traffic-history")
	ctx := context.Background()

	instanceStore, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	instance, err := instanceStore.Create(ctx, "Traffic History", "http://localhost:8080", "user", "pass", nil, nil, false, nil)
	require.NoError(t, err)

	store := models.NewInstanceDailyTrafficStore(db)

	days := []string{"2026-08-01", "2026-08-02", "2026-08-03", "2026-08-04"}
	for _, d := range days {
		_, err := store.Upsert(ctx, instance.ID, d, 100, 200, 1, 2, models.DataSourceSession, nil)
		require.NoError(t, err)
	}

	// Newest-first ordering and limit.
	items, err := store.ListHistory(ctx, instance.ID, 2)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "2026-08-04", items[0].Date)
	assert.Equal(t, "2026-08-03", items[1].Date)

	// Full listing.
	items, err = store.ListHistory(ctx, instance.ID, 0)
	require.NoError(t, err)
	require.Len(t, items, 4)

	// Prune rows strictly older than a date.
	removed, err := store.DeleteOlderThan(ctx, instance.ID, "2026-08-02")
	require.NoError(t, err)
	assert.Equal(t, int64(1), removed)

	items, err = store.ListHistory(ctx, instance.ID, 0)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, "2026-08-02", items[2].Date)

	// GetByInstanceAndDate returns nil when nothing matches.
	missing, err := store.GetByInstanceAndDate(ctx, instance.ID, "2026-01-01")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestInstanceDailyTrafficStore_IsScopedByInstance(t *testing.T) {
	db := testdb.NewMigratedSQLite(t, "instance-traffic-scope")
	ctx := context.Background()

	instanceStore, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	first, err := instanceStore.Create(ctx, "First", "http://localhost:8081", "user", "pass", nil, nil, false, nil)
	require.NoError(t, err)
	second, err := instanceStore.Create(ctx, "Second", "http://localhost:8082", "user", "pass", nil, nil, false, nil)
	require.NoError(t, err)

	store := models.NewInstanceDailyTrafficStore(db)
	_, err = store.Upsert(ctx, first.ID, "2026-08-04", 1, 2, 0, 0, models.DataSourceSession, nil)
	require.NoError(t, err)

	got, err := store.GetByInstanceAndDate(ctx, second.ID, "2026-08-04")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestInstanceDailyTrafficStore_ListByDate(t *testing.T) {
	db := testdb.NewMigratedSQLite(t, "instance-traffic-list-date")
	ctx := context.Background()

	instanceStore, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	first, err := instanceStore.Create(ctx, "First", "http://localhost:8081", "user", "pass", nil, nil, false, nil)
	require.NoError(t, err)
	second, err := instanceStore.Create(ctx, "Second", "http://localhost:8082", "user", "pass", nil, nil, false, nil)
	require.NoError(t, err)

	store := models.NewInstanceDailyTrafficStore(db)

	// Two instances on the same day, one instance on a different day.
	_, err = store.Upsert(ctx, first.ID, "2026-08-04", 100, 200, 1, 2, models.DataSourceSession, nil)
	require.NoError(t, err)
	_, err = store.Upsert(ctx, second.ID, "2026-08-04", 300, 400, 3, 4, models.DataSourceSession, nil)
	require.NoError(t, err)
	_, err = store.Upsert(ctx, first.ID, "2026-08-05", 500, 600, 5, 6, models.DataSourceSession, nil)
	require.NoError(t, err)

	rows, err := store.ListByDate(ctx, "2026-08-04")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, first.ID, rows[0].InstanceID)
	assert.Equal(t, int64(100), rows[0].Uploaded)
	assert.Equal(t, int64(200), rows[0].Downloaded)
	assert.Equal(t, second.ID, rows[1].InstanceID)
	assert.Equal(t, int64(300), rows[1].Uploaded)
	assert.Equal(t, int64(400), rows[1].Downloaded)

	// A date with no rows returns an empty slice.
	empty, err := store.ListByDate(ctx, "2026-01-01")
	require.NoError(t, err)
	assert.Len(t, empty, 0)
}
