package automations

import (
	"errors"
	"testing"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/stretchr/testify/require"
)

func TestPlanPreservingCrossSeeds(t *testing.T) {
	t.Parallel()
	torrents := []qbt.Torrent{
		{Hash: "a", InfohashV1: "alias-a", ContentPath: "/data/movie", SavePath: "/data"},
		{Hash: "b", ContentPath: "/data/movie", SavePath: "/data"},
		{Hash: "c", ContentPath: "/data/other", SavePath: "/data"},
	}
	files := map[string]qbt.TorrentFiles{
		"a": {{Name: "movie/main.mkv", Size: 100}},
		"b": {{Name: "movie/main.mkv", Size: 100}},
		"c": {{Name: "other/main.mkv", Size: 100}},
	}
	for _, tc := range []struct {
		name                   string
		selected, keep, remove []string
	}{
		{"unselected peer preserves files", []string{"a", "c"}, []string{"a"}, []string{"c"}},
		{"all group members selected", []string{"a", "b", "c"}, nil, []string{"a", "b", "c"}},
		{"hybrid aliases and duplicates", []string{"ALIAS-A", "a", "b"}, nil, []string{"a", "b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keep, remove, err := PlanPreservingCrossSeeds(torrents, tc.selected, cachedTorrentFilesFetcher(files))
			require.NoError(t, err)
			require.Equal(t, tc.keep, keep)
			require.Equal(t, tc.remove, remove)
		})
	}
	t.Run("same directory different files", func(t *testing.T) {
		distinct := map[string]qbt.TorrentFiles{"a": files["a"], "b": {{Name: "movie/different.mkv", Size: 100}}}
		keep, remove, err := PlanPreservingCrossSeeds(torrents, []string{"a"}, cachedTorrentFilesFetcher(distinct))
		require.NoError(t, err)
		require.Empty(t, keep)
		require.Equal(t, []string{"a"}, remove)
	})
	t.Run("partial overlap protects remaining files", func(t *testing.T) {
		partial := map[string]qbt.TorrentFiles{"a": files["a"], "b": {{Name: "movie/main.mkv", Size: 10}, {Name: "movie/extra.mkv", Size: 90}}}
		keep, remove, err := PlanPreservingCrossSeeds(torrents, []string{"a"}, cachedTorrentFilesFetcher(partial))
		require.NoError(t, err)
		require.Equal(t, []string{"a"}, keep)
		require.Empty(t, remove)
	})
	t.Run("unreadable files preserve", func(t *testing.T) {
		keep, remove, err := PlanPreservingCrossSeeds(torrents, []string{"a"}, func([]string) (map[string]qbt.TorrentFiles, error) { return nil, errors.New("offline") })
		require.NoError(t, err)
		require.Equal(t, []string{"a"}, keep)
		require.Empty(t, remove)
	})
	t.Run("missing file lists preserve", func(t *testing.T) {
		keep, remove, err := PlanPreservingCrossSeeds(torrents, []string{"a"}, cachedTorrentFilesFetcher(nil))
		require.NoError(t, err)
		require.Equal(t, []string{"a"}, keep)
		require.Empty(t, remove)
	})
	t.Run("unknown content path preserves", func(t *testing.T) {
		keep, remove, err := PlanPreservingCrossSeeds([]qbt.Torrent{{Hash: "unknown"}}, []string{"unknown"}, cachedTorrentFilesFetcher(nil))
		require.NoError(t, err)
		require.Equal(t, []string{"unknown"}, keep)
		require.Empty(t, remove)
	})
	t.Run("missing selection aborts plan", func(t *testing.T) {
		_, _, err := PlanPreservingCrossSeeds(torrents, []string{"a", "gone"}, cachedTorrentFilesFetcher(files))
		require.Error(t, err)
	})
}
