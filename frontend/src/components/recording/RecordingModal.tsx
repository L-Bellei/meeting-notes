import { useState } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { useThemes } from "../../hooks/useThemes"
import { useCreateMeeting } from "../../hooks/useMeetings"
import { api } from "../../hooks/useApi"
import { Button } from "../ui/button"

interface Props {
  open: boolean
  onClose: () => void
  onMeetingCreated: (id: string) => void
}

export function RecordingModal({ open, onClose, onMeetingCreated }: Props) {
  const { data: themes = [] } = useThemes()
  const createMeeting = useCreateMeeting()
  const qc = useQueryClient()
  const [title, setTitle] = useState("")
  const [themeId, setThemeId] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  if (!open) return null

  async function handleStart() {
    if (!title.trim()) { setError("Título obrigatório"); return }
    setError("")
    setLoading(true)
    try {
      const m = await createMeeting.mutateAsync({ title: title.trim(), theme_id: themeId || undefined })
      await api<void>(`/api/meetings/${m.id}/start`, { method: "POST" })
      qc.invalidateQueries({ queryKey: ["meetings"] })
      qc.invalidateQueries({ queryKey: ["meeting", m.id] })
      onMeetingCreated(m.id)
      setTitle("")
      setThemeId("")
      onClose()
    } catch (e: any) {
      const msg: string = e.message ?? ""
      if (msg.includes("503") || msg.toLowerCase().includes("unavailable")) {
        setError("Serviço de áudio indisponível")
      } else if (msg.includes("409")) {
        setError("Já existe uma gravação em andamento")
      } else {
        setError(msg)
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-background rounded-lg shadow-xl w-96 p-6">
        <h2 className="text-base font-semibold mb-4">Nova Gravação</h2>
        <div className="space-y-3">
          <div>
            <label className="text-xs text-muted-foreground">Título</label>
            <input
              autoFocus
              value={title}
              onChange={e => setTitle(e.target.value)}
              onKeyDown={e => { if (e.key === "Enter") handleStart(); if (e.key === "Escape") onClose() }}
              placeholder="Daily 28/04"
              className="w-full mt-1 text-sm border rounded px-3 py-1.5 bg-background focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
          <div>
            <label className="text-xs text-muted-foreground">Tema (opcional)</label>
            <select
              value={themeId}
              onChange={e => setThemeId(e.target.value)}
              className="w-full mt-1 text-sm border rounded px-3 py-1.5 bg-background focus:outline-none"
            >
              <option value="">Sem tema</option>
              {themes.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
            </select>
          </div>
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
        <div className="flex gap-2 justify-end mt-5">
          <Button variant="outline" size="sm" onClick={onClose}>Cancelar</Button>
          <Button size="sm" onClick={handleStart} disabled={loading || createMeeting.isPending}>
            {loading || createMeeting.isPending ? "Iniciando..." : "Iniciar Gravação"}
          </Button>
        </div>
      </div>
    </div>
  )
}
