import { useState } from "react"
import { Plus, Tag, X, Trash2, ChevronRight, Pencil } from "lucide-react"
import { useThemes, useCreateTheme, useDeleteTheme, type Theme } from "../../hooks/useThemes"
import { useMeetings } from "../../hooks/useMeetings"
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
  const { data: themes = [] } = useThemes()
  const { data: allMeetings = [] } = useMeetings()
  const createTheme = useCreateTheme()
  const deleteTheme = useDeleteTheme()

  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [creating, setCreating] = useState<"root" | string | null>(null) // "root" or parent theme ID
  const [newName, setNewName] = useState("")
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [editingTheme, setEditingTheme] = useState<Theme | null>(null)

  const parents = themes.filter(t => !t.parent_id)
  const childrenOf = (id: string) => themes.filter(t => t.parent_id === id)

  function countForTheme(id: string) {
    const children = childrenOf(id)
    const direct = allMeetings.filter(m => m.theme_id === id).length
    const fromChildren = children.reduce((acc, c) => acc + allMeetings.filter(m => m.theme_id === c.id).length, 0)
    return direct + fromChildren
  }

  async function handleCreate(parentID?: string) {
    if (!newName.trim()) return
    await createTheme.mutateAsync({
      name: newName.trim(),
      description: "",
      color: "#7c3aed",
      parent_id: parentID ?? null,
    })
    setNewName("")
    setCreating(null)
    if (parentID) setExpanded(e => ({ ...e, [parentID]: true }))
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

  function selectAndClose(id: string | null) {
    onSelectTheme(id)
    onClose()
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
          onClick={() => selectAndClose(theme.id)}
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
          <div className="hidden group-hover:flex items-center gap-0.5 flex-shrink-0">
            <button
              title="Nova subcategoria"
              onClick={e => { e.stopPropagation(); setCreating(theme.id); setNewName("") }}
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

        {/* inline create input for sub-theme */}
        {creating === theme.id && (
          <div className={cn("flex gap-1 mt-0.5", depth > 0 ? "ml-8" : "ml-4")} onClick={e => e.stopPropagation()}>
            <input
              autoFocus
              value={newName}
              onChange={e => setNewName(e.target.value)}
              onKeyDown={e => {
                if (e.key === "Enter") handleCreate(theme.id)
                if (e.key === "Escape") setCreating(null)
              }}
              placeholder="Nome da subcategoria"
              className="flex-1 text-xs rounded-lg px-2 py-1.5"
            />
            <Button size="sm" onClick={() => handleCreate(theme.id)} disabled={createTheme.isPending}>+</Button>
          </div>
        )}

        {/* children */}
        {isExpanded && children.map(c => <ThemeRow key={c.id} theme={c} depth={depth + 1} />)}
      </div>
    )
  }

  return (
    <>
      {open && (
        <div className="fixed inset-0 z-30 bg-black/40 backdrop-blur-sm" onClick={onClose} />
      )}
      <div
        className={cn(
          "fixed left-0 top-0 h-full z-40 w-64 flex flex-col",
          "bg-[#161616] border-r border-border rounded-r-2xl",
          "transform transition-transform duration-300 ease-in-out",
          open ? "translate-x-0" : "-translate-x-full"
        )}
      >
        <div className="h-14 flex items-center justify-between px-4 border-b border-border flex-shrink-0">
          <span className="font-semibold text-sm text-foreground">Meeting Notes</span>
          <Button variant="ghost" size="icon" onClick={onClose}><X size={16} /></Button>
        </div>
        <div className="px-2 py-2 flex-shrink-0">
          <span className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider px-2">Temas</span>
        </div>
        <div className="flex-1 overflow-y-auto px-2">
          <button
            onClick={() => selectAndClose(null)}
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
          {creating === "root" ? (
            <div className="flex gap-1">
              <input
                autoFocus
                value={newName}
                onChange={e => setNewName(e.target.value)}
                onKeyDown={e => { if (e.key === "Enter") handleCreate(); if (e.key === "Escape") setCreating(null) }}
                placeholder="Nome do tema"
                className="flex-1 text-xs rounded-lg px-2 py-1.5"
              />
              <Button size="sm" onClick={() => handleCreate()} disabled={createTheme.isPending}>+</Button>
            </div>
          ) : (
            <Button variant="ghost" size="sm" className="w-full text-xs" onClick={() => { setCreating("root"); setNewName("") }}>
              <Plus size={14} className="mr-1" /> Novo tema
            </Button>
          )}
        </div>
      </div>
      <ThemeEditModal theme={editingTheme} onClose={() => setEditingTheme(null)} />
    </>
  )
}
