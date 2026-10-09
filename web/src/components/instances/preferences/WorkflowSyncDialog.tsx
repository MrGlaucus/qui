/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { api } from "@/lib/api"
import type { Automation, AutomationSyncOptions, InstanceResponse } from "@/types"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { FieldHelp } from "@/components/ui/field-help"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

interface Props {
  rule: Automation
  instances: InstanceResponse[]
  onClose: () => void
}

export function WorkflowSyncDialog({ rule, instances, onClose }: Props) {
  const { t } = useTranslation("instances")
  const queryClient = useQueryClient()
  const data = useQuery({
    queryKey: ["automation-sync", rule.instanceId, rule.id, instances.map(i => i.id)],
    queryFn: async () => {
      const [peers, targets] = await Promise.all([
        api.getAutomationSync(rule.instanceId, rule.id),
        Promise.all(instances.filter(i => i.id !== rule.instanceId).map(async instance => ({
          instance,
          rules: (await api.listAutomations(instance.id)) ?? [],
        }))),
      ])
      return { peers, targets }
    },
    enabled: !rule.syncSourceId,
    staleTime: 0,
    gcTime: 0,
    refetchOnWindowFocus: false,
  })
  const detach = useMutation({
    mutationFn: () => api.detachAutomationSync(rule.instanceId, rule.id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["automations"] })
      onClose()
    },
    onError: () => toast.error(t("preferences.workflowSync.failed")),
  })
  return (
    <Dialog open onOpenChange={open => { if (!open) onClose() }}>
      <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("preferences.workflowSync.title")}</DialogTitle>
          <DialogDescription>{rule.name}</DialogDescription>
        </DialogHeader>
        {rule.syncSourceId ? (
          <>
            <p className="text-sm">{t("preferences.workflowSync.following", { instance: instances.find(i => i.id === rule.syncSourceInstanceId)?.name ?? rule.syncSourceInstanceId })}</p>
            <p className="text-sm text-muted-foreground">{t("preferences.workflowSync.detachHelp")}</p>
            <DialogFooter>
              <Button variant="outline" onClick={onClose}>{t("common:actions.cancel")}</Button>
              <Button disabled={detach.isPending} onClick={() => detach.mutate()}>{t("preferences.workflowSync.detach")}</Button>
            </DialogFooter>
          </>
        ) : data.isPending ? <p>{t("common:mobileNav.loading")}</p> : data.isError ? (
          <div className="space-y-2">
            <p role="alert">{t("preferences.workflowSync.failed")}</p>
            <Button onClick={() => void data.refetch()}>{t("preferences.workflowSync.retry")}</Button>
          </div>
        ) : (
          <SyncForm rule={rule} peers={data.data.peers} targets={data.data.targets} onClose={onClose} />
        )}
      </DialogContent>
    </Dialog>
  )
}

function SyncForm({ rule, peers, targets, onClose }: {
  rule: Automation
  peers: Automation[]
  targets: { instance: InstanceResponse; rules: Automation[] }[]
  onClose: () => void
}) {
  const { t } = useTranslation("instances")
  const queryClient = useQueryClient()
  const [automatic, setAutomatic] = useState(peers.length === 0 || peers.some(p => p.syncSourceId === rule.id))
  const [selected, setSelected] = useState<AutomationSyncOptions["targets"]>(() => peers.map(p => ({
    instanceId: p.instanceId, ruleId: p.id, preserveEnabled: p.syncPreserveEnabled ?? true,
  })))
  const save = useMutation({
    mutationFn: () => api.configureAutomationSync(rule.instanceId, rule.id, { automatic, targets: selected }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["automations"] })
      await queryClient.invalidateQueries({ queryKey: ["automation-sync"] })
      toast.success(t("preferences.workflowSync.saved"))
      onClose()
    },
    onError: () => toast.error(t("preferences.workflowSync.failed")),
  })
  return (
    <div className="space-y-4">
      <label className="flex items-center gap-2 text-sm font-medium">
        <Checkbox checked={automatic} onCheckedChange={value => setAutomatic(value === true)} disabled={save.isPending} />
        {t("preferences.workflowSync.automatic")}
        <FieldHelp>{t("preferences.workflowSync.automaticHelp")}</FieldHelp>
      </label>
      <p className="text-sm text-muted-foreground">{t("preferences.workflowSync.warning")}</p>
      <div className="space-y-2">
        {targets.map(({ instance, rules }) => {
          const target = selected.find(s => s.instanceId === instance.id)
          const peer = peers.find(p => p.instanceId === instance.id)
          return (
            <div key={instance.id} className="rounded-md border p-3 space-y-3">
              <label className="flex items-center gap-2 text-sm font-medium">
                <Checkbox checked={!!target} disabled={save.isPending} onCheckedChange={checked => setSelected(prev => checked
                  ? [...prev, { instanceId: instance.id, ruleId: peer?.id, preserveEnabled: peer?.syncPreserveEnabled ?? true }]
                  : prev.filter(s => s.instanceId !== instance.id))} />
                {instance.name}
              </label>
              {target && (
                <div className="space-y-3 pl-6">
                  <Select disabled={!!peer || save.isPending} value={target.ruleId === undefined ? "auto" : String(target.ruleId)} onValueChange={value => setSelected(prev => prev.map(s => s.instanceId === instance.id ? { ...s, ruleId: value === "auto" ? undefined : Number(value) } : s))}>
                    <SelectTrigger className="w-full" aria-label={t("preferences.workflowSync.targetRule")}><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="auto">{t("preferences.workflowSync.match")}</SelectItem>
                      <SelectItem value="0">{t("preferences.workflowSync.create")}</SelectItem>
                      {rules.filter(r => r.id === peer?.id || (!r.syncSourceId && !r.syncFollowerCount)).map(r => <SelectItem key={r.id} value={String(r.id)}>{r.name}</SelectItem>)}
                    </SelectContent>
                  </Select>
                  <label className="flex items-center gap-2 text-sm">
                    <Checkbox checked={target.preserveEnabled} disabled={save.isPending} onCheckedChange={checked => setSelected(prev => prev.map(s => s.instanceId === instance.id ? { ...s, preserveEnabled: checked === true } : s))} />
                    {t("preferences.workflowSync.preserveEnabled")}
                  </label>
                </div>
              )}
            </div>
          )
        })}
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onClose} disabled={save.isPending}>{t("common:actions.cancel")}</Button>
        <Button onClick={() => save.mutate()} disabled={save.isPending}>{t("preferences.workflowSync.save")}</Button>
      </DialogFooter>
    </div>
  )
}
