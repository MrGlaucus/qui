// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package qbittorrent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/autobrr/qui/internal/models"
)

func TestClassifyTraffic(t *testing.T) {
	tests := []struct {
		name                                    string
		sessionUlDelta, sessionDlDelta           int64
		allUlDelta, allDlDelta                   int64
		wantUploaded, wantDownloaded             int64
		wantSource                               string
	}{
		{
			name:          "normal session advance",
			sessionUlDelta: 10, sessionDlDelta: 20,
			allUlDelta: 10, allDlDelta: 20,
			wantUploaded: 10, wantDownloaded: 20,
			wantSource: models.DataSourceSession,
		},
		{
			name:          "session reset, alltime advances - restart fallback",
			sessionUlDelta: -1000, sessionDlDelta: -2000,
			allUlDelta: 5, allDlDelta: 7,
			wantUploaded: 5, wantDownloaded: 7,
			wantSource: models.DataSourceRestart,
		},
		{
			name:          "both reset - reinstall baseline",
			sessionUlDelta: -1000, sessionDlDelta: -2000,
			allUlDelta: -500, allDlDelta: -700,
			wantUploaded: 0, wantDownloaded: 0,
			wantSource: models.DataSourceReinstall,
		},
		{
			name:          "alltime reset but session advances - reinstall baseline",
			sessionUlDelta: 10, sessionDlDelta: 20,
			allUlDelta: -500, allDlDelta: -700,
			wantUploaded: 0, wantDownloaded: 0,
			wantSource: models.DataSourceReinstall,
		},
		{
			name:          "zero deltas on first sample after init are fine",
			sessionUlDelta: 0, sessionDlDelta: 0,
			allUlDelta: 0, allDlDelta: 0,
			wantUploaded: 0, wantDownloaded: 0,
			wantSource: models.DataSourceSession,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyTraffic(tt.sessionUlDelta, tt.sessionDlDelta, tt.allUlDelta, tt.allDlDelta)
			assert.Equal(t, tt.wantUploaded, got.uploaded)
			assert.Equal(t, tt.wantDownloaded, got.downloaded)
			assert.Equal(t, tt.wantSource, got.source)
		})
	}
}

// fakeTrafficStore is a minimal in-memory implementation of the store surface
// the recorder needs. It keeps rows so GetByInstanceAndDate reflects what
// Upsert persisted, and records the upsert/prune calls for assertions.
type fakeTrafficStore struct {
	rows    map[int]map[string]*models.InstanceDailyTraffic
	upserts []trafficUpsert
	prunes  []trafficPrune
}

type trafficUpsert struct {
	instanceID         int
	date               string
	uploaded           int64
	downloaded         int64
	peakUlSpeed        int64
	peakDlSpeed        int64
	dataSource         string
	baseline           *models.TrafficBaseline
}

type trafficPrune struct {
	instanceID int
	beforeDate string
}

func (f *fakeTrafficStore) GetByInstanceAndDate(_ context.Context, instanceID int, date string) (*models.InstanceDailyTraffic, error) {
	if f.rows == nil {
		return nil, nil
	}
	row, ok := f.rows[instanceID][date]
	if !ok {
		return nil, nil
	}
	return row, nil
}

