import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Checkbox } from "@/components/ui/checkbox"
import { useInstances } from "@/hooks/useInstances"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

const COLUMN_STORAGE_KEYS = [
  "qui-column-visibility",
  "qui-column-sorting",
  "qui-column-sizing",
  "qui-column-order",
] as const

interface SyncColumnSettingsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  sourceInstanceId: number
}

export function SyncColumnSettingsDialog({
  open,
  onOpenChange,
  sourceInstanceId,
}: SyncColumnSettingsDialogProps) {
  const { t } = useTranslation("torrents")
  const { instances } = useInstances()
  const [targets, setTargets] = useState<number[]>([])
  const [copying, setCopying] = useState(false)

  const otherInstances = (instances ?? []).filter(i => i.id !== sourceInstanceId)

  // Unified view (id 0) is a valid target so a single instance's layout can be
  // pushed to the All Instances view. Exclude it when the source IS the unified
  // view (handled by the dedicated toolbar button there).
  const hasUnifiedTarget = sourceInstanceId !== 0

  const copySettings = () => {
    if (targets.length === 0) return
    setCopying(true)
    try {
      // Read the source instance's column settings from localStorage.
      const payload = COLUMN_STORAGE_KEYS.map(key => {
        const value = localStorage.getItem(`${key}:${sourceInstanceId}`)
        return { key, value }
      })

      // Write them to each selected target instance.
      for (const targetId of targets) {
        for (const { key, value } of payload) {
          if (value === null) continue
          localStorage.setItem(`${key}:${targetId}`, value)
        }
      }

      toast.success(t("columnSync.success", { count: targets.length }))
      onOpenChange(false)
      setTargets([])
    } catch {
      toast.error(t("columnSync.failed"))
    } finally {
      setCopying(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{t("columnSync.title")}</DialogTitle>
          <DialogDescription>{t("columnSync.description")}</DialogDescription>
        </DialogHeader>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={copying}
            onClick={() => setTargets(otherInstances.map(i => i.id).concat(hasUnifiedTarget ? [0] : []))}
          >
            {t("columnSync.selectAll")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={copying}
            onClick={() => setTargets([])}
          >
            {t("columnSync.deselectAll")}
          </Button>
        </div>
        <div className="space-y-1 max-h-60 overflow-y-auto">
          {hasUnifiedTarget && (
            <label className="flex items-center gap-2 py-1 px-2 rounded hover:bg-accent/50 cursor-pointer">
              <Checkbox
                checked={targets.includes(0)}
                disabled={copying}
                onCheckedChange={checked => {
                  setTargets(prev =>
                    checked ? [...prev, 0] : prev.filter(id => id !== 0)
                  )
                }}
              />
              <span className="text-sm">{t("columnSync.unifiedView")}</span>
            </label>
          )}
          {otherInstances.length === 0 && !hasUnifiedTarget ? (
            <p className="text-sm text-muted-foreground px-2 py-1">
              {t("columnSync.noOtherInstances")}
            </p>
          ) : (
            otherInstances.map(inst => (
              <label
                key={inst.id}
                className="flex items-center gap-2 py-1 px-2 rounded hover:bg-accent/50 cursor-pointer"
              >
                <Checkbox
                  checked={targets.includes(inst.id)}
                  disabled={copying}
                  onCheckedChange={checked => {
                    setTargets(prev =>
                      checked ? [...prev, inst.id] : prev.filter(id => id !== inst.id)
                    )
                  }}
                />
                <span className="text-sm">{inst.name}</span>
              </label>
            ))
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={copying}>
            {t("columnSync.cancel")}
          </Button>
          <Button disabled={targets.length === 0 || copying} onClick={copySettings}>
            {copying ? t("columnSync.copying") : t("columnSync.confirm", { count: targets.length })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
