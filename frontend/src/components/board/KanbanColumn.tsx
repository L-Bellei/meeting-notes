import type { BoardColumn } from "../../hooks/useBoardColumns"
import type { BoardCardSummary } from "../../hooks/useBoard"
import { KanbanCard } from "./KanbanCard"

interface Props {
  column: BoardColumn
  cards: BoardCardSummary[]
  onCardClick: (id: string) => void
}

export function KanbanColumn({ column, cards, onCardClick }: Props) {
  return (
    <div className="flex flex-col bg-muted/30 rounded-lg w-64 flex-shrink-0 min-h-0">
      <div className="flex items-center justify-between px-3 py-2.5 border-b border-border flex-shrink-0">
        <span className="text-sm font-medium">{column.name}</span>
        <span className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded-full">
          {column.card_count}
        </span>
      </div>
      <div className="flex flex-col gap-2 p-2 overflow-y-auto flex-1">
        {cards.map(card => (
          <KanbanCard key={card.id} card={card} onClick={() => onCardClick(card.id)} />
        ))}
      </div>
    </div>
  )
}
