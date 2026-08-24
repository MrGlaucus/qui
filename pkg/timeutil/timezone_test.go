// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package timeutil

import (
	"testing"
	"time"
)

func TestLoadLocation(t *testing.T) {
	t.Parallel()

	if got := LoadLocation(""); got != time.Local {
		t.Fatalf("empty name: want time.Local, got %v", got)
	}
	if got := LoadLocation("Not/AZone"); got != time.Local {
		t.Fatalf("unknown zone: want time.Local, got %v", got)
	}
	loc := LoadLocation("Asia/Shanghai")
	if loc == time.Local {
		t.Fatalf("want Asia/Shanghai, got time.Local")
	}
}

func TestTimezoneFromSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings map[string]string
		want     *time.Location
	}{
		{name: "nil", settings: nil, want: time.Local},
		{name: "missing key", settings: map[string]string{"other": "x"}, want: time.Local},
		{name: "valid", settings: map[string]string{"qui-datetime-preferences": `{"timezone":"Asia/Shanghai","timeFormat":"24h"}`}, want: LoadLocation("Asia/Shanghai")},
		{name: "empty timezone", settings: map[string]string{"qui-datetime-preferences": `{"timezone":""}`}, want: time.Local},
		{name: "malformed json", settings: map[string]string{"qui-datetime-preferences": `{`}, want: time.Local},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := TimezoneFromSettings(tt.settings); got.String() != tt.want.String() {
				t.Fatalf("got %v, want %v", got.String(), tt.want.String())
			}
		})
	}
}

func TestProviderSetAndToday(t *testing.T) {
	t.Parallel()

	p := NewProvider()
	loc := LoadLocation("America/New_York")
	p.Set(loc)
	if p.Location().String() != loc.String() {
		t.Fatalf("Location: got %v, want %v", p.Location().String(), loc.String())
	}

	// Today renders in the set zone, not server local.
	p.Set(LoadLocation("Pacific/Kiritimati")) // UTC+14, far ahead
	nyNow := time.Now().In(LoadLocation("America/New_York"))
	sydneyNow := time.Now().In(LoadLocation("Australia/Sydney"))

	// Just sanity-check that Today returns a YYYY-MM-DD string (non-empty).
	if d := p.Today(); len(d) != len("2006-01-02") {
		t.Fatalf("Today() format wrong: %q", d)
	}
	_ = nyNow
	_ = sydneyNow
}
