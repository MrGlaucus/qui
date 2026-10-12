package scripttasks

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/notifications"
)

type cappedLog struct {
	mu   sync.Mutex
	text strings.Builder
}

func (b *cappedLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := 256*1024 - b.text.Len()
	if remaining > 0 {
		b.text.Write(p[:min(n, remaining)])
	}
	return n, nil
}
func (b *cappedLog) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.text.String() }

type request struct {
	Method     string          `json:"method"`
	InstanceID int             `json:"instance_id"`
	Hash       string          `json:"hash"`
	Key        string          `json:"key"`
	Value      json.RawMessage `json:"value"`
	Timestamp  int64           `json:"timestamp"`
	Limit      int64           `json:"limit"`
	Tags       []string        `json:"tags"`
	Title      string          `json:"title"`
	Message    string          `json:"message"`
	Category   string          `json:"category"`
	Path       string          `json:"path"`
	Categories []string        `json:"categories"`
}
type reply struct {
	Result any    `json:"result"`
	Error  string `json:"error,omitempty"`
}
type instanceView struct {
	ID                       int    `json:"id"`
	Name                     string `json:"name"`
	CountryCode              string `json:"country_code"`
	IsActive                 bool   `json:"is_active"`
	HasLocalFilesystemAccess bool   `json:"has_local_filesystem_access"`
	DailyTrafficEnabled      bool   `json:"daily_traffic_enabled"`
}
type torrentView struct {
	Ratio        float64 `json:"ratio"`
	Size         int64   `json:"size"`
	AmountLeft   int64   `json:"amount_left"`
	CompletionOn int64   `json:"completion_on"`
	LastActivity int64   `json:"last_activity"`
	SeedingTime  int64   `json:"seeding_time"`
	TimeActive   int64   `json:"time_active"`
	SavePath     string  `json:"save_path"`
	ContentPath  string  `json:"content_path"`
	NumSeeds     int64   `json:"num_seeds"`
	NumLeechs    int64   `json:"num_leechs"`

	Hash             string           `json:"hash"`
	Name             string           `json:"name"`
	AddedOn          int64            `json:"added_on"`
	TotalSize        int64            `json:"total_size"`
	Uploaded         int64            `json:"uploaded"`
	Downloaded       int64            `json:"downloaded"`
	Progress         float64          `json:"progress"`
	State            qbt.TorrentState `json:"state"`
	Category         string           `json:"category"`
	Tags             string           `json:"tags"`
	UpLimit          int64            `json:"up_limit"`
	DlLimit          int64            `json:"dl_limit"`
	UpSpeed          int64            `json:"upspeed"`
	DlSpeed          int64            `json:"dlspeed"`
	TrackerHosts     []string         `json:"tracker_hosts"`
	TrackersComplete bool             `json:"trackers_complete"`
}

func hostname(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
}
func view(t qbt.Torrent) torrentView {
	hosts := []string{}
	if host := hostname(t.Tracker); host != "" {
		hosts = append(hosts, host)
	}
	for _, tracker := range t.Trackers {
		if host := hostname(tracker.Url); host != "" && !slices.Contains(hosts, host) {
			hosts = append(hosts, host)
		}
	}
	return torrentView{Hash: t.Hash, Name: t.Name, AddedOn: t.AddedOn, TotalSize: t.TotalSize, Uploaded: t.Uploaded, Downloaded: t.Downloaded, Progress: t.Progress, State: t.State, Category: t.Category, Tags: t.Tags, UpLimit: t.UpLimit, DlLimit: t.DlLimit, UpSpeed: t.UpSpeed, DlSpeed: t.DlSpeed, Ratio: t.Ratio, Size: t.Size, AmountLeft: t.AmountLeft, CompletionOn: t.CompletionOn, LastActivity: t.LastActivity, SeedingTime: t.SeedingTime, TimeActive: t.TimeActive, SavePath: t.SavePath, ContentPath: t.ContentPath, NumSeeds: t.NumSeeds, NumLeechs: t.NumLeechs, TrackerHosts: hosts, TrackersComplete: len(t.Trackers) > 0}
}

