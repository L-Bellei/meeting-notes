import { useState, useEffect, useCallback } from "react"
import { createPortal } from "react-dom"
import { X, Eye, EyeOff } from "lucide-react"
import { useSettings, useUpdateSettings, type Settings } from "../../hooks/useSettings"
import { Button } from "../ui/button"
import { cn } from "../../lib/utils"
import { formatHotkey } from "../../lib/formatHotkey"

interface Props {
  open: boolean
  onClose: () => void
}

const ANTHROPIC_MODELS = [
  { value: "claude-sonnet-4-6", label: "claude-sonnet-4-6 — rápido e equilibrado (padrão)" },
  { value: "claude-opus-4-7",   label: "claude-opus-4-7 — mais capaz, mais lento" },
  { value: "claude-haiku-4-5",  label: "claude-haiku-4-5 — mais rápido, menos detalhado" },
]

const OPENAI_MODELS = [
  { value: "gpt-4o",      label: "gpt-4o — mais capaz, multimodal (padrão)" },
  { value: "gpt-4o-mini", label: "gpt-4o-mini — mais rápido e econômico" },
  { value: "gpt-4-turbo", label: "gpt-4-turbo — alta capacidade, contexto longo" },
]

const WHISPER_LANGUAGES = [
  { value: "pt",   label: "Português (pt)" },
  { value: "en",   label: "Inglês (en)" },
  { value: "es",   label: "Espanhol (es)" },
  { value: "auto", label: "Detectar automaticamente" },
]

const WHISPER_MODELS = [
  { value: "tiny",   label: "tiny — mais rápido, menos preciso" },
  { value: "base",   label: "base — equilíbrio básico" },
  { value: "small",  label: "small — boa precisão, rápido" },
  { value: "medium", label: "medium — alta precisão (padrão)" },
  { value: "large",  label: "large — máxima precisão, mais lento" },
]

