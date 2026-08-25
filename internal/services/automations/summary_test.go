// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package automations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/autobrr/qui/internal/models"
)

func TestAutomationSummaryMessageShowsFailureCountOnce(t *testing.T) {
	t.Parallel()

	summary := newAutomationSummary()
	summary.failed = 1
	summary.failedByAction[models.ActivityActionDeleteFailed] = 1

	msg := summary.message("zh")
	require.Equal(t, 1, strings.Count(msg, "失败: 1"))
	require.Contains(t, msg, "生效种子: 0")
}

func TestBuildAutomationRuleSummariesGroupsActionsByRule(t *testing.T) {
	t.Parallel()

	summary := newAutomationSummary()

	ruleIDRatio := 12
	ruleIDTagger := 13

	summary.recordActivity(&models.AutomationActivity{
		RuleID:   &ruleIDRatio,
		RuleName: "Ratio rule",
		Action:   models.ActivityActionDeletedRatio,
		Outcome:  models.ActivityOutcomeSuccess,
	}, 2)

	summary.recordActivity(&models.AutomationActivity{
		RuleID:   &ruleIDRatio,
		RuleName: "Ratio rule",
		Action:   models.ActivityActionDeleteFailed,
		Outcome:  models.ActivityOutcomeFailed,
		Reason:   "permission denied",
	}, 1)

	summary.recordActivity(&models.AutomationActivity{
		RuleID:   &ruleIDTagger,
		RuleName: "Tagger",
		Action:   models.ActivityActionTagsChanged,
		Outcome:  models.ActivityOutcomeSuccess,
	}, 2)

	got := buildAutomationRuleSummaries(summary, "zh")
	require.Len(t, got, 2)

	var ratioRuleFound bool
	for _, rule := range got {
		if rule.RuleID != ruleIDRatio {
			continue
		}
		ratioRuleFound = true
		require.Equal(t, "Ratio rule", rule.RuleName)
		require.Equal(t, 2, rule.Applied)
		require.Equal(t, 1, rule.Failed)
		require.Len(t, rule.Actions, 2)

		actions := make(map[string]struct {
			label   string
			applied int
			failed  int
		}, len(rule.Actions))
		for _, action := range rule.Actions {
			actions[action.Action] = struct {
				label   string
				applied int
				failed  int
			}{
				label:   action.Label,
				applied: action.Applied,
				failed:  action.Failed,
			}
		}

		require.Equal(t, "删除种子（分享率规则）", actions[models.ActivityActionDeletedRatio].label)
		require.Equal(t, 2, actions[models.ActivityActionDeletedRatio].applied)
		require.Equal(t, 0, actions[models.ActivityActionDeletedRatio].failed)

		require.Equal(t, "删除失败", actions[models.ActivityActionDeleteFailed].label)
		require.Equal(t, 0, actions[models.ActivityActionDeleteFailed].applied)
		require.Equal(t, 1, actions[models.ActivityActionDeleteFailed].failed)
	}
	require.True(t, ratioRuleFound)
}

func TestBuildAutomationRuleSummariesUsesRuleIDFallbackWhenNameMissing(t *testing.T) {
	t.Parallel()

	summary := newAutomationSummary()
	ruleID := 99

	summary.recordActivity(&models.AutomationActivity{
		RuleID:  &ruleID,
		Action:  models.ActivityActionTagsChanged,
		Outcome: models.ActivityOutcomeSuccess,
	}, 1)

	msg := summary.message("zh")
	require.Contains(t, msg, "规则: Rule #99")
	require.NotContains(t, msg, "Unknown rule")

	got := buildAutomationRuleSummaries(summary, "zh")
	require.Len(t, got, 1)
	require.Equal(t, 99, got[0].RuleID)
	require.Equal(t, "Rule #99", got[0].RuleName)
}

func TestAutomationSummaryMessageIncludesTagDetailsAndSamples(t *testing.T) {
	t.Parallel()

	summary := newAutomationSummary()
	summary.applied = 3
	summary.addTagCounts(
		map[string]int{"freeleech": 2},
		map[string]int{"temp": 1},
	)
	summary.addTagSamples([]string{"Torrent B", "Torrent A", "Torrent A"}, 3)

	msg := summary.message("zh")
	require.Contains(t, msg, "标签: +freeleech=2; -temp=1")
	require.Contains(t, msg, "标签样本:")
	require.Contains(t, msg, "Torrent A")
	require.Contains(t, msg, "Torrent B")
}

