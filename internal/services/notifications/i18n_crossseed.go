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
		"crossseed.rss.scan":           {"zh": "扫描 RSS 条目 %d 条，发现可辅种条目 %d 条", "en": "Feed items scanned: %d; matching items: %d"},
		"crossseed.rss.result":         {"zh": "已添加 %d 个 · 已跳过 %d 条 · 失败 %d 条", "en": "Added: %d · Skipped: %d · Failed: %d"},
		"crossseed.rss.indexers":       {"zh": "辅种目标站点（RSS 索引器）", "en": "Cross-seed target sites (RSS indexers)"},
		"crossseed.rss.indexerAdds":    {"zh": "%s（新增 %d 个）", "en": "%s (%d added)"},
		"crossseed.rss.unknownIndexer": {"zh": "未记录", "en": "Not recorded"},
		"crossseed.rss.sample":         {"zh": "匹配的本地种子（名称 @ 下载器实例）", "en": "Matched local torrent (name @ client instance)"},
		"crossseed.rss.error":          {"zh": "错误原因", "en": "Error"},
		"crossseed.rss.run":            {"zh": "运行记录 #%d", "en": "Run #%d"},
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
		indexers := make([]string, 0, len(data.TargetIndexerAdds))
		for _, indexer := range data.TargetIndexerAdds {
			indexers = append(indexers, fmt.Sprintf(T("crossseed.rss.indexerAdds", lang), indexer.Label, indexer.Count))
		}
		lines = append(lines, formatLine(T("crossseed.rss.indexers", lang), strings.Join(indexers, "; ")))
	} else if data.Added > 0 {
		lines = append(lines, formatLine(T("crossseed.rss.indexers", lang), T("crossseed.rss.unknownIndexer", lang)))
	}
	if len(data.Samples) > 0 {
		lines = append(lines, formatLine(T("crossseed.rss.sample", lang), strings.Join(data.Samples[:min(3, len(data.Samples))], "; ")))
	}
	if errorMessage := strings.TrimSpace(event.ErrorMessage); errorMessage != "" {
		lines = append(lines, formatLine(T("crossseed.rss.error", lang), errorMessage))
	}
	if data.RunID > 0 {
		lines = append(lines, fmt.Sprintf(T("crossseed.rss.run", lang), data.RunID))
	}
	return title, strings.Join(lines, "\n")
}
