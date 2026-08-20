// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"errors"
	"testing"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransferOps implements torrentTransferOps for testing.
type mockTransferOps struct {
	torrents map[string]qbt.Torrent

	exportData []byte
	exportErr  error

	addResp *qbt.TorrentAddResponse
	addErr  error
	// addFailOnce makes only the first AddTorrent call fail with addErr.
	addFailOnce bool

	setCommentErr error

	bulkActionErr error

	addCalls    []transferAddCall
	bulkActions []string
	comments    []string
}

type transferAddCall struct {
	instanceID  int
	fileContent []byte
	options     map[string]string
}

func (m *mockTransferOps) GetTorrents(_ context.Context, _ int, filter qbt.TorrentFilterOptions) ([]qbt.Torrent, error) {
	torrents := make([]qbt.Torrent, 0, len(filter.Hashes))
	for _, hash := range filter.Hashes {
		if t, ok := m.torrents[hash]; ok {
			torrents = append(torrents, t)
		}
	}
	return torrents, nil
}

func (m *mockTransferOps) ExportTorrent(_ context.Context, _ int, _ string) ([]byte, string, string, error) {
	return m.exportData, "", "", m.exportErr
}

func (m *mockTransferOps) AddTorrent(_ context.Context, instanceID int, fileContent []byte, options map[string]string) (*qbt.TorrentAddResponse, error) {
	m.addCalls = append(m.addCalls, transferAddCall{instanceID: instanceID, fileContent: fileContent, options: options})
	if m.addFailOnce {
		if len(m.addCalls) == 1 {
			return nil, m.addErr
		}
		return m.addResp, nil
	}
	return m.addResp, m.addErr
}

func (m *mockTransferOps) SetComment(_ context.Context, _ int, hashes []string, _ string) error {
	m.comments = append(m.comments, hashes...)
	return m.setCommentErr
}

func (m *mockTransferOps) BulkAction(_ context.Context, _ int, hashes []string, action string) error {
	m.bulkActions = append(m.bulkActions, action)
	return m.bulkActionErr
}

func transferTestOps() *mockTransferOps {
	return &mockTransferOps{
		torrents: map[string]qbt.Torrent{
			"hash1": {Hash: "hash1", Name: "Torrent One", Category: "movies", Tags: "tag-a,tag-b", SavePath: "/data/movies", UpLimit: 100, DlLimit: 200, MaxRatio: 1.5, MaxSeedingTime: 300, Comment: "a comment"},
			"hash2": {Hash: "hash2", Name: "Torrent Two"},
		},
		exportData: []byte("fake torrent data"),
		addResp:    &qbt.TorrentAddResponse{SuccessCount: 1},
	}
}

// fullCarryOver returns every carry-over option enabled.
func fullCarryOver() TransferCarryOverOptions {
	return TransferCarryOverOptions{
		SavePath:    true,
		Category:    true,
		Tags:        true,
		ShareLimits: true,
		SpeedLimits: true,
		Comment:     true,
	}
}

func TestTransferTorrents_Success(t *testing.T) {
	ops := transferTestOps()

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"hash1", "hash2"}, fullCarryOver(), true)

	require.Len(t, resp.Results, 2)
	assert.Equal(t, 2, resp.Succeeded)
	assert.Equal(t, 0, resp.Failed)
	for _, res := range resp.Results {
		assert.True(t, res.Success)
	}

	// Source torrents (and their files) are removed once transferred.
	require.Len(t, ops.bulkActions, 1)
	assert.Equal(t, "deleteWithFiles", ops.bulkActions[0])

	// Add carried over properties to the target instance.
	require.Len(t, ops.addCalls, 2)
	addOpts := ops.addCalls[0].options
	assert.Equal(t, 2, ops.addCalls[0].instanceID)
	assert.Equal(t, "false", addOpts["autoTMM"])
	assert.Equal(t, "/data/movies", addOpts["savepath"])
	assert.Equal(t, "movies", addOpts["category"])
	assert.Equal(t, "tag-a,tag-b", addOpts["tags"])
	assert.Equal(t, "100", addOpts["upLimit"])
	assert.Equal(t, "200", addOpts["dlLimit"])
	assert.Equal(t, "1.50", addOpts["ratioLimit"])
	assert.Equal(t, "300", addOpts["seedingTimeLimit"])

	// Comment is not supported by add, applied afterwards.
	assert.Equal(t, []string{"hash1"}, ops.comments)
}

func TestTransferTorrents_NoCarryOver(t *testing.T) {
	ops := transferTestOps()

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"hash1"}, TransferCarryOverOptions{}, true)

	require.Len(t, resp.Results, 1)
	assert.True(t, resp.Results[0].Success)
	require.Len(t, ops.addCalls, 1)
	addOpts := ops.addCalls[0].options
	assert.Equal(t, "false", addOpts["autoTMM"])
	// Nothing beyond the base add options is carried over.
	assert.Len(t, addOpts, 4, "no carry-over options selected must leave only the base add options")
	assert.Empty(t, ops.comments, "comment must not be set when the option is not selected")
}

