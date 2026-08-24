// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package notifications

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/autobrr/qui/internal/models"
)

const (
	dailyReportCheckInterval = 30 * time.Second
	baselineGracePeriod      = 2 * time.Minute
)

// dailyTrafficReportStore is the persistence surface the daily traffic report
// needs: a read-only listing of all instances' rows for a settled day. The
// report never writes to instance_daily_traffic, so it cannot interfere with
// the baseline collection in DailyTrafficRecorder.
type dailyTrafficReportStore interface {
	ListByDate(ctx context.Context, date string) ([]*models.InstanceDailyTraffic, error)
}

// forceSyncTrafficInstances triggers a fresh qBittorrent maindata sync for every
// active instance with daily traffic collection enabled. This guarantees the
// baseline snapshot (and hourly cumulative totals) are written on time even
// when no SSE subscriber is keeping the sync loop alive.
func (s *Service) forceSyncTrafficInstances(ctx context.Context) {
	if s == nil || s.forceSync == nil || s.instanceStore == nil {
		return
	}
	instances, err := s.instanceStore.List(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("traffic report: failed to list instances for forced sync")
		return
	}
	for _, inst := range instances {
		if inst == nil || !inst.IsActive || !inst.DailyTrafficEnabled {
			continue
		}
		if err := s.forceSync(ctx, inst.ID); err != nil {
			s.logger.Warn().Err(err).Int("instanceID", inst.ID).Msg("traffic report: forced sync failed")
		}
	}
}

// StartDailyTrafficReport launches the midnight daily traffic report scheduler.
//
// The scheduler keeps the last settled date in memory (initialised to today at
// startup) and, whenever the server-local calendar day advances, reports the
// completed day. It only reads instance_daily_traffic rows; baseline collection
// in DailyTrafficRecorder is untouched.
func (s *Service) StartDailyTrafficReport(ctx context.Context, store dailyTrafficReportStore) {
	if s == nil || s.store == nil || store == nil {
		return
	}

	lastDate := s.now().Format("2006-01-02")

	go func() {
		ticker := time.NewTicker(dailyReportCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				today := now.Format("2006-01-02")
				if today == lastDate {
					continue
				}
				reportDate := lastDate
				lastDate = today
				s.reportDailyTraffic(ctx, store, reportDate, now)
			}
		}
	}()
}

func (s *Service) reportDailyTraffic(ctx context.Context, store dailyTrafficReportStore, reportDate string, settleAt time.Time) {
	rows, err := store.ListByDate(ctx, reportDate)
	if err != nil {
		s.logger.Error().Err(err).Str("date", reportDate).Msg("daily traffic report: failed to load rows")
		return
	}
	if len(rows) == 0 {
		return
	}

	rows = s.sortTrafficRowsBySortOrder(ctx, rows)
	title, message := buildDailyTrafficReport(reportDate, settleAt, rows, s.resolveReportInstanceName(ctx))
	if strings.TrimSpace(message) == "" {
		return
	}

	s.Notify(ctx, Event{
		Type:    EventDailyTrafficReport,
		Title:   title,
		Message: message,
	})
}

// StartHourlyTrafficReport launches the hourly traffic report scheduler.
//
// It reports the current calendar day's cumulative traffic at every full hour.
// At midnight it skips the report when the daily traffic report is enabled for
// any target, so the two do not fire duplicate notifications for the same
// boundary. Only reads instance_daily_traffic rows.
func (s *Service) StartHourlyTrafficReport(ctx context.Context, store dailyTrafficReportStore) {
	if s == nil || s.store == nil || store == nil {
		return
	}

	lastHour := s.now().Format("2006-01-02-15")
	// Fire once at startup so the report covers the current accumulated data
	// without waiting for the next full hour boundary.
	startedAt := s.now()

	go func() {
		ticker := time.NewTicker(dailyReportCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := s.now()
				hourKey := now.Format("2006-01-02-15")
				hourChanged := hourKey != lastHour
				if hourChanged {
					lastHour = hourKey
				}
				// On startup, fire the first report immediately so the caller
				// does not have to wait until the next hour boundary.
				warmup := !hourChanged && startedAt.Add(dailyReportCheckInterval).After(now)

				if !hourChanged && !warmup {
					continue
				}

				// At midnight the daily report already covers the boundary; skip
				// when any target has it enabled.
				if now.Hour() == 0 && s.dailyTrafficReportEnabled(ctx) {
					continue
				}
				// Only kick the sync when the hourly report is actually wanted,
				// so we don't hammer qBittorrent every hour for nothing.
				if s.hourlyTrafficReportEnabled(ctx) {
					s.forceSyncTrafficInstances(ctx)
				}
				s.reportHourlyTraffic(ctx, store, now)
			}
		}
	}()
}

