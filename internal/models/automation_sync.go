// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/autobrr/qui/internal/dbinterface"
)

var (
	ErrAutomationSyncReadOnly = errors.New("automation is an automatic follower; detach it before editing")
	ErrAutomationSyncConflict = errors.New("automation sync conflict; select an independent target rule or detach it first")
)

type automationQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type AutomationSyncTarget struct {
	InstanceID int `json:"instanceId"`
	// nil matches identity, then a unique name; zero explicitly creates a new rule.
	RuleID          *int `json:"ruleId,omitempty"`
	PreserveEnabled bool `json:"preserveEnabled"`
}

type AutomationSyncOptions struct {
	Automatic bool                   `json:"automatic"`
	Targets   []AutomationSyncTarget `json:"targets"`
}

type AutomationCopyResult struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
}

// Execution order stays local. Only the selected enabled-state override is retained.
const automationCopyUpdate = `UPDATE automations AS dst SET
 name = src.name, tracker_pattern = src.tracker_pattern, conditions = src.conditions,
 enabled = CASE WHEN dst.sync_preserve_enabled = 1 THEN dst.enabled ELSE src.enabled END,
 dry_run = src.dry_run, notify = src.notify, interval_seconds = src.interval_seconds,
 free_space_source = src.free_space_source, sorting_config = src.sorting_config
 FROM automations AS src`

