// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/database"
	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/testutil/testdb"
)

func TestAutomationSync(t *testing.T) {
	for _, backend := range []struct {
		name string
		open func(testing.TB, string) *database.DB
	}{
		{"sqlite", testdb.NewMigratedSQLite}, {"postgres", testdb.NewMigratedPostgres},
	} {
		t.Run(backend.name, func(t *testing.T) {
			ctx := context.Background()
			db := backend.open(t, "automation-sync")
			instances, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
			require.NoError(t, err)
			ids := make([]int, 4)
			for i := range ids {
				inst, err := instances.Create(ctx, "Sync test", "http://localhost:8080", "user", "pass", nil, nil, false, nil)
				require.NoError(t, err)
				ids[i] = inst.ID
			}
			store := models.NewAutomationStore(db)
			create := func(instanceID int, name string, enabled bool) *models.Automation {
				rule, err := store.Create(ctx, &models.Automation{InstanceID: instanceID, Name: name, TrackerPattern: "*", Enabled: enabled, Conditions: &models.ActionConditions{SchemaVersion: "1", Pause: &models.PauseAction{Enabled: true}}})
				require.NoError(t, err)
				return rule
			}
			get := func(instanceID, id int) *models.Automation {
				r, err := store.Get(ctx, instanceID, id)
				require.NoError(t, err)
				return r
			}
			source := create(ids[0], "Original", true)
			legacy := create(ids[1], "Old renamed copy", false)
			legacy.SortOrder = 9
			_, err = store.Update(ctx, legacy)
			require.NoError(t, err)
			result, err := store.ConfigureSync(ctx, ids[0], source.ID, models.AutomationSyncOptions{Automatic: true, Targets: []models.AutomationSyncTarget{
				{InstanceID: ids[1], RuleID: &legacy.ID, PreserveEnabled: true}, {InstanceID: ids[2], PreserveEnabled: false},
			}})
			require.NoError(t, err)
			require.Equal(t, 1, result.Created)
			require.Equal(t, 1, result.Updated)
			peers, err := store.SyncTargets(ctx, ids[0], source.ID)
			require.NoError(t, err)
			require.Len(t, peers, 2)
			second := peers[1]
			source.Name = "Renamed"
			source.Enabled = false
			source.Notify = true
			source.DryRun = true
			interval := 30
			source.IntervalSeconds = &interval
			source.Conditions = &models.ActionConditions{SchemaVersion: "1", Resume: &models.ResumeAction{Enabled: true}}
			_, err = store.Update(ctx, source)
			require.NoError(t, err)
			follower := get(ids[1], legacy.ID)
			require.Equal(t, source.Name, follower.Name)
			require.Equal(t, source.Conditions, follower.Conditions)
			require.True(t, follower.Notify)
			require.True(t, follower.DryRun)
			require.Equal(t, &interval, follower.IntervalSeconds)
			require.Equal(t, 9, follower.SortOrder)
			require.False(t, follower.Enabled)
			require.Equal(t, source.SyncKey, follower.SyncKey)
			require.Equal(t, &source.ID, follower.SyncSourceID)
			require.Equal(t, &source.InstanceID, follower.SyncSourceInstanceID)
			require.Equal(t, 2, get(ids[0], source.ID).SyncFollowerCount)
			follower.Enabled = true
			_, err = store.Update(ctx, follower)
			require.NoError(t, err)
			source.Name = "Renamed again"
			_, err = store.Update(ctx, source)
			require.NoError(t, err)
			require.True(t, get(ids[1], legacy.ID).Enabled)
			// A manual bulk refresh must retain an automatic follower's enabled override.
			_, err = store.CopyToInstance(ctx, ids[0], ids[1])
			require.NoError(t, err)
			require.True(t, get(ids[1], legacy.ID).Enabled)
			require.True(t, get(ids[1], legacy.ID).SyncPreserveEnabled)

			follower = get(ids[1], legacy.ID)
			follower.Name = "Forbidden edit"
			_, err = store.Update(ctx, follower)
			require.ErrorIs(t, err, models.ErrAutomationSyncReadOnly)
			locked := get(ids[2], second.ID)
			locked.Enabled = true
			_, err = store.Update(ctx, locked)
			require.ErrorIs(t, err, models.ErrAutomationSyncReadOnly)
			require.False(t, get(ids[2], second.ID).Enabled)
			// Copies via followers resolve the original identity and remain manually editable.
			_, err = store.CopyToInstance(ctx, ids[1], ids[3])
			require.NoError(t, err)
			manual, err := store.ListByInstance(ctx, ids[3])
			require.NoError(t, err)
			require.Len(t, manual, 1)
			require.Nil(t, manual[0].SyncSourceID)
			source.Name = "Final name"
			_, err = store.Update(ctx, source)
			require.NoError(t, err)
			require.NotEqual(t, source.Name, get(ids[3], manual[0].ID).Name)
			result, err = store.CopyToInstance(ctx, ids[0], ids[3])
			require.NoError(t, err)
			require.Equal(t, 1, result.Updated)
			require.Zero(t, result.Created)
			require.Equal(t, source.Name, get(ids[3], manual[0].ID).Name)
			// A duplicate is independent, even when input contains sync metadata.
			duplicate, err := store.Create(ctx, get(ids[1], legacy.ID))
			require.NoError(t, err)
			require.NotEqual(t, source.SyncKey, duplicate.SyncKey)
			require.Nil(t, duplicate.SyncSourceID)
			require.NoError(t, store.DetachSync(ctx, ids[1], legacy.ID))
			detached := get(ids[1], legacy.ID)
			require.Nil(t, detached.SyncSourceID)
			require.NotEqual(t, source.SyncKey, detached.SyncKey)
			require.NoError(t, store.DeleteWithFollowers(ctx, ids[0], source.ID, false))
			require.Nil(t, get(ids[2], second.ID).SyncSourceID)
			require.NotEqual(t, source.SyncKey, get(ids[2], second.ID).SyncKey)
		})
	}
}