func (s *Service) reportHourlyTraffic(ctx context.Context, store dailyTrafficReportStore, settleAt time.Time) {
	today := settleAt.Format("2006-01-02")
	rows, err := store.ListByDate(ctx, today)
	if err != nil {
		s.logger.Error().Err(err).Str("date", today).Msg("hourly traffic report: failed to load rows")
		return
	}
	if len(rows) == 0 {
		s.logger.Info().Str("date", today).Msg("hourly traffic report: no data for today, skipped")
		return
	}

	rows = s.sortTrafficRowsBySortOrder(ctx, rows)
	title, message := buildHourlyTrafficReport(today, settleAt, rows, s.resolveReportInstanceName(ctx))
	if strings.TrimSpace(message) == "" {
		return
	}

	s.Notify(ctx, Event{
		Type:    EventHourlyTrafficReport,
		Title:   title,
		Message: message,
	})
}

// StartBaselineReport launches the midnight baseline capture report scheduler.
//
// After the daily baseline capture writes the first row of the new day, this
// waits a short grace period for all instances to capture their baseline, then
// reports the snapshot. Only reads instance_daily_traffic rows; it does not
// touch baseline collection.
func (s *Service) StartBaselineReport(ctx context.Context, store dailyTrafficReportStore) {
	if s == nil || s.store == nil || store == nil {
		return
	}

	lastBaselineDate := s.now().Format("2006-01-02")
	var graceEnd time.Time
	syncedForDay := false

	go func() {
		ticker := time.NewTicker(dailyReportCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := s.now()
				today := now.Format("2006-01-02")
				if today == lastBaselineDate {
					continue
				}

				rows, err := store.ListByDate(ctx, today)
				if err != nil {
					s.logger.Error().Err(err).Str("date", today).Msg("baseline report: failed to load rows")
					continue
				}
				baseline := baselineRows(rows)
				if len(baseline) == 0 {
					// Baseline not captured yet — kick the sync once so the
					// baseline is written on time even with no SSE subscriber.
					if !syncedForDay {
						syncedForDay = true
						s.forceSyncTrafficInstances(ctx)
					}
					continue
				}

				// First detection: start the grace period so the remaining
				// instances get a chance to capture before we report.
				if graceEnd.IsZero() {
					graceEnd = now.Add(baselineGracePeriod)
					continue
				}
				if now.Before(graceEnd) {
					continue
				}

				s.reportBaseline(ctx, today, baseline)
				lastBaselineDate = today
				graceEnd = time.Time{}
				syncedForDay = false
			}
		}
	}()
}

func (s *Service) reportBaseline(ctx context.Context, date string, rows []*models.InstanceDailyTraffic) {
	rows = s.sortTrafficRowsBySortOrder(ctx, rows)
	title, message := buildBaselineReport(date, rows, s.resolveReportInstanceName(ctx), s.location())
	if strings.TrimSpace(message) == "" {
		return
	}

	s.Notify(ctx, Event{
		Type:    EventBaselineReport,
		Title:   title,
		Message: message,
	})
}

// dailyTrafficReportEnabled reports whether any enabled notification target has
// the daily traffic report event selected (an empty event list means all
// events). Used by the hourly scheduler to skip the midnight boundary.
func (s *Service) dailyTrafficReportEnabled(ctx context.Context) bool {
	return s.trafficReportEnabled(ctx, EventDailyTrafficReport)
}