func (f *fakeTrafficStore) Upsert(_ context.Context, instanceID int, date string, uploaded, downloaded, peakUlSpeed, peakDlSpeed int64, dataSource string, baseline *models.TrafficBaseline) (*models.InstanceDailyTraffic, error) {
	f.upserts = append(f.upserts, trafficUpsert{
		instanceID:  instanceID,
		date:        date,
		uploaded:    uploaded,
		downloaded:  downloaded,
		peakUlSpeed: peakUlSpeed,
		peakDlSpeed: peakDlSpeed,
		dataSource:  dataSource,
		baseline:    baseline,
	})

	if f.rows == nil {
		f.rows = map[int]map[string]*models.InstanceDailyTraffic{}
	}
	if f.rows[instanceID] == nil {
		f.rows[instanceID] = map[string]*models.InstanceDailyTraffic{}
	}
	row := f.rows[instanceID][date]
	if row == nil {
		row = &models.InstanceDailyTraffic{}
		f.rows[instanceID][date] = row
	}
	row.Uploaded = uploaded
	row.Downloaded = downloaded
	if peakUlSpeed > row.PeakUlSpeed {
		row.PeakUlSpeed = peakUlSpeed
	}
	if peakDlSpeed > row.PeakDlSpeed {
		row.PeakDlSpeed = peakDlSpeed
	}
	if baseline != nil {
		row.BaselineSessionUploaded = baseline.SessionUploaded
		row.BaselineSessionDownloaded = baseline.SessionDownloaded
		row.BaselineAlltimeUploaded = baseline.AlltimeUploaded
		row.BaselineAlltimeDownloaded = baseline.AlltimeDownloaded
		row.BaselineAt = baseline.At
	}
	return row, nil
}

func (f *fakeTrafficStore) DeleteOlderThan(_ context.Context, instanceID int, beforeDate string) (int64, error) {
	f.prunes = append(f.prunes, trafficPrune{instanceID: instanceID, beforeDate: beforeDate})
	return 0, nil
}

func TestDailyTrafficRecorder_Record(t *testing.T) {
	ctx := context.Background()
	store := &fakeTrafficStore{}
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.Local)

	rec := &DailyTrafficRecorder{
		store:         store,
		now:           func() time.Time { return now },
		retentionDays: 7,
	}

	sample := func(sessionUl, sessionDl, allUl, allDl, upSpeed, dlSpeed int64) *qbt.ServerState {
		return &qbt.ServerState{
			UpInfoData:  sessionUl,
			DlInfoData:  sessionDl,
			AlltimeUl:   allUl,
			AlltimeDl:   allDl,
			UpInfoSpeed: upSpeed,
			DlInfoSpeed: dlSpeed,
		}
	}

	// First sample initializes the day row with a baseline snapshot and no delta.
	rec.Record(ctx, 1, sample(100, 200, 1000, 2000, 10, 20))
	require.Len(t, store.upserts, 1)
	assert.Equal(t, "2026-08-04", store.upserts[0].date)
	assert.Equal(t, int64(0), store.upserts[0].uploaded)
	assert.Equal(t, int64(0), store.upserts[0].downloaded)
	require.NotNil(t, store.upserts[0].baseline)
	assert.Equal(t, int64(100), store.upserts[0].baseline.SessionUploaded)
	assert.Equal(t, int64(200), store.upserts[0].baseline.SessionDownloaded)
	assert.Equal(t, int64(1000), store.upserts[0].baseline.AlltimeUploaded)
	assert.Equal(t, int64(2000), store.upserts[0].baseline.AlltimeDownloaded)
	assert.NotEmpty(t, store.upserts[0].baseline.At)

	// Second sample attributes current - baseline (session source).
	rec.Record(ctx, 1, sample(120, 240, 1020, 2040, 15, 25))
	require.Len(t, store.upserts, 2)
	assert.Equal(t, int64(20), store.upserts[1].uploaded)
	assert.Equal(t, int64(40), store.upserts[1].downloaded)
	assert.Equal(t, int64(15), store.upserts[1].peakUlSpeed)
	assert.Equal(t, int64(25), store.upserts[1].peakDlSpeed)
	assert.Equal(t, models.DataSourceSession, store.upserts[1].dataSource)
	assert.Equal(t, "2026-08-04", store.upserts[1].date)
	assert.Nil(t, store.upserts[1].baseline)

	// qBittorrent restarts: session regresses below baseline, alltime keeps
	// advancing. The day total falls back to alltime - baseline.
	rec.Record(ctx, 1, sample(0, 0, 1040, 2070, 0, 0))
	require.Len(t, store.upserts, 3)
	assert.Equal(t, int64(40), store.upserts[2].uploaded)
	assert.Equal(t, int64(70), store.upserts[2].downloaded)
	assert.Equal(t, models.DataSourceRestart, store.upserts[2].dataSource)

	// Reinstall: everything regresses; nothing attributed and the row is
	// re-baselined to the fresh counters.
	rec.Record(ctx, 1, sample(1, 1, 1, 1, 0, 0))
	require.Len(t, store.upserts, 4)
	assert.Equal(t, int64(0), store.upserts[3].uploaded)
	assert.Equal(t, int64(0), store.upserts[3].downloaded)
	assert.Equal(t, models.DataSourceReinstall, store.upserts[3].dataSource)
	require.NotNil(t, store.upserts[3].baseline)
	assert.Equal(t, int64(1), store.upserts[3].baseline.SessionUploaded)
}

