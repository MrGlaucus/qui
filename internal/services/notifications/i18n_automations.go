// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package notifications

func init() {
	registerText(map[string]map[string]string{
		// action labels
		"automations.action.deletedRatio":        {"zh": "删除种子（分享率规则）", "en": "Deleted torrent (ratio rule)"},
		"automations.action.deletedSeeding":      {"zh": "删除种子（做种规则）", "en": "Deleted torrent (seeding rule)"},
		"automations.action.deletedUnregistered": {"zh": "删除种子（未注册）", "en": "Deleted torrent (unregistered)"},
		"automations.action.deletedCondition":    {"zh": "删除种子（规则）", "en": "Deleted torrent (rule)"},
		"automations.action.deleteFailed":        {"zh": "删除失败", "en": "Delete failed"},
		"automations.action.limitFailed":         {"zh": "限速设置失败", "en": "Limit failed"},
		"automations.action.tagsChanged":         {"zh": "更新标签", "en": "Tags updated"},
		"automations.action.categoryChanged":     {"zh": "更新分类", "en": "Category updated"},
		"automations.action.speedLimitsChanged":  {"zh": "更新限速", "en": "Speed limits updated"},
		"automations.action.shareLimitsChanged":  {"zh": "更新分享率", "en": "Share limits updated"},
		"automations.action.paused":              {"zh": "暂停种子", "en": "Paused torrents"},
		"automations.action.resumed":             {"zh": "恢复种子", "en": "Resumed torrents"},
		"automations.action.rechecked":           {"zh": "强制校验", "en": "Rechecked torrents"},
		"automations.action.reannounced":         {"zh": "重新公告", "en": "Reannounced torrents"},
		"automations.action.moved":               {"zh": "移动种子", "en": "Moved torrents"},
		"automations.action.exportedToInstance":  {"zh": "导出到实例", "en": "Exported to instance"},
		"automations.action.dryRunNoMatch":       {"zh": "试运行：无匹配", "en": "Dry run: no matches"},

		// summary message line prefixes
		"automations.summary.applied":   {"zh": "生效种子", "en": "Affected torrents"},
		"automations.summary.failed":    {"zh": "失败", "en": "Failed"},
		"automations.summary.rules":     {"zh": "规则", "en": "Rules"},
		"automations.summary.tags":      {"zh": "标签", "en": "Tags"},
		"automations.summary.tagSample": {"zh": "标签样本", "en": "Tag samples"},
		"automations.summary.affected":  {"zh": "影响种子", "en": "Affected torrents"},
		"automations.summary.errors":    {"zh": "错误", "en": "Errors"},

		// per-torrent render field labels
		"automations.sample.action":   {"zh": "操作", "en": "Action"},
		"automations.sample.torrent":  {"zh": "种子", "en": "Torrent"},
		"automations.sample.size":     {"zh": "大小", "en": "Size"},
		"automations.sample.ratio":    {"zh": "分享率", "en": "Ratio"},
		"automations.sample.traffic":  {"zh": "流量", "en": "Traffic"},
		"automations.sample.speed":    {"zh": "速度", "en": "Speed"},
		"automations.sample.category": {"zh": "分类", "en": "Category"},
		"automations.sample.tags":     {"zh": "标签", "en": "Tags"},
		"automations.sample.state":    {"zh": "状态", "en": "State"},
		"automations.sample.tracker":  {"zh": "站点", "en": "Tracker"},
	})
}
