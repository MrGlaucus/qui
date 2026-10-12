package scripttasks

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/autobrr/qui/internal/database"
	"github.com/autobrr/qui/internal/models"
)

func testService(t *testing.T) *Service {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "qui.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	instances, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(models.NewScriptTaskStore(db), instances, nil, nil, "python3")
	t.Cleanup(s.Close)
	return s
}
func TestDryRunStateAndScope(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	script, err := s.SaveScript(ctx, models.ScriptDefinition{Name: "Example", Code: "pass", Parameters: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.SaveTask(ctx, models.ScriptTask{Name: "Test", ScriptID: script.ID, TimeoutSeconds: 5, Parameters: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	e := &execution{service: s, task: task, dry: true, state: make(map[string]json.RawMessage), log: &cappedLog{}}
	if _, err = e.call(ctx, request{Method: "state_set", Key: "baseline", Value: json.RawMessage(`123`)}); err != nil {
		t.Fatal(err)
	}
	result, err := e.call(ctx, request{Method: "state_get", Key: "baseline"})
	if err != nil || string(result.(json.RawMessage)) != "123" {
		t.Fatalf("overlay: %s %v", result, err)
	}
	persisted, err := s.Store.State(ctx, task.ID, "baseline")
	if err != nil || string(persisted) != "null" {
		t.Fatalf("dry run persisted state: %s %v", persisted, err)
	}
	if _, err = e.call(ctx, request{Method: "upload_limit", InstanceID: 42, Hash: "abc", Limit: 1024}); err == nil {
		t.Fatal("out-of-scope mutation accepted")
	}
	e.instances = []instanceView{{ID: 42}}
	if _, err = e.call(ctx, request{Method: "upload_limit", InstanceID: 42, Hash: "abc", Limit: 1024}); err == nil {
		t.Fatal("missing permission accepted")
	}
	e.task.Permissions = []string{"limits"}
	if _, err = e.call(ctx, request{Method: "upload_limit", InstanceID: 42, Hash: "abc", Limit: 1024}); err != nil {
		t.Fatal(err)
	}
}
func TestPythonExecutionAndTimeout(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 required")
	}
	s := testService(t)
	ctx := context.Background()
	script, err := s.SaveScript(ctx, models.ScriptDefinition{Name: "Example", Code: "from qui import ctx\nctx.state_set('value', 42)\nprint(ctx.state_get('value'))", Parameters: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.SaveTask(ctx, models.ScriptTask{Name: "Test", ScriptID: script.ID, TimeoutSeconds: 5, Parameters: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.execute(ctx, task, script, false); err != nil {
		t.Fatal(err)
	}
	value, err := s.Store.State(ctx, task.ID, "value")
	if err != nil || string(value) != "42" {
		t.Fatalf("state: %s %v", value, err)
	}
	script.Code = "print('plain Python output')"
	output, _, err := s.execute(ctx, task, script, false)
	if err != nil || !strings.Contains(output, "plain Python output") {
		t.Fatalf("plain stdout: %q %v", output, err)
	}
	script.Code = "import time\ntime.sleep(60)"
	deadline, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, _, err = s.execute(deadline, task, script, false); err == nil {
		t.Fatal("timeout not enforced")
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("timeout cleanup too slow")
	}
}

func TestRunningTaskCannotOverlapOrChange(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 required")
	}
	s := testService(t)
	ctx := context.Background()
	script, err := s.SaveScript(ctx, models.ScriptDefinition{Name: "Wait", Code: "import time\ntime.sleep(10)", Parameters: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.SaveTask(ctx, models.ScriptTask{Name: "Wait", ScriptID: script.ID, Parameters: json.RawMessage(`{}`), TimeoutSeconds: 20})
	if err != nil {
		t.Fatal(err)
	}
	if run, err := s.run(task.ID, false, true); err != nil || run.ID != "" {
		t.Fatalf("stale scheduled snapshot ran a disabled task: %+v %v", run, err)
	}
	if _, err = s.Run(task.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Run(task.ID, true); !errors.Is(err, ErrRunning) {
		t.Fatalf("overlap allowed: %v", err)
	}
	if _, err = s.SaveTask(ctx, task); !errors.Is(err, ErrRunning) {
		t.Fatalf("running task edited: %v", err)
	}
	if err = s.DeleteTask(ctx, task.ID); !errors.Is(err, ErrRunning) {
		t.Fatalf("running task deleted: %v", err)
	}
	if !s.Cancel(task.ID) {
		t.Fatal("cancel did not find task")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := s.Store.Runs(ctx, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) > 0 && runs[0].Status == "canceled" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("canceled execution was not persisted")
}

func TestBuiltinPythonRules(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "python3", "-B", "-m", "unittest", "discover", "-s", "python", "-p", "test_*.py").CombinedOutput()
	if err != nil {
		t.Fatalf("HDSKY rule tests: %v\n%s", err, output)
	}
}

func TestStartRecoversInterruptedRun(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	script, err := s.SaveScript(ctx, models.ScriptDefinition{Name: "Recover", Code: "pass", Parameters: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.SaveTask(ctx, models.ScriptTask{Name: "Recover", ScriptID: script.ID, Parameters: json.RawMessage(`{}`), TimeoutSeconds: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Store.SaveRun(ctx, models.ScriptTaskRun{ID: "interrupted", TaskID: task.ID, Status: "running", StartedAt: time.Now().Unix()}); err != nil {
		t.Fatal(err)
	}
	if err = s.Start(); err != nil {
		t.Fatal(err)
	}
	runs, err := s.Store.Runs(ctx, task.ID)
	if err != nil || len(runs) != 1 || runs[0].Status != "canceled" {
		t.Fatalf("recovery: %+v %v", runs, err)
	}
	if _, err = s.Store.Script(ctx, BuiltinID); err != nil {
		t.Fatal(err)
	}
}

// TestSDKMutationGuards exercises both torrent-scoped and instance-wide writes
// without a client: a dry run must never reach the sync manager.
func TestSDKMutationGuards(t *testing.T) {
	cases := []struct {
		method, permission string
		request            request
	}{
		{"set_category", "categories", request{Category: ""}},
		{"create_category", "categories", request{Category: "example", Path: "/downloads/example"}},
		{"edit_category", "categories", request{Category: "example"}},
		{"delete_categories", "categories", request{Categories: []string{"example"}}},
		{"set_tags", "tags", request{Tags: []string{}}},
		{"create_tags", "tags", request{Tags: []string{"example"}}},
		{"delete_tags", "tags", request{Tags: []string{"example"}}},
		{"recheck", "recheck", request{}},
	}
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			e := &execution{service: &Service{}, dry: true, log: &cappedLog{}, instances: []instanceView{{ID: 7}}}
			r := tc.request
			r.Method, r.InstanceID = tc.method, 7
			if tc.method == "set_category" || tc.method == "set_tags" || tc.method == "recheck" {
				r.Hash = "abc"
			}
			if _, err := e.call(context.Background(), r); err == nil {
				t.Fatal("missing permission accepted")
			}
			e.task.Permissions = []string{tc.permission}
			if _, err := e.call(context.Background(), r); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(e.log.String(), "[dry-run]") {
				t.Fatal("dry run not logged")
			}
			r.InstanceID = 8
			if _, err := e.call(context.Background(), r); err == nil {
				t.Fatal("out-of-scope operation accepted")
			}
		})
	}
}

func TestSDKEmptyTagsAndInvalidNames(t *testing.T) {
	e := &execution{service: &Service{}, log: &cappedLog{}, instances: []instanceView{{ID: 7}}, task: models.ScriptTask{Permissions: []string{"tags", "categories"}}}
	// In qBittorrent an empty removeTags argument can clear all tags.
	// An empty SDK list must not be forwarded.
	for _, method := range []string{"add_tags", "remove_tags", "create_tags", "delete_tags"} {
		if _, err := e.call(context.Background(), request{Method: method, InstanceID: 7, Hash: "abc"}); err != nil {
			t.Fatal(err)
		}
	}
	e.dry = true
	for _, r := range []request{
		{Method: "create_tags", Tags: []string{"one,two"}},
		{Method: "create_tags", Tags: []string{""}},
		{Method: "delete_categories", Categories: []string{""}},
		{Method: "delete_categories"},
		{Method: "create_category", Category: ""},
	} {
		r.InstanceID = 7
		if _, err := e.call(context.Background(), r); err == nil {
			t.Fatalf("invalid input accepted: %+v", r)
		}
	}
}
