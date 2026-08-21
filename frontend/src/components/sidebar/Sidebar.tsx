import { useEffect, useState } from "react"
import { DndContext, PointerSensor, useDroppable, useSensor, useSensors, type DragEndEvent } from "@dnd-kit/core"
import { Plus, Tag, X, Pin, PinOff } from "lucide-react"
import { useThemes, useDeleteTheme, useUpdateTheme, type Theme } from "../../hooks/useThemes"
import { useMeetings } from "../../hooks/useMeetings"
import { useSidebarPinned } from "../../hooks/useSidebarPinned"
import { useThemeExpanded } from "../../hooks/useThemeExpanded"
import { ThemeEditModal } from "./ThemeEditModal"
import { ThemeRow } from "./ThemeRow"
import { Button } from "../ui/button"
import { cn } from "../../lib/utils"

interface SidebarProps {
  open: boolean
  onClose: () => void
  selectedThemeId: string | null
  onSelectTheme: (id: string | null) => void
}

export function Sidebar({ open, onClose, selectedThemeId, onSelectTheme }: SidebarProps) {
  const { pinned, toggle: togglePinned } = useSidebarPinned()
  const { data: themes = [] } = useThemes()
  const { data: allMeetings = [] } = useMeetings()
  const deleteTheme = useDeleteTheme()
  const updateTheme = useUpdateTheme()

  const { expanded, toggle: toggleExpand, expand } = useThemeExpanded(themes.map(t => t.id))
  const [creating, setCreating] = useState<{ parentId: string | null } | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<Theme | null>(null)
  const [editingTheme, setEditingTheme] = useState<Theme | null>(null)
  const [dragError, setDragError] = useState("")
  const [activeId, setActiveId] = useState<string | null>(null)

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))
  const { setNodeRef: setRootRef, isOver: rootIsOver } = useDroppable({ id: "drop-root" })

  async function handleDragEnd(e: DragEndEvent) {
    const themeId = String(e.active.id)
    const overId = e.over ? String(e.over.id) : ""
    if (!overId) return

    const moved = themes.find(t => t.id === themeId)
    if (!moved) return

    const parentId = overId === "drop-root" ? null : overId.replace("drop-", "")
    if (parentId === themeId) return
    if ((moved.parent_id ?? null) === parentId) return

    setDragError("")
    try {
      await updateTheme.mutateAsync({
        id: moved.id,
        name: moved.name,
        description: moved.description,
        color: moved.color,
        parent_id: parentId,
        custom_prompt: moved.custom_prompt,
        auto_add_to_board: moved.auto_add_to_board,
      })
    } catch (err) {
      setDragError(err instanceof Error ? err.message : "Não foi possível mover o tema.")
    }
  }

  useEffect(() => {
    if (pinned || !open) return
    function onKey(e: KeyboardEvent) { if (e.key === "Escape") onClose() }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [pinned, open, onClose])

  const parents = themes.filter(t => !t.parent_id)
  const childrenOf = (id: string) => themes.filter(t => t.parent_id === id)

  function countForTheme(id: string) {
    const children = childrenOf(id)
    const direct = allMeetings.filter(m => m.theme_id === id).length
    const fromChildren = children.reduce((acc, c) => acc + allMeetings.filter(m => m.theme_id === c.id).length, 0)
    return direct + fromChildren
  }

  function untaggedMeetingsCopy(count: number) {
    if (count === 0) return "Nenhuma reunião fica sem tema."
    if (count === 1) return "A 1 reunião continua, sem tema."
    return `As ${count} reuniões continuam, sem tema.`
  }

  function renderRow(theme: Theme, depth = 0) {
    const children = childrenOf(theme.id)
    return (
      <div key={theme.id}>
        <ThemeRow
          theme={theme}
          depth={depth}
          count={countForTheme(theme.id)}
          selected={selectedThemeId === theme.id}
          expanded={!!expanded[theme.id]}
          hasChildren={children.length > 0}
          draggable={children.length === 0}
          droppable={depth === 0 && theme.id !== activeId}
          onSelect={() => onSelectTheme(theme.id)}
          onToggleExpand={() => toggleExpand(theme.id)}
          onCreateChild={() => setCreating({ parentId: theme.id })}
          onEdit={() => setEditingTheme(theme)}
          onDelete={() => setConfirmDelete(theme)}
        />
        {expanded[theme.id] && children.map(c => renderRow(c, depth + 1))}
      </div>
    )
  }

  const modals = (
    <>
      {editingTheme && (
        <ThemeEditModal mode="edit" theme={editingTheme} onClose={() => setEditingTheme(null)} />
      )}
      {creating && (
        <ThemeEditModal
          mode="create"
          theme={null}
          parentId={creating.parentId}
          onClose={() => setCreating(null)}
          onCreated={t => { if (t.parent_id) expand(t.parent_id) }}
        />
      )}
    </>
  )

  if (!open) return <>{modals}</>

  return (
    <>
      {!pinned && (
        <div className="fixed inset-0 z-30 bg-black/40 backdrop-blur-sm" onClick={onClose} />
      )}
      <div
        className={cn(
          "w-64 flex flex-col bg-[#161616] border-r border-border",
          pinned
            ? "h-full flex-shrink-0"
            : "fixed left-0 top-0 h-full z-40 rounded-r-2xl"
        )}
      >
        <div className="h-14 flex items-center justify-between px-4 border-b border-border flex-shrink-0">
          <span className="font-semibold text-sm text-foreground">Temas</span>
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              onClick={togglePinned}
              title={pinned ? "Desafixar painel" : "Fixar painel"}
            >
              {pinned ? <PinOff size={15} /> : <Pin size={15} />}
            </Button>
            {!pinned && (
              <Button variant="ghost" size="icon" onClick={onClose} title="Fechar (Esc)">
                <X size={16} />
              </Button>
            )}
          </div>
        </div>
        <DndContext
          sensors={sensors}
          onDragStart={e => setActiveId(String(e.active.id))}
          onDragEnd={e => { setActiveId(null); handleDragEnd(e) }}
        >
          <div className="flex-1 overflow-y-auto px-2">
            <button
              onClick={() => onSelectTheme(null)}
              className={cn(
                "w-full text-left rounded-xl px-3 py-2.5 text-sm flex items-center justify-between hover:bg-accent transition-colors",
                selectedThemeId === null && "bg-accent text-foreground font-medium"
              )}
            >
              <span className="flex items-center gap-2 text-muted-foreground hover:text-foreground">
                <Tag size={14} />Todos
              </span>
              <span className="text-xs text-muted-foreground">{allMeetings.length}</span>
            </button>

            {activeId && (
              <div
                ref={setRootRef}
                className={cn(
                  "mx-2 mb-1 px-3 py-1.5 rounded-lg border border-dashed text-[11px] text-muted-foreground text-center transition-colors",
                  rootIsOver ? "border-primary text-primary" : "border-border"
                )}
              >
                Solte aqui para mover para a raiz
              </div>
            )}

            {parents.map(theme => renderRow(theme))}
          </div>

          {dragError && <p className="mx-2 mb-2 text-[11px] text-destructive">{dragError}</p>}
        </DndContext>

        {confirmDelete && (
          <div className="mx-2 mb-2 p-3 rounded-xl bg-destructive/10 border border-destructive/30">
            <p className="text-xs text-foreground">
              Excluir <span className="font-medium">{confirmDelete.name}</span>?
            </p>
            <p className="text-[11px] text-muted-foreground mt-1">
              {untaggedMeetingsCopy(countForTheme(confirmDelete.id))}
              {childrenOf(confirmDelete.id).length > 0 && " As subcategorias sobem para a raiz."}
            </p>
            <div className="flex justify-end gap-2 mt-2">
              <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(null)}>Cancelar</Button>
              <Button
                size="sm"
                onClick={async () => {
                  const id = confirmDelete.id
                  await deleteTheme.mutateAsync(id)
                  if (selectedThemeId === id) onSelectTheme(null)
                  setConfirmDelete(null)
                }}
                disabled={deleteTheme.isPending}
              >
                Excluir
              </Button>
            </div>
          </div>
        )}

        <div className="p-3 border-t border-border">
          <Button variant="ghost" size="sm" className="w-full text-xs" onClick={() => setCreating({ parentId: null })}>
            <Plus size={14} className="mr-1" /> Novo tema
          </Button>
        </div>
      </div>
      {modals}
    </>
  )
}
