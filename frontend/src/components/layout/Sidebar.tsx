import { useState } from "react"
import { Plus, Tag } from "lucide-react"
import { useThemes, useCreateTheme } from "../../hooks/useThemes"
import { useMeetings } from "../../hooks/useMeetings"
import { Button } from "../ui/button"
import { cn } from "../../lib/utils"

interface SidebarProps {
  selectedThemeId: string | null
  onSelectTheme: (id: string | null) => void
}

export function Sidebar({ selectedThemeId, onSelectTheme }: SidebarProps) {
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
    await createTheme.mutateAsync({ name: newName.trim(), description: "", color: "#6366F1" })
    setNewName("")
    setCreating(false)
  }

  return (
    <div className="w-48 border-r h-full flex flex-col bg-muted/30">
      <div className="p-3 border-b">
        <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Temas</span>
      </div>
      <div className="flex-1 overflow-y-auto">
        <button
          onClick={() => onSelectTheme(null)}
          className={cn(
            "w-full text-left px-3 py-2 text-sm flex items-center justify-between hover:bg-accent transition-colors",
            selectedThemeId === null && "bg-accent font-medium"
          )}
        >
          <span className="flex items-center gap-2"><Tag size={14} />Todos</span>
          <span className="text-xs text-muted-foreground">{allMeetings.length}</span>
        </button>
        {themes.map(theme => (
          <button
            key={theme.id}
            onClick={() => onSelectTheme(theme.id)}
            className={cn(
              "w-full text-left px-3 py-2 text-sm flex items-center justify-between hover:bg-accent transition-colors",
              selectedThemeId === theme.id && "bg-accent font-medium"
            )}
          >
            <span className="flex items-center gap-2 truncate">
              <span className="w-2 h-2 rounded-full flex-shrink-0" style={{ backgroundColor: theme.color }} />
              <span className="truncate">{theme.name}</span>
            </span>
            <span className="text-xs text-muted-foreground">{countForTheme(theme.id)}</span>
          </button>
        ))}
      </div>
      <div className="p-2 border-t">
        {creating ? (
          <div className="flex gap-1">
            <input
              autoFocus
              value={newName}
              onChange={e => setNewName(e.target.value)}
              onKeyDown={e => { if (e.key === "Enter") handleCreate(); if (e.key === "Escape") setCreating(false) }}
              placeholder="Nome do tema"
              className="flex-1 text-xs border rounded px-2 py-1 bg-background"
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
  )
}
