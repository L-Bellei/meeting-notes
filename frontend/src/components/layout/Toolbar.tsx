import { Menu, Mic, Settings, Search } from "lucide-react"
import { Button } from "../ui/button"

interface ToolbarProps {
  onToggleSidebar: () => void
  onRecord: () => void
  onSettings: () => void
  onSearch: () => void
  recordingHotkey?: string
  activeView: "meetings" | "board"
  onChangeView: (view: "meetings" | "board") => void
}

export function Toolbar({ onToggleSidebar, onRecord, onSettings, onSearch, recordingHotkey, activeView, onChangeView }: ToolbarProps) {
  return (
    <div className="h-14 border-b border-border flex items-center px-4 gap-3 flex-shrink-0 bg-background">
      <Button variant="ghost" size="icon" onClick={onToggleSidebar}>
        <Menu size={18} />
      </Button>
      <span className="font-semibold text-sm text-foreground">Meeting Notes</span>
      <div className="flex gap-1">
        <Button
          size="sm"
          variant={activeView === "meetings" ? "outline" : "ghost"}
          onClick={() => onChangeView("meetings")}
        >
          Reuniões
        </Button>
        <Button
          size="sm"
          variant={activeView === "board" ? "outline" : "ghost"}
          onClick={() => onChangeView("board")}
        >
          Board
        </Button>
      </div>
      <button
        onClick={onSearch}
        className="flex-1 mx-2 flex items-center gap-2 px-3 py-1.5 rounded-full bg-muted/50 border border-border text-muted-foreground text-sm hover:bg-muted transition-colors"
      >
        <Search size={13} />
        <span className="flex-1 text-left text-xs">Pesquisar reuniões...</span>
        <kbd className="text-[10px] bg-background border border-border rounded px-1.5 py-0.5 font-mono leading-none">
          Ctrl K
        </kbd>
      </button>
      <Button size="sm" onClick={onRecord}>
        <Mic size={14} className="mr-1.5" />
        Gravar
        {recordingHotkey && (
          <kbd className="ml-1.5 text-[10px] bg-white/20 rounded px-1 py-0.5 font-mono leading-none">
            {recordingHotkey}
          </kbd>
        )}
      </Button>
      <Button variant="ghost" size="icon" onClick={onSettings}>
        <Settings size={18} />
      </Button>
    </div>
  )
}
