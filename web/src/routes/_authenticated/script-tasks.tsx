import { createFileRoute } from "@tanstack/react-router"
import { ScriptTasksPage } from "@/pages/ScriptTasks"

export const Route = createFileRoute("/_authenticated/script-tasks")({
  component: ScriptTasksPage,
})