func TestAutomationSummaryMessageIncludesSamplesForNonDeleteActions(t *testing.T) {
	t.Parallel()

	summary := newAutomationSummary()
	summary.recordActivity(&models.AutomationActivity{
		Action:      models.ActivityActionMoved,
		Outcome:     models.ActivityOutcomeSuccess,
		TorrentName: "Some.Release.2026",
	}, 1)

	msg := summary.message("zh")
	require.Contains(t, msg, "影响种子:")
	require.Contains(t, msg, "Some.Release.2026")
}

func TestAutomationSummaryAddTorrentSamplesUsesLimitAndDedupes(t *testing.T) {
	t.Parallel()

	summary := newAutomationSummary()
	summary.addTorrentSamples([]automationSampleTorrent{
		{action: models.ActivityActionMoved, name: "Torrent C", ratio: -1},
		{action: models.ActivityActionMoved, name: "Torrent A", ratio: -1},
		{action: models.ActivityActionMoved, name: "Torrent A", ratio: -1},
		{action: models.ActivityActionMoved, name: "Torrent B", ratio: -1},
	}, 3)

	msg := summary.message("zh")
	require.Contains(t, msg, "影响种子:")
	require.Contains(t, msg, "Torrent A")
	require.Contains(t, msg, "Torrent B")
	require.Contains(t, msg, "Torrent C")
}

func TestAutomationSummaryMessageRendersRichTorrentSamples(t *testing.T) {
	t.Parallel()

	summary := newAutomationSummary()
	summary.recordActivity(&models.AutomationActivity{
		Action:  models.ActivityActionSpeedLimitsChanged,
		Outcome: models.ActivityOutcomeSuccess,
	}, 1)
	summary.addTorrentSamples([]automationSampleTorrent{
		{
			action:        models.ActivityActionSpeedLimitsChanged,
			name:          "Some.Release.2026.2160p.WEB-DL.H.265",
			hash:          "0123456789abcdef",
			sizeBytes:     42_370_000_000,
			ratio:         4.4,
			category:      "Movies",
			trackerDomain: "tracker.example.net",
			state:         "uploading",
			upSpeedBps:    38_600_000,
			downSpeedBps:  0,
			hasFullData:   true,
		},
	}, 3)

	msg := summary.message("zh")
	require.Contains(t, msg, "生效种子: 1")
	require.Contains(t, msg, "影响种子:")
	require.Contains(t, msg, "- 更新限速")
	require.Contains(t, msg, "- 种子: Some.Release.2026.2160p.WEB-DL.H.265 (01234567)")
	require.Contains(t, msg, "- 大小: 42.37 GiB")
	require.Contains(t, msg, "- 分享率: 4.40")
	require.Contains(t, msg, "- 流量: ↑ 0 B / ↓ 0 B")
	require.Contains(t, msg, "- 速度: ↑ 38.60 MB/s / ↓ 0 B/s")
	require.Contains(t, msg, "- 分类: Movies")
	require.Contains(t, msg, "- 状态: uploading")
	require.Contains(t, msg, "- 站点: tracker.example.net")
	require.NotContains(t, msg, "成功操作")
	require.NotContains(t, msg, "失败操作")
}

