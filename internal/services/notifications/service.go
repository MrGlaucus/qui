// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	shoutrrrdiscord "github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/rs/zerolog"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/pkg/redact"
	"github.com/autobrr/qui/pkg/timeutil"
)

const (
	defaultQueueSize = 100
	defaultWorkers   = 2
)

type Notifier interface {
	Notify(ctx context.Context, event Event)
}

type Event struct {
	Type                     EventType
	Title                    string
	Message                  string
	StartedAt                *time.Time
	CompletedAt              *time.Time
	CrossSeed                *CrossSeedEventData
	Automations              *AutomationsEventData
	InstanceID               int
	InstanceName             string
	TorrentName              string
	TorrentHash              string
	TorrentAddedOn           int64
	TorrentETASeconds        int64
	TorrentState             string
	TorrentProgress          float64
	TorrentRatio             float64
	TorrentTotalSizeBytes    int64
	TorrentDownloadedBytes   int64
	TorrentAmountLeftBytes   int64
	TorrentDlSpeedBps        int64
	TorrentUpSpeedBps        int64
	TorrentNumSeeds          int64
	TorrentNumLeechs         int64
	TrackerDomain            string
	Category                 string
	Tags                     []string
	BackupKind               models.BackupRunKind
	BackupRunID              int64
	BackupTorrentCount       int
	DirScanRunID             int64
	DirScanMatchesFound      int
	DirScanTorrentsAdded     int
	OrphanScanRunID          int64
	OrphanScanFilesDeleted   int
	OrphanScanFoldersDeleted int
	ErrorMessage             string
	ErrorMessages            []string
}

type Service struct {
	store         *models.NotificationTargetStore
	instanceStore *models.InstanceStore
	logger        zerolog.Logger
	queue         chan Event
	startOnce     sync.Once
	forceSync     func(ctx context.Context, instanceID int) error
	// now returns the current time in the timezone traffic reports and day
	// boundaries should use. Defaults to server-local time; call SetTimezone
	// to follow the user's frontend timezone.
	now func() time.Time
	// loc returns the current timezone location used to format stored
	// timestamps (e.g. baseline times) in reports.
	loc func() *time.Location
}

// SetTimezone points the service's time source at the given timezone provider,
// so daily/hourly traffic reports respect the user's calendar days instead of
// the server's.
func (s *Service) SetTimezone(provider *timeutil.Provider) {
	if s == nil || provider == nil {
		return
	}
	s.now = provider.Now
	s.loc = provider.Location
}

// location returns the current timezone location for formatting stored
// timestamps. Falls back to a default that does not require a provider.
func (s *Service) location() *time.Location {
	if s == nil {
		return time.Local
	}
	if s.loc != nil {
		return s.loc()
	}
	return time.Local
}

// SetForceSync registers a callback that triggers a qBittorrent maindata sync
// for an instance. Used by the traffic report schedulers so midnight baseline
// capture and hourly reports get fresh data even when no SSE subscriber is
// keeping the sync loop alive.
func (s *Service) SetForceSync(fn func(ctx context.Context, instanceID int) error) {
	if s == nil {
		return
	}
	s.forceSync = fn
}

func NewService(store *models.NotificationTargetStore, instanceStore *models.InstanceStore, logger zerolog.Logger) *Service {
	if store == nil {
		return nil
	}

	return &Service{
		store:         store,
		instanceStore: instanceStore,
		logger:        logger,
		queue:         make(chan Event, defaultQueueSize),
		now:           time.Now,
	}
}

func ValidateURL(rawURL string) error {
	if targetScheme(rawURL) == "notifiarrapi" {
		_, err := parseNotifiarrAPIConfig(rawURL)
		return err
	}
	_, err := router.NewWithOptions(nil, types.SenderOptions{}, rawURL)
	return err
}

func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}

	s.startOnce.Do(func() {
		for range defaultWorkers {
			go s.worker(ctx)
		}
	})
}

func (s *Service) Notify(ctx context.Context, event Event) {
	if s == nil || s.store == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if s.queue == nil {
		go s.dispatch(ctx, event)
		return
	}

	select {
	case <-ctx.Done():
		return
	case s.queue <- event:
	default:
		s.logger.Warn().Str("event", string(event.Type)).Msg("notifications: queue full, dropping event")
	}
}