func TestAutomationSyncConflictRollsBack(t *testing.T) {
	ctx := context.Background()
	db := testdb.NewMigratedSQLite(t, "automation-conflict")
	instances, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	ids := make([]int, 0, 3)
	for range 3 {
		i, err := instances.Create(ctx, "Test", "http://localhost:8080", "u", "p", nil, nil, false, nil)
		require.NoError(t, err)
		ids = append(ids, i.ID)
	}
	store := models.NewAutomationStore(db)
	create := func(id int) *models.Automation {
		r, err := store.Create(ctx, &models.Automation{InstanceID: id, Name: "Same", TrackerPattern: "*", Conditions: &models.ActionConditions{SchemaVersion: "1", Pause: &models.PauseAction{Enabled: true}}})
		require.NoError(t, err)
		return r
	}
	source := create(ids[0])
	create(ids[2])
	create(ids[2])
	_, err = store.ConfigureSync(ctx, ids[0], source.ID, models.AutomationSyncOptions{Automatic: true, Targets: []models.AutomationSyncTarget{{InstanceID: ids[1]}, {InstanceID: ids[2]}}})
	require.ErrorIs(t, err, models.ErrAutomationSyncConflict)
	empty, err := store.ListByInstance(ctx, ids[1])
	require.NoError(t, err)
	require.Empty(t, empty)
	zero := 0
	_, err = store.ConfigureSync(ctx, ids[0], source.ID, models.AutomationSyncOptions{Automatic: true, Targets: []models.AutomationSyncTarget{{InstanceID: ids[2], RuleID: &zero}}})
	require.NoError(t, err)
	peers, err := store.SyncTargets(ctx, ids[0], source.ID)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	require.NoError(t, store.DeleteWithFollowers(ctx, ids[0], source.ID, true))
	_, err = store.Get(ctx, ids[2], peers[0].ID)
	require.ErrorIs(t, err, sql.ErrNoRows)
	kept, err := store.ListByInstance(ctx, ids[2])
	require.NoError(t, err)
	require.Len(t, kept, 2)
}