func TestAutomationSummaryMessageRendersEnglish(t *testing.T) {
	t.Parallel()

	summary := newAutomationSummary()
	summary.recordActivity(&models.AutomationActivity{
		Action:  models.ActivityActionSpeedLimitsChanged,
		Outcome: models.ActivityOutcomeSuccess,
	}, 1)
	summary.addTorrentSamples([]automationSampleTorrent{
		{
			action:        models.ActivityActionSpeedLimitsChanged,
			name:          "Some.Release.2026",
			hash:          "0123456789abcdef",
			sizeBytes:     42_370_000_000,
			ratio:         4.4,
			category:      "Movies",
			trackerDomain: "tracker.example.net",
			state:         "uploading",
			upSpeedBps:    38_600_000,
			downSpeedBps:  0,
			hasFullData:   true,
		},
	}, 3)

	msg := summary.message("en")
	require.Contains(t, msg, "Affected torrents: 1")
	require.Contains(t, msg, "Affected torrents:")
	require.Contains(t, msg, "- Speed limits updated")
	require.Contains(t, msg, "- Torrent: Some.Release.2026 (01234567)")
	require.Contains(t, msg, "- Size: 42.37 GiB")
	require.Contains(t, msg, "- Ratio: 4.40")
	require.Contains(t, msg, "- Traffic: ↑ 0 B / ↓ 0 B")
	require.Contains(t, msg, "- Speed: ↑ 38.60 MB/s / ↓ 0 B/s")
	require.Contains(t, msg, "- Category: Movies")
	require.Contains(t, msg, "- State: uploading")
	require.Contains(t, msg, "- Tracker: tracker.example.net")
	require.NotContains(t, msg, "生效种子")
	require.NotContains(t, msg, "影响种子")
}

func TestAutomationSummaryMessageOmitsFailureCountWhenNone(t *testing.T) {
	t.Parallel()

	summary := newAutomationSummary()
	summary.recordActivity(&models.AutomationActivity{
		Action:  models.ActivityActionSpeedLimitsChanged,
		Outcome: models.ActivityOutcomeSuccess,
	}, 1)
	// A zero failure entry must not render a "失败: 0" line.
	summary.failedByAction[models.ActivityActionSpeedLimitsChanged] = 0

	msg := summary.message("zh")
	require.Contains(t, msg, "生效种子: 1")
	require.NotContains(t, msg, "失败:")
}

func TestSampleFromTorrentCapturesRichFields(t *testing.T) {
	t.Parallel()

	sample := sampleFromTorrent(models.ActivityActionDeletedCondition, qbt.Torrent{
		Hash:       "abcdef0123456789",
		Name:       "My.Neighbor.Totoro.1988.1080p.NF.WEB-DL.H264.DDP2.0-HHWEB",
		Tracker:    "https://tracker.hhanclub.net/announce",
		Size:       42_370_000_000,
		Ratio:      4.4,
		Category:   "Movies",
		State:      qbt.TorrentStateUploading,
		Uploaded:   92_810_000_000,
		Downloaded: 21_070_000_000,
		UpSpeed:    38_600_000,
		DlSpeed:    0,
	})

	summary := newAutomationSummary()
	summary.recordActivity(&models.AutomationActivity{
		Action:  models.ActivityActionDeletedCondition,
		Outcome: models.ActivityOutcomeSuccess,
	}, 1)
	summary.addTorrentSamples([]automationSampleTorrent{sample}, 3)

	msg := summary.message("zh")
	require.Contains(t, msg, "- 删除种子（规则）")
	require.Contains(t, msg, "- 种子: My.Neighbor.Totoro.1988.1080p.NF.WEB-DL.H264.DDP2.0-HHWEB (abcdef01)")
	require.Contains(t, msg, "- 大小: 42.37 GiB")
	require.Contains(t, msg, "- 分享率: 4.40")
	require.Contains(t, msg, "- 流量: ↑ 92.81 GiB / ↓ 21.07 GiB")
	require.Contains(t, msg, "- 分类: Movies")
	require.Contains(t, msg, "- 状态: uploading")
	require.Contains(t, msg, "- 速度: ↑ 38.60 MB/s / ↓ 0 B/s")
	require.Contains(t, msg, "- 站点: tracker.hhanclub.net")
}

func TestAutomationSummaryMessageShowsZeroSpeedLine(t *testing.T) {
	t.Parallel()

	// A stalled/quiet torrent has zero speed both ways, but the notification
	// should still show the speed line so the user can confirm no activity.
	summary := newAutomationSummary()
	summary.recordActivity(&models.AutomationActivity{
		Action:  models.ActivityActionDeletedCondition,
		Outcome: models.ActivityOutcomeSuccess,
	}, 1)
	summary.addTorrentSamples([]automationSampleTorrent{
		{
			action:       models.ActivityActionDeletedCondition,
			name:         "Ballerina.2023.1080p.NF.WEB-DL",
			hash:         "a9f451e2",
			sizeBytes:    6_470_000_000,
			ratio:        2.3,
			category:     "HHCLUB-RESCUE",
			state:        "stalledUP",
			upSpeedBps:   0,
			downSpeedBps: 0,
			hasFullData:  true,
		},
	}, 3)

	msg := summary.message("zh")
	require.Contains(t, msg, "- 大小: 6.47 GiB")
	require.Contains(t, msg, "- 分享率: 2.30")
	require.Contains(t, msg, "- 流量: ↑ 0 B / ↓ 0 B")
	require.Contains(t, msg, "- 速度: ↑ 0 B/s / ↓ 0 B/s")
	require.Contains(t, msg, "- 分类: HHCLUB-RESCUE")
	require.Contains(t, msg, "- 状态: stalledUP")
	require.Contains(t, msg, "- 站点: -")
}

