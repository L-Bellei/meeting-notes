import { useRef, useState } from "react"
import { useDraggable, useDroppable } from "@dnd-kit/core"
import { ChevronRight, MoreHorizontal, Sparkles, LayoutGrid } from "lucide-react"
import type { Theme } from "../../hooks/useThemes"
import { ThemeRowMenu } from "./ThemeRowMenu"
import { cn } from "../../lib/utils"

interface Props {
  theme: Theme
  depth: number
  count: number
  selected: boolean
  expanded: boolean
  hasChildren: boolean
  draggable: boolean
  droppable: boolean
  onSelect: () => void
  onToggleExpand: () => void
  onCreateChild: () => void
  onEdit: () => void
  onDelete: () => void
}

export function ThemeRow({
  theme, depth, count, selected, expanded, hasChildren, draggable, droppable,
  onSelect, onToggleExpand, onCreateChild, onEdit, onDelete,
}: Props) {
  const [menuOpen, setMenuOpen] = useState(false)
  const menuBtn = useRef<HTMLButtonElement>(null)
  const hasPrompt = theme.custom_prompt.trim() !== ""

  const drag = useDraggable({ id: theme.id, disabled: !draggable })
  const drop = useDroppable({ id: `drop-${theme.id}`, disabled: !droppable })

  return (
    <div
      ref={node => { drag.setNodeRef(node); drop.setNodeRef(node) }}
      {...drag.listeners}
      className={cn(
        "relative flex items-center gap-1 rounded-xl pr-1 mt-0.5 hover:bg-accent transition-colors",
        selected && "bg-accent",
        depth > 0 && "ml-4",
        drag.isDragging && "opacity-40",
        drop.isOver && droppable && "ring-1 ring-primary"
      )}
    >
      <span
        aria-hidden
        className="absolute left-0 top-1.5 bottom-1.5 w-[3px] rounded-full"
        style={{ backgroundColor: theme.color }}
      />

      <button
        type="button"
        onClick={onToggleExpand}
        aria-label={expanded ? "Recolher subcategorias" : "Expandir subcategorias"}
        className={cn(
          "ml-1.5 w-4 h-4 flex items-center justify-center flex-shrink-0 text-muted-foreground",
          !hasChildren && "invisible"
        )}
      >
        <ChevronRight size={12} className={cn("transition-transform", expanded && "rotate-90")} />
      </button>

      <button
        type="button"
        onClick={onSelect}
        aria-current={selected ? "true" : undefined}
        title={theme.description || theme.name}
        className="flex-1 min-w-0 flex items-center gap-1.5 py-2 text-left text-sm"
      >
        <span className={cn("truncate", selected ? "text-foreground font-medium" : "text-muted-foreground")}>
          {theme.name}
        </span>
        {hasPrompt && (
          <span title="Prompt personalizado" className="flex-shrink-0 inline-flex">
            <Sparkles size={11} className="text-muted-foreground" />
          </span>
        )}
        {theme.auto_add_to_board && (
          <span title="Adiciona ao board automaticamente" className="flex-shrink-0 inline-flex">
            <LayoutGrid size={11} className="text-muted-foreground" />
          </span>
        )}
      </button>

      <span className="text-[11px] tabular-nums text-muted-foreground flex-shrink-0">{count}</span>

      <button
        type="button"
        ref={menuBtn}
        onClick={() => setMenuOpen(v => !v)}
        aria-label={`Ações de ${theme.name}`}
        className="p-1 rounded-md flex-shrink-0 text-muted-foreground hover:bg-primary/20 hover:text-primary"
      >
        <MoreHorizontal size={14} />
      </button>

      {menuOpen && (
        <ThemeRowMenu
          anchor={menuBtn.current}
          canAddChild={depth === 0}
          onAddChild={() => { setMenuOpen(false); onCreateChild() }}
          onEdit={() => { setMenuOpen(false); onEdit() }}
          onDelete={() => { setMenuOpen(false); onDelete() }}
          onClose={() => setMenuOpen(false)}
        />
      )}
    </div>
  )
}
