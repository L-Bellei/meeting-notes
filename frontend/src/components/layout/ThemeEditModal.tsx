import { useEffect, useState } from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"
import { useUpdateTheme, type Theme } from "../../hooks/useThemes"
import { Button } from "../ui/button"

interface Props {
  theme: Theme | null
  onClose: () => void
}

export function ThemeEditModal({ theme, onClose }: Props) {
  const updateTheme = useUpdateTheme()
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [color, setColor] = useState("#7c3aed")
  const [customPrompt, setCustomPrompt] = useState("")

  useEffect(() => {
    if (theme) {
      setName(theme.name)
      setDescription(theme.description)
      setColor(theme.color)
      setCustomPrompt(theme.custom_prompt)
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
            <label className="block text-xs text-muted-foreground mb-1">Prompt personalizado</label>
            <textarea
              value={customPrompt}
              onChange={e => setCustomPrompt(e.target.value)}
              rows={4}
              className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary resize-none"
              placeholder="Ex: Foque em oportunidades comerciais, objeções e próximos passos. Deixe em branco para usar o comportamento padrão."
            />
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
