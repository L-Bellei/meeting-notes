# Hotkey Global + System Tray — Design Spec

**Data:** 2026-04-30
**Grupo:** C (de 3 — ver backlog)
**Status:** Aprovado

---

## Contexto

O app roda como janela Wails v2 no Windows. Esta feature adiciona dois comportamentos de integração com o SO:

1. **Hotkey global (`Ctrl+Shift+R`)** — inicia ou para a gravação a partir de qualquer janela do sistema, sem precisar focar o app.
2. **System tray** — ao fechar a janela, o app permanece ativo na bandeja do sistema com um menu de controle.

---

## Decisões de design

- **Sem CGO.** Implementado via `golang.org/x/sys/windows` (Win32 API pura), consistente com `modernc/sqlite`. Sem `RegisterHotKey` de bibliotecas externas com CGO.
- **Uma janela Win32 oculta compartilhada** — `RegisterHotKey` e `Shell_NotifyIcon` precisam de uma HWND; a mesma janela serve para ambos via um único `GetMessage` loop.
- **Sem configuração de hotkey** — `Ctrl+Shift+R` fixo (YAGNI).
- **Sem ícone duplo** — um único `.ico` embeddado; estado de gravação indicado via tooltip.
- **Nenhuma migration** — estado de gravação consultado via `meetingRepo`.

---

## Arquitetura

```
App.OnStartup
  └─ TrayManager.Start(ctx, orchestrator, meetingRepo, meetingSvc)
       ├─ RegisterClassEx (registra WndProc)
       ├─ CreateWindowEx (janela oculta HWND)
       ├─ RegisterHotKey(hwnd, 1, MOD_CONTROL|MOD_SHIFT, 'R')
       ├─ Shell_NotifyIcon NIM_ADD (adiciona ícone na tray)
       └─ goroutine: GetMessage loop
            ├─ WM_HOTKEY  → toggleRecording()
            └─ WM_TRAYICON
                 ├─ WM_LBUTTONUP → Show window
                 └─ WM_RBUTTONUP → showContextMenu()

App.OnBeforeClose(ctx) → Hide(ctx); return true

TrayManager "Sair" → allowQuit = true → wailsruntime.Quit(ctx)
```

---

## Comportamento do hotkey

**Ação:** `Ctrl+Shift+R` (global, funciona com o app em segundo plano).

**Lógica `toggleRecording()`:**

```
meeting ← meetingRepo.GetRecording()   // SELECT id FROM meetings WHERE status='recording' LIMIT 1
if meeting exists:
    orchestrator.StopRecording(ctx, meeting.ID)
    tray.UpdateState(isRecording=false)
else:
    title ← "Reunião - " + time.Now().Format("02/01/2006 15:04")
    m ← meetingSvc.Create(ctx, {Title: title, ThemeID: nil})
    orchestrator.StartRecording(ctx, m.ID)
    tray.UpdateState(isRecording=true)
    wailsruntime.EventsEmit(ctx, "hotkey:recording-started", map[string]string{"meetingId": m.ID})
```

**Erros:** silenciosos (fire-and-forget) — hotkey não tem UI para mostrar erros. Falha ao criar reunião ou iniciar gravação é descartada; o tooltip da tray não muda.

---

## System tray

### Fechamento da janela

`OnBeforeClose` em `app.go` retorna `true` (previne fechamento) e chama `wailsruntime.Hide(ctx)`. O app continua rodando.

Exceção: quando o usuário clica "Sair" no menu da tray, o campo `allowQuit bool` na `App` struct é setado como `true` antes de chamar `wailsruntime.Quit(ctx)`. O `OnBeforeClose` checa esse flag e permite o encerramento.

### Menu da tray (clique direito)

```
Abrir Meeting Notes
───────────────────
Iniciar gravação      ← quando idle
Parar gravação        ← quando gravando (item diferente, não toggle)
───────────────────
Sair
```

Clique esquerdo no ícone → `wailsruntime.Show(ctx)` + `wailsruntime.WindowUnminimise(ctx)`.

### Ícone e tooltip

- Ícone: `cmd/desktop/assets/tray.ico` (16×16), embeddado via `//go:embed`.
- Tooltip idle: `"Meeting Notes"`
- Tooltip gravando: `"Meeting Notes — Gravando..."`

O ícone em si não muda entre estados (um único `.ico`). A distinção é feita pelo tooltip e pelo item do menu.

### `UpdateState(isRecording bool)`

Chamado após `toggleRecording()` e após `orchestrator.StopRecording()` para manter tooltip e menu sincronizados.

---

## Arquivos

### Novos

| Arquivo | Responsabilidade |
|---|---|
| `cmd/desktop/tray.go` | `TrayManager` — janela Win32 oculta, hotkey, tray, menu |
| `cmd/desktop/assets/tray.ico` | Ícone 16×16 para a tray (gerado manualmente ou via `convert appicon.png`) |

### Modificados

| Arquivo | Mudança |
|---|---|
| `cmd/desktop/app.go` | Campo `tray *TrayManager`; `allowQuit bool`; inicialização em `OnStartup`; `OnBeforeClose`; `OnShutdown` chama `tray.Stop()` |
| `frontend/src/App.tsx` | `useEffect` com `EventsOn("hotkey:recording-started", ...)` → `onSelectMeeting(meetingId)` |

---

## `TrayManager` — interface pública

```go
type TrayManager struct {
    ctx          context.Context
    orchestrator services.Orchestrator
    meetingRepo  *repository.MeetingRepository
    meetingSvc   *services.MeetingService
    hwnd         windows.HWND
    running      bool
    isRecording  bool
}

func NewTrayManager(
    orchestrator services.Orchestrator,
    meetingRepo  *repository.MeetingRepository,
    meetingSvc   *services.MeetingService,
) *TrayManager

func (t *TrayManager) Start(ctx context.Context) error  // registra hotkey, cria tray, inicia goroutine
func (t *TrayManager) Stop()                            // UnregisterHotKey, Shell_NotifyIcon NIM_DELETE, DestroyWindow
func (t *TrayManager) IsRunning() bool
func (t *TrayManager) UpdateState(isRecording bool)     // atualiza tooltip e item do menu
```

---

## `meetingRepo.GetRecording()`

Novo método no `MeetingRepository`:

```go
// GetRecording retorna a reunião com status 'recording', ou nil se nenhuma.
func (r *MeetingRepository) GetRecording(ctx context.Context) (*models.Meeting, error)
```

SQL: `SELECT ... FROM meetings WHERE status = 'recording' LIMIT 1` — retorna `nil, nil` se não encontrar (sem `ErrNotFound`).

---

## Frontend

`App.tsx` adiciona no `useEffect` de setup:

```typescript
import { EventsOn } from "../wailsjs/runtime/runtime"

useEffect(() => {
  const unlisten = EventsOn("hotkey:recording-started", ({ meetingId }: { meetingId: string }) => {
    setSelectedMeetingId(meetingId)
  })
  return unlisten
}, [])
```

---

## Fora de escopo

- Hotkey configurável pelo usuário
- Segundo ícone para o estado de gravação
- Notificação toast/balloon ao iniciar gravação via hotkey
- Suporte a macOS/Linux (app é Windows-only)
- Desassociar o hotkey enquanto o app está com foco
