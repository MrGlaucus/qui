// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package notifications

import (
	"fmt"
	"strings"
)

func init() {
	registerText(map[string]map[string]string{
		"crossseed.rss.added":          {"zh": "RSS 辅种：已添加 %d 个", "en": "RSS cross-seed: %d added"},
		"crossseed.rss.partial":        {"zh": "RSS 辅种：部分完成", "en": "RSS cross-seed: partially completed"},
		"crossseed.rss.failed":         {"zh": "RSS 辅种失败", "en": "RSS cross-seed failed"},
		"crossseed.rss.scan":           {"zh": "扫描结果: RSS %d 条 · 可辅种 %d 条", "en": "Scan: %d RSS items · %d matches"},
		"crossseed.rss.result":         {"zh": "执行结果: 新增 %d · 跳过 %d · 失败 %d", "en": "Result: %d added · %d skipped · %d failed"},
		"crossseed.rss.indexers":       {"zh": "辅种目标站点（RSS 索引器）", "en": "Cross-seed target sites (RSS indexers)"},
		"crossseed.rss.indexerAdds":    {"zh": "- %s · 新增 %d 个", "en": "- %s · %d added"},
		"crossseed.rss.unknownIndexer": {"zh": "未记录", "en": "Not recorded"},
		"crossseed.rss.samples":        {"zh": "来源种子（名称 @ 来源实例）", "en": "Source torrents (name @ source instance)"},
		"crossseed.rss.sample":         {"zh": "- %s", "en": "- %s"},
		"crossseed.rss.error":          {"zh": "失败原因", "en": "Failure"},
		"crossseed.rss.run":            {"zh": "运行记录: #%d", "en": "Run: #%d"},
	})
}

func formatRSSAutomationEvent(event Event, lang string) (string, string) {
	data := event.CrossSeed
	if data == nil {
		return "", ""
	}

	var title string
	switch {
	case data.Status == "partial":
		title = T("crossseed.rss.partial", lang)
	case event.Type == EventCrossSeedAutomationFailed:
		title = T("crossseed.rss.failed", lang)
	default:
		title = fmt.Sprintf(T("crossseed.rss.added", lang), data.Added)
	}

	lines := []string{
		fmt.Sprintf(T("crossseed.rss.scan", lang), data.FeedItems, data.Candidates),
		fmt.Sprintf(T("crossseed.rss.result", lang), data.Added, data.Skipped, data.Failed),
	}
	if len(data.TargetIndexerAdds) > 0 {
		lines = append(lines, T("crossseed.rss.indexers", lang)+":")
		for _, indexer := range data.TargetIndexerAdds {
			lines = append(lines, fmt.Sprintf(T("crossseed.rss.indexerAdds", lang), indexer.Label, indexer.Count))
		}
	} else if data.Added > 0 {
		lines = append(lines, formatLine(T("crossseed.rss.indexers", lang), T("crossseed.rss.unknownIndexer", lang)))
	}
	if len(data.Samples) > 0 {
		lines = append(lines, T("crossseed.rss.samples", lang)+":")
		for _, sample := range data.Samples[:min(3, len(data.Samples))] {
			lines = append(lines, fmt.Sprintf(T("crossseed.rss.sample", lang), sample))
		}
	}
	if errorMessage := strings.TrimSpace(event.ErrorMessage); errorMessage != "" {
		lines = append(lines, formatLine(T("crossseed.rss.error", lang), errorMessage))
	}
	if data.RunID > 0 {
		lines = append(lines, fmt.Sprintf(T("crossseed.rss.run", lang), data.RunID))
	}
	return title, strings.Join(lines, "\n")
}
