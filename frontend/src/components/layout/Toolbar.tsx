import { Menu, Mic, Settings } from "lucide-react"
import { Button } from "../ui/button"

interface ToolbarProps {
  onToggleSidebar: () => void
  onRecord: () => void
  onSettings: () => void
}

export function Toolbar({ onToggleSidebar, onRecord, onSettings }: ToolbarProps) {
  return (
    <div className="h-14 border-b border-border flex items-center px-4 gap-3 flex-shrink-0 bg-background">
      <Button variant="ghost" size="icon" onClick={onToggleSidebar}>
        <Menu size={18} />
      </Button>
      <span className="font-semibold text-sm text-foreground flex-1">Meeting Notes</span>
      <Button size="sm" onClick={onRecord}>
        <Mic size={14} className="mr-1.5" /> Gravar
      </Button>
      <Button variant="ghost" size="icon" onClick={onSettings}>
        <Settings size={18} />
      </Button>
    </div>
  )
}