func TestAutomationSyncUpdateRollsBack(t *testing.T) {
	ctx := context.Background()
	db := testdb.NewMigratedSQLite(t, "automation-update-rollback")
	instances, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	a, err := instances.Create(ctx, "A", "http://localhost:8080", "u", "p", nil, nil, false, nil)
	require.NoError(t, err)
	b, err := instances.Create(ctx, "B", "http://localhost:8080", "u", "p", nil, nil, false, nil)
	require.NoError(t, err)
	store := models.NewAutomationStore(db)
	source, err := store.Create(ctx, &models.Automation{InstanceID: a.ID, Name: "Before", TrackerPattern: "*", Conditions: &models.ActionConditions{SchemaVersion: "1", Pause: &models.PauseAction{Enabled: true}}})
	require.NoError(t, err)
	_, err = store.ConfigureSync(ctx, a.ID, source.ID, models.AutomationSyncOptions{Automatic: true, Targets: []models.AutomationSyncTarget{{InstanceID: b.ID}}})
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TRIGGER reject_follower_update BEFORE UPDATE OF name ON automations WHEN OLD.sync_source_id IS NOT NULL BEGIN SELECT RAISE(ABORT, 'test failure'); END`)
	require.NoError(t, err)
	source.Name = "After"
	_, err = store.Update(ctx, source)
	require.Error(t, err)
	unchanged, err := store.Get(ctx, a.ID, source.ID)
	require.NoError(t, err)
	require.Equal(t, "Before", unchanged.Name)
}

func TestAutomationSyncModeChanges(t *testing.T) {
	ctx := context.Background()
	db := testdb.NewMigratedSQLite(t, "automation-modes")
	instances, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	ids := make([]int, 3)
	for i := range ids {
		instance, err := instances.Create(ctx, "Test", "http://localhost:8080", "u", "p", nil, nil, false, nil)
		require.NoError(t, err)
		ids[i] = instance.ID
	}
	store := models.NewAutomationStore(db)
	create := func(instanceID int) *models.Automation {
		rule, err := store.Create(ctx, &models.Automation{InstanceID: instanceID, Name: "Same name", TrackerPattern: "*", Conditions: &models.ActionConditions{SchemaVersion: "1", Pause: &models.PauseAction{Enabled: true}}})
		require.NoError(t, err)
		return rule
	}
	source, oldCopy := create(ids[0]), create(ids[1])
	_, err = store.ConfigureSync(ctx, source.InstanceID, source.ID, models.AutomationSyncOptions{Automatic: true, Targets: []models.AutomationSyncTarget{{InstanceID: ids[1]}, {InstanceID: ids[2]}}})
	require.NoError(t, err)
	peers, err := store.SyncTargets(ctx, source.InstanceID, source.ID)
	require.NoError(t, err)
	require.Len(t, peers, 2)
	require.Equal(t, oldCopy.ID, peers[0].ID, "unique legacy name is adopted")
	require.ErrorIs(t, store.DetachSync(ctx, source.InstanceID, source.ID), models.ErrAutomationSyncConflict)

	_, err = store.ConfigureSync(ctx, source.InstanceID, source.ID, models.AutomationSyncOptions{Automatic: false, Targets: []models.AutomationSyncTarget{{InstanceID: ids[1]}}})
	require.NoError(t, err)
	manual, err := store.Get(ctx, ids[1], oldCopy.ID)
	require.NoError(t, err)
	require.Nil(t, manual.SyncSourceID)
	require.Equal(t, source.SyncKey, manual.SyncKey)
	detached, err := store.Get(ctx, ids[2], peers[1].ID)
	require.NoError(t, err)
	require.Nil(t, detached.SyncSourceID)
	require.NotEqual(t, source.SyncKey, detached.SyncKey)

	source.Name = "After disabling automatic sync"
	_, err = store.Update(ctx, source)
	require.NoError(t, err)
	manual, err = store.Get(ctx, ids[1], oldCopy.ID)
	require.NoError(t, err)
	require.Equal(t, "Same name", manual.Name)
	_, err = store.ConfigureSync(ctx, source.InstanceID, source.ID, models.AutomationSyncOptions{Automatic: true, Targets: []models.AutomationSyncTarget{{InstanceID: ids[1]}}})
	require.NoError(t, err)
	manual, err = store.Get(ctx, ids[1], oldCopy.ID)
	require.NoError(t, err)
	require.Equal(t, source.Name, manual.Name)
	require.Equal(t, &source.ID, manual.SyncSourceID)
}
