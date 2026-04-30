# UI Search Button, Record Hotkey Badge, and Configurable Hotkey — Design Spec

**Data:** 2026-04-30
**Status:** Aprovado

---

## Contexto

Três melhorias de UX relacionadas à descoberta e controle de atalhos:

1. **Botão de busca centralizado na toolbar** — pill visível com hint `Ctrl K`.
2. **Badge de atalho no botão Gravar** — exibe o atalho configurado inline.
3. **Atalho de gravação configurável** — seção "Atalhos" nas configurações com captura por clique; efeito imediato sem reiniciar.

---

## Decisões de design

- **Sem nova rota de API.** O atalho é salvo como `recording_hotkey` em `settings` via `PUT /api/settings` já existente.
- **Re-registro imediato.** `SettingsHandler` chama um callback `onUpdate` após salvar; `app.go` wires esse callback para `tray.ApplySettings()`.
- **Captura por clique no frontend.** O campo de atalho entra em modo escuta (`keydown`) ao clicar, captura a combinação e sai do modo escuta ao soltar a tecla não-modificadora.
- **Default hardcoded:** `ctrl+shift+r` — usado quando `recording_hotkey` não existe na tabela `settings`.
- **Sem migration.** `settings` já é um key-value store genérico.
- **`cmd/api/main.go` não muda.** `SetOnUpdate` é wired apenas no entry point desktop; na API standalone o callback fica nil (sem-op).

---

## Arquitetura

```
SettingsModal (keydown capture)
  └─ PUT /api/settings { recording_hotkey: "ctrl+shift+r" }
       └─ SettingsHandler.onUpdate(map[string]string)
            └─ TrayManager.ApplySettings(settings)
                 ├─ parseHotkey("ctrl+shift+r") → mods=0x0006, vk=0x52
                 ├─ UnregisterHotKey(hwnd, hotkeyID)
                 └─ RegisterHotKey(hwnd, hotkeyID, mods, vk)

App.tsx → Toolbar (onSearch, recordingHotkey)
  ├─ pill click / Ctrl+K → SearchModal
  └─ Gravar button → badge com atalho formatado
```

---

## Seção 1 — Toolbar

### Layout

```
┌─ ☰  Meeting Notes  [Reuniões][Board] ──[🔍 Pesquisar reuniões...  Ctrl K]──  [🎙 Gravar  Ctrl+Shift+R]  ⚙ ─┐
```

O `flex-1` atual (espaçador vazio entre título+nav e ações) é substituído pelo pill de busca, que usa todo o espaço disponível. A navegação Reuniões/Board continua à esquerda do pill.

### `Toolbar.tsx` — mudanças

**Nova prop:** `onSearch: () => void`

**Pill de busca** (substitui o `<div className="flex gap-1 flex-1">`):
```tsx
<button
  onClick={onSearch}
  className="flex-1 mx-4 flex items-center gap-2 px-3 py-1.5 rounded-full
             bg-muted/50 border border-border text-muted-foreground text-sm
             hover:bg-muted transition-colors max-w-xl"
>
  <Search size={14} />
  <span className="flex-1 text-left">Pesquisar reuniões...</span>
  <kbd className="text-[10px] bg-background border border-border rounded px-1.5 py-0.5 font-mono">
    Ctrl K
  </kbd>
</button>
```

**Botão Gravar** — adiciona badge com atalho:
```tsx
<Button size="sm" onClick={onRecord}>
  <Mic size={14} className="mr-1.5" />
  Gravar
  {recordingHotkey && (
    <span className="ml-1.5 text-[10px] bg-white/20 rounded px-1 py-0.5 font-mono">
      {recordingHotkey}
    </span>
  )}
</Button>
```

**Props adicionadas:**
```ts
interface ToolbarProps {
  onToggleSidebar: () => void
  onRecord: () => void
  onSettings: () => void
  onSearch: () => void           // novo
  recordingHotkey?: string       // novo — ex: "Ctrl+Shift+R"
  activeView: "meetings" | "board"
  onChangeView: (view: "meetings" | "board") => void
}
```

### `App.tsx` — mudanças

Adiciona `useSettings()` no `AppInner` (já disponível via React Query cache):
```tsx
const { data: settings } = useSettings()
const recordingHotkey = formatHotkey(settings?.recording_hotkey ?? "ctrl+shift+r")
```

Passa as novas props para Toolbar:
```tsx
<Toolbar
  ...
  onSearch={() => setSearchOpen(true)}
  recordingHotkey={recordingHotkey}
/>
```

### `frontend/src/lib/formatHotkey.ts` — novo utilitário

```ts
export function formatHotkey(raw: string): string {
  return raw
    .split("+")
    .map(p => p.charAt(0).toUpperCase() + p.slice(1))
    .join("+")
}
// "ctrl+shift+r" → "Ctrl+Shift+R"
```

---

## Seção 2 — SettingsModal: captura de atalho

### `useSettings.ts` — mudança

Adiciona `recording_hotkey` ao tipo `Settings`:
```ts
export interface Settings {
  // ...existing fields...
  recording_hotkey: string  // ex: "ctrl+shift+r"
}
```

### `SettingsModal.tsx` — nova seção "Atalhos"

Inserida antes do footer, após a seção Transcrição:

```tsx
{/* Atalhos */}
<div className="px-5 py-4 border-b border-border space-y-3">
  <p className="text-[10px] uppercase tracking-widest text-muted-foreground">Atalhos</p>
  <div>
    <label className="text-xs text-muted-foreground">Atalho de gravação rápida</label>
    <HotkeyCapture
      value={form.recording_hotkey ?? "ctrl+shift+r"}
      onChange={v => set("recording_hotkey", v)}
    />
    <p className="text-[10px] text-muted-foreground/60 mt-1">
      Padrão: Ctrl+Shift+R — funciona com o app em segundo plano
    </p>
  </div>
</div>
```

