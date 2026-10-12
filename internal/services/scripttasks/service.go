// Package scripttasks runs administrator-authored Python scripts as trusted local processes.
package scripttasks

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/qbittorrent"
	"github.com/autobrr/qui/internal/services/notifications"
)

//go:embed python/qui.py
var sdk string

//go:embed python/hdsky.py
var hdsky string

const BuiltinID = "hdsky"

var ErrRunning = errors.New("script task is already running")

type Service struct {
	Store         *models.ScriptTaskStore
	instances     *models.InstanceStore
	syncManager   *qbittorrent.SyncManager
	notifications *notifications.Service
	python        string
	mu            sync.Mutex
	running       map[string]context.CancelFunc
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	closed        bool
}

func NewService(store *models.ScriptTaskStore, instances *models.InstanceStore, manager *qbittorrent.SyncManager, notifier *notifications.Service, python string) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	if python == "" {
		python = "python3"
	}
	return &Service{Store: store, instances: instances, syncManager: manager, notifications: notifier, python: python, running: make(map[string]context.CancelFunc), ctx: ctx, cancel: cancel}
}
func (s *Service) Start() error {
	if err := s.Store.RecoverRuns(s.ctx, time.Now().Unix()); err != nil {
		return err
	}
	if err := s.Store.SaveScript(s.ctx, models.ScriptDefinition{ID: BuiltinID, Name: "HDSKY刷流规则", Description: "HDSKY staged upload limits and persistent upload baseline", Builtin: true, Code: hdsky, Parameters: json.RawMessage(`{"tracker_domains":[]}`)}); err != nil {
		return err
	}
	s.wg.Go(func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case now := <-ticker.C:
				tasks, err := s.Store.Tasks(s.ctx)
				if err != nil {
					log.Error().Err(err).Msg("Read script tasks")
					continue
				}
				for _, task := range tasks {
					if task.Enabled && task.NextRun > 0 && task.NextRun <= now.Unix() {
						if _, err = s.run(task.ID, false, true); err != nil && !errors.Is(err, ErrRunning) {
							log.Error().Err(err).Str("task", task.ID).Msg("Start script task")
						}
					}
				}
				if now.Unix()%3600 == 0 {
					if err = s.Store.PruneRuns(s.ctx, now.AddDate(0, 0, -30).Unix()); err != nil {
						log.Error().Err(err).Msg("Prune script task runs")
					}
				}
			}
		}
	})
	return nil
}
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
}
func (s *Service) Runtime() (string, error) { return exec.LookPath(s.python) }
func (s *Service) Cancel(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cancel, ok := s.running[id]
	if ok {
		cancel()
	}
	return ok
}

