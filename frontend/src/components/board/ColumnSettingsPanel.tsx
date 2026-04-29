import { useState } from "react"
import { createPortal } from "react-dom"
import { X, Trash2, ChevronUp, ChevronDown, Pencil, Check } from "lucide-react"
import { Button } from "../ui/button"
import {
  useColumns, useCreateColumn, useUpdateColumn,
  useDeleteColumn, useReorderColumns, type BoardColumn,
} from "../../hooks/useBoardColumns"

interface Props { onClose: () => void }

export function ColumnSettingsPanel({ onClose }: Props) {
  const { data: columns = [] } = useColumns()
  const createCol = useCreateColumn()
  const updateCol = useUpdateColumn()
  const deleteCol = useDeleteColumn()
  const reorder   = useReorderColumns()

  const [newName, setNewName]         = useState("")
  const [editingId, setEditingId]     = useState<string | null>(null)
  const [editName, setEditName]       = useState("")
  const [deletingCol, setDeletingCol] = useState<BoardColumn | null>(null)
  const [moveToId, setMoveToId]       = useState("")

  function handleCreate() {
    if (!newName.trim()) return
    createCol.mutate(newName.trim(), { onSuccess: () => setNewName("") })
  }

  function handleMove(col: BoardColumn, dir: -1 | 1) {
    const idx = columns.findIndex(c => c.id === col.id)
    const swap = columns[idx + dir]
    if (!swap) return
    reorder.mutate([
      { id: col.id, position: swap.position },
      { id: swap.id, position: col.position },
    ])
  }

  function handleDeleteClick(col: BoardColumn) {
    if (col.card_count > 0) {
      setMoveToId(columns.find(c => c.id !== col.id)?.id ?? "")
      setDeletingCol(col)
    } else {
      deleteCol.mutate({ id: col.id })
    }
  }

  function handleConfirmDelete() {
    if (!deletingCol) return
    deleteCol.mutate(
      { id: deletingCol.id, moveTo: deletingCol.card_count > 0 ? moveToId : undefined },
      { onSuccess: () => setDeletingCol(null) },
    )
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-background border border-border rounded-lg w-96 shadow-xl">
        <div className="flex items-center justify-between px-4 py-3 border-b border-border">
          <h2 className="font-semibold text-sm">Configurar Colunas</h2>
          <Button variant="ghost" size="icon" onClick={onClose}><X size={16} /></Button>
        </div>
        <div className="p-4 space-y-2 max-h-80 overflow-y-auto">
          {columns.map((col, idx) => (
            <div key={col.id} className="flex items-center gap-2 group">
              {editingId === col.id ? (
                <>
                  <input
                    className="flex-1 text-sm bg-input border border-border rounded px-2 py-1"
                    value={editName}
                    onChange={e => setEditName(e.target.value)}
                    onKeyDown={e => {
                      if (e.key === "Enter") {
                        if (!editName.trim()) return
                        updateCol.mutate({ id: col.id, name: editName.trim() }, { onSuccess: () => setEditingId(null) })
                      }
                      if (e.key === "Escape") setEditingId(null)
                    }}
                    autoFocus
                  />
                  <Button variant="ghost" size="icon" onClick={() => {
                    if (!editName.trim()) return
                    updateCol.mutate({ id: col.id, name: editName.trim() }, { onSuccess: () => setEditingId(null) })
                  }}><Check size={14} /></Button>
                </>
              ) : (
                <>
                  <span className="flex-1 text-sm">{col.name}</span>
                  <span className="text-xs text-muted-foreground">{col.card_count}</span>
                  <Button variant="ghost" size="icon" className="opacity-0 group-hover:opacity-100"
                    onClick={() => { setEditingId(col.id); setEditName(col.name) }}>
                    <Pencil size={12} />
                  </Button>
                  <Button variant="ghost" size="icon" disabled={idx === 0}
                    onClick={() => handleMove(col, -1)}><ChevronUp size={14} /></Button>
                  <Button variant="ghost" size="icon" disabled={idx === columns.length - 1}
                    onClick={() => handleMove(col, 1)}><ChevronDown size={14} /></Button>
                  <Button variant="ghost" size="icon" disabled={columns.length <= 1}
                    onClick={() => handleDeleteClick(col)}><Trash2 size={12} /></Button>
                </>
              )}
            </div>
          ))}
        </div>
        <div className="flex gap-2 px-4 pb-4">
          <input
            className="flex-1 text-sm bg-input border border-border rounded px-2 py-1"
            placeholder="Nova coluna..."
            value={newName}
            onChange={e => setNewName(e.target.value)}
            onKeyDown={e => e.key === "Enter" && handleCreate()}
          />
          <Button size="sm" onClick={handleCreate}>Adicionar</Button>
        </div>
      </div>

      {deletingCol && (
        <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60">
          <div className="bg-background border border-border rounded-lg w-80 p-4 shadow-xl space-y-3">
            <p className="text-sm">
              A coluna <strong>{deletingCol.name}</strong> tem {deletingCol.card_count} card(s).
              Mover para:
            </p>
            <select
              className="w-full text-sm bg-input border border-border rounded px-2 py-1"
              value={moveToId}
              onChange={e => setMoveToId(e.target.value)}
            >
              {columns.filter(c => c.id !== deletingCol.id).map(c => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
            <div className="flex justify-end gap-2">
              <Button variant="ghost" size="sm" onClick={() => setDeletingCol(null)}>Cancelar</Button>
              <Button variant="destructive" size="sm" onClick={handleConfirmDelete}>Deletar</Button>
            </div>
          </div>
        </div>
      )}
    </div>,
    document.body,
  )
}
