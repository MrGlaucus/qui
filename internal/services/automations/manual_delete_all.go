// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package automations

import (
	"fmt"
	"path"
	"strings"

	qbt "github.com/autobrr/go-qbittorrent"
)

// PlanIncludingCrossSeeds expands a manual selection through shared file paths
// within one client. Every newly included torrent is checked too: deleting a pack
// can affect another torrent that shares only one of its files.
func PlanIncludingCrossSeeds(torrents []qbt.Torrent, hashes []string, fetchFiles func([]string) (map[string]qbt.TorrentFiles, error)) (targets []qbt.Torrent, selectedCount int, err error) {
	byHash := make(map[string]qbt.Torrent, len(torrents))
	for _, torrent := range torrents {
		for _, hash := range []string{torrent.Hash, torrent.InfohashV1, torrent.InfohashV2} {
			if hash != "" {
				byHash[strings.ToLower(hash)] = torrent
			}
		}
	}
	included := make(map[string]bool)
	for _, hash := range hashes {
		torrent, ok := byHash[strings.ToLower(hash)]
		if !ok {
			return nil, 0, fmt.Errorf("selected torrent no longer exists: %s", hash)
		}
		if !included[torrent.Hash] {
			included[torrent.Hash] = true
			targets = append(targets, torrent)
		}
	}
	selectedCount = len(targets)
	files := make(map[string]map[string]struct{})
	for i := 0; i < len(targets); i++ {
		target := targets[i]
		root := manualDeleteRoot(target)
		if root == "" {
			return nil, 0, fmt.Errorf("missing file location for torrent %s", target.Hash)
		}
		candidates := []qbt.Torrent{target}
		for _, other := range torrents {
			if included[other.Hash] {
				continue
			}
			otherRoot := manualDeleteRoot(other)
			if otherRoot == "" || root == otherRoot || strings.HasPrefix(root, strings.TrimRight(otherRoot, "/")+"/") || strings.HasPrefix(otherRoot, strings.TrimRight(root, "/")+"/") {
				candidates = append(candidates, other)
			}
		}
		var missing []string
		for _, candidate := range candidates {
			if files[candidate.Hash] == nil {
				missing = append(missing, candidate.Hash)
			}
		}
		if len(missing) > 0 {
			loaded, loadErr := fetchFiles(missing)
			if loadErr != nil {
				return nil, 0, loadErr
			}
			for _, candidate := range candidates {
				if files[candidate.Hash] != nil {
					continue
				}
				list := loaded[candidate.Hash]
				if len(list) == 0 || candidate.SavePath == "" {
					return nil, 0, fmt.Errorf("cannot verify files for torrent %s", candidate.Hash)
				}
				paths := make(map[string]struct{}, len(list))
				for _, file := range list {
					name := strings.ReplaceAll(file.Name, `\`, "/")
					clean := path.Clean(name)
					if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
						return nil, 0, fmt.Errorf("unresolvable file path for torrent %s", candidate.Hash)
					}
					paths[manualDeletePath(strings.TrimRight(candidate.SavePath, `/\`)+"/"+name)] = struct{}{}
				}
				files[candidate.Hash] = paths
			}
		}
		for _, candidate := range candidates[1:] {
			for file := range files[candidate.Hash] {
				if _, shared := files[target.Hash][file]; shared {
					included[candidate.Hash] = true
					targets = append(targets, candidate)
					break
				}
			}
		}
	}
	return targets, selectedCount, nil
}

// These paths belong to the remote qBittorrent host, not the QUI host.
func manualDeletePath(value string) string {
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, `\`, "/")
	if strings.HasPrefix(value, "//") || (len(value) >= 2 && value[1] == ':') {
		value = strings.ToLower(value)
	}
	return path.Clean(value)
}

func manualDeleteRoot(torrent qbt.Torrent) string {
	if torrent.ContentPath != "" {
		return manualDeletePath(torrent.ContentPath)
	}
	return manualDeletePath(torrent.SavePath)
}