### Componente `HotkeyCapture` (inline no SettingsModal)

Estado: `listening: boolean`

**Modo normal:** exibe o atalho atual formatado + botão "Restaurar padrão".
**Modo escuta:** exibe "Pressione o atalho..." com borda destacada; listener `keydown` ativo.

Lógica de captura:
```ts
function handleKeyDown(e: KeyboardEvent) {
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
```

Cleanup: `return () => window.removeEventListener("keydown", handleKeyDown)` no `useEffect`.

Teclas aceitas: somente `length === 1` (letras/números) com ao menos um modificador (`ctrlKey || shiftKey || altKey`). Se apenas teclas modificadoras forem pressionadas, aguarda.

---

## Seção 3 — Backend: callback de atualização de settings

### `internal/handlers/settings_handler.go` — mudança

```go
type SettingsHandler struct {
  repo     *repository.SettingsRepository
  onUpdate func(map[string]string)
}

func (h *SettingsHandler) SetOnUpdate(fn func(map[string]string)) {
  h.onUpdate = fn
}
```

No handler `Update` (PUT /api/settings), após `repo.Set` de cada chave:
```go
if h.onUpdate != nil {
  all, _ := h.repo.GetAll(r.Context())
  h.onUpdate(all)
}
```

### `cmd/desktop/app.go` — mudança

Em `OnStartup`, após criar o tray e o settings handler:
```go
settingsHandler.SetOnUpdate(func(s map[string]string) {
  if a.tray != nil {
    a.tray.ApplySettings(s)
  }
})
```

---

## Seção 4 — TrayManager: re-registro de hotkey

### `cmd/desktop/tray.go` — mudanças

**Constante default:**
```go
const defaultHotkey = "ctrl+shift+r"
```

**Campos na struct:**
```go
type TrayManager struct {
  // ...existing...
  hotkeyMods uint32
  hotkeyVK   uint32
}
```

**`parseHotkey(s string) (mods, vk uint32, err error)`:**
```go
func parseHotkey(s string) (mods, vk uint32, err error) {
  parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "+")
  if len(parts) < 2 {
    return 0, 0, fmt.Errorf("hotkey must have modifier + key: %q", s)
  }
  keyPart := parts[len(parts)-1]
  for _, mod := range parts[:len(parts)-1] {
    switch mod {
    case "ctrl":  mods |= 0x0002
    case "shift": mods |= 0x0004
    case "alt":   mods |= 0x0001
    case "win":   mods |= 0x0008
    default:      return 0, 0, fmt.Errorf("unknown modifier: %q", mod)
    }
  }
  if len(keyPart) != 1 || keyPart[0] < 'a' || keyPart[0] > 'z' {
    return 0, 0, fmt.Errorf("key must be a single letter a-z: %q", keyPart)
  }
  vk = uint32(keyPart[0] - 'a' + 0x41)
  return mods, vk, nil
}
```

**`Start()` — usa setting ao invés dos constantes:**
```go
hotkeyStr := "ctrl+shift+r"
if s, err := t.meetingRepo ...; // lê via settingsRepo passado no Start
// → na prática: recebe settingsRepo no construtor
```

> **Nota:** `TrayManager` precisa de acesso ao `SettingsRepository` para ler o atalho no `Start()`. `NewTrayManager` recebe `settingsRepo *repository.SettingsRepository` como parâmetro adicional.

**`ApplySettings(settings map[string]string)`:**
```go
func (t *TrayManager) ApplySettings(settings map[string]string) {
  key := settings["recording_hotkey"]
  if key == "" {
    key = defaultHotkey
  }
  mods, vk, err := parseHotkey(key)
  if err != nil {
    log.Printf("tray: invalid hotkey %q: %v", key, err)
    return
  }
  if mods == t.hotkeyMods && vk == t.hotkeyVK {
    return
  }
  procUnregisterHotKey.Call(t.hwnd, hotkeyID)
  t.hotkeyMods = mods
  t.hotkeyVK = vk
  if ret, _, err := procRegisterHotKey.Call(t.hwnd, hotkeyID, uintptr(mods), uintptr(vk)); ret == 0 {
    log.Printf("tray: RegisterHotKey %q: %v", key, err)
  }
}
```

---

## Arquivos modificados

| Arquivo | Mudança |
|---|---|
| `frontend/src/components/layout/Toolbar.tsx` | Pill busca + badge atalho no Gravar |
| `frontend/src/App.tsx` | `onSearch` + `recordingHotkey` para Toolbar; `useSettings()` |
| `frontend/src/lib/formatHotkey.ts` | **Novo** — formata `"ctrl+shift+r"` → `"Ctrl+Shift+R"` |
| `frontend/src/components/settings/SettingsModal.tsx` | Seção Atalhos + `HotkeyCapture` |
| `frontend/src/hooks/useSettings.ts` | Adiciona `recording_hotkey` ao tipo `Settings` |
| `internal/handlers/settings_handler.go` | `onUpdate` callback + `SetOnUpdate` |
| `cmd/desktop/app.go` | Wire `SetOnUpdate` → `tray.ApplySettings` |
| `cmd/desktop/tray.go` | `parseHotkey`, `ApplySettings`, `hotkeyMods/VK` na struct, `settingsRepo` no construtor |

---

## Fora de escopo

- Suporte a teclas não-letra (F1–F12, numpad) no parser de atalho
- Validação de conflito com outros atalhos do sistema
- Atalho configurável para a busca (Ctrl+K fixo)
- Múltiplos atalhos configuráveis
