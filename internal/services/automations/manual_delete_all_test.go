package automations

import (
	"errors"
	"testing"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/stretchr/testify/require"
)

func TestPlanIncludingCrossSeeds(t *testing.T) {
	t.Parallel()
	torrents := []qbt.Torrent{
		{Hash: "a", InfohashV1: "alias-a", SavePath: "/data", ContentPath: "/data/pack/one.mkv"},
		{Hash: "b", SavePath: "/data", ContentPath: "/data/pack"},
		{Hash: "c", SavePath: "/data", ContentPath: "/data/pack/two.mkv"},
		{Hash: "unrelated", SavePath: "/data", ContentPath: "/data/pack"},
		{Hash: "elsewhere", SavePath: "/backup", ContentPath: "/backup/pack"},
	}
	files := map[string]qbt.TorrentFiles{
		"a":         {{Name: "pack/one.mkv", Size: 100}},
		"b":         {{Name: "pack/one.mkv", Size: 100}, {Name: "pack/two.mkv", Size: 200}},
		"c":         {{Name: "pack/two.mkv", Size: 200}},
		"unrelated": {{Name: "pack/other.mkv", Size: 100}},
	}
	t.Run("expands partial and transitive overlaps without deleting unrelated files", func(t *testing.T) {
		targets, selected, err := PlanIncludingCrossSeeds(torrents, []string{"ALIAS-A", "a"}, cachedTorrentFilesFetcher(files))
		require.NoError(t, err)
		require.Equal(t, 1, selected)
		require.Equal(t, []qbt.Torrent{torrents[0], torrents[1], torrents[2]}, targets)
	})
	t.Run("all peers already selected are not counted again", func(t *testing.T) {
		targets, selected, err := PlanIncludingCrossSeeds(torrents, []string{"a", "b", "c"}, cachedTorrentFilesFetcher(files))
		require.NoError(t, err)
		require.Equal(t, 3, selected)
		require.Len(t, targets, 3)
	})
	t.Run("missing selected task aborts", func(t *testing.T) {
		_, _, err := PlanIncludingCrossSeeds(torrents, []string{"gone"}, cachedTorrentFilesFetcher(files))
		require.Error(t, err)
	})
	t.Run("unreadable or missing candidate files abort", func(t *testing.T) {
		_, _, err := PlanIncludingCrossSeeds(torrents, []string{"a"}, func([]string) (map[string]qbt.TorrentFiles, error) { return nil, errors.New("offline") })
		require.Error(t, err)
		_, _, err = PlanIncludingCrossSeeds(torrents, []string{"a"}, cachedTorrentFilesFetcher(nil))
		require.Error(t, err)
	})
	t.Run("rechecks changed manifests", func(t *testing.T) {
		fresh := map[string]qbt.TorrentFiles{"a": files["a"], "b": files["b"], "c": files["c"], "unrelated": files["a"]}
		targets, _, err := PlanIncludingCrossSeeds(torrents, []string{"a"}, cachedTorrentFilesFetcher(fresh))
		require.NoError(t, err)
		require.Len(t, targets, 4)
	})
}

func TestPlanIncludingCrossSeedsPaths(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, first, second string
		want                int
	}{
		{"unix paths are case sensitive", "/data/Pack", "/data/pack", 1},
		{"windows paths ignore case and separators", `C:\Data\Pack`, "c:/data/pack", 2},
		{"UNC paths", `\\host\share\Pack`, "//host/share/pack", 2},
		{"prefix is not directory overlap", "/data/pack", "/data/pack-other", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			torrents := []qbt.Torrent{{Hash: "a", SavePath: tc.first, ContentPath: tc.first}, {Hash: "b", SavePath: tc.second, ContentPath: tc.second}}
			files := map[string]qbt.TorrentFiles{"a": {{Name: "one.mkv", Size: 1}}, "b": {{Name: "one.mkv", Size: 1}}}
			targets, _, err := PlanIncludingCrossSeeds(torrents, []string{"a"}, cachedTorrentFilesFetcher(files))
			require.NoError(t, err)
			require.Len(t, targets, tc.want)
		})
	}
	for _, name := range []string{"../bad", `..\bad`, "/bad", `\bad`, "C:/bad", `\\host\share`, ""} {
		t.Run(name, func(t *testing.T) {
			_, _, err := PlanIncludingCrossSeeds([]qbt.Torrent{{Hash: "a", SavePath: "/data"}}, []string{"a"}, cachedTorrentFilesFetcher(map[string]qbt.TorrentFiles{"a": {{Name: name}}}))
			require.Error(t, err)
		})
	}
}