func (s *Service) SendTest(ctx context.Context, target *models.NotificationTarget, title, message string) error {
	if target == nil {
		return errors.New("notification target required")
	}

	event := Event{}
	if targetScheme(target.URL) == "notifiarrapi" {
		if len(target.EventTypes) == 0 {
			return errors.New("notifiarr api test requires at least one event type")
		}
		event.Type = EventType(target.EventTypes[0])
	}

	return s.send(ctx, target, event, title, message)
}

func (s *Service) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.queue:
			s.dispatch(ctx, event)
		}
	}
}

func (s *Service) dispatch(ctx context.Context, event Event) {
	if s == nil || s.store == nil {
		return
	}

	targets, err := s.store.ListEnabled(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("notifications: failed to list targets")
		return
	}
	if len(targets) == 0 {
		return
	}

	for _, target := range targets {
		if !allowsEvent(target.EventTypes, event.Type) {
			continue
		}

		title, message := s.formatEvent(ctx, event, targetScheme(target.URL) != "notifiarrapi")
		if strings.TrimSpace(message) == "" {
			continue
		}

		if err := s.send(ctx, target, event, title, message); err != nil {
			// Redact: shoutrrr errors embed the post URL, which carries the
			// webhook token / bot token for most services.
			s.logger.Error().Str("error", redact.String(err.Error())).Str("target", target.Name).Str("event", string(event.Type)).Msg("notifications: send failed")
		}
	}
}

func (s *Service) send(ctx context.Context, target *models.NotificationTarget, event Event, title, message string) error {
	if target == nil {
		return errors.New("notification target required")
	}

	switch targetScheme(target.URL) {
	case "discord":
		return s.sendDiscord(target.URL, event, title, message)
	case "notifiarr":
		return s.sendNotifiarr(target.URL, event, title, message)
	case "notifiarrapi":
		return s.sendNotifiarrAPI(ctx, target.URL, event, title, message)
	default:
		return s.sendDefault(target.URL, title, message, event)
	}
}

func (s *Service) sendDefault(rawURL, title, message string, event Event) error {
	sender, err := router.NewWithOptions(nil, types.SenderOptions{}, rawURL)
	if err != nil {
		return err
	}

	params := types.Params{}
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		params.SetTitle(truncateMessage(trimmed, maxTitleLength))
	}

	maxMessage := maxMessageLengthFor(rawURL, event.Type)
	trimmedMessage := truncateMessage(message, maxMessage)
	results := sender.Send(trimmedMessage, &params)
	var errs []error
	for _, sendErr := range results {
		if sendErr != nil {
			errs = append(errs, sendErr)
		}
	}
	if len(errs) == 0 {
		return nil
	}

	return errors.Join(errs...)
}

func (s *Service) sendDiscord(rawURL string, event Event, title, message string) error {
	configURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	service := &shoutrrrdiscord.Service{}
	if err := service.Initialize(configURL, nil); err != nil {
		return err
	}
	service.Config.JSON = true

	payload, err := buildDiscordPayload(service.Config, event, title, message)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return service.Send(string(encoded), nil)
}

func (s *Service) sendNotifiarr(rawURL string, _ Event, title, message string) error {
	sender, err := router.NewWithOptions(nil, types.SenderOptions{}, rawURL)
	if err != nil {
		return err
	}

	description, fields := buildStructuredMessage(message)
	if description == "" {
		description = message
	}
	description = truncateMessage(description, notifiarrDescriptionLimit)

	params := types.Params{}
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		params.SetTitle(truncateMessage(trimmed, notifiarrTitleLimit))
	}
	if len(fields) > 0 {
		encoded, err := json.Marshal(buildNotifiarrFields(fields))
		if err != nil {
			return err
		}
		params["fields"] = string(encoded)
	}

	results := sender.Send(description, &params)
	var errs []error
	for _, sendErr := range results {
		if sendErr != nil {
			errs = append(errs, sendErr)
		}
	}
	if len(errs) == 0 {
		return nil
	}

	return errors.Join(errs...)
}

