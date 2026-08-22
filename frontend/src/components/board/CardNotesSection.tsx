import { Pencil } from "lucide-react"
import { Button } from "../ui/button"

interface Props {
  value: string
  editing: boolean
  pending: boolean
  onChange: (v: string) => void
  onStartEditing: () => void
  onSave: () => void
  onCancel: () => void
}

export function CardNotesSection({
  value, editing, pending, onChange, onStartEditing, onSave, onCancel,
}: Props) {
  return (
    <section>
      <div className="flex items-center gap-2 mb-2">
        <h3 className="text-xs font-medium text-muted-foreground uppercase">Suas anotações</h3>
        {!editing && (
          <button
            onClick={onStartEditing}
            aria-label="Editar anotações"
            className="text-muted-foreground/60 hover:text-foreground transition-colors"
          >
            <Pencil size={12} />
          </button>
        )}
      </div>

      {editing ? (
        <div className="space-y-2">
          <textarea
            className="w-full text-sm bg-input border border-border rounded px-3 py-2 h-40 resize-none"
            value={value}
            onChange={e => onChange(e.target.value)}
            autoFocus
          />
          <div className="flex gap-2">
            <Button size="sm" onClick={onSave} disabled={pending}>
              {pending ? "Salvando..." : "Salvar"}
            </Button>
            <Button variant="ghost" size="sm" onClick={onCancel}>Cancelar</Button>
          </div>
        </div>
      ) : value ? (
        <p className="text-sm text-muted-foreground whitespace-pre-wrap leading-relaxed">{value}</p>
      ) : (
        <p className="text-sm italic text-muted-foreground/50">Nada anotado ainda</p>
      )}
    </section>
  )
}
