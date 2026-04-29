import { useState, useEffect } from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"
import { Button } from "../ui/button"
import { useCardDetail, useUpdateCard } from "../../hooks/useBoard"
import { useUpdateTask } from "../../hooks/useMeeting"

interface Props {
  cardId: string | null
  onClose: () => void
}

export function CardDetailModal({ cardId, onClose }: Props) {
  const { data: card } = useCardDetail(cardId)
  const updateCard = useUpdateCard()
  const [description, setDescription] = useState("")
  const [editing, setEditing] = useState(false)

  useEffect(() => {
    if (card) setDescription(card.description)
  }, [card?.id])

  if (!cardId) return null

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-background border border-border rounded-lg w-[640px] max-h-[80vh] flex flex-col shadow-xl"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center gap-3 px-5 py-4 border-b border-border flex-shrink-0">
          {card && (
            <>
              <span className="text-xs text-muted-foreground">#{card.number}</span>
              {card.theme_color && (
                <span
                  className="text-xs px-1.5 py-0.5 rounded-full"
                  style={{ background: card.theme_color + "22", color: card.theme_color }}
                >
                  {card.theme_name}
                </span>
              )}
              <h2 className="font-semibold text-sm flex-1">{card.meeting_title}</h2>
              <span className="text-xs text-muted-foreground">{card.status}</span>
            </>
          )}
          <Button variant="ghost" size="icon" onClick={onClose}><X size={16} /></Button>
        </div>

        <div className="flex-1 overflow-y-auto p-5 space-y-5">
          <section>
            <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Descrição</h3>
            {editing ? (
              <div className="space-y-2">
                <textarea
                  className="w-full text-sm bg-input border border-border rounded px-3 py-2 min-h-24 resize-y"
                  value={description}
                  onChange={e => setDescription(e.target.value)}
                  autoFocus
                />
                <div className="flex gap-2">
                  <Button size="sm" onClick={() => {
                    if (cardId) updateCard.mutate({ id: cardId, description }, { onSuccess: () => setEditing(false) })
                  }}>Salvar</Button>
                  <Button variant="ghost" size="sm" onClick={() => {
                    setDescription(card?.description ?? "")
                    setEditing(false)
                  }}>Cancelar</Button>
                </div>
              </div>
            ) : (
              <p
                className="text-sm text-muted-foreground cursor-pointer hover:text-foreground transition-colors min-h-8"
                onClick={() => setEditing(true)}
              >
                {description || <span className="italic">Clique para editar...</span>}
              </p>
            )}
          </section>

          {card && card.tasks.length > 0 && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">
                Tasks ({card.tasks.filter(t => t.completed).length}/{card.tasks.length})
              </h3>
              <div className="space-y-1.5">
                {card.tasks.map(task => (
                  <TaskRow key={task.id} task={task} meetingId={card.meeting_id} />
                ))}
              </div>
            </section>
          )}

          {card?.summary && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Resumo</h3>
              <p className="text-sm text-muted-foreground whitespace-pre-wrap">{card.summary.content}</p>
            </section>
          )}

          {card && card.key_points.length > 0 && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Pontos-chave</h3>
              <ul className="space-y-1">
                {card.key_points.map(kp => (
                  <li key={kp.id} className="text-sm text-muted-foreground flex gap-2">
                    <span className="text-primary mt-0.5">·</span>
                    {kp.content}
                  </li>
                ))}
              </ul>
            </section>
          )}
        </div>
      </div>
    </div>,
    document.body,
  )
}

function TaskRow({ task, meetingId }: {
  task: { id: string; description: string; completed: boolean; priority: string }
  meetingId: string
}) {
  const updateTask = useUpdateTask(meetingId, task.id)
  return (
    <label className="flex items-start gap-2 cursor-pointer">
      <input
        type="checkbox"
        className="mt-0.5 accent-primary"
        checked={task.completed}
        onChange={e => updateTask.mutate({ completed: e.target.checked })}
      />
      <span className={`text-sm ${task.completed ? "line-through text-muted-foreground" : ""}`}>
        {task.description}
      </span>
    </label>
  )
}
