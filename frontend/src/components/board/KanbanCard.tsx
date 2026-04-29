import type { BoardCardSummary } from "../../hooks/useBoard"

interface Props {
  card: BoardCardSummary
  onClick: () => void
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins <= 0) return "agora"
  if (mins < 60) return `há ${mins}m`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `há ${hours}h`
  return `há ${Math.floor(hours / 24)}d`
}

export function KanbanCard({ card, onClick }: Props) {
  const { total, completed } = card.task_progress
  return (
    <div
      className="bg-card border border-border rounded-md p-3 cursor-pointer hover:border-primary/50 transition-colors select-none"
      onClick={onClick}
    >
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs text-muted-foreground">#{card.number}</span>
        {card.theme_color && (
          <span
            className="text-xs px-1.5 py-0.5 rounded-full"
            style={{ background: card.theme_color + "22", color: card.theme_color }}
          >
            {card.theme_name}
          </span>
        )}
      </div>
      <p className="text-sm font-medium text-foreground mb-1 line-clamp-1">{card.meeting_title}</p>
      {card.description && (
        <p className="text-xs text-muted-foreground line-clamp-2 mb-2">{card.description}</p>
      )}
      <div className="flex items-center gap-2">
        {total > 0 && (
          <div className="flex gap-1 flex-wrap">
            {Array.from({ length: Math.min(total, 10) }, (_, i) => (
              <div
                key={i}
                className={`w-2 h-2 rounded-full ${i < completed ? "bg-green-500" : "bg-muted-foreground/30"}`}
              />
            ))}
            {total > 10 && <span className="text-xs text-muted-foreground">+{total - 10}</span>}
          </div>
        )}
        <span className="text-xs text-muted-foreground ml-auto">{relativeTime(card.created_at)}</span>
      </div>
    </div>
  )
}