// hourlyTrafficReportEnabled reports whether any enabled notification target
// has the hourly traffic report event selected.
func (s *Service) hourlyTrafficReportEnabled(ctx context.Context) bool {
	return s.trafficReportEnabled(ctx, EventHourlyTrafficReport)
}

func (s *Service) trafficReportEnabled(ctx context.Context, eventType EventType) bool {
	if s == nil || s.store == nil {
		return false
	}
	targets, err := s.store.ListEnabled(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("traffic report: failed to list targets")
		return false
	}
	for _, target := range targets {
		if allowsEvent(target.EventTypes, eventType) {
			return true
		}
	}
	return false
}

// resolveReportInstanceName returns a resolver that maps an instance id to its
// display name for the report body.
func (s *Service) resolveReportInstanceName(ctx context.Context) func(instanceID int) string {
	return func(instanceID int) string {
		return s.resolveInstanceLabel(ctx, Event{InstanceID: instanceID})
	}
}

// sortTrafficRowsBySortOrder reorders a slice of daily traffic rows to match
// the instance sort_order configured in the UI/dashboard.
func (s *Service) sortTrafficRowsBySortOrder(ctx context.Context, rows []*models.InstanceDailyTraffic) []*models.InstanceDailyTraffic {
	if s == nil || s.instanceStore == nil || len(rows) <= 1 {
		return rows
	}
	instances, err := s.instanceStore.List(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("traffic report: failed to load instances for sort")
		return rows
	}
	orderMap := make(map[int]int, len(instances))
	for _, inst := range instances {
		orderMap[inst.ID] = inst.SortOrder
	}
	sorted := make([]*models.InstanceDailyTraffic, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool {
		return orderMap[sorted[i].InstanceID] < orderMap[sorted[j].InstanceID]
	})
	return sorted
}

// buildDailyTrafficReport formats the midnight report for a settled day.
// reportDate is the completed calendar day being reported (format 2006-01-02);
// settleAt is the settlement moment (the run after midnight).
func buildDailyTrafficReport(reportDate string, settleAt time.Time, rows []*models.InstanceDailyTraffic, instanceName func(int) string) (string, string) {
	title := fmt.Sprintf("📅 每日流量报告（%s）", reportDate)
	return title, buildTrafficReportMessage(reportDate, rows, instanceName)
}

// buildHourlyTrafficReport formats the on-the-hour report for the current
// calendar day's cumulative traffic.
func buildHourlyTrafficReport(reportDate string, settleAt time.Time, rows []*models.InstanceDailyTraffic, instanceName func(int) string) (string, string) {
	title := fmt.Sprintf("🕐 整点流量报告（%s）", settleAt.Format("2006-01-02 15:04:05"))
	return title, buildTrafficReportMessage(reportDate, rows, instanceName)
}

// buildTrafficReportMessage renders the shared per-day traffic summary body:
// a totals block followed by one block per instance.
func buildTrafficReportMessage(reportDate string, rows []*models.InstanceDailyTraffic, instanceName func(int) string) string {
	var totalUploaded, totalDownloaded int64
	for _, row := range rows {
		if row == nil {
			continue
		}
		totalUploaded += row.Uploaded
		totalDownloaded += row.Downloaded
	}

	var sb strings.Builder
	sb.WriteString("📊 汇总\n")
	sb.WriteString(fmt.Sprintf("⬆️ 今日上传：%s\n", formatBytes(totalUploaded)))
	sb.WriteString(fmt.Sprintf("⬇️ 今日下载：%s\n", formatBytes(totalDownloaded)))
	sb.WriteString(fmt.Sprintf("📈 今日流量：%s", formatBytes(totalUploaded+totalDownloaded)))

	for _, row := range rows {
		if row == nil {
			continue
		}
		name := "实例"
		if instanceName != nil {
			if resolved := strings.TrimSpace(instanceName(row.InstanceID)); resolved != "" {
				name = resolved
			}
		}
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("🏷️ %s\n", name))
		sb.WriteString(fmt.Sprintf("⬆️ 今日上传：%s\n", formatBytes(row.Uploaded)))
		sb.WriteString(fmt.Sprintf("⬇️ 今日下载：%s\n", formatBytes(row.Downloaded)))
		sb.WriteString(fmt.Sprintf("📈 今日流量：%s", formatBytes(row.Uploaded+row.Downloaded)))
	}

	return sb.String()
}

