import { useState } from "react"
import { X } from "lucide-react"
import { useCreateTheme, useUpdateTheme, type Theme } from "../../hooks/useThemes"
import { useAIConfigured } from "../../hooks/useAIConfigured"
import { Button } from "../ui/button"
import { Dialog } from "../ui/Dialog"

interface Props {
  mode: "create" | "edit"
  theme: Theme | null
  parentId?: string | null
  onClose: () => void
  onCreated?: (theme: Theme) => void
}

const DEFAULT_COLOR = "#7c3aed"

export function ThemeEditModal(props: Props) {
  const key = `${props.mode}:${props.parentId ?? ""}:${JSON.stringify(props.theme)}`
  return <ThemeEditModalContent key={key} {...props} />
}

function ThemeEditModalContent({ mode, theme, parentId = null, onClose, onCreated }: Props) {
  const createTheme = useCreateTheme()
  const updateTheme = useUpdateTheme()
  const { configured: aiConfigured } = useAIConfigured()
  const initialTheme = mode === "edit" ? theme : null
  const [name, setName] = useState(initialTheme?.name ?? "")
  const [description, setDescription] = useState(initialTheme?.description ?? "")
  const [color, setColor] = useState(initialTheme?.color ?? DEFAULT_COLOR)
  const [customPrompt, setCustomPrompt] = useState(initialTheme?.custom_prompt ?? "")
  const [autoAddToBoard, setAutoAddToBoard] = useState(initialTheme?.auto_add_to_board ?? false)
  const [error, setError] = useState("")

  const pending = createTheme.isPending || updateTheme.isPending

  async function handleSave() {
    if (!name.trim()) return
    setError("")
    try {
      if (mode === "edit" && theme) {
        await updateTheme.mutateAsync({
          id: theme.id,
          name: name.trim(),
          description,
          color,
          parent_id: theme.parent_id,
          custom_prompt: customPrompt,
          auto_add_to_board: autoAddToBoard,
        })
      } else {
        const created = await createTheme.mutateAsync({
          name: name.trim(),
          description,
          color,
          parent_id: parentId,
          custom_prompt: customPrompt,
          auto_add_to_board: autoAddToBoard,
        })
        onCreated?.(created)
      }
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível salvar o tema.")
    }
  }

  const title = mode === "edit"
    ? "Editar tema"
    : parentId
      ? "Nova subcategoria"
      : "Novo tema"

  return (
    <Dialog
      open
      onClose={onClose}
      labelledBy="theme-edit-title"
      overlayClassName="bg-black/60 backdrop-blur-sm"
      panelClassName="relative z-10 w-full max-w-md mx-4 bg-[#1a1a1a] border border-border rounded-2xl p-6 shadow-xl"
      closeOnBackdrop
    >
      <div className="flex items-center justify-between mb-5">
        <h2 id="theme-edit-title" className="font-semibold text-sm text-foreground">{title}</h2>
        <button
          type="button"
          onClick={onClose}
          aria-label="Fechar edição de tema"
          className="text-muted-foreground hover:text-foreground"
        >
          <X size={16} />
        </button>
      </div>

      <div className="space-y-4">
        <div>
          <label htmlFor="theme-name" className="block text-xs text-muted-foreground mb-1">Nome</label>
          <input
            id="theme-name"
            autoFocus
            value={name}
            onChange={e => setName(e.target.value)}
            onKeyDown={e => { if (e.key === "Enter") handleSave() }}
            className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary"
            placeholder="Nome do tema"
          />
        </div>

        <div>
          <label htmlFor="theme-description" className="block text-xs text-muted-foreground mb-1">Descrição</label>
          <input
            id="theme-description"
            value={description}
            onChange={e => setDescription(e.target.value)}
            className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary"
            placeholder="Descrição opcional"
          />
        </div>

        <div>
          <label htmlFor="theme-color" className="block text-xs text-muted-foreground mb-1">Cor</label>
          <div className="flex items-center gap-2">
            <input
              id="theme-color"
              type="color"
              value={color}
              onChange={e => setColor(e.target.value)}
              className="w-8 h-8 rounded cursor-pointer border-0 bg-transparent"
            />
            <span className="text-xs text-muted-foreground">{color}</span>
          </div>
        </div>

        <div>
          <label htmlFor="theme-custom-prompt" className="block text-xs text-muted-foreground mb-1">Prompt personalizado</label>
          <textarea
            id="theme-custom-prompt"
            value={customPrompt}
            onChange={e => setCustomPrompt(e.target.value)}
            rows={4}
            disabled={!aiConfigured}
            title={!aiConfigured ? "Disponível quando a IA estiver configurada" : undefined}
            className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary resize-none disabled:opacity-50 disabled:cursor-not-allowed"
            placeholder="Ex: Foque em oportunidades comerciais, objeções e próximos passos."
          />
          <p className="text-[11px] text-muted-foreground mt-1">Vale para resumo, pontos-chave e tarefas. Vazio → usa o prompt padrão.</p>
        </div>

        {!aiConfigured && (
          <p className="text-[10px] text-amber-500">Prompt disponível quando a IA estiver configurada (Configurações → IA).</p>
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

        {error && <p role="alert" className="text-[11px] text-destructive">{error}</p>}
      </div>

      <div className="flex justify-end gap-2 mt-6">
        <Button variant="ghost" size="sm" onClick={onClose}>Cancelar</Button>
        <Button size="sm" onClick={handleSave} disabled={!name.trim() || pending} aria-busy={pending}>
          {pending ? "Salvando…" : "Salvar"}
        </Button>
      </div>
    </Dialog>
  )
}