func (s *Service) formatEvent(ctx context.Context, event Event, humanReadableMetrics bool) (string, string) {
	instanceLabel := s.resolveInstanceLabel(ctx, event)
	customMessage := strings.TrimSpace(event.Message)

	switch event.Type {
	case EventTorrentAdded:
		title := "种子已添加"
		lines := []string{
			formatLine("种子", fmt.Sprintf("%s%s", event.TorrentName, formatHashSuffix(event.TorrentHash))),
		}
		if eta := formatETA(event.TorrentETASeconds); eta != "" {
			lines = append(lines, formatLine("ETA", eta))
		}
		lines = append(lines, formatTorrentMetricLines(event, humanReadableMetrics)...)
		if tracker := strings.TrimSpace(event.TrackerDomain); tracker != "" {
			lines = append(lines, formatLine("Tracker", tracker))
		}
		if category := strings.TrimSpace(event.Category); category != "" {
			lines = append(lines, formatLine("分类", category))
		}
		if len(event.Tags) > 0 {
			tags := append([]string(nil), event.Tags...)
			slices.Sort(tags)
			lines = append(lines, formatLine("标签", strings.Join(tags, ", ")))
		}
		return title, buildMessage(instanceLabel, lines)
	case EventTorrentCompleted:
		title := "种子已完成"
		lines := []string{
			formatLine("种子", fmt.Sprintf("%s%s", event.TorrentName, formatHashSuffix(event.TorrentHash))),
		}
		if !humanReadableMetrics {
			lines = append(lines, formatTorrentMetricLines(event, humanReadableMetrics)...)
		}
		if tracker := strings.TrimSpace(event.TrackerDomain); tracker != "" {
			lines = append(lines, formatLine("Tracker", tracker))
		}
		if category := strings.TrimSpace(event.Category); category != "" {
			lines = append(lines, formatLine("分类", category))
		}
		if len(event.Tags) > 0 {
			tags := append([]string(nil), event.Tags...)
			slices.Sort(tags)
			lines = append(lines, formatLine("标签", strings.Join(tags, ", ")))
		}
		return title, buildMessage(instanceLabel, lines)
	case EventBackupSucceeded:
		title := "备份已完成"
		lines := []string{
			formatLine("备份", formatKind(event.BackupKind)),
			formatLine("运行", strconv.FormatInt(event.BackupRunID, 10)),
			formatLine("种子数", strconv.Itoa(event.BackupTorrentCount)),
		}
		return title, buildMessage(instanceLabel, lines)
	case EventBackupFailed:
		title := "备份失败"
		lines := []string{
			formatLine("备份", formatKind(event.BackupKind)),
			formatLine("运行", strconv.FormatInt(event.BackupRunID, 10)),
			formatLine("错误", formatErrorMessage(event.ErrorMessage)),
		}
		return title, buildMessage(instanceLabel, lines)
	case EventDirScanCompleted:
		title := "目录扫描完成"
		lines := []string{
			formatLine("运行", strconv.FormatInt(event.DirScanRunID, 10)),
			formatLine("匹配数", strconv.Itoa(event.DirScanMatchesFound)),
			formatLine("已添加种子", strconv.Itoa(event.DirScanTorrentsAdded)),
		}
		return title, buildMessage(instanceLabel, lines)
	case EventDirScanFailed:
		title := "目录扫描失败"
		lines := []string{
			formatLine("运行", strconv.FormatInt(event.DirScanRunID, 10)),
			formatLine("错误", formatErrorMessage(event.ErrorMessage)),
		}
		return title, buildMessage(instanceLabel, lines)
	case EventOrphanScanCompleted:
		title := "孤立文件扫描完成"
		lines := []string{
			formatLine("运行", strconv.FormatInt(event.OrphanScanRunID, 10)),
			formatLine("已删除文件", strconv.Itoa(event.OrphanScanFilesDeleted)),
			formatLine("已删除文件夹", strconv.Itoa(event.OrphanScanFoldersDeleted)),
		}
		return title, buildMessage(instanceLabel, lines)
	case EventOrphanScanFailed:
		title := "孤立文件扫描失败"
		lines := []string{
			formatLine("运行", strconv.FormatInt(event.OrphanScanRunID, 10)),
			formatLine("错误", formatErrorMessage(event.ErrorMessage)),
		}
		return title, buildMessage(instanceLabel, lines)
	case EventCrossSeedAutomationSucceeded:
		title := "Cross-seed RSS 自动化完成"
		return formatCustomEvent(instanceLabel, title, event.Title, customMessage)
	case EventCrossSeedAutomationFailed:
		title := "Cross-seed RSS 自动化失败"
		return formatCustomEvent(instanceLabel, title, event.Title, customMessage)
	case EventCrossSeedSearchSucceeded:
		title := "Cross-seed 做种搜索完成"
		return formatCustomEvent(instanceLabel, title, event.Title, customMessage)
	case EventCrossSeedSearchFailed:
		title := "Cross-seed 做种搜索失败"
		return formatCustomEvent(instanceLabel, title, event.Title, customMessage)
	case EventCrossSeedCompletionSucceeded:
		title := "Cross-seed 完成搜索完成"
		return formatCustomEvent(instanceLabel, title, event.Title, customMessage)
	case EventCrossSeedCompletionFailed:
		title := "Cross-seed 完成搜索失败"
		return formatCustomEvent(instanceLabel, title, event.Title, customMessage)
	case EventCrossSeedWebhookSucceeded:
		title := "Cross-seed Webhook 检查完成"
		return formatCustomEvent(instanceLabel, title, event.Title, customMessage)
	case EventCrossSeedWebhookFailed:
		title := "Cross-seed Webhook 检查失败"
		return formatCustomEvent(instanceLabel, title, event.Title, customMessage)
	case EventAutomationsActionsApplied:
		title := "自动化操作已应用"
		return formatAutomationsEvent(instanceLabel, title, event.Title, customMessage, humanReadableMetrics)
	case EventAutomationsRunFailed:
		title := "自动化运行失败"
		return formatAutomationsEvent(instanceLabel, title, event.Title, customMessage, humanReadableMetrics)
	case EventDailyTrafficReport, EventHourlyTrafficReport, EventBaselineReport:
		// The traffic reports carry their own fully-formatted title and message
		// (built by the report schedulers); pass them through verbatim.
		return strings.TrimSpace(event.Title), strings.TrimSpace(event.Message)
	default:
		return "", ""
	}
}

