import { useState } from "react"
import { Plus, Tag, X } from "lucide-react"
import { useThemes, useCreateTheme } from "../../hooks/useThemes"
import { useMeetings } from "../../hooks/useMeetings"
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
  const [creating, setCreating] = useState(false)
  const [newName, setNewName] = useState("")

  function countForTheme(id: string) {
    return allMeetings.filter(m => m.theme_id === id).length
  }

  async function handleCreate() {
    if (!newName.trim()) return
    await createTheme.mutateAsync({ name: newName.trim(), description: "", color: "#7c3aed" })
    setNewName("")
    setCreating(false)
  }

  function selectAndClose(id: string | null) {
    onSelectTheme(id)
    onClose()
  }

  return (
    <>
      {open && (
        <div
          className="fixed inset-0 z-30 bg-black/40 backdrop-blur-sm"
          onClick={onClose}
        />
      )}
      <div
        className={cn(
          "fixed left-0 top-0 h-full z-40 w-64 flex flex-col",
          "bg-[#110d1e] border-r border-border rounded-r-2xl",
          "transform transition-transform duration-300 ease-in-out",
          open ? "translate-x-0" : "-translate-x-full"
        )}
      >
        <div className="h-14 flex items-center justify-between px-4 border-b border-border flex-shrink-0">
          <span className="font-semibold text-sm text-foreground">Meeting Notes</span>
          <Button variant="ghost" size="icon" onClick={onClose}>
            <X size={16} />
          </Button>
        </div>
        <div className="px-2 py-2 flex-shrink-0">
          <span className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider px-2">Temas</span>
        </div>
        <div className="flex-1 overflow-y-auto px-2">
          <button
            onClick={() => selectAndClose(null)}
            className={cn(
              "w-full text-left rounded-xl mx-0 px-3 py-2.5 text-sm flex items-center justify-between hover:bg-accent transition-colors",
              selectedThemeId === null && "bg-accent text-foreground font-medium"
            )}
          >
            <span className="flex items-center gap-2 text-muted-foreground hover:text-foreground">
              <Tag size={14} />Todos
            </span>
            <span className="text-xs text-muted-foreground">{allMeetings.length}</span>
          </button>
          {themes.map(theme => (
            <button
              key={theme.id}
              onClick={() => selectAndClose(theme.id)}
              className={cn(
                "w-full text-left rounded-xl px-3 py-2.5 text-sm flex items-center justify-between hover:bg-accent transition-colors mt-0.5",
                selectedThemeId === theme.id && "bg-accent font-medium"
              )}
            >
              <span className="flex items-center gap-2 truncate text-muted-foreground">
                <span className="w-2 h-2 rounded-full flex-shrink-0" style={{ backgroundColor: theme.color }} />
                <span className="truncate">{theme.name}</span>
              </span>
              <span className="text-xs text-muted-foreground">{countForTheme(theme.id)}</span>
            </button>
          ))}
        </div>
        <div className="p-3 border-t border-border">
          {creating ? (
            <div className="flex gap-1">
              <input
                autoFocus
                value={newName}
                onChange={e => setNewName(e.target.value)}
                onKeyDown={e => { if (e.key === "Enter") handleCreate(); if (e.key === "Escape") setCreating(false) }}
                placeholder="Nome do tema"
                className="flex-1 text-xs rounded-lg px-2 py-1.5"
              />
              <Button size="sm" onClick={handleCreate} disabled={createTheme.isPending}>+</Button>
            </div>
          ) : (
            <Button variant="ghost" size="sm" className="w-full text-xs" onClick={() => setCreating(true)}>
              <Plus size={14} className="mr-1" /> Novo tema
            </Button>
          )}
        </div>
      </div>
    </>
  )
}
