import { X, Pencil, Trash2 } from "lucide-react"
import { Button } from "../ui/button"
import { useCards, useMoveCard, type BoardCardDetail } from "../../hooks/useBoard"
import { useColumns } from "../../hooks/useBoardColumns"
import { cn } from "../../lib/utils"

interface Props {
  card: BoardCardDetail
  confirmDelete: boolean
  onDelete: () => void
  onClose: () => void
}

export function CardModalHeader({ card, confirmDelete, onDelete, onClose }: Props) {
  const { data: columns = [] } = useColumns()
  // Sem filtro de propósito: com os filtros ativos do board a carta que define o
  // maior position pode estar filtrada para fora, e o card cairia no meio da coluna.
  const { data: cards = [], isLoading: cardsLoading } = useCards()
  const moveCard = useMoveCard()

  function handleMove(columnID: string) {
    if (columnID === card.column_id) return
    const inTarget = cards.filter(c => c.column_id === columnID)
    const position = inTarget.length === 0
      ? 1000
      : Math.max(...inTarget.map(c => c.position)) + 1000
    moveCard.mutate({ id: card.id, column_id: columnID, position })
  }

  const isManual = card.source === "manual"

  return (
    <div className="flex items-center gap-3 px-5 py-4 border-b border-border flex-shrink-0">
      <span className="text-xs text-muted-foreground flex-shrink-0">#{card.number}</span>
      {isManual && <Pencil size={11} className="text-muted-foreground/60 flex-shrink-0" />}

      <h2 id="card-modal-title" className="text-base font-semibold flex-1 truncate" title={card.meeting_title}>
        {card.meeting_title}
      </h2>

      {!isManual && card.theme_name && (
        <span className="text-xs text-muted-foreground hidden sm:inline flex-shrink-0">
          {card.theme_name}
        </span>
      )}

      <select
        value={card.column_id}
        onChange={e => handleMove(e.target.value)}
        disabled={moveCard.isPending || cardsLoading}
        aria-label="Mover para outra coluna"
        className="text-xs rounded-lg px-2 py-1 bg-input border border-border text-foreground focus:outline-none focus:ring-1 focus:ring-primary flex-shrink-0"
      >
        {columns.map(col => (
          <option key={col.id} value={col.id}>{col.name}</option>
        ))}
      </select>

      <button
        onClick={onDelete}
        aria-label={confirmDelete ? "Confirmar exclusão do card" : "Excluir card"}
        className={cn(
          "flex items-center gap-1 p-1 rounded transition-colors flex-shrink-0",
          confirmDelete
            ? "text-destructive bg-destructive/20"
            : "text-muted-foreground hover:text-destructive hover:bg-destructive/10",
        )}
      >
        <Trash2 size={14} />
        {confirmDelete && <span className="text-xs">Confirmar?</span>}
      </button>

      <Button variant="ghost" size="icon" onClick={onClose} aria-label="Fechar"><X size={16} /></Button>
    </div>
  )
}
