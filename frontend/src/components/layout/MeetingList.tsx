import { useState } from "react"
import { Plus, Calendar } from "lucide-react"
import { useMeetings, useCreateMeeting } from "../../hooks/useMeetings"
import { useThemes } from "../../hooks/useThemes"
import { Badge } from "../ui/badge"
import { Waveform } from "../ui/waveform"
import { Button } from "../ui/button"
import { cn } from "../../lib/utils"
import type { Meeting } from "../../hooks/useMeetings"

interface MeetingListProps {
  themeId: string | null
  selectedMeetingId: string | null
  onSelectMeeting: (id: string) => void
}

function statusVariant(s: Meeting["status"]) {
  const map: Record<Meeting["status"], string> = {
    pending: "pending", recording: "recording", transcribing: "transcribing",
    processing: "processing", completed: "completed", failed: "failed",
  }
  return map[s] as any
}

function statusLabel(s: Meeting["status"]) {
  const map: Record<Meeting["status"], string> = {
    pending: "Pendente", recording: "Gravando", transcribing: "Transcrevendo",
    processing: "Processando", completed: "Concluído", failed: "Falhou",
  }
  return map[s]
}

function formatDate(iso: string | null) {
  if (!iso) return ""
  return new Date(iso).toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit", year: "2-digit" })
}

export function MeetingList({ themeId, selectedMeetingId, onSelectMeeting }: MeetingListProps) {
  const { data: meetings = [] } = useMeetings(themeId)
  const { data: themes = [] } = useThemes()
  const createMeeting = useCreateMeeting()
  const [creating, setCreating] = useState(false)
  const [newTitle, setNewTitle] = useState("")

  async function handleCreate() {
    if (!newTitle.trim()) return
    const m = await createMeeting.mutateAsync({ title: newTitle.trim(), theme_id: themeId ?? undefined })
    setNewTitle("")
    setCreating(false)
    onSelectMeeting(m.id)
  }

  function themeColor(id: string | null) {
    return themes.find(t => t.id === id)?.color ?? "#8b7aaa"
  }

  return (
    <div className="w-72 border-r border-border h-full flex flex-col">
      <div className="h-14 px-4 border-b border-border flex items-center justify-between flex-shrink-0">
        <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Reuniões</span>
        <span className="text-xs text-muted-foreground">{meetings.length}</span>
      </div>
      <div className="flex-1 overflow-y-auto px-2 py-2 space-y-1">
        {meetings.length === 0 && (
          <div className="p-4 text-center text-sm text-muted-foreground">Nenhuma reunião</div>
        )}
        {meetings.map(m => (
          <button
            key={m.id}
            onClick={() => onSelectMeeting(m.id)}
            className={cn(
              "w-full text-left rounded-xl px-3 py-3 hover:bg-accent transition-colors",
              selectedMeetingId === m.id ? "bg-accent" : "bg-accent/40"
            )}
          >
            <div className="flex items-start justify-between gap-2">
              <div className="flex items-center gap-2 min-w-0">
                <span className="w-2 h-2 rounded-full flex-shrink-0 mt-0.5" style={{ backgroundColor: themeColor(m.theme_id) }} />
                <span className="text-sm font-medium truncate">{m.title}</span>
              </div>
              {m.status === "recording" ? (
                <Waveform className="text-red-400 flex-shrink-0" />
              ) : (
                <Badge variant={statusVariant(m.status)} className="flex-shrink-0 text-[10px]">
                  {statusLabel(m.status)}
                </Badge>
              )}
            </div>
            {m.started_at && (
              <div className="flex items-center gap-1 mt-1 ml-4 text-xs text-muted-foreground">
                <Calendar size={10} />
                {formatDate(m.started_at)}
              </div>
            )}
          </button>
        ))}
      </div>
      <div className="p-3 border-t border-border">
        {creating ? (
          <div className="flex gap-1">
            <input
              autoFocus
              value={newTitle}
              onChange={e => setNewTitle(e.target.value)}
              onKeyDown={e => { if (e.key === "Enter") handleCreate(); if (e.key === "Escape") setCreating(false) }}
              placeholder="Título da reunião"
              className="flex-1 text-xs rounded-lg px-2 py-1.5"
            />
            <Button size="sm" onClick={handleCreate} disabled={createMeeting.isPending}>+</Button>
          </div>
        ) : (
          <Button variant="ghost" size="sm" className="w-full text-xs" onClick={() => setCreating(true)}>
            <Plus size={14} className="mr-1" /> Nova reunião
          </Button>
        )}
      </div>
    </div>
  )
}