type execution struct {
	service   *Service
	task      models.ScriptTask
	dry       bool
	state     map[string]json.RawMessage
	next      int64
	log       *cappedLog
	instances []instanceView
}

func (s *Service) execute(ctx context.Context, task models.ScriptTask, script models.ScriptDefinition, dry bool) (string, int64, error) {
	output := &cappedLog{}
	execution := &execution{service: s, task: task, dry: dry, state: make(map[string]json.RawMessage), log: output, instances: []instanceView{}}
	instances, err := s.instances.List(ctx)
	if err != nil {
		return "", 0, err
	}
	for _, i := range instances {
		if i.IsActive && (task.AllInstances || slices.Contains(task.InstanceIDs, i.ID)) {
			execution.instances = append(execution.instances, instanceView{ID: i.ID, Name: i.Name, CountryCode: i.CountryCode, IsActive: i.IsActive, HasLocalFilesystemAccess: i.HasLocalFilesystemAccess, DailyTrafficEnabled: i.DailyTrafficEnabled})
		}
	}
	dir, err := os.MkdirTemp("", "qui-script-")
	if err != nil {
		return "", 0, err
	}
	defer os.RemoveAll(dir)
	for name, code := range map[string]string{
		"qui.py":       sdk,
		"task.py":      script.Code,
		"bootstrap.py": "import qui\nimport runpy\nrunpy.run_path('task.py', run_name='__main__')\n",
	} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(code), 0o600); err != nil {
			return "", 0, err
		}
	}
	// Administrator-selected Python runs trusted code by design; no shell is used.
	cmd := exec.CommandContext(ctx, s.python, "-u", "-B", filepath.Join(dir, "bootstrap.py")) // #nosec G204
	cmd.Dir = dir
	cmd.WaitDelay = 2 * time.Second
	cmd.Stderr = output
	// Keep Python independent from inherited PYTHONPATH/startup hooks. This is not a sandbox.
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(entry), "PYTHON") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "PYTHONUTF8=1", "PYTHONNOUSERSITE=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", 0, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", 0, err
	}
	if err = cmd.Start(); err != nil {
		return "", 0, err
	}
	// A child inheriting stdout must not keep a canceled run blocked in Scan.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = stdout.Close()
		case <-done:
		}
	}()
	writer := json.NewEncoder(stdin)
	init := struct {
		Parameters     json.RawMessage `json:"parameters"`
		Dry            bool            `json:"dry_run"`
		TaskID         string          `json:"task_id"`
		TrackerDomains []string        `json:"tracker_domains"`
	}{task.Parameters, dry, task.ID, task.TrackerDomains}
	if err = writer.Encode(init); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return output.String(), 0, err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
	for scanner.Scan() {
		var req request
		response := reply{}
		if err = json.Unmarshal(scanner.Bytes(), &req); err != nil {
			response.Error = "invalid SDK request"
		} else {
			response.Result, err = execution.call(ctx, req)
			if err != nil {
				response.Error = err.Error()
			}
		}
		if err = writer.Encode(response); err != nil {
			break
		}
	}
	scanErr := scanner.Err()
	_ = stdin.Close()
	if scanErr != nil || err != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	return output.String(), execution.next, errors.Join(scanErr, err, waitErr)
}

