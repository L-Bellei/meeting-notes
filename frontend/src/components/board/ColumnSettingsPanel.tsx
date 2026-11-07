import { useState } from "react"
import { X, Trash2, ChevronUp, ChevronDown, Pencil, Check } from "lucide-react"
import { Button } from "../ui/button"
import { Dialog } from "../ui/Dialog"
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

  return (
    <Dialog
      open
      onClose={onClose}
      labelledBy="column-settings-title"
      overlayClassName="bg-black/50"
      panelClassName="bg-background border border-border rounded-lg w-96 shadow-xl"
    >
        <div className="flex items-center justify-between px-4 py-3 border-b border-border">
          <h2 id="column-settings-title" className="font-semibold text-sm">Configurar Colunas</h2>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="Fechar configuração de colunas"><X size={16} /></Button>
        </div>
        <div className="p-4 space-y-2 max-h-80 overflow-y-auto">
          {columns.map((col, idx) => (
            <div key={col.id} className="flex items-center gap-2 group">
              {editingId === col.id ? (
                <>
                  <input
                    aria-label={`Nome da coluna ${col.name}`}
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
                  }} aria-label={`Salvar nome da coluna ${col.name}`}><Check size={14} /></Button>
                </>
              ) : (
                <>
                  <span className="flex-1 text-sm">{col.name}</span>
                  <span className="text-xs text-muted-foreground">{col.card_count}</span>
                  <Button variant="ghost" size="icon" className="opacity-0 group-hover:opacity-100"
                    onClick={() => { setEditingId(col.id); setEditName(col.name) }} aria-label={`Editar coluna ${col.name}`}>
                    <Pencil size={12} />
                  </Button>
                  <Button variant="ghost" size="icon" disabled={idx === 0}
                    onClick={() => handleMove(col, -1)} aria-label={`Mover coluna ${col.name} para cima`}><ChevronUp size={14} /></Button>
                  <Button variant="ghost" size="icon" disabled={idx === columns.length - 1}
                    onClick={() => handleMove(col, 1)} aria-label={`Mover coluna ${col.name} para baixo`}><ChevronDown size={14} /></Button>
                  <Button variant="ghost" size="icon" disabled={columns.length <= 1}
                    onClick={() => handleDeleteClick(col)} aria-label={`Excluir coluna ${col.name}`}><Trash2 size={12} /></Button>
                </>
              )}
            </div>
          ))}
        </div>
        <div className="flex gap-2 px-4 pb-4">
          <input
            aria-label="Nome da nova coluna"
            className="flex-1 text-sm bg-input border border-border rounded px-2 py-1"
            placeholder="Nova coluna..."
            value={newName}
            onChange={e => setNewName(e.target.value)}
            onKeyDown={e => e.key === "Enter" && handleCreate()}
          />
          <Button size="sm" onClick={handleCreate}>Adicionar</Button>
        </div>

      {deletingCol && (
        <div
          className="border-t border-border p-4 space-y-3"
          role="group"
          aria-labelledby="delete-column-title"
          aria-live="polite"
        >
            <p id="delete-column-title" className="text-sm">
              A coluna <strong>{deletingCol.name}</strong> tem {deletingCol.card_count} card(s).
              Mover para:
            </p>
            <select
              aria-label="Coluna de destino"
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
              <Button variant="destructive" size="sm" onClick={handleConfirmDelete} disabled={!moveToId || deleteCol.isPending}>
                {deleteCol.isPending ? "Deletando..." : "Deletar"}
              </Button>
            </div>
        </div>
      )}
    </Dialog>
  )
}
