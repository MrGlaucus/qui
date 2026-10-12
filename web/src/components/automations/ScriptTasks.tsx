import { ScriptTrackerSelect } from "./ScriptTrackerSelect"
import { ScriptApiReference } from "./ScriptApiReference"
import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { api } from "@/lib/api"
import { useInstances } from "@/hooks/useInstances"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Checkbox } from "@/components/ui/checkbox"
import { Badge } from "@/components/ui/badge"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import type { ScriptDefinition, ScriptTask } from "@/types/script-tasks"

const permissions = ["limits", "tags", "categories", "recheck", "reannounce", "pause_resume", "delete", "notify"] as const
const newScript: ScriptDefinition = { id: "", name: "", description: "", code: "from qui import ctx\n\nfor instance in ctx.instances():\n    ctx.log(instance['name'])\n", parameters: {}, builtin: false }

export function ScriptTasks() {
  const { t } = useTranslation("automations")
  const cache = useQueryClient()
  const { instances } = useInstances()
  const query = useQuery({ queryKey: ["script-tasks"], queryFn: () => api.getScriptTasks(), refetchInterval: 5000 })
  const [script, setScript] = useState<ScriptDefinition | null>(null)
  const [task, setTask] = useState<ScriptTask | null>(null)
  const [parameters, setParameters] = useState("{}")
  const [history, setHistory] = useState<ScriptTask | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const [remove, setRemove] = useState<{ id: string; kind: "scripts" | "tasks" } | null>(null)
  const runs = useQuery({ queryKey: ["script-runs", history?.id], queryFn: () => api.getScriptTaskRuns(history!.id), enabled: !!history, refetchInterval: history ? 2000 : false })
  const act = async (action: () => Promise<unknown>) => {
    setError("")
    setBusy(true)
    try { await action(); await cache.invalidateQueries({ queryKey: ["script-tasks"] }); await cache.invalidateQueries({ queryKey: ["script-runs"] }) }
    catch (error) { setError(error instanceof Error ? error.message : t("scripts.failed")) }
    finally { setBusy(false) }
  }
  const editScript = (value: ScriptDefinition) => { setError(""); setScript(value); setParameters(JSON.stringify(value.parameters, null, 2)) }
  const editTask = (value: ScriptTask) => { setError(""); setTask(value); setParameters(JSON.stringify(value.parameters, null, 2)) }
  const createTask = (value: ScriptDefinition) => editTask({ id: "", name: value.name, script_id: value.id, enabled: false, instance_ids: [], all_instances: false, tracker_domains: [], parameters: value.parameters, interval_seconds: 10, timeout_seconds: 60, run_at: 0, permissions: value.id === "hdsky" ? ["limits", "tags", "reannounce"] : [], next_run: 0 })
  const errorMessage = error || (query.error instanceof Error ? query.error.message : "")

  return <section className="rounded-lg border p-4 sm:p-6 space-y-4 min-w-0">
    <div className="flex flex-wrap items-center justify-between gap-3"><h1 className="text-2xl font-semibold">{t("scripts.title")}</h1><div className="flex flex-wrap gap-2"><ScriptApiReference /><Button variant="outline" onClick={() => editScript({ ...newScript })}>{t("scripts.newScript")}</Button></div></div>
    <p className="text-sm text-muted-foreground">{t("scripts.description")}</p>
    <p className="text-sm rounded-md border border-amber-500/30 p-3">{t("scripts.trustedWarning")}</p>
    {query.data && !query.data.python_available && <p className="text-sm text-destructive">{t("scripts.pythonMissing")}</p>}
    {errorMessage && <p role="alert" className="text-sm text-destructive break-words">{errorMessage}</p>}
    <div className="grid gap-3 md:grid-cols-2">
      {query.data?.scripts.map(value => <div key={value.id} className="rounded-md border p-3 space-y-3 min-w-0">
        <div className="flex flex-wrap gap-2 items-center"><span className="font-medium break-words">{value.name}</span>{value.builtin && <Badge variant="outline">{t("scripts.builtin")}</Badge>}</div>
        <div className="flex flex-wrap gap-2"><Button size="sm" onClick={() => createTask(value)}>{t("scripts.newTask")}</Button><Button size="sm" variant="outline" onClick={() => editScript(value)}>{t(value.builtin ? "scripts.viewCode" : "scripts.edit")}</Button><Button size="sm" variant="outline" onClick={() => editScript({ ...value, id: "", builtin: false })}>{t("scripts.copy")}</Button>{!value.builtin && <Button size="sm" variant="ghost" onClick={() => setRemove({ id: value.id, kind: "scripts" })}>{t("scripts.delete")}</Button>}</div>
      </div>)}
    </div>
    <h3 className="font-medium">{t("scripts.tasks")}</h3>
    {query.data?.tasks.length === 0 && <p className="text-sm text-muted-foreground">{t("scripts.empty")}</p>}
    <div className="space-y-3">{query.data?.tasks.map(value => <div key={value.id} className="border rounded-md p-3 space-y-3">
      <div className="flex flex-wrap items-center gap-2"><span className="font-medium break-words">{value.name}</span><Badge variant="outline">{t(value.enabled ? "scripts.enabled" : "scripts.disabled")}</Badge></div>
      <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">{value.all_instances ? t("scripts.allInstances") : value.instance_ids.map(id => <Badge key={id} variant="outline">{instances?.find(i => i.id === id)?.name ?? id}</Badge>)}{value.next_run > 0 && <span>{t("scripts.nextRun", { time: new Date(value.next_run * 1000).toLocaleString() })}</span>}</div>
      <div className="flex flex-wrap gap-2">
        <Button size="sm" variant="outline" disabled={busy || !query.data?.python_available} onClick={() => act(async () => { await api.runScriptTask(value.id, true); setHistory(value) })}>{t("scripts.dryRun")}</Button>
        <Button size="sm" variant="outline" disabled={busy || !query.data?.python_available} onClick={() => act(async () => { await api.runScriptTask(value.id, false); setHistory(value) })}>{t("scripts.run")}</Button>
        <Button size="sm" variant="outline" onClick={() => editTask(value)}>{t("scripts.edit")}</Button>
        <Button size="sm" variant="ghost" onClick={() => setHistory(value)}>{t("scripts.history")}</Button>
        <Button size="sm" variant="ghost" onClick={() => setRemove({ id: value.id, kind: "tasks" })}>{t("scripts.delete")}</Button>
      </div>
    </div>)}</div>
    <Dialog open={!!script} onOpenChange={open => !open && setScript(null)}><DialogContent className="sm:max-w-3xl max-h-[90dvh] overflow-y-auto"><DialogHeader><DialogTitle>{t("scripts.scriptEditor")}</DialogTitle><DialogDescription>{t("scripts.sdkHint")}</DialogDescription></DialogHeader>
      {script && <div className="space-y-4">
        <ScriptApiReference />
        <Label className="grid gap-2">{t("scripts.name")}<Input value={script.name} disabled={script.builtin} onChange={e => setScript({ ...script, name: e.target.value })} /></Label>
        <Label className="grid gap-2">{t("scripts.scriptDescription")}<Input value={script.description} disabled={script.builtin} onChange={e => setScript({ ...script, description: e.target.value })} /></Label>
        <Label className="grid gap-2">{t("scripts.code")}<Textarea className="font-mono text-xs min-h-64" spellCheck={false} value={script.code} readOnly={script.builtin} onChange={e => setScript({ ...script, code: e.target.value })} /></Label>
        <Label className="grid gap-2">{t("scripts.parameters")}<Textarea className="font-mono text-xs" value={parameters} readOnly={script.builtin} onChange={e => setParameters(e.target.value)} /></Label>
        {error && <p role="alert" className="text-destructive text-sm">{error}</p>}
        {!script.builtin && <Button disabled={busy} onClick={() => act(async () => { await api.saveScript({ ...script, parameters: JSON.parse(parameters) }); setScript(null) })}>{t("scripts.save")}</Button>}
      </div>}
    </DialogContent></Dialog>
    <Dialog open={!!task} onOpenChange={open => !open && setTask(null)}><DialogContent className="sm:max-w-2xl max-h-[90dvh] overflow-y-auto"><DialogHeader><DialogTitle>{t("scripts.taskEditor")}</DialogTitle><DialogDescription>{t("scripts.taskHint")}</DialogDescription></DialogHeader>
      {task && <div className="space-y-4">
        <Label className="grid gap-2">{t("scripts.name")}<Input value={task.name} onChange={e => setTask({ ...task, name: e.target.value })} /></Label>
        <Label className="flex items-center gap-2"><Checkbox checked={task.enabled} onCheckedChange={checked => setTask({ ...task, enabled: checked === true })} />{t("scripts.enabled")}</Label>
        {task.script_id !== "hdsky" && <Label className="flex items-center gap-2"><Checkbox checked={task.all_instances} onCheckedChange={checked => setTask({ ...task, all_instances: checked === true })} />{t("scripts.allInstances")}</Label>}
        <fieldset className="border rounded-md p-3"><legend className="text-sm px-1">{t("scripts.instances")}</legend><div className="flex flex-wrap gap-3">{instances?.map(i => <Label key={i.id} className="flex items-center gap-2"><Checkbox checked={task.instance_ids.includes(i.id)} disabled={task.all_instances} onCheckedChange={checked => setTask({ ...task, instance_ids: checked ? [...task.instance_ids, i.id] : task.instance_ids.filter(id => id !== i.id) })} />{i.name}</Label>)}</div></fieldset>
        <ScriptTrackerSelect instanceIds={task.all_instances ? (instances ?? []).filter(i => i.isActive).map(i => i.id) : task.instance_ids} selected={task.tracker_domains ?? []} onChange={tracker_domains => setTask({ ...task, tracker_domains })} />
        <div className="grid grid-cols-2 gap-3"><Label className="grid gap-2">{t("scripts.interval")}<Input type="number" min="0" max="86400" value={task.interval_seconds} onChange={e => setTask({ ...task, interval_seconds: Number(e.target.value), run_at: 0 })} /></Label><Label className="grid gap-2">{t("scripts.timeout")}<Input type="number" min="1" max="3600" value={task.timeout_seconds} onChange={e => setTask({ ...task, timeout_seconds: Number(e.target.value) })} /></Label></div>
        <Label className="grid gap-2">{t("scripts.runAt")}<Input type="datetime-local" value={task.run_at ? new Date(task.run_at * 1000 - new Date(task.run_at * 1000).getTimezoneOffset() * 60000).toISOString().slice(0, 16) : ""} onChange={e => setTask({ ...task, run_at: e.target.value ? Math.floor(new Date(e.target.value).getTime() / 1000) : 0, interval_seconds: 0 })} /></Label>
        <Label className="grid gap-2">{t("scripts.parameters")}<Textarea className="font-mono text-xs" value={parameters} onChange={e => setParameters(e.target.value)} /></Label>
        {task.script_id === "hdsky" && <p className="text-sm rounded-md border p-3">{t("scripts.hdskyHint")}</p>}
        <fieldset className="border rounded-md p-3"><legend className="text-sm px-1">{t("scripts.permissions")}</legend><div className="flex flex-wrap gap-3">{permissions.map(permission => <Label key={permission} className="flex items-center gap-2"><Checkbox checked={task.permissions.includes(permission)} onCheckedChange={checked => setTask({ ...task, permissions: checked ? [...task.permissions, permission] : task.permissions.filter(p => p !== permission) })} />{t(`scripts.permission.${permission}`)}</Label>)}</div></fieldset>
        {error && <p role="alert" className="text-destructive text-sm">{error}</p>}
        <Button disabled={busy || (task.script_id === "hdsky" && !task.tracker_domains?.length)} onClick={() => act(async () => { await api.saveScriptTask({ ...task, parameters: JSON.parse(parameters) }); setTask(null) })}>{t("scripts.save")}</Button>
      </div>}
    </DialogContent></Dialog>
    <Dialog open={!!history} onOpenChange={open => !open && setHistory(null)}><DialogContent className="sm:max-w-3xl max-h-[90dvh] overflow-y-auto"><DialogHeader><DialogTitle>{t("scripts.history")} · {history?.name}</DialogTitle><DialogDescription>{t("scripts.historyHint")}</DialogDescription></DialogHeader>
      <Button variant="outline" disabled={busy} onClick={() => history && act(() => api.cancelScriptTask(history.id))}>{t("scripts.cancelRun")}</Button>
      {runs.error && <p role="alert" className="text-destructive">{runs.error.message}</p>}
      {runs.data?.map(run => <details key={run.id} className="rounded-md border p-3"><summary className="cursor-pointer text-sm flex flex-wrap gap-2"><span>{new Date(run.started_at * 1000).toLocaleString()}</span><Badge variant="outline">{t(`scripts.status.${["running", "success", "failed", "canceled", "timeout"].includes(run.status) ? run.status : "failed"}`)}</Badge>{run.dry_run && <Badge variant="outline">{t("scripts.dryRun")}</Badge>}</summary><pre className="mt-3 text-xs whitespace-pre-wrap break-all max-h-80 overflow-y-auto">{run.log || t("scripts.noLog")}</pre></details>)}
    </DialogContent></Dialog>
    <Dialog open={!!remove} onOpenChange={open => !open && setRemove(null)}><DialogContent><DialogHeader><DialogTitle>{t("scripts.delete")}</DialogTitle><DialogDescription>{t("scripts.deleteWarning")}</DialogDescription></DialogHeader>{error && <p role="alert" className="text-destructive">{error}</p>}<Button variant="destructive" disabled={busy} onClick={() => remove && act(async () => { await api.deleteScriptItem(remove.kind, remove.id); setRemove(null) })}>{t("scripts.delete")}</Button></DialogContent></Dialog>
  </section>
}