func TestTransferTorrents_SelectiveCarryOver(t *testing.T) {
	ops := transferTestOps()

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"hash1"}, TransferCarryOverOptions{
		Category: true,
		Tags:     true,
	}, true)

	require.Len(t, resp.Results, 1)
	assert.True(t, resp.Results[0].Success)
	require.Len(t, ops.addCalls, 1)
	addOpts := ops.addCalls[0].options
	assert.Equal(t, "movies", addOpts["category"])
	assert.Equal(t, "tag-a,tag-b", addOpts["tags"])
	_, hasSavePath := addOpts["savepath"]
	assert.False(t, hasSavePath, "savepath must not be carried over when not selected")
	_, hasUpLimit := addOpts["upLimit"]
	assert.False(t, hasUpLimit, "speed limits must not be carried over when not selected")
	_, hasRatio := addOpts["ratioLimit"]
	assert.False(t, hasRatio, "share limits must not be carried over when not selected")
	assert.Empty(t, ops.comments, "comment must not be carried over when not selected")
}

func TestTransferTorrents_SavePathCarriedOverWhenSelected(t *testing.T) {
	ops := transferTestOps()
	ops.torrents["hash1"] = qbt.Torrent{Hash: "hash1", Name: "Torrent One", SavePath: "/data/movies"}

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"hash1"}, TransferCarryOverOptions{SavePath: true}, true)

	require.Len(t, resp.Results, 1)
	assert.True(t, resp.Results[0].Success)
	require.Len(t, ops.addCalls, 1)
	assert.Equal(t, "/data/movies", ops.addCalls[0].options["savepath"])
}

func TestTransferTorrents_KeepsSourceFilesWhenNotDeleting(t *testing.T) {
	ops := transferTestOps()

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"hash1"}, fullCarryOver(), false)

	require.Len(t, resp.Results, 1)
	assert.True(t, resp.Results[0].Success)
	require.Len(t, ops.bulkActions, 1)
	assert.Equal(t, "delete", ops.bulkActions[0], "source files must be kept when deleteSourceFiles is false")
}

func TestTransferTorrents_TorrentNotFoundOnSource(t *testing.T) {
	ops := transferTestOps()

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"missing"}, fullCarryOver(), true)

	require.Len(t, resp.Results, 1)
	assert.False(t, resp.Results[0].Success)
	assert.Contains(t, resp.Results[0].Error, "not found")
	assert.Empty(t, ops.addCalls, "no add should happen for a missing source torrent")
	assert.Empty(t, ops.bulkActions, "no source deletion should happen when nothing was transferred")
}

func TestTransferTorrents_AddFailsKeepsSource(t *testing.T) {
	ops := transferTestOps()
	ops.addErr = errors.New("target rejected torrent")

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"hash1"}, fullCarryOver(), true)

	require.Len(t, resp.Results, 1)
	assert.False(t, resp.Results[0].Success)
	assert.Contains(t, resp.Results[0].Error, "add torrent to target")
	assert.Empty(t, ops.bulkActions, "source torrent must not be deleted when the target rejected it")
}

func TestTransferTorrents_TargetRejectsAdd(t *testing.T) {
	ops := transferTestOps()
	ops.addResp = &qbt.TorrentAddResponse{SuccessCount: 0}

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"hash1"}, fullCarryOver(), true)

	require.Len(t, resp.Results, 1)
	assert.False(t, resp.Results[0].Success)
	assert.Contains(t, resp.Results[0].Error, "did not accept")
	assert.Empty(t, ops.bulkActions)
}

func TestTransferTorrents_ExportFailsKeepsSource(t *testing.T) {
	ops := transferTestOps()
	ops.exportErr = errors.New("export failed")

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"hash1"}, fullCarryOver(), true)

	require.Len(t, resp.Results, 1)
	assert.False(t, resp.Results[0].Success)
	assert.Contains(t, resp.Results[0].Error, "export torrent")
	assert.Empty(t, ops.addCalls)
	assert.Empty(t, ops.bulkActions)
}

func TestTransferTorrents_SourceDeletionFails(t *testing.T) {
	ops := transferTestOps()
	ops.bulkActionErr = errors.New("delete failed")

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"hash1"}, fullCarryOver(), true)

	// The torrent reached the target but the source could not be removed.
	require.Len(t, resp.Results, 1)
	assert.False(t, resp.Results[0].Success)
	assert.Contains(t, resp.Results[0].Error, "failed to remove the source")
	assert.Equal(t, 0, resp.Succeeded)
	assert.Equal(t, 1, resp.Failed)
}

func TestTransferTorrents_PartialFailure(t *testing.T) {
	ops := transferTestOps()
	ops.addFailOnce = true
	ops.addErr = errors.New("target rejected torrent")

	resp := transferTorrents(context.Background(), ops, 1, 2, []string{"hash1", "hash2"}, fullCarryOver(), true)

	require.Len(t, resp.Results, 2)
	// hash1 is processed first and fails; hash2 still succeeds and triggers the source delete.
	require.Len(t, ops.addCalls, 2)
	require.Len(t, ops.bulkActions, 1)
	assert.Equal(t, "deleteWithFiles", ops.bulkActions[0])
	assert.Equal(t, 1, resp.Succeeded)
	assert.Equal(t, 1, resp.Failed)
}
