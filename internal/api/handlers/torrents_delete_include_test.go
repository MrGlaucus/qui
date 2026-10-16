package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/qbittorrent"
	"github.com/autobrr/qui/internal/testutil/testdb"
)

func TestBulkAction_IncludeCrossSeeds(t *testing.T) {
	t.Parallel()
	for _, failFiles := range []bool{false, true} {
		t.Run(map[bool]string{false: "filter selection expands", true: "file failure aborts all instances"}[failFiles], func(t *testing.T) {
			db := testdb.NewMigratedSQLite(t, "delete-include")
			store, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
			require.NoError(t, err)
			pool, err := qbittorrent.NewClientPool(store, models.NewInstanceErrorStore(db), time.Minute)
			require.NoError(t, err)
			t.Cleanup(func() { _ = pool.Close() })
			torrents := []qbt.Torrent{
				{Hash: "aaa", Name: "Synthetic selected", Category: "selected", SavePath: "/data", ContentPath: "/data/shared", AddedOn: 1},
				{Hash: "bbb", Name: "Synthetic related", Category: "other", Tags: "Archive,Ready", Size: 2048, State: "uploading", Progress: 1, Ratio: 2.5, SavePath: "/data", ContentPath: "/data/shared", AddedOn: 1700000000},
				{Hash: "ccc", Name: "Synthetic independent", Category: "other", SavePath: "/data", ContentPath: "/data/elsewhere", AddedOn: 1},
			}
			var mu sync.Mutex
			deleted := make(map[int][]string)
			clients := make(map[int]*qbittorrent.Client)
			ids := make([]int, 0, 2)
			for i := range 2 {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/v2/torrents/info":
						_ = json.NewEncoder(w).Encode(torrents)
					case "/api/v2/torrents/files":
						if failFiles && i == 1 {
							http.Error(w, "offline", http.StatusInternalServerError)
							return
						}
						_ = json.NewEncoder(w).Encode(qbt.TorrentFiles{{Name: "shared/payload.mkv", Size: 100}})
					case "/api/v2/sync/maindata":
						rows := make(map[string]qbt.Torrent)
						for _, torrent := range torrents {
							rows[torrent.Hash] = torrent
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"rid": 1, "full_update": true, "torrents": rows})
					case "/api/v2/torrents/delete":
						_ = r.ParseForm()
						if r.Form.Get("deleteFiles") != "true" {
							http.Error(w, "files must be deleted", http.StatusBadRequest)
							return
						}
						mu.Lock()
						deleted[i] = append(deleted[i], strings.Split(r.Form.Get("hashes"), "|")...)
						mu.Unlock()
					default:
						_, _ = w.Write([]byte("Ok."))
					}
				}))
				t.Cleanup(srv.Close)
				instance, err := store.Create(t.Context(), "Synthetic instance", srv.URL, "user", "pass", nil, nil, false, nil)
				require.NoError(t, err)
				ids = append(ids, instance.ID)
				clients[instance.ID] = newStaleCachedClient(t, srv.URL, torrents)
			}
			setUnexportedField(t, pool, "clients", clients)
			handler := NewTorrentsHandler(qbittorrent.NewSyncManager(pool, nil), nil, store, nil)
			payload := map[string]any{"action": "delete", "includeCrossSeeds": true, "previewOnly": true, "selectAll": true, "instanceIds": ids, "filters": map[string]any{"categories": []string{"selected"}}}
			recorder := httptest.NewRecorder()
			handler.BulkAction(recorder, newTorrentFieldRequest(t, 0, payload))
			mu.Lock()
			require.Empty(t, deleted)
			mu.Unlock()
			if !failFiles {
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				var plan includingDeleteSummary
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &plan))
				require.Equal(t, 2, plan.Selected)
				require.Equal(t, 2, plan.Related)
				require.Equal(t, 4, plan.Total)
				require.NotEqual(t, plan.RelatedTorrents[0].InstanceID, plan.RelatedTorrents[1].InstanceID)
				for _, related := range plan.RelatedTorrents {
					require.Equal(t, includingDeleteTorrent{InstanceID: related.InstanceID, Hash: "bbb", Name: "Synthetic related", Category: "other", Tags: "Archive,Ready", Size: 2048, State: "uploading", Progress: 1, AddedOn: 1700000000, Ratio: 2.5}, related)
				}
			}
			payload["previewOnly"] = false
			recorder = httptest.NewRecorder()
			handler.BulkAction(recorder, newTorrentFieldRequest(t, 0, payload))
			mu.Lock()
			defer mu.Unlock()
			if failFiles {
				require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
				require.Empty(t, deleted)
			} else {
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.ElementsMatch(t, []string{"aaa", "bbb"}, deleted[0])
				require.ElementsMatch(t, []string{"aaa", "bbb"}, deleted[1])
			}
		})
	}
}
