import { useState, useEffect, useRef } from "react"
import { Search } from "lucide-react"
import { Button } from "../ui/button"
import type { BoardCardFilters } from "../../hooks/useBoard"

interface Props {
  filters: BoardCardFilters
  onChange: (f: BoardCardFilters) => void
}

export function BoardFilters({ filters, onChange }: Props) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const hasFilters = !!(filters.title || filters.number != null ||
    filters.created_after || filters.created_before)

  useEffect(() => {
    if (!open) return
    function handleOutsideClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener("mousedown", handleOutsideClick)
    return () => document.removeEventListener("mousedown", handleOutsideClick)
  }, [open])

  function clear() {
    onChange({})
  }

  return (
    <div ref={containerRef} className="relative">
      <Button
        variant={hasFilters ? "outline" : "ghost"}
        size="sm"
        onClick={() => setOpen(o => !o)}
        className="gap-1.5"
      >
        <Search size={14} />
        {hasFilters ? "Filtros ativos" : "Filtrar"}
      </Button>

      {open && (
        <div className="absolute right-0 top-full mt-1 z-20 bg-background border border-border rounded-lg shadow-lg p-4 w-72 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">Filtros</span>
            {hasFilters && (
              <Button variant="ghost" size="sm" onClick={clear} className="text-xs h-6 px-2">
                Limpar
              </Button>
            )}
          </div>

          <div>
            <label className="text-xs text-muted-foreground">Título</label>
            <input
              className="w-full text-sm bg-input border border-border rounded px-2 py-1 mt-1"
              placeholder="Buscar..."
              value={filters.title ?? ""}
              onChange={e => onChange({ ...filters, title: e.target.value || undefined })}
            />
          </div>

          <div>
            <label className="text-xs text-muted-foreground">Número (#)</label>
            <input
              type="number"
              min={1}
              step={1}
              className="w-full text-sm bg-input border border-border rounded px-2 py-1 mt-1"
              placeholder="ex: 5"
              value={filters.number ?? ""}
              onChange={e => onChange({ ...filters, number: e.target.value ? Number(e.target.value) : undefined })}
            />
          </div>

          <div className="grid grid-cols-2 gap-2">
            <div>
              <label className="text-xs text-muted-foreground">Criado após</label>
              <input
                type="date"
                className="w-full text-xs bg-input border border-border rounded px-2 py-1 mt-1"
                value={filters.created_after ? filters.created_after.slice(0, 10) : ""}
                onChange={e => onChange({ ...filters, created_after: e.target.value ? e.target.value + "T00:00:00Z" : undefined })}
              />
            </div>
            <div>
              <label className="text-xs text-muted-foreground">Criado antes</label>
              <input
                type="date"
                className="w-full text-xs bg-input border border-border rounded px-2 py-1 mt-1"
                value={filters.created_before ? filters.created_before.slice(0, 10) : ""}
                onChange={e => onChange({ ...filters, created_before: e.target.value ? e.target.value + "T23:59:59Z" : undefined })}
              />
            </div>
          </div>

          <div className="flex justify-end">
            <Button size="sm" onClick={() => setOpen(false)}>Aplicar</Button>
          </div>
        </div>
      )}
    </div>
  )
}