func (s *Service) resolveInstanceLabel(ctx context.Context, event Event) string {
	if strings.TrimSpace(event.InstanceName) != "" {
		return event.InstanceName
	}
	if event.InstanceID <= 0 || s.instanceStore == nil {
		return "实例"
	}

	instance, err := s.instanceStore.Get(ctx, event.InstanceID)
	if err != nil || instance == nil {
		return fmt.Sprintf("实例 %d", event.InstanceID)
	}
	if strings.TrimSpace(instance.Name) == "" {
		return fmt.Sprintf("实例 %d", event.InstanceID)
	}

	return instance.Name
}

func allowsEvent(eventTypes []string, eventType EventType) bool {
	if len(eventTypes) == 0 {
		return true
	}

	return slices.Contains(eventTypes, string(eventType))
}

func formatHashSuffix(hash string) string {
	trimmed := strings.TrimSpace(hash)
	if len(trimmed) < 8 {
		return ""
	}
	return fmt.Sprintf(" [%s]", trimmed[:8])
}

func formatETA(seconds int64) string {
	if seconds <= 0 {
		return ""
	}
	return (time.Duration(seconds) * time.Second).Round(time.Second).String()
}

func formatTorrentMetricLines(event Event, humanReadable bool) []string {
	lines := make([]string, 0, 10)

	if state := strings.TrimSpace(event.TorrentState); state != "" {
		lines = append(lines, formatLine("状态", state))
	}
	progressPrecision := 4
	if humanReadable {
		progressPrecision = 2
	}
	lines = append(lines, formatLine("进度", strconv.FormatFloat(event.TorrentProgress, 'f', progressPrecision, 64)))
	lines = append(lines, formatLine("分享率", strconv.FormatFloat(event.TorrentRatio, 'f', 4, 64)))
	if humanReadable {
		lines = append(lines, formatLine("总大小", formatGigabytes(event.TorrentTotalSizeBytes)))
		lines = append(lines, formatLine("已下载", formatGigabytes(event.TorrentDownloadedBytes)))
		lines = append(lines, formatLine("剩余", formatGigabytes(event.TorrentAmountLeftBytes)))
		lines = append(lines, formatLine("下载速度", formatTransferSpeed(event.TorrentDlSpeedBps)))
		lines = append(lines, formatLine("上传速度", formatTransferSpeed(event.TorrentUpSpeedBps)))
	} else {
		lines = append(lines, formatLine("总大小（字节）", strconv.FormatInt(event.TorrentTotalSizeBytes, 10)))
		lines = append(lines, formatLine("已下载（字节）", strconv.FormatInt(event.TorrentDownloadedBytes, 10)))
		lines = append(lines, formatLine("剩余（字节）", strconv.FormatInt(event.TorrentAmountLeftBytes, 10)))
		lines = append(lines, formatLine("下载速度 (bps)", strconv.FormatInt(event.TorrentDlSpeedBps, 10)))
		lines = append(lines, formatLine("上传速度 (bps)", strconv.FormatInt(event.TorrentUpSpeedBps, 10)))
	}
	lines = append(lines, formatLine("做种", strconv.FormatInt(event.TorrentNumSeeds, 10)))
	lines = append(lines, formatLine("下载", strconv.FormatInt(event.TorrentNumLeechs, 10)))

	return lines
}

