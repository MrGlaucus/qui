// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package automations

import (
	"testing"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/models"
)

func TestAutomationNotificationRatios(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		torrent   qbt.Torrent
		wantRatio string
		wantSize  string
	}{
		{
			name:      "use full size instead of selected size",
			torrent:   qbt.Torrent{Uploaded: 1000, Downloaded: 400, Ratio: 2.5, Size: 200, TotalSize: 800},
			wantRatio: "2.50", wantSize: "1.25",
		},
		{
			name:      "cross-seeded torrent",
			torrent:   qbt.Torrent{Uploaded: 1000, Ratio: -1, TotalSize: 800},
			wantRatio: "∞", wantSize: "1.25",
		},
		{
			name:      "no uploaded bytes",
			torrent:   qbt.Torrent{TotalSize: 800},
			wantRatio: "0.00", wantSize: "0.00",
		},
		{
			name:      "full size unavailable",
			torrent:   qbt.Torrent{Uploaded: 1000, Downloaded: 400, Ratio: 2.5, Size: 200},
			wantRatio: "2.50", wantSize: "—",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sample := sampleFromTorrent(models.ActivityActionPaused, tt.torrent)
			zh := sample.render("zh", time.UTC)
			require.Contains(t, zh, "- 分享率（上传量/下载量）: "+tt.wantRatio+"\n")
			require.Contains(t, zh, "- 分享率（上传量/种子大小）: "+tt.wantSize+"\n")
			en := sample.render("en", time.UTC)
			require.Contains(t, en, "- Ratio (uploaded/downloaded): "+tt.wantRatio+"\n")
			require.Contains(t, en, "- Ratio (uploaded/torrent size): "+tt.wantSize+"\n")
		})
	}
}
