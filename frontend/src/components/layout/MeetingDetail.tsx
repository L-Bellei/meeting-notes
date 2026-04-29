import { useState, useEffect, useRef } from "react"
import { Play, Square, RefreshCw, Wand2, Trash2 } from "lucide-react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import {
  useMeeting, useUpdateMeeting, useStartRecording, useStopRecording,
  useReprocess, useGenerateSummary, useGenerateKeyPoints, useGenerateTasks,
  useUpdateTask,
} from "../../hooks/useMeeting"
import { useDeleteMeeting } from "../../hooks/useMeetings"
import { useSettings } from "../../hooks/useSettings"
import { useCardForMeeting, useCreateCard } from "../../hooks/useBoard"
import { Badge } from "../ui/badge"
import { Button } from "../ui/button"
import { Spinner } from "../ui/spinner"
import { cn } from "../../lib/utils"

interface Props { meetingId: string | null; onDeleted?: () => void }

type Tab = "transcript" | "summary" | "keypoints" | "tasks" | "notes"

function statusVariant(s: string) {
  return s as any
}

export function MeetingDetail({ meetingId, onDeleted }: Props) {
  const { data: meeting } = useMeeting(meetingId)
  const { data: settings } = useSettings()
  const [tab, setTab] = useState<Tab>("transcript")

  const generateSummary   = useGenerateSummary(meetingId ?? "")
  const generateKeyPoints = useGenerateKeyPoints(meetingId ?? "")
  const generateTasks     = useGenerateTasks(meetingId ?? "")

  useEffect(() => {
    if (
      meeting?.status === "completed" &&
      settings?.auto_generate === "true" &&
      !meeting.summary &&
      (!meeting.key_points || meeting.key_points.length === 0) &&
      (!meeting.tasks || meeting.tasks.length === 0) &&
      !generateSummary.isPending &&
      !generateKeyPoints.isPending &&
      !generateTasks.isPending
    ) {
      generateSummary.mutate()
      generateKeyPoints.mutate()
      generateTasks.mutate()
    }
  }, [
    meeting?.status,
    meeting?.id,
    meeting?.summary,
    meeting?.key_points,
    meeting?.tasks,
    settings?.auto_generate,
    generateSummary,
    generateKeyPoints,
    generateTasks,
  ])

  const { data: existingCard } = useCardForMeeting(meetingId)
  const createCard = useCreateCard()

  if (!meetingId || !meeting) {
    return (
      <div className="flex-1 h-full flex items-center justify-center text-sm text-muted-foreground animate-fade-in">
        Selecione uma reunião
      </div>
    )
  }

  const tabLabels: Record<Tab, string> = {
    transcript: "Transcrição", summary: "Resumo", keypoints: "Pontos-chave", tasks: "Tarefas", notes: "Notas",
  }

  return (
    <div className="flex-1 h-full flex flex-col overflow-hidden animate-fade-in">
      <MeetingHeader meeting={meeting} onDeleted={onDeleted} />
      {meetingId && !existingCard && meeting?.status === "completed" && (
        <div className="px-4 pb-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => createCard.mutate({ meeting_id: meetingId })}
            disabled={createCard.isPending}
          >
            Adicionar ao Board
          </Button>
        </div>
      )}
      {existingCard && (
        <div className="px-4 pb-2">
          <span className="text-xs text-muted-foreground bg-muted px-2 py-1 rounded-full">
            #{existingCard.number} · {existingCard.status}
          </span>
        </div>
      )}
      <div className="px-4 pt-3 pb-0 border-b border-border flex-shrink-0">
        <div className="flex gap-1">
          {(["transcript", "summary", "keypoints", "tasks", "notes"] as Tab[]).map(t => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={cn(
                "px-3 py-1.5 text-xs font-medium rounded-full transition-colors",
                tab === t
                  ? "bg-primary/20 text-primary"
                  : "text-muted-foreground hover:text-foreground hover:bg-accent"
              )}
            >
              {tabLabels[t]}
            </button>
          ))}
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        {tab === "transcript" && <TranscriptTab meeting={meeting} />}
        {tab === "summary" && <SummaryTab meeting={meeting} />}
        {tab === "keypoints" && <KeyPointsTab meeting={meeting} />}
        {tab === "tasks" && <TasksTab meeting={meeting} />}
        {tab === "notes" && <NotesTab meeting={meeting} />}
      </div>
    </div>
  )
}

