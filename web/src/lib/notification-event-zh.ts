export interface LocalizedNotificationEvent {
  label: string
  description: string
}

export const notificationEventZh: Record<string, LocalizedNotificationEvent> = {
  torrent_added: {
    label: "种子已添加",
    description: "有种子被添加（包含 Tracker、分类、标签，以及可用的预计完成时间）。",
  },
  torrent_completed: {
    label: "种子已完成",
    description: "种子下载完成（包含 Tracker、分类和标签，如果有的话）。",
  },
  backup_succeeded: {
    label: "备份成功",
    description: "备份运行成功完成。",
  },
  backup_failed: {
    label: "备份失败",
    description: "备份运行失败。",
  },
  dir_scan_completed: {
    label: "目录扫描完成",
    description: "目录扫描运行结束。",
  },
  dir_scan_failed: {
    label: "目录扫描失败",
    description: "目录扫描运行失败。",
  },
  orphan_scan_completed: {
    label: "孤立文件扫描完成",
    description: "孤立文件扫描运行完成（包括干净的运行）。",
  },
  orphan_scan_failed: {
    label: "孤立文件扫描失败",
    description: "孤立文件扫描运行失败。",
  },
  cross_seed_automation_succeeded: {
    label: "Cross-seed RSS 自动化完成",
    description: "RSS 自动化运行完成（包含汇总计数和样本）。",
  },
  cross_seed_automation_failed: {
    label: "Cross-seed RSS 自动化失败",
    description: "RSS 自动化运行失败或带错误完成。",
  },
  cross_seed_search_succeeded: {
    label: "Cross-seed 做种搜索完成",
    description: "做种搜索运行完成（包含汇总计数和样本）。",
  },
  cross_seed_search_failed: {
    label: "Cross-seed 做种搜索失败",
    description: "做种搜索运行失败或被取消。",
  },
  cross_seed_completion_succeeded: {
    label: "Cross-seed 完成搜索完成",
    description: "完成搜索运行完成（包含汇总计数和样本）。",
  },
  cross_seed_completion_failed: {
    label: "Cross-seed 完成搜索失败",
    description: "完成搜索运行失败。",
  },
  cross_seed_webhook_succeeded: {
    label: "Cross-seed Webhook 检查完成",
    description: "Webhook 检查运行完成（包含汇总计数和样本）。",
  },
  cross_seed_webhook_failed: {
    label: "Cross-seed Webhook 检查失败",
    description: "Webhook 检查运行失败。",
  },
  automations_actions_applied: {
    label: "自动化操作已应用",
    description: "自动化规则已应用操作（包含汇总计数和样本；仅在发生操作时触发）。",
  },
  automations_run_failed: {
    label: "自动化运行失败",
    description: "自动化规则在某个实例上运行失败（系统错误）。",
  },
  daily_traffic_report: {
    label: "每日流量报告",
    description: "每天 0 点播报各实例当天的上传/下载流量汇总。",
  },
  hourly_traffic_report: {
    label: "整点流量报告",
    description: "每个整点播报各实例当日累计的上传/下载流量汇总。",
  },
  baseline_report: {
    label: "基准采集报告",
    description: "每天 0 点执行基准值采集后，通知各实例的基准值快照结果。",
  },
}
