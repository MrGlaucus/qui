// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/autobrr/qui/internal/models"
	"github.com/stretchr/testify/require"
)

func TestUpdateAutomationSettingsNilCheck(t *testing.T) {
	s := &Service{}

	_, err := s.UpdateAutomationSettings(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil settings but got none")
	}

	expectedMsg := "settings cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestRSSAutomationInterval(t *testing.T) {
	for _, minutes := range []int{1, 5, 29, 30, 120, 0, -1} {
		t.Run(fmt.Sprintf("%d_minutes", minutes), func(t *testing.T) {
			store, _, _ := newPartialPoolCoordinatorStore(t)
			s := &Service{automationStore: store}
			settings := models.DefaultCrossSeedAutomationSettings()
			settings.Enabled = true
			settings.RunIntervalMinutes = minutes
			updated, err := s.UpdateAutomationSettings(t.Context(), settings)
			require.NoError(t, err)
			want := minutes
			if want <= 0 {
				want = 120
			}
			require.Equal(t, want, updated.RunIntervalMinutes)
			stored, err := s.GetAutomationSettings(t.Context())
			require.NoError(t, err)
			require.Equal(t, want, stored.RunIntervalMinutes)

			// A run just older than the configured interval must be due,
			// including intervals below the former 30-minute floor.
			_, err = store.CreateRun(t.Context(), &models.CrossSeedRun{
				Mode:      models.CrossSeedRunModeAuto,
				Status:    models.CrossSeedRunStatusSuccess,
				StartedAt: time.Now().Add(-time.Duration(want)*time.Minute - time.Second),
			})
			require.NoError(t, err)
			delay, due := s.computeNextRunDelay(t.Context(), stored)
			require.True(t, due)
			require.Zero(t, delay)

			// A recent run must still wait for the configured interval.
			_, err = store.CreateRun(t.Context(), &models.CrossSeedRun{
				Mode:      models.CrossSeedRunModeAuto,
				Status:    models.CrossSeedRunStatusSuccess,
				StartedAt: time.Now().Add(-10 * time.Second),
			})
			require.NoError(t, err)
			delay, due = s.computeNextRunDelay(t.Context(), stored)
			require.False(t, due)
			require.InDelta(t, float64(time.Duration(want)*time.Minute-10*time.Second), float64(delay), float64(5*time.Second))
		})
	}
}