func (e *execution) call(ctx context.Context, r request) (any, error) {
	s := e.service
	switch r.Method {
	case "instances":
		return e.instances, nil
	case "state_get":
		if value, ok := e.state[r.Key]; ok {
			return value, nil
		}
		return s.Store.State(ctx, e.task.ID, r.Key)
	case "state_set":
		if len(r.Key) == 0 || len(r.Key) > 512 || !json.Valid(r.Value) {
			return nil, errors.New("invalid state key or JSON value")
		}
		if e.dry {
			e.state[r.Key] = r.Value
			return nil, nil
		}
		return nil, s.Store.SetState(ctx, e.task.ID, r.Key, r.Value)
	case "wake_at":
		if r.Timestamp > time.Now().Unix() && (e.next == 0 || r.Timestamp < e.next) {
			e.next = r.Timestamp
		}
		return nil, nil
	case "notify":
		if !slices.Contains(e.task.Permissions, "notify") {
			return nil, errors.New("notify permission required")
		}
		if e.dry {
			fmt.Fprintln(e.log, "[dry-run] notify", r.Title)
			return nil, nil
		}
		if s.notifications == nil {
			return nil, errors.New("notification service unavailable")
		}
		s.notifications.Notify(ctx, notifications.Event{Type: notifications.EventAutomationsActionsApplied, Title: r.Title, Message: r.Message})
		return nil, nil
	}
	if !slices.ContainsFunc(e.instances, func(i instanceView) bool { return i.ID == r.InstanceID }) {
		return nil, errors.New("instance is outside task scope")
	}
	switch r.Method {
	case "instance":
		for _, instance := range e.instances {
			if instance.ID == r.InstanceID {
				return instance, nil
			}
		}
	case "transfer":
		client, err := s.syncManager.GetClient(ctx, r.InstanceID)
		if err != nil {
			return nil, err
		}
		return client.GetTransferInfoCtx(ctx)
	case "categories":
		return s.syncManager.GetCategories(ctx, r.InstanceID)
	case "tags":
		return s.syncManager.GetTags(ctx, r.InstanceID)
	}
	if r.Method == "torrents" || r.Method == "torrent" {
		client, err := s.syncManager.GetClient(ctx, r.InstanceID)
		if err != nil {
			return nil, err
		}
		options := qbt.TorrentFilterOptions{IncludeTrackers: client.SupportsTrackerHealth()}
		if r.Method == "torrent" {
			if r.Hash == "" {
				return nil, errors.New("torrent hash required")
			}
			options.Hashes = []string{r.Hash}
		}
		torrents, err := client.GetTorrentsCtx(ctx, options)
		if err != nil {
			return nil, err
		}
		result := []torrentView{}
		for _, t := range torrents {
			result = append(result, view(t))
		}
		if r.Method == "torrent" {
			if len(result) != 1 {
				return nil, errors.New("torrent no longer exists")
			}
			return result[0], nil
		}
		return result, nil
	}
	instanceAction := slices.Contains([]string{"create_category", "edit_category", "delete_categories", "create_tags", "delete_tags"}, r.Method)
	if r.Hash == "" && !instanceAction {
		return nil, errors.New("torrent hash required")
	}
	switch r.Method {
	case "properties":
		return s.syncManager.GetTorrentProperties(ctx, r.InstanceID, r.Hash)
	case "files":
		return s.syncManager.GetTorrentFiles(ctx, r.InstanceID, r.Hash)
	}
	if r.Method == "trackers" {
		trackers, err := s.syncManager.GetTorrentTrackers(ctx, r.InstanceID, r.Hash)
		if err != nil {
			return nil, err
		}
		hosts := []string{}
		for _, tracker := range trackers {
			if host := hostname(tracker.Url); host != "" && !slices.Contains(hosts, host) {
				hosts = append(hosts, host)
			}
		}
		return hosts, nil
	}
	permission := ""
	switch r.Method {
	case "upload_limit", "download_limit":
		permission = "limits"
		if r.Limit < 0 || r.Limit%1024 != 0 {
			return nil, errors.New("limit must be a nonnegative multiple of 1024 bytes/s")
		}
	case "add_tags", "remove_tags", "set_tags", "create_tags", "delete_tags":
		permission = "tags"
	case "set_category", "create_category", "edit_category", "delete_categories":
		permission = "categories"
		if (r.Method == "create_category" || r.Method == "edit_category") && strings.TrimSpace(r.Category) == "" {
			return nil, errors.New("category name required")
		}
		if r.Method == "delete_categories" && len(r.Categories) == 0 {
			return nil, errors.New("categories required")
		}
	case "recheck":
		permission = "recheck"
	case "reannounce":
		permission = "reannounce"
	case "pause", "resume":
		permission = "pause_resume"
	case "delete", "delete_files":
		permission = "delete"
	default:
		return nil, errors.New("unknown SDK method")
	}
	if !slices.Contains(e.task.Permissions, permission) {
		return nil, fmt.Errorf("%s permission required", permission)
	}
	if slices.Contains([]string{"add_tags", "remove_tags", "set_tags", "create_tags", "delete_tags"}, r.Method) {
		for _, tag := range r.Tags {
			if strings.TrimSpace(tag) == "" || strings.ContainsAny(tag, ",\n\r") {
				return nil, errors.New("tags must be nonempty names without commas or newlines")
			}
		}
		if len(r.Tags) == 0 && r.Method != "set_tags" {
			return nil, nil
		}
	}
	if slices.Contains([]string{"set_category", "create_category", "edit_category"}, r.Method) && strings.ContainsAny(r.Category, "\n\r") {
		return nil, errors.New("category cannot contain newlines")
	}
	for _, category := range r.Categories {
		if strings.TrimSpace(category) == "" || strings.ContainsAny(category, "\n\r") {
			return nil, errors.New("categories must contain nonempty names without newlines")
		}
	}
	fmt.Fprintf(e.log, "%sinstance=%d hash=%s action=%s limit=%d tags=%s category=%q categories=%q path=%q\n", map[bool]string{true: "[dry-run] ", false: ""}[e.dry], r.InstanceID, r.Hash, r.Method, r.Limit, strings.Join(r.Tags, ","), r.Category, r.Categories, r.Path)
	if e.dry {
		return nil, nil
	}
	hashes := []string{r.Hash}
	switch r.Method {
	case "upload_limit":
		return nil, s.syncManager.SetTorrentUploadLimit(ctx, r.InstanceID, hashes, r.Limit/1024)
	case "download_limit":
		return nil, s.syncManager.SetTorrentDownloadLimit(ctx, r.InstanceID, hashes, r.Limit/1024)
	case "add_tags":
		return nil, s.syncManager.AddTorrentTags(ctx, r.InstanceID, hashes, r.Tags)
	case "remove_tags":
		return nil, s.syncManager.RemoveTorrentTags(ctx, r.InstanceID, hashes, r.Tags)
	case "set_tags":
		return nil, s.syncManager.SetTorrentTags(ctx, r.InstanceID, hashes, r.Tags)
	case "create_tags":
		return nil, s.syncManager.CreateTags(ctx, r.InstanceID, r.Tags)
	case "delete_tags":
		return nil, s.syncManager.DeleteTags(ctx, r.InstanceID, r.Tags)
	case "set_category":
		return nil, s.syncManager.SetCategory(ctx, r.InstanceID, hashes, r.Category)
	case "create_category":
		return nil, s.syncManager.CreateCategory(ctx, r.InstanceID, r.Category, r.Path)
	case "edit_category":
		return nil, s.syncManager.EditCategory(ctx, r.InstanceID, r.Category, r.Path)
	case "delete_categories":
		return nil, s.syncManager.RemoveCategories(ctx, r.InstanceID, r.Categories)
	case "delete_files":
		return nil, s.syncManager.BulkAction(ctx, r.InstanceID, hashes, "deleteWithFiles")
	default:
		return nil, s.syncManager.BulkAction(ctx, r.InstanceID, hashes, r.Method)
	}
}

var _ io.Writer = (*cappedLog)(nil)
