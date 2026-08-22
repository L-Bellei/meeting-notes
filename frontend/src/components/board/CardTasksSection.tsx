import { useState } from "react"
import { Plus, Trash2 } from "lucide-react"
import { Button } from "../ui/button"
import { useUpdateTask, useGenerateTasks, type Task } from "../../hooks/useMeeting"
import type { BoardCardDetail } from "../../hooks/useBoard"
import { cn } from "../../lib/utils"

const PRIORITY_LABEL: Record<string, string> = { high: "alta", medium: "média", low: "baixa" }

interface Props {
  card: BoardCardDetail
  manualTasks: string[]
  onToggleManual: (index: number) => void
  onAddManual: (text: string) => void
  onRemoveManual: (index: number) => void
}

export function parseManualTask(s: string): { text: string; done: boolean } {
  if (s.startsWith("[x] ")) return { text: s.slice(4), done: true }
  if (s.startsWith("[ ] ")) return { text: s.slice(4), done: false }
  return { text: s, done: false }
}

export function encodeManualTask(text: string, done: boolean): string {
  return `${done ? "[x]" : "[ ]"} ${text}`
}

function TaskRow({ task, meetingId }: { task: Task; meetingId: string }) {
  const updateTask = useUpdateTask(meetingId, task.id)
  return (
    <div>
      <label className="flex items-start gap-2 cursor-pointer">
        <input
          type="checkbox"
          className="mt-0.5 accent-primary flex-shrink-0"
          checked={task.completed}
          onChange={e => updateTask.mutate({ ...task, completed: e.target.checked })}
        />
        <span className={cn("text-sm flex-1", task.completed && "line-through text-muted-foreground")}>
          {task.description}
        </span>
        {task.priority && (
          <span className={cn(
            "text-[10px] font-medium px-1 rounded mt-0.5 flex-shrink-0",
            task.priority === "high" ? "bg-destructive/15 text-destructive" :
            task.priority === "medium" ? "bg-yellow-500/15 text-yellow-600" :
            "bg-muted text-muted-foreground",
          )}>
            {PRIORITY_LABEL[task.priority] ?? task.priority}
          </span>
        )}
        {task.assignee && (
          <span className="text-[10px] text-muted-foreground/70 mt-0.5 flex-shrink-0">{task.assignee}</span>
        )}
      </label>
      {updateTask.isError && (
        <p className="text-xs text-destructive ml-6 mt-0.5">
          Falha ao salvar: {updateTask.error?.message ?? "erro desconhecido"}
        </p>
      )}
    </div>
  )
}

export function CardTasksSection({ card, manualTasks, onToggleManual, onAddManual, onRemoveManual }: Props) {
  const [newTask, setNewTask] = useState("")
  const generateTasks = useGenerateTasks(card.meeting_id ?? "")
  const isManual = card.source === "manual"

  if (isManual) {
    const done = manualTasks.filter(t => parseManualTask(t).done).length
    return (
      <section>
        <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">
          Tasks {manualTasks.length > 0 && `(${done}/${manualTasks.length})`}
        </h3>
        <div className="space-y-1.5 mb-2">
          {manualTasks.map((raw, i) => {
            const { text, done } = parseManualTask(raw)
            return (
              <div key={i} className="flex items-center gap-2 group">
                <input
                  type="checkbox"
                  className="accent-primary flex-shrink-0"
                  checked={done}
                  onChange={() => onToggleManual(i)}
                />
                <span className={cn("text-sm flex-1", done && "line-through text-muted-foreground")}>{text}</span>
                <button
                  onClick={() => onRemoveManual(i)}
                  aria-label="Remover task"
                  className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-all"
                >
                  <Trash2 size={13} />
                </button>
              </div>
            )
          })}
        </div>
        <div className="flex gap-2">
          <input
            className="flex-1 text-sm rounded-lg px-3 py-1.5 bg-input border border-border text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-primary"
            placeholder="Nova task..."
            value={newTask}
            onChange={e => setNewTask(e.target.value)}
            onKeyDown={e => {
              if (e.key !== "Enter" || !newTask.trim()) return
              onAddManual(newTask.trim())
              setNewTask("")
            }}
          />
          <Button
            size="sm"
            variant="ghost"
            disabled={!newTask.trim()}
            onClick={() => { onAddManual(newTask.trim()); setNewTask("") }}
          >
            <Plus size={14} />
          </Button>
        </div>
      </section>
    )
  }

  const done = card.tasks.filter(t => t.completed).length
  return (
    <section>
      <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">
        Tasks {card.tasks.length > 0 && `(${done}/${card.tasks.length})`}
      </h3>
      {card.tasks.length === 0 ? (
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground/70">Nenhuma task nesta reunião.</p>
          {card.has_transcript ? (
            <Button
              size="sm"
              variant="outline"
              disabled={generateTasks.isPending}
              onClick={() => generateTasks.mutate()}
            >
              {generateTasks.isPending ? "Gerando..." : "Gerar tasks"}
            </Button>
          ) : (
            <p className="text-xs text-muted-foreground/60">
              Gerar tasks precisa da transcrição da reunião.
            </p>
          )}
          {generateTasks.isError && (
            <p className="text-xs text-destructive">
              Falha ao gerar: {generateTasks.error?.message ?? "erro desconhecido"}
            </p>
          )}
        </div>
      ) : (
        <div className="space-y-1.5">
          {card.tasks.map(task => (
            <TaskRow key={task.id} task={task} meetingId={card.meeting_id ?? ""} />
          ))}
        </div>
      )}
    </section>
  )
}
