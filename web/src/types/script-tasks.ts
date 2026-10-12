export interface ScriptDefinition {
  id: string
  name: string
  description: string
  code: string
  parameters: Record<string, unknown>
  builtin: boolean
}
export interface ScriptTask {
  id: string
  name: string
  script_id: string
  enabled: boolean
  instance_ids: number[]
  all_instances: boolean
  tracker_domains: string[]
  parameters: Record<string, unknown>
  interval_seconds: number
  timeout_seconds: number
  run_at: number
  permissions: string[]
  next_run: number
}
export interface ScriptTaskRun {
  id: string
  task_id: string
  started_at: number
  finished_at: number
  status: string
  dry_run: boolean
  log: string
}
export interface ScriptTaskOverview {
  scripts: ScriptDefinition[]
  tasks: ScriptTask[]
  python_available: boolean
}
