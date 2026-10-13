package automations

import (
	"fmt"
	"strings"

	qbt "github.com/autobrr/go-qbittorrent"
)

// PlanPreservingCrossSeeds applies the workflow file-overlap policy to a whole
// manual selection. Selected peers do not require files after this deletion.
func PlanPreservingCrossSeeds(torrents []qbt.Torrent, hashes []string, fetchFiles func([]string) (map[string]qbt.TorrentFiles, error)) (keepFiles, deleteFiles []string, err error) {
	byHash := make(map[string]qbt.Torrent, len(torrents))
	for _, torrent := range torrents {
		for _, hash := range []string{torrent.Hash, torrent.InfohashV1, torrent.InfohashV2} {
			if hash != "" {
				byHash[strings.ToLower(hash)] = torrent
			}
		}
	}
	selected := make(map[string]struct{}, len(hashes))
	var targets []qbt.Torrent
	for _, hash := range hashes {
		torrent, ok := byHash[strings.ToLower(hash)]
		if !ok {
			return nil, nil, fmt.Errorf("selected torrent no longer exists: %s", hash)
		}
		if _, exists := selected[torrent.Hash]; exists {
			continue
		}
		selected[torrent.Hash] = struct{}{}
		targets = append(targets, torrent)
	}
	var remaining []qbt.Torrent
	for _, torrent := range torrents {
		if _, deleting := selected[torrent.Hash]; !deleting {
			remaining = append(remaining, torrent)
		}
	}
	index := buildContentPathIndex(remaining)
	for _, target := range targets {
		// Missing paths cannot prove that deleting files is safe.
		if normalizePath(target.ContentPath) == "" {
			keepFiles = append(keepFiles, target.Hash)
			continue
		}
		members, verified := crossSeedGroupMembers(target, findCrossSeedGroup(target, index), nil, fetchFiles)
		if !verified || len(members) > 1 {
			keepFiles = append(keepFiles, target.Hash)
		} else {
			deleteFiles = append(deleteFiles, target.Hash)
		}
	}
	return keepFiles, deleteFiles, nil
}
