import { useState } from "react"
import { useTranslation } from "react-i18next"
import { BookOpen } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { scriptApiExample } from "./script-api-example"

interface ReferenceEntry {
  signature: string
  result: string
  description: string
}

export function ScriptApiReference() {
  const { t } = useTranslation("automations")
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")
  const entries = t("scripts.reference.entries", { returnObjects: true }) as ReferenceEntry[]
  const query = search.trim().toLowerCase()

  return <>
    <Button variant="outline" onClick={() => setOpen(true)}><BookOpen className="size-4" />{t("scripts.reference.title")}</Button>
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="sm:max-w-4xl max-h-[90dvh] overflow-y-auto min-w-0">
        <DialogHeader>
          <DialogTitle>{t("scripts.reference.title")}</DialogTitle>
          <DialogDescription>{t("scripts.reference.intro")}</DialogDescription>
        </DialogHeader>
        <section className="space-y-2 text-sm">
          <h3 className="font-semibold">{t("scripts.reference.context")}</h3>
          <p className="text-muted-foreground">{t("scripts.reference.contextBody")}</p>
        </section>
        <Input aria-label={t("scripts.reference.search")} placeholder={t("scripts.reference.search")} value={search} onChange={event => setSearch(event.target.value)} />
        <div className="space-y-2">
          {entries.filter(entry => `${entry.signature} ${entry.description} ${entry.result}`.toLowerCase().includes(query)).map(entry => <article key={entry.signature} className="rounded-md border p-3 space-y-2 min-w-0 text-sm">
            <code className="block break-all font-semibold">ctx.{entry.signature}</code>
            <code className="block text-xs text-muted-foreground">{entry.result}</code>
            <p>{entry.description}</p>
          </article>)}
        </div>
        <section className="space-y-2 text-sm">
          <h3 className="font-semibold">{t("scripts.reference.fields")}</h3>
          <p className="whitespace-pre-wrap break-words text-muted-foreground">{t("scripts.reference.fieldsBody")}</p>
        </section>
        <section className="space-y-2 text-sm min-w-0">
          <h3 className="font-semibold">{t("scripts.reference.example")}</h3>
          <p className="text-muted-foreground">{t("scripts.reference.exampleHint")}</p>
          <pre className="rounded-md bg-muted p-3 overflow-x-auto text-xs"><code>{scriptApiExample}</code></pre>
        </section>
      </DialogContent>
    </Dialog>
  </>
}
