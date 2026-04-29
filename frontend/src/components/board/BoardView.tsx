import { useState } from "react"
import { Settings2 } from "lucide-react"
import {
  DndContext, DragOverlay, PointerSensor, useSensor, useSensors,
  type DragEndEvent, type DragStartEvent,
} from "@dnd-kit/core"
import { Button } from "../ui/button"
import { useColumns } from "../../hooks/useBoardColumns"
import { useCards, useMoveCard, EMPTY_FILTERS, type BoardCardSummary } from "../../hooks/useBoard"
import { KanbanColumn } from "./KanbanColumn"
import { KanbanCard } from "./KanbanCard"
import { ColumnSettingsPanel } from "./ColumnSettingsPanel"
import { useQueryClient } from "@tanstack/react-query"

export function BoardView() {
  const { data: columns = [] } = useColumns()
  const { data: cards = [] } = useCards(EMPTY_FILTERS)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [selectedCardId, setSelectedCardId] = useState<string | null>(null)
  const [activeCard, setActiveCard] = useState<BoardCardSummary | null>(null)
  const moveCard = useMoveCard()
  const qc = useQueryClient()

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))

  const cardsByColumn = columns.reduce<Record<string, BoardCardSummary[]>>((acc, col) => {
    acc[col.id] = cards.filter(c => c.column_id === col.id)
    return acc
  }, {})

  function onDragStart({ active }: DragStartEvent) {
    setActiveCard(cards.find(c => c.id === active.id) ?? null)
  }

  function onDragEnd({ active, over }: DragEndEvent) {
    setActiveCard(null)
    if (!over || active.id === over.id) return

    const card = cards.find(c => c.id === active.id)
    if (!card) return

    const targetColumnId = columns.find(col => col.id === over.id)
      ? (over.id as string)
      : (cards.find(c => c.id === over.id)?.column_id ?? card.column_id)

    const targetColumnCards = cards
      .filter(c => c.column_id === targetColumnId && c.id !== card.id)
      .sort((a, b) => a.position - b.position)

    const overCardIdx = targetColumnCards.findIndex(c => c.id === over.id)
    let newPosition: number
    if (overCardIdx === -1 || targetColumnCards.length === 0) {
      newPosition = (targetColumnCards[targetColumnCards.length - 1]?.position ?? 0) + 1000
    } else if (overCardIdx === 0) {
      newPosition = targetColumnCards[0].position / 2
    } else {
      newPosition = (targetColumnCards[overCardIdx - 1].position + targetColumnCards[overCardIdx].position) / 2
    }

    qc.setQueryData(["board-cards", EMPTY_FILTERS], (old: BoardCardSummary[] | undefined) =>
      (old ?? []).map(c =>
        c.id === card.id ? { ...c, column_id: targetColumnId, position: newPosition } : c
      )
    )

    moveCard.mutate({ id: card.id, column_id: targetColumnId, position: newPosition })
  }

  return (
    <DndContext sensors={sensors} onDragStart={onDragStart} onDragEnd={onDragEnd}>
      <div className="flex flex-col flex-1 overflow-hidden">
        <div className="flex items-center gap-3 px-4 py-3 border-b border-border flex-shrink-0">
          <span className="font-semibold text-sm flex-1">Board</span>
          <Button variant="ghost" size="icon" onClick={() => setSettingsOpen(true)}>
            <Settings2 size={16} />
          </Button>
        </div>
        <div className="flex flex-1 gap-4 p-4 overflow-x-auto overflow-y-hidden">
          {columns.map(col => (
            <KanbanColumn
              key={col.id}
              column={col}
              cards={cardsByColumn[col.id] ?? []}
              onCardClick={setSelectedCardId}
            />
          ))}
          {columns.length === 0 && (
            <div className="flex-1 flex items-center justify-center text-muted-foreground text-sm">
              Nenhuma coluna. Use o ícone de configurações para adicionar.
            </div>
          )}
        </div>
        <DragOverlay>
          {activeCard && <KanbanCard card={activeCard} onClick={() => {}} />}
        </DragOverlay>
      </div>
      {settingsOpen && <ColumnSettingsPanel onClose={() => setSettingsOpen(false)} />}
      {/* CardDetailModal wired in Task 5 */}
      {selectedCardId && null}
    </DndContext>
  )
}
