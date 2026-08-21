import { useDroppable } from "@dnd-kit/core"
import { cn } from "../../lib/utils"

interface Props {
  active: boolean
}

export function RootDropZone({ active }: Props) {
  const { setNodeRef, isOver } = useDroppable({ id: "drop-root" })

  return (
    <div
      ref={setNodeRef}
      className={cn(
        "mx-2 mb-1 px-3 py-1.5 rounded-lg border border-dashed text-[11px] text-center transition-colors",
        active && isOver
          ? "border-primary text-primary"
          : "border-border/50 text-muted-foreground/50"
      )}
    >
      Solte aqui para mover para a raiz
    </div>
  )
}