func formatGigabytes(value int64) string {
	const gb = 1_000_000_000.0
	if value < 0 {
		value = 0
	}
	return fmt.Sprintf("%.2f GB", float64(value)/gb)
}

func formatTransferSpeed(value int64) string {
	if value < 0 {
		value = 0
	}

	switch {
	case value < 1_000:
		return fmt.Sprintf("%d B/s", value)
	case value < 1_000_000:
		return fmt.Sprintf("%.2f KB/s", float64(value)/1_000.0)
	case value < 1_000_000_000:
		return fmt.Sprintf("%.2f MB/s", float64(value)/1_000_000.0)
	case value < 1_000_000_000_000:
		return fmt.Sprintf("%.2f GB/s", float64(value)/1_000_000_000.0)
	default:
		return fmt.Sprintf("%.2f TB/s", float64(value)/1_000_000_000_000.0)
	}
}

func formatLine(label, value string) string {
	trimmedLabel := strings.TrimSpace(label)
	trimmedValue := strings.TrimSpace(value)
	if trimmedLabel == "" || trimmedValue == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", trimmedLabel, trimmedValue)
}

func buildMessage(instanceLabel string, lines []string) string {
	payload := make([]string, 0, len(lines)+1)
	if trimmed := strings.TrimSpace(instanceLabel); trimmed != "" {
		payload = append(payload, formatLine("实例", trimmed))
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			payload = append(payload, "")
			continue
		}
		payload = append(payload, line)
	}
	return strings.Join(payload, "\n")
}

func splitMessageLines(message string) []string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if line := strings.TrimSpace(part); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

type messageField struct {
	Label  string
	Value  string
	Inline bool
}

type discordWebhookPayload struct {
	Content   string         `json:"content,omitempty"`
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Embeds    []discordEmbed `json:"embeds,omitempty"`
}

type discordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type notifiarrField struct {
	Title  string `json:"title"`
	Text   string `json:"text"`
	Inline bool   `json:"inline"`
}

func formatCustomEvent(instanceLabel, defaultTitle, overrideTitle, message string) (string, string) {
	title := defaultTitle
	if strings.TrimSpace(overrideTitle) != "" {
		title = strings.TrimSpace(overrideTitle)
	}
	if strings.TrimSpace(message) == "" {
		return title, ""
	}
	return title, buildMessage(instanceLabel, splitMessageLines(message))
}

func formatAutomationsEvent(instanceLabel, defaultTitle, overrideTitle, message string, dedupeSampleLines bool) (string, string) {
	title := defaultTitle
	if strings.TrimSpace(overrideTitle) != "" {
		title = strings.TrimSpace(overrideTitle)
	}
	if strings.TrimSpace(message) == "" {
		return title, ""
	}

	lines := strings.Split(strings.TrimSpace(message), "\n")
	if dedupeSampleLines {
		lines = mergeAutomationSampleLines(lines)
	}

	return title, buildMessage(instanceLabel, lines)
}

