// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package notifications

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaxMessageLengthForTarget(t *testing.T) {
	t.Parallel()

	telegramURL := "telegram://123456:token@telegram?chats=@channel"

	tests := []struct {
		name      string
		rawURL    string
		eventType EventType
		want      int
	}{
		{
			name:      "telegram uses the telegram cap",
			rawURL:    telegramURL,
			eventType: EventAutomationsActionsApplied,
			want:      telegramMaxMessageLength,
		},
		{
			name:      "generic target uses the generic cap",
			rawURL:    "mailto://user:pass@host",
			eventType: EventAutomationsActionsApplied,
			want:      maxMessageLength,
		},
		{
			name:      "generic daily report uses the long report cap",
			rawURL:    "mailto://user:pass@host",
			eventType: EventDailyTrafficReport,
			want:      dailyReportMaxMessageLength,
		},
		{
			name:      "telegram daily report still respects telegram cap",
			rawURL:    telegramURL,
			eventType: EventDailyTrafficReport,
			want:      telegramMaxMessageLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, maxMessageLengthFor(tt.rawURL, tt.eventType))
		})
	}
}

func TestFormatEventTorrentAddedIncludesMetricLines(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	title, message := svc.formatEvent(context.Background(), Event{
		Type:                   EventTorrentAdded,
		InstanceID:             1,
		TorrentName:            "Example.Release",
		TorrentHash:            "0123456789abcdef",
		TorrentETASeconds:      30,
		TorrentProgress:        0,
		TorrentRatio:           0,
		TorrentTotalSizeBytes:  0,
		TorrentDownloadedBytes: 0,
		TorrentAmountLeftBytes: 0,
		TorrentDlSpeedBps:      0,
		TorrentUpSpeedBps:      0,
		TorrentNumSeeds:        0,
		TorrentNumLeechs:       0,
	}, true)

	require.Equal(t, "种子已添加", title)
	require.Contains(t, message, "进度: 0.00")
	require.Contains(t, message, "分享率: 0.0000")
	require.Contains(t, message, "总大小: 0.00 GB")
	require.Contains(t, message, "下载速度: 0 B/s")
	require.Contains(t, message, "上传速度: 0 B/s")
	require.Contains(t, message, "做种: 0")
	require.Contains(t, message, "下载: 0")
}

func TestFormatEventTorrentCompletedOmitsMetricLinesOutsideNotifiarrAPI(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	title, message := svc.formatEvent(context.Background(), Event{
		Type:                   EventTorrentCompleted,
		InstanceID:             1,
		TorrentName:            "Done.Release",
		TorrentHash:            "fedcba9876543210",
		TrackerDomain:          "tracker.example",
		Category:               "movies",
		Tags:                   []string{"tag-b", "tag-a"},
		TorrentProgress:        1,
		TorrentRatio:           1.5,
		TorrentTotalSizeBytes:  123,
		TorrentDownloadedBytes: 123,
		TorrentAmountLeftBytes: 0,
		TorrentDlSpeedBps:      0,
		TorrentUpSpeedBps:      42,
		TorrentNumSeeds:        7,
		TorrentNumLeechs:       2,
	}, true)

	require.Equal(t, "种子已完成", title)
	require.Contains(t, message, "种子: Done.Release [fedcba98]")
	require.Contains(t, message, "Tracker: tracker.example")
	require.Contains(t, message, "分类: movies")
	require.Contains(t, message, "标签: tag-a, tag-b")
	require.NotContains(t, message, "进度:")
	require.NotContains(t, message, "分享率:")
	require.NotContains(t, message, "总大小")
	require.NotContains(t, message, "已下载")
	require.NotContains(t, message, "剩余")
	require.NotContains(t, message, "下载速度")
	require.NotContains(t, message, "上传速度")
	require.NotContains(t, message, "做种:")
	require.NotContains(t, message, "下载:")
}

