// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package qbittorrent

import (
	"context"
	"strings"
	"sync"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/pkg/timeutil"
	"github.com/rs/zerolog/log"
)

type trackerTrafficSampleStore interface {
	RecordSamples(context.Context, int, string, []models.TrackerTrafficSample) error
	DeleteCheckpoints(context.Context, int, []string) error
}

func (r *TrackerTrafficRecorder) Remove(instanceID int, hashes []string) {
	if r == nil || r.store == nil || len(hashes) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.store.DeleteCheckpoints(context.Background(), instanceID, hashes); err != nil {
		log.Error().Err(err).Int("instanceID", instanceID).Msg("Failed to remove tracker traffic checkpoints")
	}
	for _, hash := range hashes {
		delete(r.observed[instanceID], strings.ToUpper(strings.TrimSpace(hash)))
	}
}

// TrackerTrafficRecorder turns per-torrent cumulative counters into persistent
// positive deltas attributed to tracker groups.
type TrackerTrafficRecorder struct {
	store    trackerTrafficSampleStore
	now      func() time.Time
	mu       sync.Mutex
	observed map[int]map[string]trackerTrafficObservation
}

type trackerTrafficObservation struct {
	key                  string
	uploaded, downloaded int64
}

func NewTrackerTrafficRecorder(store *models.TrackerTrafficStore, provider *timeutil.Provider) *TrackerTrafficRecorder {
	r := &TrackerTrafficRecorder{
		store: store, now: time.Now,
		observed: make(map[int]map[string]trackerTrafficObservation),
	}
	if provider != nil {
		r.now = provider.Now
	}
	return r
}

func (r *TrackerTrafficRecorder) Record(ctx context.Context, instanceID int, torrents []qbt.Torrent, completeSnapshot bool) {
	if r == nil || r.store == nil || (len(torrents) == 0 && !completeSnapshot) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	previous := r.observed[instanceID]
	if previous == nil {
		previous = make(map[string]trackerTrafficObservation)
	}
	next := make(map[string]trackerTrafficObservation, len(previous)+len(torrents))
	if !completeSnapshot {
		for hash, observation := range previous {
			next[hash] = observation
		}
	}
	samples := make([]models.TrackerTrafficSample, 0, len(torrents))
	for _, torrent := range torrents {
		hash := strings.ToUpper(strings.TrimSpace(torrent.Hash))
		if hash == "" {
			continue
		}
		domain := ExtractDomainFromURL(torrent.Tracker)
		key := ""
		if domain != "" && domain != "unknown" {
			key = "domain:" + domain
		}
		if key == "" {
			key = previous[hash].key
		}
		observation := trackerTrafficObservation{key: key, uploaded: torrent.Uploaded, downloaded: torrent.Downloaded}
		next[hash] = observation
		if old, ok := previous[hash]; ok && old == observation {
			continue
		}
		samples = append(samples, models.TrackerTrafficSample{
			Hash: hash, TrackerKey: key,
			Uploaded: torrent.Uploaded, Downloaded: torrent.Downloaded,
		})
	}
	if err := r.store.RecordSamples(ctx, instanceID, r.now().Format("2006-01-02"), samples); err != nil {
		log.Error().Err(err).Int("instanceID", instanceID).Msg("Failed to record tracker traffic")
		return
	}
	if completeSnapshot {
		removed := make([]string, 0)
		for hash := range previous {
			if _, ok := next[hash]; !ok {
				removed = append(removed, hash)
			}
		}
		if err := r.store.DeleteCheckpoints(ctx, instanceID, removed); err != nil {
			log.Error().Err(err).Int("instanceID", instanceID).Msg("Failed to remove tracker traffic checkpoints")
			return
		}
	}
	r.observed[instanceID] = next
}