function MeetingHeader({ meeting, onDeleted }: { meeting: any; onDeleted?: () => void }) {
  const start = useStartRecording(meeting.id)
  const stop = useStopRecording(meeting.id)
  const reprocess = useReprocess(meeting.id)
  const deleteMeeting = useDeleteMeeting()
  const [error, setError] = useState("")
  const [confirmDelete, setConfirmDelete] = useState(false)

  async function handleStart() {
    try { await start.mutateAsync() } catch (e: any) { setError(e.message) }
  }
  async function handleStop() {
    try { await stop.mutateAsync() } catch (e: any) { setError(e.message) }
  }
  async function handleReprocess() {
    try { await reprocess.mutateAsync() } catch (e: any) { setError(e.message) }
  }
  async function handleDelete() {
    if (!confirmDelete) { setConfirmDelete(true); return }
    await deleteMeeting.mutateAsync(meeting.id)
    onDeleted?.()
  }

  return (
    <div className="px-4 py-3 border-b border-border flex-shrink-0">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-base font-semibold truncate">{meeting.title}</h2>
        <div className="flex items-center gap-1.5 flex-shrink-0">
          <Badge variant={statusVariant(meeting.status)}>{meeting.status}</Badge>
          <button
            onClick={handleDelete}
            title={confirmDelete ? "Clique novamente para confirmar" : "Excluir reunião"}
            className={cn(
              "p-1 rounded transition-colors",
              confirmDelete
                ? "text-destructive bg-destructive/20"
                : "text-muted-foreground hover:text-destructive hover:bg-destructive/10"
            )}
          >
            <Trash2 size={14} />
          </button>
        </div>
      </div>
      {error && <p className="text-xs text-destructive mt-1">{error}</p>}
      <div className="flex gap-2 mt-2">
        {(meeting.status === "pending" || meeting.status === "failed") && (
          <Button size="sm" onClick={handleStart} disabled={start.isPending}>
            {start.isPending ? <Spinner size={14} className="mr-1.5" /> : <Play size={14} className="mr-1" />}
            Start
          </Button>
        )}
        {meeting.status === "recording" && (
          <Button size="sm" variant="destructive" onClick={handleStop} disabled={stop.isPending}>
            {stop.isPending ? <Spinner size={14} className="mr-1.5" /> : <Square size={14} className="mr-1" />}
            Stop
          </Button>
        )}
        {(meeting.status === "failed" || meeting.status === "completed") && meeting.transcript && (
          <Button size="sm" variant="outline" onClick={handleReprocess} disabled={reprocess.isPending}>
            {reprocess.isPending ? <Spinner size={14} className="mr-1.5" /> : <RefreshCw size={14} className="mr-1" />}
            Reprocessar
          </Button>
        )}
      </div>
    </div>
  )
}

function TranscriptTab({ meeting }: { meeting: any }) {
  const update = useUpdateMeeting(meeting.id)
  const [value, setValue] = useState(meeting.transcript ?? "")
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => { setValue(meeting.transcript ?? "") }, [meeting.transcript])

  function handleChange(v: string) {
    setValue(v)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => update.mutate({ ...meeting, transcript: v }), 1000)
  }

  return (
    <textarea
      value={value}
      onChange={e => handleChange(e.target.value)}
      placeholder="Nenhuma transcrição ainda..."
      className="w-full h-full min-h-[300px] text-sm rounded-xl p-3 resize-none focus:outline-none focus:ring-1 focus:ring-primary bg-muted/40 border border-border"
    />
  )
}

function SummaryTab({ meeting }: { meeting: any }) {
  const generate = useGenerateSummary(meeting.id)
  return (
    <div>
      <div className="flex justify-end mb-3">
        <Button size="sm" onClick={() => generate.mutate()} disabled={generate.isPending || !meeting.transcript}>
          {generate.isPending ? <Spinner size={14} className="mr-1.5" /> : <Wand2 size={14} className="mr-1" />}
          {generate.isPending ? "Gerando..." : "Gerar resumo"}
        </Button>
      </div>
      {meeting.summary ? (
        <p className="text-sm leading-relaxed whitespace-pre-wrap">{meeting.summary.content}</p>
      ) : (
        <p className="text-sm text-muted-foreground">Nenhum resumo ainda.</p>
      )}
    </div>
  )
}

