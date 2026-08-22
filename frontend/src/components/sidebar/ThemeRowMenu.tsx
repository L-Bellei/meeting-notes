import { useEffect, useRef } from "react"
import { createPortal } from "react-dom"
import { Plus, Pencil, Trash2 } from "lucide-react"

interface Props {
  anchor: HTMLElement | null
  canAddChild: boolean
  onAddChild: () => void
  onEdit: () => void
  onDelete: () => void
  onClose: () => void
}

export function ThemeRowMenu({ anchor, canAddChild, onAddChild, onEdit, onDelete, onClose }: Props) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (anchor && anchor.contains(e.target as Node)) return
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.stopPropagation()
        onClose()
      }
    }
    document.addEventListener("mousedown", onDown)
    document.addEventListener("keydown", onKey, true)
    // scroll events don't bubble, so capture phase is needed to see the inner list container scrolling
    document.addEventListener("scroll", onClose, true)
    window.addEventListener("resize", onClose)
    return () => {
      document.removeEventListener("mousedown", onDown)
      document.removeEventListener("keydown", onKey, true)
      document.removeEventListener("scroll", onClose, true)
      window.removeEventListener("resize", onClose)
    }
  }, [onClose, anchor])

  if (!anchor) return null
  const r = anchor.getBoundingClientRect()

  return createPortal(
    <div
      ref={ref}
      style={{ top: Math.min(r.bottom + 4, window.innerHeight - 130), left: r.left - 150 }}
      className="fixed z-50 w-44 py-1 bg-[#1a1a1a] border border-border rounded-xl shadow-xl"
    >
      {canAddChild && (
        <button type="button" onClick={onAddChild} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-muted-foreground hover:bg-accent hover:text-foreground">
          <Plus size={12} /> Nova subcategoria
        </button>
      )}
      <button type="button" onClick={onEdit} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-muted-foreground hover:bg-accent hover:text-foreground">
        <Pencil size={12} /> Editar tema
      </button>
      <button type="button" onClick={onDelete} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-destructive hover:bg-destructive/10">
        <Trash2 size={12} /> Excluir tema
      </button>
    </div>,
    document.body
  )
}
