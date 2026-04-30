import { useState, useEffect } from "react"
import { createPortal } from "react-dom"
import { X, Pencil, Plus, Trash2 } from "lucide-react"
import { Button } from "../ui/button"
import { useCardDetail, useUpdateCard, useLinkCardToMeeting, type BoardCardDetail } from "../../hooks/useBoard"
import { useUpdateTask } from "../../hooks/useMeeting"
import { useMeetings } from "../../hooks/useMeetings"

interface Props {
  cardId: string | null
  onClose: () => void
}

type TaskItem = BoardCardDetail["tasks"][number]

export function CardDetailModal({ cardId, onClose }: Props) {
  const { data: card, isLoading } = useCardDetail(cardId)
  const updateCard = useUpdateCard()
  const linkCard = useLinkCardToMeeting()
  const [description, setDescription] = useState("")
  const [descriptionAtEditStart, setDescriptionAtEditStart] = useState("")
  const [editing, setEditing] = useState(false)
  const [newTask, setNewTask] = useState("")
  const [linkingMeeting, setLinkingMeeting] = useState(false)
  const [selectedMeetingId, setSelectedMeetingId] = useState("")
  const { data: meetings = [] } = useMeetings()

  useEffect(() => {
    if (card) {
      setDescription(card.description)
      setLinkingMeeting(false)
      setSelectedMeetingId("")
    }
  }, [card?.id])

  if (!cardId) return null

  const isManual = card?.source === "manual"
  const manualTasks = card?.manual_tasks ?? []

  function startEditing() {
    setDescriptionAtEditStart(description)
    setEditing(true)
  }

  function cancelEditing() {
    setDescription(descriptionAtEditStart)
    setEditing(false)
  }

  function saveDescription() {
    if (!cardId) return
    updateCard.mutate(
      { id: cardId, description, tasks: isManual ? manualTasks : [] },
      { onSuccess: () => setEditing(false) },
    )
  }

  function addTask() {
    if (!cardId || !newTask.trim()) return
    const updated = [...manualTasks, newTask.trim()]
    updateCard.mutate({ id: cardId, description, tasks: updated })
    setNewTask("")
  }

  function removeTask(index: number) {
    if (!cardId) return
    const updated = manualTasks.filter((_, i) => i !== index)
    updateCard.mutate({ id: cardId, description, tasks: updated })
  }

  function handleLink() {
    if (!cardId || !selectedMeetingId) return
    linkCard.mutate({ cardId, meetingId: selectedMeetingId }, { onSuccess: onClose })
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-background border border-border rounded-lg w-[640px] max-h-[80vh] flex flex-col shadow-xl overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center gap-3 px-5 py-4 border-b border-border flex-shrink-0">
          {isLoading && <span className="text-xs text-muted-foreground flex-1">Carregando...</span>}
          {card && (
            <>
              <div className="flex items-center gap-1.5">
                <span className="text-xs text-muted-foreground">#{card.number}</span>
                {isManual && <Pencil size={11} className="text-muted-foreground/60" />}
              </div>
              {!isManual && card.theme_name && (
                <span
                  className="text-xs px-1.5 py-0.5 rounded-full"
                  style={card.theme_color ? { background: card.theme_color + "22", color: card.theme_color } : undefined}
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
                  <Button size="sm" onClick={saveDescription}>Salvar</Button>
                  <Button variant="ghost" size="sm" onClick={cancelEditing}>Cancelar</Button>
                </div>
              </div>
            ) : (
              <p
                className="text-sm text-muted-foreground cursor-pointer hover:text-foreground transition-colors min-h-8"
                onClick={startEditing}
              >
                {description || <span className="italic">Clique para editar...</span>}
              </p>
            )}
          </section>

          {isManual && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Tasks</h3>
              <div className="space-y-1.5 mb-2">
                {manualTasks.map((task, i) => (
                  <div key={i} className="flex items-center gap-2 group">
                    <span className="text-sm text-foreground flex-1">{task}</span>
                    <button
                      onClick={() => removeTask(i)}
                      className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-all"
                    >
                      <Trash2 size={13} />
                    </button>
                  </div>
                ))}
              </div>
              <div className="flex gap-2">
                <input
                  className="flex-1 text-sm rounded-lg px-3 py-1.5 bg-input border border-border text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-primary"
                  placeholder="Nova task..."
                  value={newTask}
                  onChange={e => setNewTask(e.target.value)}
                  onKeyDown={e => e.key === "Enter" && addTask()}
                />
                <Button size="sm" variant="ghost" onClick={addTask} disabled={!newTask.trim()}>
                  <Plus size={14} />
                </Button>
              </div>
            </section>
          )}

          {!isManual && card && card.tasks.length > 0 && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">
                Tasks ({card.tasks.filter(t => t.completed).length}/{card.tasks.length})
              </h3>
              <div className="space-y-1.5">
                {card.tasks.map(task => (
                  <TaskRow key={task.id} task={task} meetingId={card.meeting_id ?? ""} />
                ))}
              </div>
            </section>
          )}

          {!isManual && card?.summary && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Resumo</h3>
              <p className="text-sm text-muted-foreground whitespace-pre-wrap">{card.summary.content}</p>
            </section>
          )}

          {!isManual && card && card.key_points.length > 0 && (
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

          {isManual && !card?.meeting_id && (
            <section className="border-t border-border pt-4">
              {!linkingMeeting ? (
                <Button variant="ghost" size="sm" onClick={() => setLinkingMeeting(true)}>
                  Associar a uma reunião
                </Button>
              ) : (
                <div className="space-y-2">
                  <h3 className="text-xs font-medium text-muted-foreground uppercase">Associar a reunião</h3>
                  <select
                    className="w-full text-sm rounded-lg px-3 py-2 bg-input border border-border text-foreground focus:outline-none"
                    value={selectedMeetingId}
                    onChange={e => setSelectedMeetingId(e.target.value)}
                  >
                    <option value="">Selecionar reunião...</option>
                    {meetings.map(m => (
                      <option key={m.id} value={m.id}>{m.title}</option>
                    ))}
                  </select>
                  <div className="flex gap-2">
                    <Button size="sm" onClick={handleLink} disabled={!selectedMeetingId || linkCard.isPending}>
                      {linkCard.isPending ? "Associando..." : "Confirmar"}
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => { setLinkingMeeting(false); setSelectedMeetingId("") }}>
                      Cancelar
                    </Button>
                  </div>
                </div>
              )}
            </section>
          )}
        </div>
      </div>
    </div>,
    document.body,
  )
}

function TaskRow({ task, meetingId }: { task: TaskItem; meetingId: string }) {
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
