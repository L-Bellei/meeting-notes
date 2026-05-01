import { useState, useEffect } from "react"
import { createPortal } from "react-dom"
import { useQueryClient } from "@tanstack/react-query"
import { useThemes } from "../../hooks/useThemes"
import { useCreateMeeting } from "../../hooks/useMeetings"
import { api } from "../../hooks/useApi"
import { Button } from "../ui/button"
import { Spinner } from "../ui/spinner"

interface Props {
  open: boolean
  onClose: () => void
  onMeetingCreated: (id: string) => void
  initialTitle?: string
}

export function RecordingModal({ open, onClose, onMeetingCreated, initialTitle }: Props) {
  const { data: themes = [] } = useThemes()
  const createMeeting = useCreateMeeting()
  const qc = useQueryClient()
  const [title, setTitle] = useState(initialTitle ?? "")
  const [themeId, setThemeId] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (open) setTitle(initialTitle ?? "")
  }, [open, initialTitle])

  if (!open) return null

  async function handleStart() {
    if (!title.trim()) { setError("Título obrigatório"); return }
    setError("")
    setLoading(true)
    let createdId: string | null = null
    try {
      const m = await createMeeting.mutateAsync({ title: title.trim(), theme_id: themeId || undefined })
      createdId = m.id
      await api<void>(`/api/meetings/${m.id}/start`, { method: "POST" })
      onMeetingCreated(m.id)
      setTitle("")
      setThemeId("")
      onClose()
    } catch (e: any) {
      if (createdId) {
        try {
          await api<void>(`/api/meetings/${createdId}`, { method: "DELETE" })
          qc.invalidateQueries({ queryKey: ["meetings"] })
        } catch {
          // ignore cleanup error — meeting stays pending but doesn't block the user
        }
      }
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

  const content = (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70">
      <div className="bg-[#1a1a1a] border border-border rounded-2xl shadow-2xl shadow-black/50 w-96 p-6">
        <h2 className="text-base font-semibold mb-4 text-foreground">Nova Gravação</h2>
        <div className="space-y-3">
          <div>
            <label className="text-xs text-muted-foreground">Título</label>
            <input
              autoFocus
              value={title}
              onChange={e => setTitle(e.target.value)}
              onKeyDown={e => { if (e.key === "Enter") handleStart(); if (e.key === "Escape") onClose() }}
              placeholder="Daily 28/04"
              className="w-full mt-1 text-sm rounded-xl px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary bg-[#111111] border border-border"
            />
          </div>
          <div>
            <label className="text-xs text-muted-foreground">Tema (opcional)</label>
            <select
              value={themeId}
              onChange={e => setThemeId(e.target.value)}
              className="w-full mt-1 text-sm rounded-xl px-3 py-2 focus:outline-none bg-[#111111] border border-border"
            >
              <option value="">Sem tema</option>
              {themes.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
            </select>
          </div>
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
        <div className="flex gap-2 justify-end mt-5">
          <Button variant="outline" size="sm" onClick={onClose}>Cancelar</Button>
          <Button
            size="sm"
            onClick={handleStart}
            disabled={loading || createMeeting.isPending}
            className="bg-gradient-to-r from-purple-600 to-purple-500 border-0"
          >
            {(loading || createMeeting.isPending) && <Spinner size={14} className="mr-1.5" />}
            {loading || createMeeting.isPending ? "Iniciando..." : "Iniciar Gravação"}
          </Button>
        </div>
      </div>
    </div>
  )

  return createPortal(content, document.body)
}
