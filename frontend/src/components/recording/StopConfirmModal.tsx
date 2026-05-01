import { useState } from "react"
import { createPortal } from "react-dom"
import { useQueryClient } from "@tanstack/react-query"
import { api } from "../../hooks/useApi"
import { Button } from "../ui/button"
import { Spinner } from "../ui/spinner"

interface Props {
  open: boolean
  meetingId: string | null
  onClose: () => void
}

export function StopConfirmModal({ open, meetingId, onClose }: Props) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const qc = useQueryClient()

  if (!open || !meetingId) return null

  async function handleStop() {
    if (!meetingId) return
    setLoading(true)
    setError("")
    try {
      await api<void>(`/api/meetings/${meetingId}/stop`, { method: "POST" })
      qc.invalidateQueries({ queryKey: ["meetings"] })
      qc.invalidateQueries({ queryKey: ["meeting", meetingId] })
      onClose()
    } catch (e: any) {
      setError(e.message ?? "Erro ao encerrar gravação")
    } finally {
      setLoading(false)
    }
  }

  const content = (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70">
      <div className="bg-[#1a1a1a] border border-border rounded-2xl shadow-2xl w-80 p-6">
        <h2 className="text-base font-semibold text-foreground mb-2">Encerrar gravação?</h2>
        <p className="text-sm text-muted-foreground mb-5">
          A gravação será encerrada e o processamento iniciará automaticamente.
        </p>
        {error && <p className="text-xs text-destructive mb-3">{error}</p>}
        <div className="flex gap-2 justify-end">
          <Button variant="outline" size="sm" onClick={onClose} disabled={loading}>
            Cancelar
          </Button>
          <Button
            size="sm"
            onClick={handleStop}
            disabled={loading}
            className="bg-destructive hover:bg-destructive/90 text-destructive-foreground border-0"
          >
            {loading && <Spinner size={14} className="mr-1.5" />}
            {loading ? "Encerrando..." : "Encerrar"}
          </Button>
        </div>
      </div>
    </div>
  )

  return createPortal(content, document.body)
}