function KeyPointsTab({ meeting }: { meeting: any }) {
  const generate = useGenerateKeyPoints(meeting.id)
  return (
    <div>
      <div className="flex justify-end mb-3">
        <Button size="sm" onClick={() => generate.mutate()} disabled={generate.isPending || !meeting.transcript}>
          {generate.isPending ? <Spinner size={14} className="mr-1.5" /> : <Wand2 size={14} className="mr-1" />}
          {generate.isPending ? "Gerando..." : "Gerar pontos"}
        </Button>
      </div>
      {meeting.key_points.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nenhum ponto-chave ainda.</p>
      ) : (
        <ol className="space-y-2">
          {meeting.key_points.map((kp: any, i: number) => (
            <li key={kp.id} className="flex gap-2 text-sm">
              <span className="text-muted-foreground w-5 flex-shrink-0">{i + 1}.</span>
              <span>{kp.content}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}

function TasksTab({ meeting }: { meeting: any }) {
  const generate = useGenerateTasks(meeting.id)
  return (
    <div>
      <div className="flex justify-end mb-3">
        <Button size="sm" onClick={() => generate.mutate()} disabled={generate.isPending || !meeting.transcript}>
          {generate.isPending ? <Spinner size={14} className="mr-1.5" /> : <Wand2 size={14} className="mr-1" />}
          {generate.isPending ? "Gerando..." : "Gerar tarefas"}
        </Button>
      </div>
      {meeting.tasks.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nenhuma tarefa ainda.</p>
      ) : (
        <ul className="space-y-2">
          {meeting.tasks.map((task: any) => (
            <TaskItem key={task.id} task={task} meetingId={meeting.id} />
          ))}
        </ul>
      )}
    </div>
  )
}

function TaskItem({ task, meetingId }: { task: any; meetingId: string }) {
  const update = useUpdateTask(meetingId, task.id)
  return (
    <li className="flex items-start gap-2 text-sm">
      <input
        type="checkbox"
        checked={task.completed}
        onChange={e => update.mutate({ ...task, completed: e.target.checked })}
        className="mt-0.5"
      />
      <div className={cn("flex-1", task.completed && "line-through text-muted-foreground")}>
        <span>{task.description}</span>
        {task.assignee && <span className="ml-2 text-xs text-muted-foreground">@{task.assignee}</span>}
      </div>
    </li>
  )
}

type NotesMode = "edit" | "preview"

function NotesTab({ meeting }: { meeting: any }) {
  const update = useUpdateMeeting(meeting.id)
  const [value, setValue] = useState(meeting.notes ?? "")
  const [mode, setMode] = useState<NotesMode>("edit")
  const [showToolbar, setShowToolbar] = useState(true)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => { setValue(meeting.notes ?? "") }, [meeting.notes])

  function handleChange(v: string) {
    setValue(v)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => update.mutate({ ...meeting, notes: v }), 1000)
  }

  function insertMarkdown(before: string, after = "", placeholder = "texto") {
    const el = textareaRef.current
    if (!el) return
    const start = el.selectionStart
    const end = el.selectionEnd
    const selected = value.slice(start, end) || placeholder
    const newVal = value.slice(0, start) + before + selected + after + value.slice(end)
    handleChange(newVal)
    setTimeout(() => {
      el.focus()
      el.setSelectionRange(start + before.length, start + before.length + selected.length)
    }, 0)
  }

  const toolbarActions = [
    { label: "B",   title: "Bold",         action: () => insertMarkdown("**", "**", "negrito") },
    { label: "I",   title: "Italic",       action: () => insertMarkdown("*", "*", "itálico") },
    { label: "H1",  title: "Heading 1",    action: () => insertMarkdown("# ", "", "Título") },
    { label: "H2",  title: "Heading 2",    action: () => insertMarkdown("## ", "", "Subtítulo") },
    { label: "|",   title: "",             action: () => {} },
    { label: "•",   title: "Bullet list",  action: () => insertMarkdown("- ", "", "item") },
    { label: "1.",  title: "Ordered list", action: () => insertMarkdown("1. ", "", "item") },
    { label: "`",   title: "Inline code",  action: () => insertMarkdown("`", "`", "código") },
    { label: "```", title: "Code block",   action: () => insertMarkdown("```\n", "\n```", "código") },
  ]

  return (
    <div className="flex flex-col h-full gap-2">
      <div className="flex items-center justify-between flex-shrink-0">
        <button
          onClick={() => setShowToolbar(v => !v)}
          className="text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          {showToolbar ? "Ocultar toolbar" : "Mostrar toolbar"}
        </button>
        <div className="flex rounded-lg overflow-hidden border border-border">
          {(["edit", "preview"] as NotesMode[]).map(m => (
            <button
              key={m}
              onClick={() => setMode(m)}
              className={cn(
                "px-3 py-1 text-xs font-medium transition-colors",
                mode === m ? "bg-primary/20 text-primary" : "text-muted-foreground hover:bg-accent"
              )}
            >
              {m === "edit" ? "Editar" : "Preview"}
            </button>
          ))}
        </div>
      </div>

      {showToolbar && mode === "edit" && (
        <div className="flex gap-1 flex-wrap p-2 rounded-lg bg-muted/40 border border-border flex-shrink-0">
          {toolbarActions.map((a, i) =>
            a.label === "|" ? (
              <span key={i} className="w-px h-5 bg-border mx-1 self-center" />
            ) : (
              <button
                key={i}
                title={a.title}
                onClick={a.action}
                className="px-2 py-1 text-xs font-mono rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
              >
                {a.label}
              </button>
            )
          )}
        </div>
      )}

      {mode === "edit" ? (
        <textarea
          ref={textareaRef}
          value={value}
          onChange={e => handleChange(e.target.value)}
          placeholder="Escreva suas notas em Markdown..."
          className="flex-1 min-h-[200px] text-sm rounded-xl p-3 resize-none focus:outline-none focus:ring-1 focus:ring-primary bg-muted/40 border border-border font-mono"
        />
      ) : (
        <div className="flex-1 overflow-y-auto rounded-xl p-4 bg-muted/40 border border-border prose prose-invert prose-sm max-w-none">
          {value
            ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{value}</ReactMarkdown>
            : <p className="text-muted-foreground text-sm">Nenhuma nota ainda.</p>
          }
        </div>
      )}
    </div>
  )
}
