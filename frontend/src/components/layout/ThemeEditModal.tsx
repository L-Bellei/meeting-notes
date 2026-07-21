import { useEffect, useState } from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"
import { useUpdateTheme, type Theme } from "../../hooks/useThemes"
import { useAIConfigured } from "../../hooks/useAIConfigured"
import { Button } from "../ui/button"

interface Props {
  theme: Theme | null
  onClose: () => void
}

export function ThemeEditModal({ theme, onClose }: Props) {
  const updateTheme = useUpdateTheme()
  const { configured: aiConfigured } = useAIConfigured()
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [color, setColor] = useState("#7c3aed")
  const [customPrompt, setCustomPrompt] = useState("")
  const [summaryPrompt, setSummaryPrompt] = useState("")
  const [keyPointsPrompt, setKeyPointsPrompt] = useState("")
  const [tasksPrompt, setTasksPrompt] = useState("")
  const [autoAddToBoard, setAutoAddToBoard] = useState(false)

  useEffect(() => {
    if (theme) {
      setName(theme.name)
      setDescription(theme.description)
      setColor(theme.color)
      setCustomPrompt(theme.custom_prompt)
      setSummaryPrompt(theme.custom_summary_prompt)
      setKeyPointsPrompt(theme.custom_key_points_prompt)
      setTasksPrompt(theme.custom_tasks_prompt)
      setAutoAddToBoard(theme.auto_add_to_board)
    }
  }, [theme])

  if (!theme) return null

  async function handleSave() {
    if (!name.trim() || !theme) return
    await updateTheme.mutateAsync({
      id: theme.id,
      name: name.trim(),
      description,
      color,
      parent_id: theme.parent_id,
      custom_prompt: customPrompt,
      custom_summary_prompt: summaryPrompt,
      custom_key_points_prompt: keyPointsPrompt,
      custom_tasks_prompt: tasksPrompt,
      auto_add_to_board: autoAddToBoard,
    })
    onClose()
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative z-10 w-full max-w-md mx-4 bg-[#1a1a1a] border border-border rounded-2xl p-6 shadow-xl">
        <div className="flex items-center justify-between mb-5">
          <h2 className="font-semibold text-sm text-foreground">Editar tema</h2>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X size={16} />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-xs text-muted-foreground mb-1">Nome</label>
            <input
              value={name}
              onChange={e => setName(e.target.value)}
              className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary"
              placeholder="Nome do tema"
            />
          </div>

          <div>
            <label className="block text-xs text-muted-foreground mb-1">Descrição</label>
            <input
              value={description}
              onChange={e => setDescription(e.target.value)}
              className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary"
              placeholder="Descrição opcional"
            />
          </div>

          <div>
            <label className="block text-xs text-muted-foreground mb-1">Cor</label>
            <div className="flex items-center gap-2">
              <input
                type="color"
                value={color}
                onChange={e => setColor(e.target.value)}
                className="w-8 h-8 rounded cursor-pointer border-0 bg-transparent"
              />
              <span className="text-xs text-muted-foreground">{color}</span>
            </div>
          </div>

          <div>
            <label className="block text-xs text-muted-foreground mb-1">Prompt geral</label>
            <textarea
              value={customPrompt}
              onChange={e => setCustomPrompt(e.target.value)}
              rows={3}
              disabled={!aiConfigured}
              title={!aiConfigured ? "Disponível quando a IA estiver configurada" : undefined}
              className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary resize-none disabled:opacity-50 disabled:cursor-not-allowed"
              placeholder="Ex: Foque em oportunidades comerciais, objeções e próximos passos."
            />
            <p className="text-[11px] text-muted-foreground mt-1">Vazio nos campos abaixo → usa o prompt geral; geral vazio → usa o padrão.</p>
          </div>

          <div>
            <label className="block text-xs text-muted-foreground mb-1">Prompt do resumo</label>
            <textarea
              value={summaryPrompt}
              onChange={e => setSummaryPrompt(e.target.value)}
              rows={2}
              disabled={!aiConfigured}
              title={!aiConfigured ? "Disponível quando a IA estiver configurada" : undefined}
              className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary resize-none disabled:opacity-50 disabled:cursor-not-allowed"
              placeholder="Instruções específicas para o resumo (opcional)"
            />
          </div>

          <div>
            <label className="block text-xs text-muted-foreground mb-1">Prompt dos pontos-chave</label>
            <textarea
              value={keyPointsPrompt}
              onChange={e => setKeyPointsPrompt(e.target.value)}
              rows={2}
              disabled={!aiConfigured}
              title={!aiConfigured ? "Disponível quando a IA estiver configurada" : undefined}
              className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary resize-none disabled:opacity-50 disabled:cursor-not-allowed"
              placeholder="Instruções específicas para os pontos-chave (opcional)"
            />
          </div>

          <div>
            <label className="block text-xs text-muted-foreground mb-1">Prompt das tarefas</label>
            <textarea
              value={tasksPrompt}
              onChange={e => setTasksPrompt(e.target.value)}
              rows={2}
              disabled={!aiConfigured}
              title={!aiConfigured ? "Disponível quando a IA estiver configurada" : undefined}
              className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary resize-none disabled:opacity-50 disabled:cursor-not-allowed"
              placeholder="Instruções específicas para as tarefas (opcional)"
            />
          </div>

          {!aiConfigured && (
            <p className="text-[10px] text-amber-500">Prompts disponíveis quando a IA estiver configurada (Configurações → IA).</p>
          )}

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="auto_add_to_board"
              checked={autoAddToBoard}
              onChange={e => setAutoAddToBoard(e.target.checked)}
              className="w-4 h-4 rounded border-border accent-primary cursor-pointer"
            />
            <label htmlFor="auto_add_to_board" className="text-xs text-muted-foreground cursor-pointer select-none">
              Adicionar ao board automaticamente após processamento
            </label>
          </div>
        </div>

        <div className="flex justify-end gap-2 mt-6">
          <Button variant="ghost" size="sm" onClick={onClose}>Cancelar</Button>
          <Button size="sm" onClick={handleSave} disabled={!name.trim() || updateTheme.isPending}>
            {updateTheme.isPending ? "Salvando…" : "Salvar"}
          </Button>
        </div>
      </div>
    </div>,
    document.body
  )
}
