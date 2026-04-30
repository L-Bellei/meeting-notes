import { SortableContext, verticalListSortingStrategy } from "@dnd-kit/sortable"
import { useDroppable } from "@dnd-kit/core"
import { Plus } from "lucide-react"
import type { BoardColumn } from "../../hooks/useBoardColumns"
import type { BoardCardSummary } from "../../hooks/useBoard"
import { KanbanCard } from "./KanbanCard"

interface Props {
  column: BoardColumn
  cards: BoardCardSummary[]
  onCardClick: (id: string) => void
  onAddCard?: (columnId: string) => void
}

export function KanbanColumn({ column, cards, onCardClick, onAddCard }: Props) {
  const { setNodeRef } = useDroppable({ id: column.id })

  return (
    <div className="flex flex-col bg-muted/30 rounded-lg w-64 flex-shrink-0 min-h-0">
      <div className="flex items-center justify-between px-3 py-2.5 border-b border-border flex-shrink-0">
        <span className="text-sm font-medium">{column.name}</span>
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded-full">
            {cards.length}
          </span>
          {onAddCard && (
            <button
              onClick={e => { e.stopPropagation(); onAddCard(column.id) }}
              className="text-muted-foreground hover:text-foreground transition-colors p-0.5 rounded"
              title="Novo card"
            >
              <Plus size={14} />
            </button>
          )}
        </div>
      </div>
      <div ref={setNodeRef} className="flex flex-col gap-2 p-2 overflow-y-auto flex-1 min-h-16">
        <SortableContext items={cards.map(c => c.id)} strategy={verticalListSortingStrategy}>
          {cards.map(card => (
            <KanbanCard key={card.id} card={card} onClick={() => onCardClick(card.id)} />
          ))}
        </SortableContext>
      </div>
    </div>
  )
}
