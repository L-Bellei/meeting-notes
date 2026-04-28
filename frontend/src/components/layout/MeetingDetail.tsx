import { useState, useEffect, useRef } from "react"
import { Play, Square, RefreshCw, Wand2 } from "lucide-react"
import {
  useMeeting, useUpdateMeeting, useStartRecording, useStopRecording,
  useReprocess, useGenerateSummary, useGenerateKeyPoints, useGenerateTasks,
  useUpdateTask,
} from "../../hooks/useMeeting"
import { Badge } from "../ui/badge"
import { Button } from "../ui/button"
import { cn } from "../../lib/utils"

interface Props { meetingId: string | null }

type Tab = "transcript" | "summary" | "keypoints" | "tasks"

function statusVariant(s: string) {
  return s as any
}

export function MeetingDetail({ meetingId }: Props) {
  const { data: meeting } = useMeeting(meetingId)
  const [tab, setTab] = useState<Tab>("transcript")

  if (!meetingId || !meeting) {
    return (
      <div className="flex-1 h-full flex items-center justify-center text-sm text-muted-foreground">
        Selecione uma reunião
      </div>
    )
  }

  return (
    <div className="flex-1 h-full flex flex-col overflow-hidden">
      <MeetingHeader meeting={meeting} />
      <div className="border-b">
        <div className="flex">
          {(["transcript", "summary", "keypoints", "tasks"] as Tab[]).map(t => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={cn(
                "px-4 py-2 text-sm font-medium border-b-2 transition-colors",
                tab === t ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground"
              )}
            >
              {{ transcript: "Transcrição", summary: "Resumo", keypoints: "Pontos-chave", tasks: "Tarefas" }[t]}
            </button>
          ))}
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        {tab === "transcript" && <TranscriptTab meeting={meeting} />}
        {tab === "summary" && <SummaryTab meeting={meeting} />}
        {tab === "keypoints" && <KeyPointsTab meeting={meeting} />}
        {tab === "tasks" && <TasksTab meeting={meeting} />}
      </div>
    </div>
  )
}

function MeetingHeader({ meeting }: { meeting: any }) {
  const start = useStartRecording(meeting.id)
  const stop = useStopRecording(meeting.id)
  const reprocess = useReprocess(meeting.id)
  const [error, setError] = useState("")

  async function handleStart() {
    try { await start.mutateAsync() } catch (e: any) { setError(e.message) }
  }
  async function handleStop() {
    try { await stop.mutateAsync() } catch (e: any) { setError(e.message) }
  }
  async function handleReprocess() {
    try { await reprocess.mutateAsync() } catch (e: any) { setError(e.message) }
  }

  return (
    <div className="p-4 border-b">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-base font-semibold truncate">{meeting.title}</h2>
        <Badge variant={statusVariant(meeting.status)}>{meeting.status}</Badge>
      </div>
      {error && <p className="text-xs text-destructive mt-1">{error}</p>}
      <div className="flex gap-2 mt-2">
        {(meeting.status === "pending" || meeting.status === "failed") && (
          <Button size="sm" onClick={handleStart} disabled={start.isPending}>
            <Play size={14} className="mr-1" /> Start
          </Button>
        )}
        {meeting.status === "recording" && (
          <Button size="sm" variant="destructive" onClick={handleStop} disabled={stop.isPending}>
            <Square size={14} className="mr-1" /> Stop
          </Button>
        )}
        {(meeting.status === "failed" || meeting.status === "completed") && meeting.transcript && (
          <Button size="sm" variant="outline" onClick={handleReprocess} disabled={reprocess.isPending}>
            <RefreshCw size={14} className="mr-1" /> Reprocessar
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
    timer.current = setTimeout(() => update.mutate({ transcript: v }), 1000)
  }

  return (
    <textarea
      value={value}
      onChange={e => handleChange(e.target.value)}
      placeholder="Nenhuma transcrição ainda..."
      className="w-full h-full min-h-[300px] text-sm bg-background border rounded p-3 resize-none focus:outline-none focus:ring-1 focus:ring-primary"
    />
  )
}

function SummaryTab({ meeting }: { meeting: any }) {
  const generate = useGenerateSummary(meeting.id)
  return (
    <div>
      <div className="flex justify-end mb-3">
        <Button size="sm" onClick={() => generate.mutate()} disabled={generate.isPending || !meeting.transcript}>
          <Wand2 size={14} className="mr-1" />
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
          <Wand2 size={14} className="mr-1" />
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
          <Wand2 size={14} className="mr-1" />
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
