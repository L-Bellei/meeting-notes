import { useState } from "react"
import { Plus, Calendar, Search, Filter, X, Trash2 } from "lucide-react"
import { useMeetings, useCreateMeeting, useDeleteMeeting, type MeetingFilters } from "../../hooks/useMeetings"
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
  onMeetingDeleted?: (id: string) => void
  onOpenSearch: () => void
}

const STATUS_OPTIONS = [
  { value: "", label: "Todos" },
  { value: "pending", label: "Pendente" },
  { value: "recording", label: "Gravando" },
  { value: "transcribing", label: "Transcrevendo" },
  { value: "processing", label: "Processando" },
  { value: "completed", label: "Concluído" },
  { value: "failed", label: "Falhou" },
]

function statusVariant(s: Meeting["status"]) {
  return s as any
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

export function MeetingList({ themeId, selectedMeetingId, onSelectMeeting, onMeetingDeleted, onOpenSearch }: MeetingListProps) {
  const [q, setQ] = useState("")
  const [status, setStatus] = useState("")
  const [startedAfter, setStartedAfter] = useState("")
  const [startedBefore, setStartedBefore] = useState("")
  const [showFilters, setShowFilters] = useState(false)

  const filters: MeetingFilters = {
    theme_id: themeId,
    q: q || undefined,
    status: status || undefined,
    started_after: startedAfter || undefined,
    started_before: startedBefore || undefined,
  }

  const { data: meetings = [] } = useMeetings(filters)
  const { data: themes = [] } = useThemes()
  const createMeeting = useCreateMeeting()
  const deleteMeeting = useDeleteMeeting()

  const [creating, setCreating] = useState(false)
  const [newTitle, setNewTitle] = useState("")
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)

  const hasActiveFilters = !!(q || status || startedAfter || startedBefore)

  function clearFilters() {
    setQ("")
    setStatus("")
    setStartedAfter("")
    setStartedBefore("")
  }

  async function handleCreate() {
    if (!newTitle.trim()) return
    const m = await createMeeting.mutateAsync({ title: newTitle.trim(), theme_id: themeId ?? undefined })
    setNewTitle("")
    setCreating(false)
    onSelectMeeting(m.id)
  }

  async function handleDelete(id: string, e: React.MouseEvent) {
    e.stopPropagation()
    if (confirmDelete === id) {
      await deleteMeeting.mutateAsync(id)
      setConfirmDelete(null)
      onMeetingDeleted?.(id)
    } else {
      setConfirmDelete(id)
    }
  }

  function themeColor(id: string | null) {
    return themes.find(t => t.id === id)?.color ?? "#8b7aaa"
  }

  return (
    <div className="w-72 border-r border-border h-full flex flex-col">
      <div className="h-14 px-4 border-b border-border flex items-center justify-between flex-shrink-0">
        <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Reuniões</span>
        <div className="flex items-center gap-1">
          <span className="text-xs text-muted-foreground">{meetings.length}</span>
          <button
            onClick={onOpenSearch}
            title="Busca global (Ctrl+K)"
            className="p-1 rounded-md hover:bg-accent transition-colors text-muted-foreground"
          >
            <Search size={14} />
          </button>
          <button
            onClick={() => setShowFilters(v => !v)}
            title="Filtros"
            className={cn(
              "p-1 rounded-md hover:bg-accent transition-colors",
              (showFilters || hasActiveFilters) ? "text-primary" : "text-muted-foreground"
            )}
          >
            <Filter size={14} />
          </button>
        </div>
      </div>

      {/* search bar — always visible */}
      <div className="px-2 pt-2 flex-shrink-0">
        <div className="flex items-center gap-1.5 rounded-lg bg-muted/40 border border-border px-2 py-1">
          <Search size={12} className="text-muted-foreground flex-shrink-0" />
          <input
            value={q}
            onChange={e => setQ(e.target.value)}
            placeholder="Buscar reunião..."
            className="flex-1 bg-transparent text-xs focus:outline-none text-foreground placeholder:text-muted-foreground"
          />
          {q && (
            <button onClick={() => setQ("")} className="text-muted-foreground hover:text-foreground">
              <X size={11} />
            </button>
          )}
        </div>
      </div>

      {/* filter panel */}
      {showFilters && (
        <div className="px-2 pt-2 flex-shrink-0 space-y-2">
          <div className="flex gap-1 items-center">
            <select
              value={status}
              onChange={e => setStatus(e.target.value)}
              className="flex-1 text-xs rounded-lg px-2 py-1.5 bg-muted/40 border border-border text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
            >
              {STATUS_OPTIONS.map(o => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
            {hasActiveFilters && (
              <button onClick={clearFilters} className="text-xs text-muted-foreground hover:text-destructive px-1">
                <X size={12} />
              </button>
            )}
          </div>
          <div className="flex gap-1">
            <div className="flex-1">
              <label className="text-[10px] text-muted-foreground block mb-0.5">De</label>
              <input
                type="date"
                value={startedAfter}
                onChange={e => setStartedAfter(e.target.value)}
                className="w-full text-xs rounded-lg px-2 py-1 bg-muted/40 border border-border text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
              />
            </div>
            <div className="flex-1">
              <label className="text-[10px] text-muted-foreground block mb-0.5">Até</label>
              <input
                type="date"
                value={startedBefore}
                onChange={e => setStartedBefore(e.target.value)}
                className="w-full text-xs rounded-lg px-2 py-1 bg-muted/40 border border-border text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
              />
            </div>
          </div>
        </div>
      )}

      <div className="flex-1 overflow-y-auto px-2 py-2 space-y-1 mt-2">
        {meetings.length === 0 && (
          <div className="p-4 text-center text-sm text-muted-foreground">Nenhuma reunião</div>
        )}
        {meetings.map(m => (
          <div key={m.id} className="relative group">
            <button
              onClick={() => { setConfirmDelete(null); onSelectMeeting(m.id) }}
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

            {/* delete button */}
            <button
              title={confirmDelete === m.id ? "Confirmar exclusão" : "Excluir reunião"}
              onClick={e => handleDelete(m.id, e)}
              className={cn(
                "absolute right-2 top-2 p-1 rounded opacity-0 group-hover:opacity-100 transition-all",
                confirmDelete === m.id
                  ? "opacity-100 text-destructive bg-destructive/20"
                  : "text-muted-foreground hover:text-destructive hover:bg-destructive/10"
              )}
            >
              <Trash2 size={12} />
            </button>
          </div>
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
