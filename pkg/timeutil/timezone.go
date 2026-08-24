// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// Package timeutil provides a server-wide timezone provider that can be
// updated at runtime from frontend settings, so day/month boundaries for
// traffic attribution and reports follow the user's chosen timezone instead
// of the server's local timezone.
package timeutil

import (
	"encoding/json"
	"sync/atomic"
	"time"
)

// Provider holds the current *time.Location and derives timestamps/dates in it.
// It is safe for concurrent use and can be swapped at runtime when the user
// changes their timezone preference. The zero value is not usable; create one
// with NewProvider.
type Provider struct {
	loc atomic.Value // *time.Location
}

// NewProvider returns a Provider whose initial location is time.Local (the
// server's timezone, preserving the historical behaviour).
func NewProvider() *Provider {
	p := &Provider{}
	p.loc.Store(time.Local)
	return p
}

// Set swaps the active timezone to loc. Pass nil to reset to time.Local.
func (p *Provider) Set(loc *time.Location) {
	if loc == nil {
		loc = time.Local
	}
	p.loc.Store(loc)
}

// datetimePreferencesKey is the client_settings key holding the frontend's
// date/time preferences as a JSON object with a "timezone" field.
const datetimePreferencesKey = "qui-datetime-preferences"

// TimezoneFromSettings extracts the timezone from a client_settings snapshot.
// The preferences value is a JSON object like
// {"timezone":"Asia/Shanghai","timeFormat":"24h","dateFormat":"iso"}; the
// timezone field is an IANA name. Falls back to time.Local on any error.
func TimezoneFromSettings(settings map[string]string) *time.Location {
	if settings == nil {
		return time.Local
	}
	raw, ok := settings[datetimePreferencesKey]
	if !ok {
		return time.Local
	}
	var prefs struct {
		Timezone string `json:"timezone"`
	}
	if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
		return time.Local
	}
	return LoadLocation(prefs.Timezone)
}

// Location returns the current timezone location.
func (p *Provider) Location() *time.Location {
	if p == nil {
		return time.Local
	}
	if v := p.loc.Load(); v != nil {
		if loc, ok := v.(*time.Location); ok {
			return loc
		}
	}
	return time.Local
}

// Now returns the current time in the provider's timezone.
func (p *Provider) Now() time.Time {
	return time.Now().In(p.Location())
}

// Today returns the current date in YYYY-MM-DD format in the provider's
// timezone. This is what is used to key instance_daily_traffic rows, so the
// "day boundary" matches the user's calendar day.
func (p *Provider) Today() string {
	return p.Now().Format("2006-01-02")
}

// LoadLocation parses an IANA timezone name (e.g. "Asia/Shanghai") into a
// *time.Location, falling back to time.Local when the name is empty, malformed
// or unknown. Unknown zones fall back silently rather than erroring so a stale
// setting never breaks the recorder.
func LoadLocation(name string) *time.Location {
	name = trimSpace(name)
	if name == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Local
	}
	return loc
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
