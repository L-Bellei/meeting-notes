import { useState } from "react"
import { Settings2 } from "lucide-react"
import { Button } from "../ui/button"
import { useColumns } from "../../hooks/useBoardColumns"
import { useCards, type BoardCardFilters } from "../../hooks/useBoard"
import { KanbanColumn } from "./KanbanColumn"
import { ColumnSettingsPanel } from "./ColumnSettingsPanel"

export function BoardView() {
  const { data: columns = [] } = useColumns()
  const [filters] = useState<BoardCardFilters>({})
  const { data: cards = [] } = useCards(filters)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [selectedCardId, setSelectedCardId] = useState<string | null>(null)

  const cardsByColumn = columns.reduce<Record<string, typeof cards>>((acc, col) => {
    acc[col.id] = cards.filter(c => c.column_id === col.id)
    return acc
  }, {})

  return (
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
      {settingsOpen && <ColumnSettingsPanel onClose={() => setSettingsOpen(false)} />}
      {/* selectedCardId used in Task 5 for CardDetailModal */}
      {selectedCardId && null}
    </div>
  )
}