// baselineRows filters rows that carry a day-boundary baseline snapshot.
func baselineRows(rows []*models.InstanceDailyTraffic) []*models.InstanceDailyTraffic {
	out := make([]*models.InstanceDailyTraffic, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.BaselineAt != "" {
			out = append(out, row)
		}
	}
	return out
}

// buildBaselineReport formats the midnight baseline capture report. Each row
// contributes one instance block: display name, captured baseline bytes, data
// source, and capture time (rendered in loc).
func buildBaselineReport(date string, rows []*models.InstanceDailyTraffic, instanceName func(int) string, loc *time.Location) (string, string) {
	title := fmt.Sprintf("🌙 基准采集结果 %s", date)

	var sb strings.Builder
	for i, row := range rows {
		if row == nil {
			continue
		}
		if i > 0 {
			sb.WriteString("\n\n")
		}
		name := "实例"
		if instanceName != nil {
			if resolved := strings.TrimSpace(instanceName(row.InstanceID)); resolved != "" {
				name = resolved
			}
		}
		uploaded, downloaded := baselineBytes(row)
		sb.WriteString(fmt.Sprintf("🏷️ %s\n", name))
		sb.WriteString(fmt.Sprintf("🎯 基准: ↑ %s / ↓ %s\n", formatBytes(uploaded), formatBytes(downloaded)))
		sb.WriteString(fmt.Sprintf("🧭 来源: %s\n", baselineSource(row)))
		sb.WriteString(fmt.Sprintf("⏱️ 时间: %s", formatBaselineTime(row, loc)))
	}

	return title, sb.String()
}

// baselineBytes returns the captured baseline bytes for a row, choosing the
// session counters for session-sourced rows and the alltime counters otherwise
// (alltime / restart fallback / reinstall re-baselines).
func baselineBytes(row *models.InstanceDailyTraffic) (uploaded, downloaded int64) {
	if row == nil {
		return 0, 0
	}
	if row.DataSource == models.DataSourceSession {
		return row.BaselineSessionUploaded, row.BaselineSessionDownloaded
	}
	return row.BaselineAlltimeUploaded, row.BaselineAlltimeDownloaded
}

// baselineSource returns the data source label recorded on the row.
func baselineSource(row *models.InstanceDailyTraffic) string {
	if row == nil || strings.TrimSpace(row.DataSource) == "" {
		return models.DataSourceSession
	}
	return row.DataSource
}

// formatBaselineTime renders the captured baseline timestamp in loc.
func formatBaselineTime(row *models.InstanceDailyTraffic, loc *time.Location) string {
	if row == nil || strings.TrimSpace(row.BaselineAt) == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, row.BaselineAt)
	if err != nil {
		return row.BaselineAt
	}
	if loc == nil {
		loc = time.Local
	}
	return parsed.In(loc).Format("2006-01-02 15:04:05")
}

// formatBytes renders a byte count with an auto-selected binary-decimal unit.
func formatBytes(value int64) string {
	if value < 0 {
		value = 0
	}

	switch {
	case value >= 1_000_000_000_000:
		return fmt.Sprintf("%.2f TB", float64(value)/1_000_000_000_000.0)
	case value >= 1_000_000_000:
		return fmt.Sprintf("%.2f GB", float64(value)/1_000_000_000.0)
	case value >= 1_000_000:
		return fmt.Sprintf("%.2f MB", float64(value)/1_000_000.0)
	case value >= 1_000:
		return fmt.Sprintf("%.2f KB", float64(value)/1_000.0)
	default:
		return fmt.Sprintf("%d B", value)
	}
}
