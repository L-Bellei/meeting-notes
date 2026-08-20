import { useEffect, useState } from "react"
import { Plus, Tag, X, Trash2, ChevronRight, Pencil, Pin, PinOff } from "lucide-react"
import { useThemes, useDeleteTheme, type Theme } from "../../hooks/useThemes"
import { useMeetings } from "../../hooks/useMeetings"
import { useSidebarPinned } from "../../hooks/useSidebarPinned"
import { ThemeEditModal } from "./ThemeEditModal"
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

  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [creating, setCreating] = useState<{ parentId: string | null } | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [editingTheme, setEditingTheme] = useState<Theme | null>(null)

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

  async function handleDelete(id: string, e: React.MouseEvent) {
    e.stopPropagation()
    if (confirmDelete === id) {
      await deleteTheme.mutateAsync(id)
      setConfirmDelete(null)
      if (selectedThemeId === id) onSelectTheme(null)
    } else {
      setConfirmDelete(id)
    }
  }

  function toggleExpand(id: string, e: React.MouseEvent) {
    e.stopPropagation()
    setExpanded(prev => ({ ...prev, [id]: !prev[id] }))
  }

  function ThemeRow({ theme, depth = 0 }: { theme: Theme; depth?: number }) {
    const children = childrenOf(theme.id)
    const hasChildren = children.length > 0
    const isExpanded = expanded[theme.id]
    const isSelected = selectedThemeId === theme.id
    const isConfirming = confirmDelete === theme.id

    return (
      <div>
        <div
          className={cn(
            "group w-full text-left rounded-xl px-2 py-2 text-sm flex items-center gap-1 hover:bg-accent transition-colors mt-0.5 cursor-pointer",
            isSelected && "bg-accent font-medium",
            depth > 0 && "ml-4"
          )}
          onClick={() => onSelectTheme(theme.id)}
        >
          {/* expand arrow */}
          <button
            onClick={e => toggleExpand(theme.id, e)}
            className={cn("w-4 h-4 flex items-center justify-center flex-shrink-0 text-muted-foreground transition-transform", !hasChildren && "invisible")}
          >
            <ChevronRight size={12} className={cn("transition-transform", isExpanded && "rotate-90")} />
          </button>

          <span className="w-2 h-2 rounded-full flex-shrink-0" style={{ backgroundColor: theme.color }} />
          <span className="truncate flex-1 text-muted-foreground">{theme.name}</span>

          <span className="text-xs text-muted-foreground mr-1">{countForTheme(theme.id)}</span>

          {/* actions: add sub-theme + edit + delete */}
          <div className="hidden group-hover:flex items-center gap-1 flex-shrink-0">
            <button
              title="Nova subcategoria"
              onClick={e => { e.stopPropagation(); setCreating({ parentId: theme.id }) }}
              className="p-0.5 rounded hover:bg-primary/20 text-muted-foreground hover:text-primary"
            >
              <Plus size={11} />
            </button>
            <button
              title="Editar tema"
              onClick={e => { e.stopPropagation(); setEditingTheme(theme) }}
              className="p-0.5 rounded hover:bg-primary/20 text-muted-foreground hover:text-primary"
            >
              <Pencil size={11} />
            </button>
            <button
              title={isConfirming ? "Clique novamente para confirmar" : "Excluir tema"}
              onClick={e => handleDelete(theme.id, e)}
              className={cn("p-0.5 rounded hover:bg-destructive/20 text-muted-foreground hover:text-destructive", isConfirming && "text-destructive bg-destructive/20")}
            >
              <Trash2 size={11} />
            </button>
          </div>
        </div>

        {/* children */}
        {isExpanded && children.map(c => <ThemeRow key={c.id} theme={c} depth={depth + 1} />)}
      </div>
    )
  }

  const modals = (
    <>
      {editingTheme && (
        <ThemeEditModal mode="edit" theme={editingTheme} onClose={() => setEditingTheme(null)} />
      )}
      {creating && (
        <ThemeEditModal mode="create" theme={null} parentId={creating.parentId} onClose={() => setCreating(null)} />
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

          {parents.map(theme => <ThemeRow key={theme.id} theme={theme} />)}
        </div>

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
