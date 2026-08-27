package qbittorrent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

// TorrentTrackerAnnounce is the raw qBittorrent torrents/trackers payload with
// the announce timing fields introduced in qBittorrent 5.2.0 (WebAPI 2.13.0,
// PR #23045). next_announce and min_announce are Unix timestamps in seconds.
type TorrentTrackerAnnounce struct {
	Url           string `json:"url"`
	Status        int    `json:"status"`
	NumPeers      int    `json:"num_peers"`
	NumSeeds      int    `json:"num_seeds"`
	NumLeeches    int    `json:"num_leeches"`
	NumDownloaded int    `json:"num_downloaded"`
	Message       string `json:"msg"`
	NextAnnounce  int64  `json:"next_announce"`
	MinAnnounce   int64  `json:"min_announce"`
}

// TorrentTrackerView is the display representation of a torrent tracker, merging
// the base tracker info with announce timing where available.
type TorrentTrackerView struct {
	Url           string `json:"url"`
	Status        int    `json:"status"`
	NumPeers      int    `json:"num_peers"`
	NumSeeds      int    `json:"num_seeds"`
	NumLeeches    int    `json:"num_leeches"`
	NumDownloaded int    `json:"num_downloaded"`
	Message       string `json:"msg"`
	NextAnnounce  int64  `json:"next_announce"`
	MinAnnounce   int64  `json:"min_announce"`
}

// GetTorrentTrackersWithAnnounce fetches the trackers for a torrent directly from
// the qBittorrent WebAPI and decodes the announce timing fields. It reuses the
// underlying authenticated HTTP client, mirroring the library's own request
// authentication (cookie jar for session auth, Basic auth header, or Bearer API
// key header).
func (c *Client) GetTorrentTrackersWithAnnounce(ctx context.Context, hash string) ([]TorrentTrackerAnnounce, error) {
	base := strings.TrimRight(c.host, "/") + "/api/v2/torrents/trackers"
	reqURL := base + "?hash=" + url.QueryEscape(hash)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build trackers request: %w", err)
	}
	if c.basicUser != "" && c.basicPass != "" {
		req.SetBasicAuth(c.basicUser, c.basicPass)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch torrent trackers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch torrent trackers: unexpected status %d", resp.StatusCode)
	}

	var out []TorrentTrackerAnnounce
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode torrent trackers: %w", err)
	}
	return out, nil
}

// GetTorrentTrackersView returns the display representation of a torrent's
// trackers, including announce timing when the connected qBittorrent version
// exposes it (5.2.0+). Trackers whose announce timing cannot be retrieved keep
// the zero values for NextAnnounce/MinAnnounce, and a transient failure to fetch
// announce times never breaks the base tracker listing.
func (sm *SyncManager) GetTorrentTrackersView(ctx context.Context, instanceID int, hash string) ([]TorrentTrackerView, error) {
	client, err := sm.clientPool.GetClient(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	base, err := client.GetTorrentTrackersCtx(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrent trackers: %w", err)
	}

	view := make([]TorrentTrackerView, 0, len(base))
	for _, t := range base {
		view = append(view, TorrentTrackerView{
			Url:           t.Url,
			Status:        int(t.Status),
			NumPeers:      t.NumPeers,
			NumSeeds:      t.NumSeeds,
			NumLeeches:    t.NumLeechers,
			NumDownloaded: t.NumDownloaded,
			Message:       t.Message,
		})
	}

	announce, err := client.GetTorrentTrackersWithAnnounce(ctx, hash)
	if err != nil {
		log.Warn().Err(err).Int("instanceID", instanceID).Str("hash", hash).Msg("Failed to fetch tracker announce times")
		return view, nil
	}

	nextByURL := make(map[string]int64, len(announce))
	minByURL := make(map[string]int64, len(announce))
	for _, a := range announce {
		nextByURL[a.Url] = a.NextAnnounce
		minByURL[a.Url] = a.MinAnnounce
	}
	for i := range view {
		if next, ok := nextByURL[view[i].Url]; ok {
			view[i].NextAnnounce = next
		}
		if min, ok := minByURL[view[i].Url]; ok {
			view[i].MinAnnounce = min
		}
	}

	return view, nil
}
