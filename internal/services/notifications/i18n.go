// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package notifications

// This is a small bilingual catalog for notification text. qui only ships zh-CN
// and en to keep the backend translations minimal; the frontend has its own
// richer i18n. Keys are terse English identifiers; each maps to a zh and en
// string. The "zh" variant is the historical hard-coded Chinese, so existing
// Chinese notifications are unchanged.

var notificationText = map[string]map[string]string{}

// RegisterText adds translations for a key. A nil map is a no-op. Values may be
// added across files in this package via init().
func registerText(translations map[string]map[string]string) {
	for key, langs := range translations {
		if len(langs) > 0 {
			notificationText[key] = langs
		}
	}
}

// NormalizeLang maps a frontend language code (e.g. "zh-CN", "en-US") to the
// backend's two-variant set: "zh" or "en". Unknown/empty maps to "zh" so the
// historical default is preserved.
func NormalizeLang(code string) string {
	if len(code) >= 2 && (code[0] == 'e' || code[0] == 'E') {
		return "en"
	}
	return "zh"
}

// T returns the translation for key in lang (NormalizeLang already applied).
// Missing keys or variants fall back to the zh text, then to the key itself.
func T(key, lang string) string {
	lang = NormalizeLang(lang)
	if langs, ok := notificationText[key]; ok {
		if v, ok := langs[lang]; ok {
			return v
		}
	}
	return key
}
