import { useState } from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"
import { Button } from "../ui/button"
import { useCreateManualCard } from "../../hooks/useBoard"
import type { BoardColumn } from "../../hooks/useBoardColumns"

interface Props {
  columns: BoardColumn[]
  defaultColumnId?: string
  onClose: () => void
}

export function CreateManualCardModal({ columns, defaultColumnId, onClose }: Props) {
  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")
  const [columnId, setColumnId] = useState(defaultColumnId ?? columns[0]?.id ?? "")
  const createCard = useCreateManualCard()

  function handleSubmit() {
    if (!title.trim()) return
    createCard.mutate(
      { column_id: columnId, title: title.trim(), description },
      { onSuccess: onClose },
    )
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-background border border-border rounded-lg w-[420px] shadow-xl overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-border">
          <span className="font-semibold text-sm">Novo card</span>
          <Button variant="ghost" size="icon" onClick={onClose}><X size={16} /></Button>
        </div>

        <div className="px-5 py-4 space-y-4">
          <div>
            <label className="text-xs text-muted-foreground uppercase tracking-widest block mb-1">
              Título <span className="text-destructive">*</span>
            </label>
            <input
              className="w-full text-sm rounded-xl px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary bg-input border border-border text-foreground placeholder:text-muted-foreground/60"
              placeholder="Ex: Revisar proposta comercial"
              value={title}
              onChange={e => setTitle(e.target.value)}
              onKeyDown={e => e.key === "Enter" && handleSubmit()}
              autoFocus
            />
          </div>

          <div>
            <label className="text-xs text-muted-foreground uppercase tracking-widest block mb-1">
              Coluna
            </label>
            <select
              className="w-full text-sm rounded-xl px-3 py-2 focus:outline-none bg-input border border-border text-foreground"
              value={columnId}
              onChange={e => setColumnId(e.target.value)}
              disabled={!!defaultColumnId}
            >
              {columns.map(col => (
                <option key={col.id} value={col.id}>{col.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="text-xs text-muted-foreground uppercase tracking-widest block mb-1">
              Descrição
            </label>
            <textarea
              className="w-full text-sm rounded-xl px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary bg-input border border-border text-foreground placeholder:text-muted-foreground/60 resize-none"
              placeholder="Opcional..."
              rows={3}
              value={description}
              onChange={e => setDescription(e.target.value)}
            />
          </div>
        </div>

        <div className="flex gap-2 px-5 pb-5">
          <Button variant="ghost" className="flex-1" onClick={onClose}>Cancelar</Button>
          <Button
            className="flex-1"
            onClick={handleSubmit}
            disabled={!title.trim() || createCard.isPending}
          >
            {createCard.isPending ? "Criando..." : "Criar card"}
          </Button>
        </div>
      </div>
    </div>,
    document.body,
  )
}
