import { useState, useEffect, useRef } from "react"
import { createPortal } from "react-dom"
import { Button } from "../ui/button"
import { useCardDetail, useUpdateCard, useLinkCardToMeeting, useDeleteCard } from "../../hooks/useBoard"
import { useMeetings } from "../../hooks/useMeetings"
import { ExpandableText } from "../ui/ExpandableText"
import { CardModalHeader } from "./CardModalHeader"
import { CardTasksSection, parseManualTask, encodeManualTask } from "./CardTasksSection"
import { CardNotesSection } from "./CardNotesSection"

interface Props {
  cardId: string | null
  onClose: () => void
}

// ─── main component ──────────────────────────────────────────────────────────
export function CardDetailModal({ cardId, onClose }: Props) {
  const { data: card, isLoading } = useCardDetail(cardId)
  const updateCard = useUpdateCard()
  const linkCard = useLinkCardToMeeting()
  const deleteCard = useDeleteCard()
  const [description, setDescription] = useState("")
  const [descriptionAtEditStart, setDescriptionAtEditStart] = useState("")
  const [editingNotes, setEditingNotes] = useState(false)
  const [linkingMeeting, setLinkingMeeting] = useState(false)
  const [selectedMeetingId, setSelectedMeetingId] = useState("")
  const [confirmDelete, setConfirmDelete] = useState(false)
  const { data: meetings = [] } = useMeetings()
  const panelRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (card) {
      setDescription(card.description)
      setLinkingMeeting(false)
      setSelectedMeetingId("")
    }
  }, [card?.id])

  const onCloseRef = useRef(onClose)
  useEffect(() => {
    onCloseRef.current = onClose
  })

  useEffect(() => {
    if (!cardId) return
    const previouslyFocused = document.activeElement as HTMLElement | null
    panelRef.current?.focus()
    return () => {
      previouslyFocused?.focus()
    }
  }, [cardId])

  useEffect(() => {
    if (!cardId) return

    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault()
        // Dois estágios: com a edição aberta, o primeiro Escape cancela a edição
        // em vez de fechar o modal e descartar o texto digitado.
        if (editingNotes) {
          cancelEditing()
          return
        }
        onCloseRef.current()
        return
      }
      if (e.key !== "Tab") return
      const panel = panelRef.current
      if (!panel) return
      const focusable = panel.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
      )
      if (focusable.length === 0) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (!Array.from(focusable).includes(document.activeElement as HTMLElement)) {
        e.preventDefault()
        const target = e.shiftKey ? last : first
        target.focus()
        return
      }
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }

    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [cardId, editingNotes])

  useEffect(() => {
    setEditingNotes(false)
    setConfirmDelete(false)
  }, [cardId])

  useEffect(() => {
    if (!confirmDelete) return
    const timer = setTimeout(() => setConfirmDelete(false), 4000)
    return () => clearTimeout(timer)
  }, [confirmDelete])

  if (!cardId) return null

  const isManual = card?.source === "manual"
  const manualTasks = card?.manual_tasks ?? []

  function startEditing() {
    setDescriptionAtEditStart(description)
    setEditingNotes(true)
  }
  function cancelEditing() {
    setDescription(descriptionAtEditStart)
    setEditingNotes(false)
  }
  function saveDescription() {
    if (!cardId) return
    updateCard.mutate(
      { id: cardId, description, tasks: isManual ? manualTasks : [] },
      { onSuccess: () => setEditingNotes(false) },
    )
  }

  // O PUT do card substitui a descrição, então mexer nas tasks tem de reenviar a
  // descrição já persistida: mandar o state local gravaria um rascunho não salvo.
  function addTask(text: string) {
    if (!cardId || !card || !text) return
    const updated = [...manualTasks, encodeManualTask(text, false)]
    updateCard.mutate({ id: cardId, description: card.description, tasks: updated })
  }

  function toggleTask(index: number) {
    if (!cardId || !card) return
    const { text, done } = parseManualTask(manualTasks[index])
    const updated = manualTasks.map((t, i) =>
      i === index ? encodeManualTask(text, !done) : t
    )
    updateCard.mutate({ id: cardId, description: card.description, tasks: updated })
  }

  function removeTask(index: number) {
    if (!cardId || !card) return
    const updated = manualTasks.filter((_, i) => i !== index)
    updateCard.mutate({ id: cardId, description: card.description, tasks: updated })
  }

  function handleLink() {
    if (!cardId || !selectedMeetingId) return
    linkCard.mutate({ cardId, meetingId: selectedMeetingId }, { onSuccess: onClose })
  }

  function handleDelete() {
    if (!confirmDelete) {
      setConfirmDelete(true)
      return
    }
    if (!cardId) return
    deleteCard.mutate(cardId, { onSuccess: onClose })
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="card-modal-title"
        tabIndex={-1}
        className="bg-background border border-border rounded-lg w-[640px] max-w-[calc(100vw-2rem)] max-h-[80vh] flex flex-col shadow-xl overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        <div
          className="h-[3px] flex-shrink-0"
          style={{ background: (!isManual && card?.theme_color) || "#2a2a2a" }}
        />
        {isLoading && (
          <div className="px-5 py-4 border-b border-border flex-shrink-0">
            <span id="card-modal-title" className="text-xs text-muted-foreground">Carregando...</span>
          </div>
        )}
        {card && (
          <CardModalHeader
            card={card}
            confirmDelete={confirmDelete}
            onDelete={handleDelete}
            onClose={onClose}
          />
        )}

        {/* Body */}
        <div className="flex-1 overflow-y-auto p-5 space-y-5">
          {card && (
            <CardTasksSection
              card={card}
              manualTasks={manualTasks}
              onToggleManual={toggleTask}
              onAddManual={addTask}
              onRemoveManual={removeTask}
            />
          )}

          {/* Resumo da reunião */}
          {!isManual && card?.summary && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Resumo</h3>
              <ExpandableText text={card.summary.content} lines={6} />
            </section>
          )}

          {/* Pontos-chave da reunião */}
          {!isManual && card && card.key_points.length > 0 && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Pontos-chave</h3>
              <ul className="space-y-1">
                {card.key_points.map(kp => (
                  <li key={kp.id} className="text-sm text-muted-foreground flex gap-2">
                    <span className="text-primary mt-0.5 flex-shrink-0">·</span>
                    <ExpandableText text={kp.content} lines={8} className="text-sm" />
                  </li>
                ))}
              </ul>
            </section>
          )}

          {card && (
            <CardNotesSection
              value={description}
              editing={editingNotes}
              pending={updateCard.isPending}
              onChange={setDescription}
              onStartEditing={startEditing}
              onSave={saveDescription}
              onCancel={cancelEditing}
            />
          )}

          {/* Associar a reunião (manual card sem link) */}
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
