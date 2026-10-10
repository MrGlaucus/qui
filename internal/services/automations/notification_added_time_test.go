// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package automations

import (
	"context"
	"testing"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/pkg/timeutil"
)

func TestAutomationSampleAddedTime(t *testing.T) {
	t.Parallel()

	addedOn := time.Date(2026, 10, 9, 23, 30, 15, 0, time.UTC).Unix()
	for _, tt := range []struct {
		name    string
		lang    string
		addedOn int64
		loc     *time.Location
		want    string
	}{
		{"Chinese", "zh-CN", addedOn, time.FixedZone("UTC+8", 8*60*60), "- 添加时间: 2026-10-10 07:30:15 +08:00"},
		{"English", "en", addedOn, time.UTC, "- Added at: 2026-10-09 23:30:15 +00:00"},
		{"missing", "zh", 0, time.UTC, "- 添加时间: —"},
		{"invalid", "en", -1, time.UTC, "- Added at: —"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sample := sampleFromTorrent(models.ActivityActionDeletedCondition, qbt.Torrent{
				Name: "Synthetic notification sample", AddedOn: tt.addedOn,
			})
			require.Equal(t, tt.addedOn, sample.addedOn)
			require.Contains(t, sample.render(tt.lang, tt.loc), tt.want)
		})
	}

	bare := automationSampleTorrent{name: "Name-only sample"}
	require.NotContains(t, bare.render("en", time.UTC), "Added at")
}

func TestAutomationNotificationUsesCurrentTimezone(t *testing.T) {
	t.Parallel()

	provider := timeutil.NewProvider()
	provider.Set(time.UTC)
	notifier := &automationRecordingNotifier{}
	svc := &Service{notifier: notifier}
	svc.SetTimezone(provider)
	svc.SetLanguage(func() string { return "en" })
	ruleID := 1
	summary := newAutomationSummary()
	summary.addTorrentSamples([]automationSampleTorrent{sampleFromTorrent(models.ActivityActionPaused, qbt.Torrent{
		Name: "Synthetic notification sample", AddedOn: time.Date(2026, 10, 9, 23, 30, 15, 0, time.UTC).Unix(),
	})}, 3)
	summary.recordActivity(&models.AutomationActivity{
		RuleID: &ruleID, Action: models.ActivityActionPaused, Outcome: models.ActivityOutcomeSuccess,
	}, 1)
	rules := []*models.Automation{{ID: ruleID, Notify: true}}
	svc.notifyAutomationSummary(context.Background(), 1, summary, rules)
	provider.Set(time.FixedZone("UTC+8", 8*60*60))
	svc.notifyAutomationSummary(context.Background(), 1, summary, rules)

	events := notifier.Events()
	require.Len(t, events, 2)
	require.Contains(t, events[0].Message, "- Added at: 2026-10-09 23:30:15 +00:00")
	require.Contains(t, events[1].Message, "- Added at: 2026-10-10 07:30:15 +08:00")
}
