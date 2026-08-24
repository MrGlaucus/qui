// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package notifications

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/models"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		value int64
		want  string
	}{
		{name: "bytes", value: 500, want: "500 B"},
		{name: "kilobytes", value: 1_500, want: "1.50 KB"},
		{name: "megabytes", value: 5_000_000, want: "5.00 MB"},
		{name: "gigabytes", value: 85_330_000_000, want: "85.33 GB"},
		{name: "terabytes", value: 21_960_000_000_000, want: "21.96 TB"},
		{name: "negative clamped", value: -100, want: "0 B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatBytes(tt.value))
		})
	}
}

func TestBuildDailyTrafficReport(t *testing.T) {
	settleAt := time.Date(2026, 8, 9, 0, 0, 5, 0, time.Local)
	rows := []*models.InstanceDailyTraffic{
		{InstanceID: 1, Uploaded: 4_950_000_000_000, Downloaded: 1_860_000_000_000},
		{InstanceID: 2, Uploaded: 85_000_000_000, Downloaded: 41_000_000_000},
	}
	resolve := func(instanceID int) string {
		return map[int]string{1: "OVH-KS1B-DE-1", 2: "HostDZire-US"}[instanceID]
	}

	title, message := buildDailyTrafficReport("2026-08-08", settleAt, rows, resolve)

	require.Equal(t, "📅 每日流量报告（2026-08-08）", title)
	require.Contains(t, message, "📊 汇总")
	require.Contains(t, message, "⬆️ 今日上传：5.04 TB")
	require.Contains(t, message, "⬇️ 今日下载：1.90 TB")
	require.Contains(t, message, "📈 今日流量：6.94 TB")
	require.Contains(t, message, "🏷️ OVH-KS1B-DE-1")
	require.Contains(t, message, "⬆️ 今日上传：4.95 TB")
	require.Contains(t, message, "⬇️ 今日下载：1.86 TB")
	require.Contains(t, message, "📈 今日流量：6.81 TB")
	require.Contains(t, message, "🏷️ HostDZire-US")
	require.Contains(t, message, "⬆️ 今日上传：85.00 GB")
	require.Contains(t, message, "⬇️ 今日下载：41.00 GB")
	require.Contains(t, message, "📈 今日流量：126.00 GB")
}

func TestBuildDailyTrafficReportResolvesInstanceNames(t *testing.T) {
	rows := []*models.InstanceDailyTraffic{
		{InstanceID: 7, Uploaded: 100, Downloaded: 200},
	}
	title, message := buildDailyTrafficReport("2026-08-08", time.Now(), rows, func(int) string { return "" })
	require.Contains(t, title, "每日流量报告")
	require.Contains(t, message, "🏷️ 实例")
}

func TestFormatEventDailyTrafficReportPassesThrough(t *testing.T) {
	svc := &Service{}
	title, message := svc.formatEvent(nil, Event{
		Type:    EventDailyTrafficReport,
		Title:   "实例数据统计（每日结算 2026-08-09 00:00:00）",
		Message: "【汇总】（2026-08-08）\n总上传：21.96 TB",
	}, true)

	require.Equal(t, "实例数据统计（每日结算 2026-08-09 00:00:00）", title)
	require.Equal(t, "【汇总】（2026-08-08）\n总上传：21.96 TB", message)
}

func TestBuildHourlyTrafficReport(t *testing.T) {
	settleAt := time.Date(2026, 8, 9, 14, 0, 0, 0, time.Local)
	rows := []*models.InstanceDailyTraffic{
		{InstanceID: 1, Uploaded: 4_950_000_000_000, Downloaded: 1_860_000_000_000},
		{InstanceID: 2, Uploaded: 85_000_000_000, Downloaded: 41_000_000_000},
	}
	resolve := func(instanceID int) string {
		return map[int]string{1: "OVH-KS1B-DE-1", 2: "HostDZire-US"}[instanceID]
	}

	title, message := buildHourlyTrafficReport("2026-08-09", settleAt, rows, resolve)

	require.Equal(t, "🕐 整点流量报告（2026-08-09 14:00:00）", title)
	require.Contains(t, message, "📊 汇总")
	require.Contains(t, message, "⬆️ 今日上传：5.04 TB")
	require.Contains(t, message, "🏷️ HostDZire-US")
	require.Contains(t, message, "📈 今日流量：126.00 GB")
}

func TestBuildBaselineReport(t *testing.T) {
	rows := []*models.InstanceDailyTraffic{
		{
			InstanceID:                1,
			DataSource:                models.DataSourceSession,
			BaselineSessionUploaded:   68_940_000_000_000,
			BaselineSessionDownloaded: 29_500_000_000_000,
			BaselineAt:                "2026-08-10T16:00:00Z",
		},
		{
			InstanceID:                2,
			DataSource:                models.DataSourceAlltime,
			BaselineAlltimeUploaded:   14_860_000_000_000,
			BaselineAlltimeDownloaded: 4_120_000_000_000,
			BaselineAt:                "2026-08-10T16:00:00Z",
		},
	}
	resolve := func(instanceID int) string {
		return map[int]string{1: "OVH-KS1B-DE-1", 2: "HostDZire-US"}[instanceID]
	}

	title, message := buildBaselineReport("2026-08-11", rows, resolve, time.Local)

	require.Equal(t, "🌙 基准采集结果 2026-08-11", title)
	require.Contains(t, message, "🏷️ OVH-KS1B-DE-1")
	require.Contains(t, message, "🎯 基准: ↑ 68.94 TB / ↓ 29.50 TB")
	require.Contains(t, message, "🧭 来源: session")
	require.Contains(t, message, "⏱️ 时间: 2026-08-11 00:00:00")
	require.Contains(t, message, "🏷️ HostDZire-US")
	require.Contains(t, message, "🎯 基准: ↑ 14.86 TB / ↓ 4.12 TB")
	require.Contains(t, message, "🧭 来源: alltime")
}

func TestBaselineBytesSelectsCountersBySource(t *testing.T) {
	session := &models.InstanceDailyTraffic{
		DataSource:                models.DataSourceSession,
		BaselineSessionUploaded:   10,
		BaselineSessionDownloaded: 20,
		BaselineAlltimeUploaded:   100,
		BaselineAlltimeDownloaded: 200,
	}
	u, d := baselineBytes(session)
	assert.Equal(t, int64(10), u)
	assert.Equal(t, int64(20), d)

	alltime := &models.InstanceDailyTraffic{
		DataSource:                models.DataSourceRestart,
		BaselineSessionUploaded:   10,
		BaselineSessionDownloaded: 20,
		BaselineAlltimeUploaded:   100,
		BaselineAlltimeDownloaded: 200,
	}
	u, d = baselineBytes(alltime)
	assert.Equal(t, int64(100), u)
	assert.Equal(t, int64(200), d)
}

func TestBaselineRowsFiltersNoBaseline(t *testing.T) {
	rows := []*models.InstanceDailyTraffic{
		{InstanceID: 1, BaselineAt: "2026-08-11T00:00:00Z"},
		{InstanceID: 2, BaselineAt: ""},
		nil,
	}
	got := baselineRows(rows)
	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0].InstanceID)
}

func TestFormatEventReportEventsPassThrough(t *testing.T) {
	svc := &Service{}
	for _, eventType := range []EventType{EventHourlyTrafficReport, EventBaselineReport} {
		title, message := svc.formatEvent(nil, Event{
			Type:    eventType,
			Title:   "报告标题",
			Message: "报告正文",
		}, true)

		require.Equal(t, "报告标题", title)
		require.Equal(t, "报告正文", message)
	}
}
