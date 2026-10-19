// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package automations

import (
	"testing"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/stretchr/testify/assert"
)

func crossSeedPair() ([]qbt.Torrent, map[string]qbt.TorrentFiles) {
	shared := qbt.TorrentFiles{{Name: "shared/movie.mkv", Size: 1_000}}
	torrents := []qbt.Torrent{
		{Hash: "aaa", SavePath: "/downloads", ContentPath: "/downloads/shared"},
		{Hash: "bbb", SavePath: "/downloads", ContentPath: "/downloads/shared"},
	}
	return torrents, map[string]qbt.TorrentFiles{"aaa": shared, "bbb": shared}
}

func TestShouldPreserveCrossSeedFiles(t *testing.T) {
	t.Parallel()

	torrents, filesByHash := crossSeedPair()
	cpIndex := buildContentPathIndex(torrents)
	trigger := torrents[0]

	tests := []struct {
		name string
		skip map[string]struct{}
		want bool
	}{
		{
			name: "sibling stays keeps files",
			skip: map[string]struct{}{"aaa": {}},
			want: true,
		},
		{
			name: "no skip set keeps files",
			want: true,
		},
		{
			name: "every member leaves this run deletes files",
			skip: map[string]struct{}{"aaa": {}, "bbb": {}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldPreserveCrossSeedFiles(trigger, cpIndex, filesByHash, tt.skip)

			assert.Equal(t, tt.want, got)
		})
	}
}

// A sibling that cannot be verified is not proof the files are safe to delete.
// The decision has to stay conservative so a broken file list never removes
// data another torrent still needs.
func TestShouldPreserveCrossSeedFiles_UnverifiableSiblingKeepsFiles(t *testing.T) {
	t.Parallel()

	torrents, _ := crossSeedPair()
	cpIndex := buildContentPathIndex(torrents)

	got := shouldPreserveCrossSeedFiles(torrents[0], cpIndex, nil, map[string]struct{}{"aaa": {}})

	assert.True(t, got)
}

// Two cross-seeds that match the same preserve rule used to protect each other:
// each one saw the other as a remaining seed, so both were removed with their
// files left on disk. They must both decide to delete files now.
func TestPreserveCrossSeeds_BothSiblingsLeaveThisRun(t *testing.T) {
	t.Parallel()

	torrents, filesByHash := crossSeedPair()
	cpIndex := buildContentPathIndex(torrents)
	states := map[string]*torrentDesiredState{
		"aaa": {hash: "aaa", shouldDelete: true, deleteMode: DeleteModeWithFilesPreserveCrossSeeds},
		"bbb": {hash: "bbb", shouldDelete: true, deleteMode: DeleteModeWithFilesPreserveCrossSeeds},
	}

	deleting := deletingHashesThisRun(states)

	for _, torrent := range torrents {
		assert.False(t, shouldPreserveCrossSeedFiles(torrent, cpIndex, filesByHash, deleting),
			"cross-seed %s must delete its files when its sibling leaves too", torrent.Hash)
	}
}

func TestDeletingHashesThisRun(t *testing.T) {
	t.Parallel()

	states := map[string]*torrentDesiredState{
		"with-files":   {hash: "with-files", shouldDelete: true, deleteMode: DeleteModeWithFiles},
		"preserve":     {hash: "preserve", shouldDelete: true, deleteMode: DeleteModeWithFilesPreserveCrossSeeds},
		"keep":         {hash: "keep", shouldDelete: true, deleteMode: DeleteModeKeepFiles},
		"keep-grouped": {hash: "keep-grouped", shouldDelete: true, deleteMode: DeleteModeKeepFiles, deleteGroupID: GroupCrossSeedContentPath},
		"include":      {hash: "include", shouldDelete: true, deleteMode: DeleteModeWithFilesIncludeCrossSeeds},
		"survivor":     {hash: "survivor"},
	}

	got := deletingHashesThisRun(states)

	// Include-cross-seeds may be skipped when group verification fails, and a
	// grouped keep-files delete may be skipped when the group does not fully
	// match. Neither is guaranteed to leave, so neither may suppress files.
	assert.Equal(t, map[string]struct{}{
		"with-files": {},
		"preserve":   {},
		"keep":       {},
	}, got)
}
