// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package qbittorrent

import (
	"context"
	"testing"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/autobrr/qui/internal/models"
	"github.com/stretchr/testify/require"
)

type fakeTrackerTrafficStore struct {
	batches [][]models.TrackerTrafficSample
	removed [][]string
}

func (f *fakeTrackerTrafficStore) RecordSamples(_ context.Context, _ int, _ string, samples []models.TrackerTrafficSample) error {
	if len(samples) > 0 {
		f.batches = append(f.batches, append([]models.TrackerTrafficSample(nil), samples...))
	}
	return nil
}

func (f *fakeTrackerTrafficStore) DeleteCheckpoints(_ context.Context, _ int, hashes []string) error {
	if len(hashes) > 0 {
		f.removed = append(f.removed, append([]string(nil), hashes...))
	}
	return nil
}

func TestTrackerTrafficRecorderFiltersUnchangedSnapshots(t *testing.T) {
	store := &fakeTrackerTrafficStore{}
	recorder := &TrackerTrafficRecorder{
		store:    store,
		now:      time.Now,
		observed: make(map[int]map[string]trackerTrafficObservation),
	}
	torrents := []qbt.Torrent{
		{Hash: "a", Tracker: "https://one.example/announce", Uploaded: 10},
		{Hash: "b", Tracker: "https://two.example/announce", Downloaded: 20},
	}
	recorder.Record(context.Background(), 1, torrents, true)
	require.Len(t, store.batches, 1)
	keys := []string{store.batches[0][0].TrackerKey, store.batches[0][1].TrackerKey}
	require.ElementsMatch(t, []string{"domain:one.example", "domain:two.example"}, keys)

	recorder.Record(context.Background(), 1, torrents, true)
	require.Len(t, store.batches, 1, "an unchanged full snapshot must not write checkpoints again")

	torrents[0].Uploaded = 15
	recorder.Record(context.Background(), 1, torrents[:1], true)
	require.Len(t, store.batches, 2)
	require.Equal(t, int64(15), store.batches[1][0].Uploaded)
	require.Equal(t, []string{"B"}, store.removed[0])
}

func TestTrackerTrafficRecorderTargetedSettlementDoesNotRemoveOtherHashes(t *testing.T) {
	store := &fakeTrackerTrafficStore{}
	recorder := &TrackerTrafficRecorder{store: store, now: time.Now, observed: make(map[int]map[string]trackerTrafficObservation)}
	recorder.Record(context.Background(), 1, []qbt.Torrent{
		{Hash: "a", Tracker: "https://one.example/announce", Uploaded: 10},
		{Hash: "b", Tracker: "https://two.example/announce", Uploaded: 20},
	}, true)
	recorder.Record(context.Background(), 1, []qbt.Torrent{
		{Hash: "a", Tracker: "https://one.example/announce", Uploaded: 11},
	}, false)
	require.Empty(t, store.removed)
	require.Contains(t, recorder.observed[1], "B")
}