func lockAutomation(ctx context.Context, q automationQuerier, instanceID, id int) error {
	result, err := q.ExecContext(ctx, `UPDATE automations SET sync_key = sync_key WHERE id = ? AND instance_id = ?`, id, instanceID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// SyncTargets includes manual copies, allowing switching a manual group to automatic.
func (s *AutomationStore) SyncTargets(ctx context.Context, instanceID, id int) ([]*Automation, error) {
	source, err := s.Get(ctx, instanceID, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT instance_id, id FROM automations WHERE sync_key = ? AND id != ? ORDER BY instance_id`, source.SyncKey, id)
	if err != nil {
		return nil, err
	}
	type ref struct{ instanceID, id int }
	refs := []ref{}
	for rows.Next() {
		var r ref
		if err := rows.Scan(&r.instanceID, &r.id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		refs = append(refs, r)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	result := make([]*Automation, 0, len(refs))
	for _, r := range refs {
		rule, err := s.Get(ctx, r.instanceID, r.id)
		if err != nil {
			return nil, err
		}
		result = append(result, rule)
	}
	return result, nil
}

func (s *AutomationStore) ConfigureSync(ctx context.Context, instanceID, id int, options AutomationSyncOptions) (*AutomationCopyResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockAutomation(ctx, tx, instanceID, id); err != nil {
		return nil, err
	}
	source, err := getAutomation(ctx, tx, instanceID, id)
	if err != nil {
		return nil, err
	}
	if source.SyncSourceID != nil {
		return nil, ErrAutomationSyncReadOnly
	}
	// A group already following another source cannot acquire a second writer.
	var foreignFollowers int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM automations WHERE sync_key = ? AND sync_source_id IS NOT NULL AND sync_source_id != ?`, source.SyncKey, id).Scan(&foreignFollowers); err != nil {
		return nil, err
	}
	if foreignFollowers > 0 {
		return nil, ErrAutomationSyncConflict
	}
	result := &AutomationCopyResult{}
	seen := make(map[int]bool)
	for _, target := range options.Targets {
		if target.InstanceID <= 0 || target.InstanceID == instanceID || seen[target.InstanceID] {
			return nil, ErrAutomationSyncConflict
		}
		seen[target.InstanceID] = true
		created, err := copyAutomation(ctx, tx, source, target, options.Automatic)
		if err != nil {
			return nil, err
		}
		if created {
			result.Created++
		} else {
			result.Updated++
		}
	}
	// Removed targets become independent; selected manual targets retain identity.
	followers, err := followerRefs(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	for _, follower := range followers {
		if seen[follower.InstanceID] && options.Automatic {
			continue
		}
		key := source.SyncKey
		if !seen[follower.InstanceID] {
			key = uuid.NewString()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE automations SET sync_source_id = NULL, sync_key = ? WHERE id = ?`, key, follower.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// ResolveSyncTarget uses the same matching as a copy, for target-specific validation.
func (s *AutomationStore) ResolveSyncTarget(ctx context.Context, source *Automation, target AutomationSyncTarget) (*Automation, error) {
	return resolveSyncTarget(ctx, s.db, source, target)
}

func resolveSyncTarget(ctx context.Context, q automationQuerier, source *Automation, target AutomationSyncTarget) (*Automation, error) {
	rules, err := listAutomations(ctx, q, target.InstanceID)
	if err != nil {
		return nil, err
	}
	var existing *Automation
	for _, rule := range rules {
		if rule.SyncKey == source.SyncKey {
			existing = rule
			break
		}
	}
	if target.RuleID != nil {
		if existing != nil && *target.RuleID != existing.ID {
			return nil, ErrAutomationSyncConflict
		}
		if *target.RuleID > 0 {
			existing, err = getAutomation(ctx, q, target.InstanceID, *target.RuleID)
			if err != nil {
				return nil, err
			}
		} else if *target.RuleID < 0 {
			return nil, ErrAutomationSyncConflict
		}
	} else if existing == nil {
		for _, rule := range rules {
			if rule.Name == source.Name {
				if existing != nil {
					return nil, ErrAutomationSyncConflict
				}
				existing = rule
			}
		}
	}
	return existing, nil
}

func copyAutomation(ctx context.Context, tx dbinterface.TxQuerier, source *Automation, target AutomationSyncTarget, automatic bool) (bool, error) {
	if source.InstanceID == target.InstanceID {
		return false, ErrAutomationSyncConflict
	}
	existing, err := resolveSyncTarget(ctx, tx, source, target)
	if err != nil {
		return false, err
	}
	var sourceID *int
	if automatic {
		sourceID = &source.ID
	}
	if existing == nil {
		// FK validation also rejects a nonexistent target instance.
		_, err := tx.ExecContext(ctx, `INSERT INTO automations
   (instance_id, name, tracker_pattern, conditions, enabled, dry_run, notify, sort_order,
    interval_seconds, free_space_source, sorting_config, sync_key, sync_source_id, sync_preserve_enabled)
   SELECT ?, name, tracker_pattern, conditions, enabled, dry_run, notify,
    (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM automations WHERE instance_id = ?),
    interval_seconds, free_space_source, sorting_config, sync_key, ?, ? FROM automations WHERE id = ?`,
			target.InstanceID, target.InstanceID, sourceID, boolToInt(target.PreserveEnabled), source.ID)
		return true, err
	}
	if existing.SyncFollowerCount > 0 || (existing.SyncSourceID != nil && *existing.SyncSourceID != source.ID) {
		return false, ErrAutomationSyncConflict
	}
	if existing.SyncKey != source.SyncKey {
		var related int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM automations WHERE sync_key = ? AND id != ?`, existing.SyncKey, existing.ID).Scan(&related); err != nil {
			return false, err
		}
		if related > 0 || existing.SyncFollowerCount > 0 {
			return false, ErrAutomationSyncConflict
		}
	}
	// Manual refreshes must not silently detach an existing automatic follower.
	if !automatic {
		sourceID = existing.SyncSourceID
	}
	if _, err := tx.ExecContext(ctx, `UPDATE automations SET sync_key = ?, sync_source_id = ?, sync_preserve_enabled = ? WHERE id = ?`, source.SyncKey, sourceID, boolToInt(target.PreserveEnabled), existing.ID); err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, automationCopyUpdate+` WHERE src.id = ? AND dst.id = ?`, source.ID, existing.ID)
	return false, err
}

// CopyToInstance is atomic and keeps a common identity even when copying via a follower.
func (s *AutomationStore) CopyToInstance(ctx context.Context, instanceID, targetID int) (*AutomationCopyResult, error) {
	if instanceID == targetID {
		return nil, ErrAutomationSyncConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Lock before reading so copies and edits cannot race on rule identity.
	if _, err := tx.ExecContext(ctx, `UPDATE automations SET sync_key = sync_key WHERE instance_id = ?`, instanceID); err != nil {
		return nil, err
	}
	rules, err := listAutomations(ctx, tx, instanceID)
	if err != nil {
		return nil, err
	}
	result := &AutomationCopyResult{}
	for _, rule := range rules {
		if rule.SyncSourceID != nil {
			rule, err = getAutomation(ctx, tx, *rule.SyncSourceInstanceID, *rule.SyncSourceID)
			if err != nil {
				return nil, err
			}
		}
		if rule.InstanceID == targetID {
			continue
		}
		target := AutomationSyncTarget{InstanceID: targetID}
		existing, err := resolveSyncTarget(ctx, tx, rule, target)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.SyncSourceID != nil {
			target.PreserveEnabled = existing.SyncPreserveEnabled
		}
		created, err := copyAutomation(ctx, tx, rule, target, false)
		if err != nil {
			return nil, fmt.Errorf("rule %d: %w", rule.ID, err)
		}
		if created {
			result.Created++
		} else {
			result.Updated++
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func followerRefs(ctx context.Context, q automationQuerier, id int) ([]AutomationReference, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, instance_id, name FROM automations WHERE sync_source_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AutomationReference{}
	for rows.Next() {
		var r AutomationReference
		if err := rows.Scan(&r.ID, &r.InstanceID, &r.Name); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *AutomationStore) DetachSync(ctx context.Context, instanceID, id int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockAutomation(ctx, tx, instanceID, id); err != nil {
		return err
	}
	rule, err := getAutomation(ctx, tx, instanceID, id)
	if err != nil {
		return err
	}
	if rule.SyncFollowerCount > 0 {
		return ErrAutomationSyncConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE automations SET sync_key = ?, sync_source_id = NULL WHERE id = ?`, uuid.NewString(), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AutomationStore) DeleteWithFollowers(ctx context.Context, instanceID, id int, deleteFollowers bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockAutomation(ctx, tx, instanceID, id); err != nil {
		return err
	}
	if deleteFollowers {
		if _, err := tx.ExecContext(ctx, `DELETE FROM automations WHERE sync_source_id = ?`, id); err != nil {
			return err
		}
	} else {
		followers, err := followerRefs(ctx, tx, id)
		if err != nil {
			return err
		}
		for _, follower := range followers {
			if _, err := tx.ExecContext(ctx, `UPDATE automations SET sync_source_id = NULL, sync_key = ? WHERE id = ?`, uuid.NewString(), follower.ID); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM automations WHERE id = ? AND instance_id = ?`, id, instanceID); err != nil {
		return err
	}
	return tx.Commit()
}