func mergeAutomationSampleLines(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	const (
		tagPrefix    = "标签样本:"
		samplePrefix = "样本:"
	)

	tagLineIndex := -1
	sampleLineIndex := -1

	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, tagPrefix):
			tagLineIndex = i
		case strings.HasPrefix(line, samplePrefix):
			sampleLineIndex = i
		}
	}

	// The affected-torrent section is now a multi-line "影响种子:" block, so the
	// only mergeable case is a legacy "样本:" line that duplicates tag samples.
	// Without a real sample line there is nothing to merge.
	if tagLineIndex < 0 || sampleLineIndex < 0 {
		return lines
	}

	tagSamples := parseSampleListLine(lines[tagLineIndex], tagPrefix)
	samples := []string(nil)
	if sampleLineIndex >= 0 {
		samples = parseSampleListLine(lines[sampleLineIndex], samplePrefix)
	}

	merged := make([]string, 0, len(tagSamples)+len(samples))
	seen := make(map[string]struct{}, len(tagSamples)+len(samples))
	for _, sample := range tagSamples {
		if _, exists := seen[sample]; exists {
			continue
		}
		seen[sample] = struct{}{}
		merged = append(merged, sample)
	}
	for _, sample := range samples {
		if _, exists := seen[sample]; exists {
			continue
		}
		seen[sample] = struct{}{}
		merged = append(merged, sample)
	}

	out := make([]string, 0, len(lines))
	insertIndex := tagLineIndex
	if sampleLineIndex >= 0 && sampleLineIndex < insertIndex {
		insertIndex = sampleLineIndex
	}

	for i, line := range lines {
		if i == insertIndex && len(merged) > 0 {
			out = append(out, samplePrefix+" "+strings.Join(merged, "; "))
		}
		if i == tagLineIndex || i == sampleLineIndex {
			continue
		}
		out = append(out, line)
	}

	return out
}

func parseSampleListLine(line, prefix string) []string {
	content := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if content == "" {
		return nil
	}

	parts := strings.Split(content, ";")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}

	return out
}

const (
	maxMessageLength            = 420
	telegramMaxMessageLength    = 4000
	maxTitleLength              = 80
	dailyReportMaxMessageLength = 8192
	discordTitleLimit           = 256
	discordDescriptionLimit     = 4096
	discordFieldNameLimit       = 256
	discordFieldValueLimit      = 1024
	discordFieldsLimit          = 25
	notifiarrTitleLimit         = 256
	notifiarrDescriptionLimit   = 4096
)

const (
	discordColorInfo    = 0x58b9ff
	discordColorSuccess = 0x57f287
	discordColorError   = 0xed4245
)

// maxMessageLengthFor returns the message length cap for a target and event.
// Telegram accepts up to 4096 characters per message, while generic targets
// (email, Slack, …) are capped much lower so longer automation notifications
// (per-torrent details for several affected seeds) are not truncated there.
func maxMessageLengthFor(rawURL string, eventType EventType) int {
	if targetScheme(rawURL) == "telegram" {
		return telegramMaxMessageLength
	}
	switch eventType {
	case EventDailyTrafficReport, EventHourlyTrafficReport, EventBaselineReport:
		return dailyReportMaxMessageLength
	default:
		return maxMessageLength
	}
}

func targetScheme(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme == "" {
		return ""
	}
	if strings.Contains(scheme, "+") {
		parts := strings.SplitN(scheme, "+", 2)
		return parts[0]
	}
	return scheme
}

func buildStructuredMessage(message string) (string, []messageField) {
	lines := splitMessageLines(message)
	if len(lines) == 0 {
		return "", nil
	}

	var description string
	fields := make([]messageField, 0, len(lines))

	for _, line := range lines {
		label, value, ok := splitLine(line)
		if !ok {
			if description == "" {
				description = line
			} else {
				fields = append(fields, messageField{
					Label:  "详情",
					Value:  normalizeField("详情", line).Value,
					Inline: false,
				})
			}
			continue
		}
		lowerLabel := strings.ToLower(label)
		if lowerLabel == "实例" {
			fields = append(fields, normalizeField(label, value))
			continue
		}
		if description == "" {
			switch lowerLabel {
			case "种子":
				description = value
			case "运行":
				description = "运行 " + value
			default:
				description = fmt.Sprintf("%s: %s", label, value)
			}
			continue
		}
		fields = append(fields, normalizeField(label, value))
	}

	if description == "" && len(fields) > 0 {
		description = fmt.Sprintf("%s: %s", fields[0].Label, fields[0].Value)
		fields = fields[1:]
	}

	return description, fields
}