func TestAutomationSummaryNameOnlySampleRendersHeaderOnly(t *testing.T) {
	t.Parallel()

	// Samples built from bare activity records have no metrics; they must not
	// render fake zero fields like "分享率: -1.00".
	summary := newAutomationSummary()
	summary.addTorrentSamples([]automationSampleTorrent{
		{
			action: models.ActivityActionExportedToInstance,
			name:   "Exported.Release.2026",
			hash:   "deadbeef",
			ratio:  -1,
		},
	}, 3)

	msg := summary.message("zh")
	require.Contains(t, msg, "- 导出到实例")
	require.Contains(t, msg, "- 种子: Exported.Release.2026 (deadbeef)")
	require.NotContains(t, msg, "分享率")
	require.NotContains(t, msg, "大小")
	require.NotContains(t, msg, "速度")
}

func TestRecordMoveFailureRuleCountsContributesToNotifyGate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		automations        []*models.Automation
		ruleByHash         map[string]ruleRef
		wantNotify         bool
		wantFailedByRuleID map[int]int
	}{
		{
			name: "notify enabled",
			automations: []*models.Automation{
				{ID: 42, Notify: true},
			},
			ruleByHash: map[string]ruleRef{
				"hash-a": {id: 42, name: "Move rule"},
				"hash-b": {id: 42, name: "Move rule"},
			},
			wantNotify:         true,
			wantFailedByRuleID: map[int]int{42: 2},
		},
		{
			name: "notify disabled",
			automations: []*models.Automation{
				{ID: 42, Notify: false},
			},
			ruleByHash: map[string]ruleRef{
				"hash-a": {id: 42, name: "Move rule"},
				"hash-b": {id: 42, name: "Move rule"},
			},
			wantNotify:         false,
			wantFailedByRuleID: map[int]int{42: 2},
		},
		{
			name: "mixed rules suppress non-notifying rule",
			automations: []*models.Automation{
				{ID: 42, Notify: false},
				{ID: 43, Notify: true},
			},
			ruleByHash: map[string]ruleRef{
				"hash-a": {id: 42, name: "Suppressed rule"},
				"hash-b": {id: 43, name: "Notifying rule"},
			},
			wantNotify:         true,
			wantFailedByRuleID: map[int]int{42: 1, 43: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			summary := newAutomationSummary()
			summary.recordActivity(&models.AutomationActivity{
				Action:  models.ActivityActionMoved,
				Outcome: models.ActivityOutcomeFailed,
			}, 2)

			recordMoveFailureRuleCounts(summary, map[string][]string{
				"/library/destination": {"hash-a", "hash-b"},
			}, tt.ruleByHash)

			require.Equal(t, tt.wantNotify, shouldNotifyAutomationSummary(summary, tt.automations))

			got := buildAutomationRuleSummaries(summary, "zh")
			require.Len(t, got, len(tt.wantFailedByRuleID))
			for _, rule := range got {
				require.Equal(t, tt.wantFailedByRuleID[rule.RuleID], rule.Failed)
			}
		})
	}
}

func TestInheritRuleRefForMoveGroupIncludesExpandedMembers(t *testing.T) {
	t.Parallel()

	moveRuleByHash := map[string]ruleRef{
		"trigger-hash": {id: 77, name: "Grouped move"},
	}

	inheritRuleRefForMoveGroup("member-hash", "trigger-hash", moveRuleByHash)

	counts := buildRuleCountsFromHashes([]string{"trigger-hash", "member-hash"}, moveRuleByHash)
	require.Equal(t, 2, counts[ruleRef{id: 77, name: "Grouped move"}])
}
