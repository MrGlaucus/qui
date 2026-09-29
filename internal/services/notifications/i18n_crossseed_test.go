// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package notifications

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRSSAutomationNotificationUsesCurrentLanguageAndStructuredSummary(t *testing.T) {
	lang := "zh-CN"
	svc := &Service{}
	svc.SetLanguage(func() string { return lang })
	event := Event{
		Type:    EventCrossSeedAutomationSucceeded,
		Message: "Run: 18846\nMode: auto\nMessage: processed=16 candidates=1 added=1 skipped=15 failed=0",
		CrossSeed: &CrossSeedEventData{
			RunID:             18846,
			Mode:              "auto",
			Status:            "success",
			FeedItems:         20,
			Candidates:        1,
			Added:             1,
			Skipped:           15,
			TargetIndexerAdds: []LabelCount{{Label: "Target Site", Count: 1}},
			Samples:           []string{"Example.Release @ qbit-1"},
		},
	}

	title, message := svc.formatEvent(context.Background(), event, true)
	require.Equal(t, "RSS 辅种：已添加 1 个", title)
	require.Equal(t, "扫描结果: RSS 20 条 · 可辅种 1 条\n执行结果: 新增 1 · 跳过 15 · 失败 0\n辅种目标站点（RSS 索引器）:\n- Target Site · 新增 1 个\n来源种子（名称 @ 来源实例）:\n- Example.Release @ qbit-1\n运行记录: #18846", message)
	require.NotContains(t, message, "processed=")
	require.NotContains(t, message, "Mode:")

	lang = "en"
	title, message = svc.formatEvent(context.Background(), event, true)
	require.Equal(t, "RSS cross-seed: 1 added", title)
	require.Equal(t, "Scan: 20 RSS items · 1 matches\nResult: 1 added · 15 skipped · 0 failed\nCross-seed target sites (RSS indexers):\n- Target Site · 1 added\nSource torrents (name @ source instance):\n- Example.Release @ qbit-1\nRun: #18846", message)
}

func TestRSSAutomationNotificationPartialAndFailed(t *testing.T) {
	for _, tt := range []struct {
		name      string
		status    string
		wantTitle string
	}{
		{name: "partial", status: "partial", wantTitle: "RSS 辅种：部分完成"},
		{name: "failed", status: "failed", wantTitle: "RSS 辅种失败"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{}
			event := Event{
				Type:         EventCrossSeedAutomationFailed,
				ErrorMessage: "tracker unavailable",
				CrossSeed: &CrossSeedEventData{
					Status: tt.status,
					Failed: 2,
				},
			}
			title, message := svc.formatEvent(context.Background(), event, true)
			require.Equal(t, tt.wantTitle, title)
			require.Contains(t, message, "失败 2")
			require.Contains(t, message, "失败原因: tracker unavailable")
		})
	}
}