func splitLine(line string) (string, string, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	label := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if label == "" || value == "" {
		return "", "", false
	}
	return label, value, true
}

func normalizeField(label, value string) messageField {
	trimmedLabel := truncateMessage(label, discordFieldNameLimit)
	trimmedValue := truncateMessage(value, discordFieldValueLimit)
	return messageField{
		Label:  trimmedLabel,
		Value:  trimmedValue,
		Inline: shouldInlineField(trimmedLabel, trimmedValue),
	}
}

func shouldInlineField(label, value string) bool {
	if label == "" || value == "" {
		return false
	}
	switch strings.ToLower(label) {
	case "种子", "样本", "错误", "消息", "标签":
		return false
	}
	return utf8.RuneCountInString(value) <= 48
}

func buildDiscordPayload(config *shoutrrrdiscord.Config, event Event, title, message string) (discordWebhookPayload, error) {
	description, fields := buildStructuredMessage(message)
	if description == "" {
		description = message
	}

	title = truncateMessage(strings.TrimSpace(title), discordTitleLimit)
	description = truncateMessage(description, discordDescriptionLimit)

	embedFields := make([]discordEmbedField, 0, min(len(fields), discordFieldsLimit))
	for _, field := range fields {
		if len(embedFields) >= discordFieldsLimit {
			break
		}
		embedFields = append(embedFields, discordEmbedField{
			Name:   field.Label,
			Value:  field.Value,
			Inline: field.Inline,
		})
	}

	embed := discordEmbed{
		Title:       title,
		Description: description,
		Color:       discordEventColor(event.Type),
		Fields:      embedFields,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	payload := discordWebhookPayload{
		Embeds: []discordEmbed{embed},
	}
	if config != nil {
		if strings.TrimSpace(config.Username) != "" {
			payload.Username = strings.TrimSpace(config.Username)
		}
		if strings.TrimSpace(config.Avatar) != "" {
			payload.AvatarURL = strings.TrimSpace(config.Avatar)
		}
	}

	if payload.Embeds[0].Title == "" && payload.Embeds[0].Description == "" && len(payload.Embeds[0].Fields) == 0 {
		return discordWebhookPayload{}, errors.New("notification has no content to send")
	}

	return payload, nil
}

func discordEventColor(eventType EventType) int {
	switch eventType {
	case EventBackupFailed,
		EventDirScanFailed,
		EventOrphanScanFailed,
		EventCrossSeedAutomationFailed,
		EventCrossSeedSearchFailed,
		EventCrossSeedCompletionFailed,
		EventCrossSeedWebhookFailed,
		EventAutomationsRunFailed:
		return discordColorError
	case EventTorrentCompleted,
		EventBackupSucceeded,
		EventDirScanCompleted,
		EventOrphanScanCompleted,
		EventCrossSeedAutomationSucceeded,
		EventCrossSeedSearchSucceeded,
		EventCrossSeedCompletionSucceeded,
		EventCrossSeedWebhookSucceeded,
		EventAutomationsActionsApplied:
		return discordColorSuccess
	case EventTorrentAdded:
		return discordColorInfo
	default:
		return discordColorInfo
	}
}

func buildNotifiarrFields(fields []messageField) []notifiarrField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]notifiarrField, 0, min(len(fields), discordFieldsLimit))
	for _, field := range fields {
		if len(out) >= discordFieldsLimit {
			break
		}
		out = append(out, notifiarrField{
			Title:  truncateMessage(field.Label, discordFieldNameLimit),
			Text:   truncateMessage(field.Value, discordFieldValueLimit),
			Inline: field.Inline,
		})
	}
	return out
}

func truncateMessage(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if utf8.RuneCountInString(trimmed) <= limit {
		return trimmed
	}
	runes := []rune(trimmed)
	if limit <= 1 {
		return string(runes[:limit])
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}

func formatKind(kind models.BackupRunKind) string {
	raw := strings.TrimSpace(string(kind))
	if raw == "" {
		return "备份"
	}
	return raw
}

func formatErrorMessage(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return "未知错误"
	}
	return trimmed
}