func TestDailyTrafficRecorder_DayRolloverBaselinesAndPrunes(t *testing.T) {
	ctx := context.Background()
	store := &fakeTrafficStore{}

	day := time.Date(2026, 8, 4, 23, 59, 0, 0, time.Local)
	rec := &DailyTrafficRecorder{
		store:         store,
		now:           func() time.Time { return day },
		retentionDays: 7,
	}

	sample := func(sessionUl, sessionDl int64) *qbt.ServerState {
		return &qbt.ServerState{
			UpInfoData:  sessionUl,
			DlInfoData:  sessionDl,
			AlltimeUl:   sessionUl + 1000,
			AlltimeDl:   sessionDl + 2000,
		}
	}

	rec.Record(ctx, 1, sample(100, 200))
	// First sample of the day: persists the baseline snapshot with no delta,
	// and prunes old history.
	require.Len(t, store.upserts, 1)
	assert.Equal(t, int64(0), store.upserts[0].uploaded)
	assert.Equal(t, int64(0), store.upserts[0].downloaded)
	assert.Equal(t, "2026-08-04", store.upserts[0].date)
	require.NotNil(t, store.upserts[0].baseline)
	assert.Equal(t, int64(100), store.upserts[0].baseline.SessionUploaded)
	assert.Equal(t, int64(200), store.upserts[0].baseline.SessionDownloaded)
	require.Len(t, store.prunes, 1)

	// Same day: records normally, no extra prune.
	rec.Record(ctx, 1, sample(110, 220))
	require.Len(t, store.upserts, 2)
	assert.Equal(t, int64(10), store.upserts[1].uploaded)
	assert.Equal(t, int64(20), store.upserts[1].downloaded)
	require.Len(t, store.prunes, 1)

	// Advance to the next day: a fresh row with a fresh baseline snapshot is
	// created, and pruning runs once.
	day = time.Date(2026, 8, 5, 0, 0, 0, 0, time.Local)
	rec.Record(ctx, 1, sample(110, 220))
	require.Len(t, store.upserts, 3)
	assert.Equal(t, "2026-08-05", store.upserts[2].date)
	assert.Equal(t, int64(0), store.upserts[2].uploaded)
	require.NotNil(t, store.upserts[2].baseline)
	assert.Equal(t, int64(110), store.upserts[2].baseline.SessionUploaded)
	require.Len(t, store.prunes, 2)
	assert.Equal(t, "2026-07-30", store.prunes[1].beforeDate)

	// Now the new day records normally.
	rec.Record(ctx, 1, sample(125, 250))
	require.Len(t, store.upserts, 4)
	assert.Equal(t, int64(15), store.upserts[3].uploaded)
	assert.Equal(t, int64(30), store.upserts[3].downloaded)
	assert.Equal(t, "2026-08-05", store.upserts[3].date)
}

func TestDailyTrafficRecorder_IgnoresDisabledInputs(t *testing.T) {
	rec := &DailyTrafficRecorder{}
	// nil store or nil state must not panic.
	rec.Record(context.Background(), 1, &qbt.ServerState{})
	assert.NotPanics(t, func() {
		rec.Record(context.Background(), 1, nil)
	})
}