func TestFormatEventTorrentAddedNotifiarrAPIMetricsStayRaw(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	title, message := svc.formatEvent(context.Background(), Event{
		Type:                   EventTorrentAdded,
		InstanceID:             1,
		TorrentName:            "Example.Release",
		TorrentHash:            "0123456789abcdef",
		TorrentProgress:        0.0306,
		TorrentRatio:           0,
		TorrentTotalSizeBytes:  7_926_201_054,
		TorrentDownloadedBytes: 176_551_163,
		TorrentAmountLeftBytes: 7_683_996_382,
		TorrentDlSpeedBps:      29_308_908,
		TorrentUpSpeedBps:      0,
		TorrentNumSeeds:        26,
		TorrentNumLeechs:       1,
	}, false)

	require.Equal(t, "种子已添加", title)
	require.Contains(t, message, "进度: 0.0306")
	require.Contains(t, message, "总大小（字节）: 7926201054")
	require.Contains(t, message, "下载速度 (bps): 29308908")
	require.Contains(t, message, "上传速度 (bps): 0")
}

func TestFormatEventTorrentCompletedNotifiarrAPIMetricsStayRaw(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	title, message := svc.formatEvent(context.Background(), Event{
		Type:                   EventTorrentCompleted,
		InstanceID:             1,
		TorrentName:            "Done.Release",
		TorrentHash:            "fedcba9876543210",
		TorrentProgress:        1,
		TorrentRatio:           1.5,
		TorrentTotalSizeBytes:  123,
		TorrentDownloadedBytes: 123,
		TorrentAmountLeftBytes: 0,
		TorrentDlSpeedBps:      0,
		TorrentUpSpeedBps:      42,
		TorrentNumSeeds:        7,
		TorrentNumLeechs:       2,
	}, false)

	require.Equal(t, "种子已完成", title)
	require.Contains(t, message, "进度: 1.0000")
	require.Contains(t, message, "分享率: 1.5000")
	require.Contains(t, message, "总大小（字节）: 123")
	require.Contains(t, message, "已下载（字节）: 123")
	require.Contains(t, message, "剩余（字节）: 0")
	require.Contains(t, message, "下载速度 (bps): 0")
	require.Contains(t, message, "上传速度 (bps): 42")
	require.Contains(t, message, "做种: 7")
	require.Contains(t, message, "下载: 2")
}

func TestFormatEventAutomationsActionsAppliedMergesSamplesOutsideNotifiarrAPI(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	title, message := svc.formatEvent(context.Background(), Event{
		Type: EventAutomationsActionsApplied,
		Message: "生效种子: 1\n" +
			"标签: +no_hl=1\n" +
			"标签样本: Godzilla.Minus.One.2023.Hybrid.1080p.BluRay.DUAL.DDP7.1.x264-ZoroSenpai.mkv; Mercy.2026.720p.AMZN.WEB-DL.DDP5.1.Atmos.H.264-BYNDR\n" +
			"影响种子:\n" +
			"――――――――――――――――\n" +
			"🏷️ 更新标签\n" +
			"  种子: Hamnet.2025.Hybrid.1080p.BluRay.DDP7.1.x264-ZoroSenpai.mkv",
	}, true)

	require.Equal(t, "自动化操作已应用", title)
	require.Contains(t, message, "标签样本: Godzilla.Minus.One.2023.Hybrid.1080p.BluRay.DUAL.DDP7.1.x264-ZoroSenpai.mkv; Mercy.2026.720p.AMZN.WEB-DL.DDP5.1.Atmos.H.264-BYNDR")
	require.Contains(t, message, "影响种子:")
	require.Contains(t, message, "种子: Hamnet.2025.Hybrid.1080p.BluRay.DDP7.1.x264-ZoroSenpai.mkv")
	require.NotContains(t, message, "\n样本:")
}

func TestFormatEventAutomationsActionsAppliedKeepsSamplesForNotifiarrAPI(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	title, message := svc.formatEvent(context.Background(), Event{
		Type: EventAutomationsActionsApplied,
		Message: "生效种子: 1\n" +
			"标签: +no_hl=1\n" +
			"标签样本: Hamnet.2025.720p.Blu-ray.DD5.1.x264-TRT\n" +
			"影响种子:\n" +
			"――――――――――――――――\n" +
			"🏷️ 更新标签\n" +
			"  种子: Hamnet.2025.720p.Blu-ray.DD5.1.x264-TRT",
	}, false)

	require.Equal(t, "自动化操作已应用", title)
	require.Contains(t, message, "标签样本: Hamnet.2025.720p.Blu-ray.DD5.1.x264-TRT")
	require.Contains(t, message, "影响种子:")
	require.Contains(t, message, "🏷️ 更新标签")
	require.Contains(t, message, "种子: Hamnet.2025.720p.Blu-ray.DD5.1.x264-TRT")
}