function HotkeyCapture({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const [listening, setListening] = useState(false)

  useEffect(() => {
    if (!listening) return
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault()
        setListening(false)
        return
      }
      e.preventDefault()
      const parts: string[] = []
      if (e.ctrlKey)  parts.push("ctrl")
      if (e.shiftKey) parts.push("shift")
      if (e.altKey)   parts.push("alt")
      const key = e.key.toLowerCase()
      if (!["control", "shift", "alt", "meta"].includes(key) && key.length === 1) {
        parts.push(key)
        onChange(parts.join("+"))
        setListening(false)
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [listening, onChange])

  const display = formatHotkey(value)

  return (
    <div className="flex items-center gap-2 mt-1">
      <button
        onClick={() => setListening(l => !l)}
        className={cn(
          "flex-1 text-sm rounded-xl px-3 py-2 text-left border transition-colors font-mono",
          listening
            ? "bg-primary/10 border-primary text-primary animate-pulse"
            : "bg-[#111111] border-border text-foreground hover:border-primary/50"
        )}
      >
        {listening ? "Pressione o atalho..." : display}
      </button>
      <Button
        variant="outline"
        size="sm"
        onClick={() => { onChange("ctrl+shift+r"); setListening(false) }}
      >
        Restaurar
      </Button>
    </div>
  )
}

export function SettingsModal({ open, onClose }: Props) {
  const { data: settings } = useSettings()
  const update = useUpdateSettings()

  const [form, setForm] = useState<Partial<Settings>>({})
  const [showKey, setShowKey] = useState(false)
  const [error, setError] = useState("")
  const [activeTab, setActiveTab] = useState<"geral" | "ia" | "transcricao" | "atalhos">("geral")

  useEffect(() => {
    if (open && settings) setForm(settings)
  }, [open, settings])

  const handleHotkeyChange = useCallback(
    (v: string) => setForm(f => ({ ...f, recording_hotkey: v })),
    []
  )

  if (!open) return null

  function set<K extends keyof Settings>(key: K, value: Settings[K]) {
    setForm(f => ({ ...f, [key]: value }))
  }

  async function handleSave() {
    setError("")
    try {
      await update.mutateAsync(form)
      onClose()
    } catch (e: any) {
      setError(e.message ?? "Erro ao salvar")
    }
  }

  const provider = form.ai_provider ?? "anthropic"
  const models = provider === "anthropic" ? ANTHROPIC_MODELS : OPENAI_MODELS
  const apiKeyField = provider === "anthropic" ? "anthropic_api_key" : "openai_api_key"
  const modelField  = provider === "anthropic" ? "anthropic_model"   : "openai_model"
  const apiKeyPlaceholder = provider === "anthropic" ? "sk-ant-api03-..." : "sk-proj-..."
  const apiKeyHint = provider === "anthropic"
    ? "Obtenha em console.anthropic.com → API Keys"
    : "Obtenha em platform.openai.com → API Keys"
  const apiKeyLabel = provider === "anthropic" ? "Chave de API Anthropic" : "Chave de API OpenAI"

  const content = (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70">
      <div className="bg-[#1a1a1a] border border-border rounded-2xl shadow-2xl w-[440px] max-h-[90vh] flex flex-col">
        {/* header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-border">
          <span className="font-semibold text-sm text-foreground">Configurações</span>
          <Button variant="ghost" size="icon" onClick={onClose}><X size={16} /></Button>
        </div>

        {/* tabs */}
        <div className="flex border-b border-border">
          {(["geral", "ia", "transcricao", "atalhos"] as const).map(tab => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={cn(
                "px-4 py-2.5 text-xs transition-colors border-b-2 -mb-px",
                activeTab === tab
                  ? "border-primary text-foreground font-medium"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              )}
            >
              {tab === "ia" ? "IA" : tab === "transcricao" ? "Transcrição" : tab.charAt(0).toUpperCase() + tab.slice(1)}
            </button>
          ))}
        </div>

        {/* tab content */}
        <div className="overflow-y-auto flex-1">
          {/* Aba Geral */}
          {activeTab === "geral" && (
            <div className="px-5 py-4 space-y-4">
              <div>
                <label className="text-xs text-muted-foreground">Nome do usuário</label>
                <input
                  value={form.user_name ?? ""}
                  onChange={e => set("user_name", e.target.value)}
                  placeholder="Seu nome completo (ex: Leonardo Bellei)"
                  className="w-full mt-1 text-sm rounded-xl px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary bg-[#111111] border border-border text-foreground placeholder:text-muted-foreground/60"
                />
                <p className="text-[10px] text-muted-foreground/60 mt-1">Usado para identificar o autor nas reuniões</p>
              </div>
              <div>
                <label className="text-xs text-muted-foreground">Template de nome de reunião</label>
                <input
                  value={form.meeting_name_template ?? "Reunião {date}"}
                  onChange={e => set("meeting_name_template", e.target.value)}
                  placeholder="Reunião {date}"
                  className="w-full mt-1 text-sm rounded-xl px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary bg-[#111111] border border-border text-foreground placeholder:text-muted-foreground/60"
                />
                <p className="text-[10px] text-muted-foreground/60 mt-1">
                  Usado ao iniciar por atalho. Variáveis: {"{date}"} (dd/mm/aaaa), {"{time}"} (hh:mm)
                </p>
              </div>
            </div>
          )}

          {/* Aba IA */}
          {activeTab === "ia" && (
            <div className="px-5 py-4 space-y-4">
              {/* provider toggle */}
              <div>
                <label className="text-xs text-muted-foreground">Provedor</label>
                <div className="flex mt-1 border border-border rounded-lg overflow-hidden w-fit">
                  {(["anthropic", "openai"] as const).map(p => (
                    <button
                      key={p}
                      onClick={() => set("ai_provider", p)}
                      className={cn(
                        "px-4 py-1.5 text-xs font-medium transition-colors capitalize",
                        provider === p ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-accent"
                      )}
                    >
                      {p === "anthropic" ? "Anthropic" : "OpenAI"}
                    </button>
                  ))}
                </div>
              </div>

              {/* API key */}
              <div>
                <label className="text-xs text-muted-foreground">{apiKeyLabel} <span className="text-destructive">*</span></label>
                <div className="flex gap-1 mt-1">
                  <input
                    type={showKey ? "text" : "password"}
                    value={(form[apiKeyField] as string) ?? ""}
                    onChange={e => set(apiKeyField, e.target.value)}
                    placeholder={apiKeyPlaceholder}
                    className="flex-1 text-sm rounded-xl px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary bg-[#111111] border border-border text-foreground font-mono placeholder:text-muted-foreground/60 placeholder:font-sans"
                  />
                  <Button variant="ghost" size="icon" onClick={() => setShowKey(v => !v)}>
                    {showKey ? <EyeOff size={14} /> : <Eye size={14} />}
                  </Button>
                </div>
                <p className="text-[10px] text-muted-foreground/60 mt-1">{apiKeyHint}</p>
              </div>

              {/* model */}
              <div>
                <label className="text-xs text-muted-foreground">Modelo</label>
                <select
                  value={(form[modelField] as string) ?? ""}
                  onChange={e => set(modelField, e.target.value)}
                  className="w-full mt-1 text-sm rounded-xl px-3 py-2 focus:outline-none bg-[#111111] border border-border text-foreground"
                >
                  {models.map(m => (
                    <option key={m.value} value={m.value}>{m.label}</option>
                  ))}
                </select>
              </div>

              {/* auto generate toggle */}
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-xs text-foreground">Auto-gerar ao terminar gravação</p>
                  <p className="text-[10px] text-muted-foreground/60 mt-0.5">Gera resumo, pontos-chave e tarefas automaticamente</p>
                </div>
                <button
                  onClick={() => set("auto_generate", form.auto_generate === "true" ? "false" : "true")}
                  className={cn(
                    "w-10 h-6 rounded-full transition-colors relative flex-shrink-0",
                    form.auto_generate === "true" ? "bg-primary" : "bg-muted"
                  )}
                >
                  <span className={cn(
                    "absolute top-1 w-4 h-4 bg-white rounded-full shadow transition-all",
                    form.auto_generate === "true" ? "right-1" : "left-1"
                  )} />
                </button>
              </div>
            </div>
          )}

          {/* Aba Transcrição */}
          {activeTab === "transcricao" && (
            <div className="px-5 py-4 space-y-4">
              <div>
                <label className="text-xs text-muted-foreground">Idioma do áudio</label>
                <select
                  value={form.whisper_language ?? "pt"}
                  onChange={e => set("whisper_language", e.target.value)}
                  className="w-full mt-1 text-sm rounded-xl px-3 py-2 focus:outline-none bg-[#111111] border border-border text-foreground"
                >
                  {WHISPER_LANGUAGES.map(l => (
                    <option key={l.value} value={l.value}>{l.label}</option>
                  ))}
                </select>
                <p className="text-[10px] text-muted-foreground/60 mt-1">Idioma principal das reuniões gravadas</p>
              </div>

              <div>
                <label className="text-xs text-muted-foreground">Modelo Whisper</label>
                <select
                  value={form.whisper_model ?? "medium"}
                  onChange={e => set("whisper_model", e.target.value)}
                  className="w-full mt-1 text-sm rounded-xl px-3 py-2 focus:outline-none bg-[#111111] border border-border text-foreground"
                >
                  {WHISPER_MODELS.map(m => (
                    <option key={m.value} value={m.value}>{m.label}</option>
                  ))}
                </select>
                <p className="text-[10px] text-muted-foreground/60 mt-1">Modelos maiores transcrevem melhor, mas demoram mais. Requer reinício do serviço de áudio para ter efeito.</p>
              </div>
            </div>
          )}

          {/* Aba Atalhos */}
          {activeTab === "atalhos" && (
            <div className="px-5 py-4 space-y-3">
              <div>
                <label className="text-xs text-muted-foreground">Atalho de gravação rápida</label>
                <HotkeyCapture
                  value={form.recording_hotkey ?? "ctrl+shift+r"}
                  onChange={handleHotkeyChange}
                />
                <p className="text-[10px] text-muted-foreground/60 mt-1">
                  Padrão: Ctrl+Shift+R — funciona com o app em segundo plano
                </p>
              </div>
            </div>
          )}
        </div>

        {/* footer */}
        <div className="px-5 py-4 flex items-center justify-between border-t border-border">
          {error ? <p className="text-xs text-destructive">{error}</p> : <span />}
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={onClose}>Cancelar</Button>
            <Button size="sm" onClick={handleSave} disabled={update.isPending}>
              {update.isPending ? "Salvando..." : "Salvar"}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )

  return createPortal(content, document.body)
}