func (s *Service) SaveScript(ctx context.Context, item models.ScriptDefinition) (models.ScriptDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.ID == BuiltinID {
		return item, errors.New("built-in scripts are read-only; create a copy")
	}
	if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Code) == "" || len(item.Code) > 1024*1024 {
		return item, errors.New("name and Python code (up to 1 MiB) are required")
	}
	if !jsonObject(item.Parameters) {
		return item, errors.New("default parameters must be a JSON object")
	}
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	item.Builtin = false
	return item, s.Store.SaveScript(ctx, item)
}
func jsonObject(raw json.RawMessage) bool {
	var value map[string]json.RawMessage
	return json.Unmarshal(raw, &value) == nil && value != nil
}
func (s *Service) SaveTask(ctx context.Context, item models.ScriptTask) (models.ScriptTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.InstanceIDs == nil {
		item.InstanceIDs = []int{}
	}
	if item.Permissions == nil {
		item.Permissions = []string{}
	}
	if _, ok := s.running[item.ID]; ok {
		return item, ErrRunning
	}
	if strings.TrimSpace(item.Name) == "" || item.TimeoutSeconds < 1 || item.TimeoutSeconds > 3600 || item.IntervalSeconds < 0 || item.IntervalSeconds > 86400 {
		return item, errors.New("name, timeout (1–3600 seconds) and interval (0–86400 seconds) are required")
	}
	if item.RunAt < 0 || (item.RunAt > 0 && item.IntervalSeconds > 0) {
		return item, errors.New("choose interval or one-shot scheduling")
	}
	if !jsonObject(item.Parameters) {
		return item, errors.New("parameters must be a JSON object")
	}
	if _, err := s.Store.Script(ctx, item.ScriptID); err != nil {
		return item, err
	}
	instances, err := s.instances.List(ctx)
	if err != nil {
		return item, err
	}
	for _, id := range item.InstanceIDs {
		if !slices.ContainsFunc(instances, func(i *models.Instance) bool { return i.ID == id }) {
			return item, fmt.Errorf("unknown instance: %d", id)
		}
	}
	for _, p := range item.Permissions {
		if !slices.Contains([]string{"limits", "tags", "categories", "recheck", "reannounce", "pause_resume", "delete", "notify"}, p) {
			return item, fmt.Errorf("unknown permission: %s", p)
		}
	}
	if item.ScriptID == BuiltinID {
		var params struct {
			Domains []string `json:"tracker_domains"`
		}
		if json.Unmarshal(item.Parameters, &params) != nil || len(params.Domains) == 0 {
			return item, errors.New("HDSKY requires tracker_domains")
		}
		for _, domain := range params.Domains {
			if strings.TrimSpace(domain) == "" || strings.ContainsAny(domain, "/:?@ ") {
				return item, errors.New("use bare tracker hostnames without URLs or passkeys")
			}
		}
		if item.AllInstances || len(item.InstanceIDs) == 0 {
			return item, errors.New("HDSKY requires explicitly selected instances")
		}
		for _, p := range []string{"limits", "tags", "reannounce"} {
			if !slices.Contains(item.Permissions, p) {
				return item, fmt.Errorf("HDSKY requires %s permission", p)
			}
		}
	}
	if item.ID == "" {
		item.ID = uuid.NewString()
	} else if previous, err := s.Store.Task(ctx, item.ID); err != nil {
		return item, err
	} else if previous.ScriptID != item.ScriptID {
		return item, errors.New("create a new task to change its script; state belongs to this script")
	}
	item.NextRun = 0
	if item.Enabled {
		if item.RunAt > 0 {
			item.NextRun = item.RunAt
		} else if item.IntervalSeconds > 0 {
			item.NextRun = time.Now().Unix()
		}
	}
	return item, s.Store.SaveTask(ctx, item)
}
func (s *Service) DeleteTask(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[id]; ok {
		return ErrRunning
	}
	return s.Store.DeleteTask(ctx, id)
}
func (s *Service) DeleteScript(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == BuiltinID {
		return errors.New("built-in scripts cannot be deleted")
	}
	return s.Store.DeleteScript(ctx, id)
}

func (s *Service) Run(id string, dry bool) (models.ScriptTaskRun, error) {
	return s.run(id, dry, false)
}

func (s *Service) run(id string, dry, scheduled bool) (models.ScriptTaskRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var run models.ScriptTaskRun
	if s.closed {
		return run, errors.New("script service is stopping")
	}
	if _, ok := s.running[id]; ok {
		return run, ErrRunning
	}
	if _, err := s.Runtime(); err != nil {
		return run, fmt.Errorf("python interpreter unavailable: %w", err)
	}
	task, err := s.Store.Task(s.ctx, id)
	if err != nil {
		return run, err
	}
	// Recheck under the same lock used by SaveTask: a scheduler snapshot may be stale.
	if scheduled && (!task.Enabled || task.NextRun <= 0 || task.NextRun > time.Now().Unix()) {
		return run, nil
	}
	script, err := s.Store.Script(s.ctx, task.ScriptID)
	if err != nil {
		return run, err
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(task.TimeoutSeconds)*time.Second)
	run = models.ScriptTaskRun{ID: uuid.NewString(), TaskID: id, StartedAt: time.Now().Unix(), Status: "running", DryRun: dry}
	if err = s.Store.SaveRun(s.ctx, run); err != nil {
		cancel()
		return run, err
	}
	s.running[id] = cancel
	s.wg.Add(1)
	go func(run models.ScriptTaskRun) {
		defer s.wg.Done()
		defer cancel()
		output, next, runErr := s.execute(ctx, task, script, dry)
		run.Log = output
		run.FinishedAt = time.Now().Unix()
		run.Status = "success"
		if runErr != nil {
			run.Status = "failed"
			run.Log += "\n" + runErr.Error()
		}
		if ctx.Err() != nil {
			run.Status = "canceled"
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				run.Status = "timeout"
			}
		}
		persistCtx, persistCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer persistCancel()
		if err := s.Store.SaveRun(persistCtx, run); err != nil {
			log.Error().Err(err).Msg("Save script run")
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if !dry {
			scheduled := int64(0)
			if task.Enabled && task.IntervalSeconds > 0 {
				scheduled = time.Now().Unix() + int64(task.IntervalSeconds)
				if next > 0 && next < scheduled {
					scheduled = max(next, time.Now().Unix()+1)
				}
			}
			if err := s.Store.Schedule(persistCtx, id, scheduled); err != nil {
				log.Error().Err(err).Msg("Schedule script task")
			}
		}
		delete(s.running, id)
	}(run)
	return run, nil
}
