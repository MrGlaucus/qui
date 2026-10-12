package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/autobrr/qui/internal/dbinterface"
)

type ScriptDefinition struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Code        string          `json:"code"`
	Parameters  json.RawMessage `json:"parameters"`
	Builtin     bool            `json:"builtin"`
}

type ScriptTask struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	ScriptID        string          `json:"script_id"`
	Enabled         bool            `json:"enabled"`
	InstanceIDs     []int           `json:"instance_ids"`
	AllInstances    bool            `json:"all_instances"`
	Parameters      json.RawMessage `json:"parameters"`
	IntervalSeconds int             `json:"interval_seconds"`
	TimeoutSeconds  int             `json:"timeout_seconds"`
	// RunAt is a one-shot Unix timestamp; zero uses interval or manual scheduling.
	RunAt       int64    `json:"run_at"`
	Permissions []string `json:"permissions"`
	NextRun     int64    `json:"next_run"`
}

type ScriptTaskRun struct {
	ID         string `json:"id"`
	TaskID     string `json:"task_id"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	Status     string `json:"status"`
	DryRun     bool   `json:"dry_run"`
	Log        string `json:"log"`
}

type ScriptTaskStore struct{ db dbinterface.Querier }

func NewScriptTaskStore(db dbinterface.Querier) *ScriptTaskStore { return &ScriptTaskStore{db: db} }

func (s *ScriptTaskStore) Scripts(ctx context.Context) ([]ScriptDefinition, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT document FROM script_definitions ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ScriptDefinition{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item ScriptDefinition
		if err = json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *ScriptTaskStore) Script(ctx context.Context, id string) (ScriptDefinition, error) {
	var raw string
	var item ScriptDefinition
	err := s.db.QueryRowContext(ctx, "SELECT document FROM script_definitions WHERE id = ?", id).Scan(&raw)
	if err != nil {
		return item, err
	}
	err = json.Unmarshal([]byte(raw), &item)
	return item, err
}
func (s *ScriptTaskStore) SaveScript(ctx context.Context, item ScriptDefinition) error {
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO script_definitions(id,document) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET document=excluded.document", item.ID, string(raw))
	return err
}
func (s *ScriptTaskStore) DeleteScript(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM script_definitions WHERE id=?", id)
	return err
}
func (s *ScriptTaskStore) Tasks(ctx context.Context) ([]ScriptTask, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT document,next_run FROM script_tasks ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ScriptTask{}
	for rows.Next() {
		var raw string
		var next int64
		if err = rows.Scan(&raw, &next); err != nil {
			return nil, err
		}
		var item ScriptTask
		if err = json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		item.NextRun = next
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *ScriptTaskStore) Task(ctx context.Context, id string) (ScriptTask, error) {
	var raw string
	var item ScriptTask
	var next int64
	err := s.db.QueryRowContext(ctx, "SELECT document,next_run FROM script_tasks WHERE id=?", id).Scan(&raw, &next)
	if err != nil {
		return item, err
	}
	err = json.Unmarshal([]byte(raw), &item)
	item.NextRun = next
	return item, err
}
func (s *ScriptTaskStore) SaveTask(ctx context.Context, item ScriptTask) error {
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO script_tasks(id,script_id,document,next_run) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET script_id=excluded.script_id,document=excluded.document,next_run=excluded.next_run", item.ID, item.ScriptID, string(raw), item.NextRun)
	return err
}
func (s *ScriptTaskStore) Schedule(ctx context.Context, id string, next int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE script_tasks SET next_run=? WHERE id=?", next, id)
	return err
}
func (s *ScriptTaskStore) DeleteTask(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM script_tasks WHERE id=?", id)
	return err
}
func (s *ScriptTaskStore) State(ctx context.Context, id, key string) (json.RawMessage, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM script_task_state WHERE task_id=? AND key=?", id, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return json.RawMessage("null"), nil
	}
	return json.RawMessage(value), err
}
func (s *ScriptTaskStore) SetState(ctx context.Context, id, key string, value json.RawMessage) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO script_task_state(task_id,key,value) VALUES(?,?,?) ON CONFLICT(task_id,key) DO UPDATE SET value=excluded.value", id, key, string(value))
	return err
}
func (s *ScriptTaskStore) SaveRun(ctx context.Context, item ScriptTaskRun) error {
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO script_task_runs(id,task_id,started_at,status,document) VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status,document=excluded.document", item.ID, item.TaskID, item.StartedAt, item.Status, string(raw))
	return err
}
func (s *ScriptTaskStore) Runs(ctx context.Context, id string) ([]ScriptTaskRun, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT document FROM script_task_runs WHERE task_id=? ORDER BY started_at DESC LIMIT 100", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ScriptTaskRun{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item ScriptTaskRun
		if err = json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *ScriptTaskStore) PruneRuns(ctx context.Context, before int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM script_task_runs WHERE started_at<?", before)
	return err
}

// RecoverRuns closes records left behind by an interrupted QUI process.
func (s *ScriptTaskStore) RecoverRuns(ctx context.Context, now int64) error {
	rows, err := s.db.QueryContext(ctx, "SELECT document FROM script_task_runs WHERE status='running'")
	if err != nil {
		return err
	}
	var interrupted []ScriptTaskRun
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		var run ScriptTaskRun
		if err = json.Unmarshal([]byte(raw), &run); err != nil {
			rows.Close()
			return err
		}
		if run.Status == "running" {
			run.Status = "canceled"
			run.FinishedAt = now
			run.Log += "\nQUI restarted before this execution finished."
			interrupted = append(interrupted, run)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, run := range interrupted {
		if err = s.SaveRun(ctx, run); err != nil {
			return err
		}
	}
	return nil
}
