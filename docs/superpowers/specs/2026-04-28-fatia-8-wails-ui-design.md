# Spec: Fatia 8 — UI Wails (app desktop React)

## Objetivo

Construir um app desktop com Wails v2 que consome a API Go via HTTP interno. O frontend é React + Vite + Shadcn/ui + Tailwind. O entrypoint `cmd/desktop/` sobe o servidor HTTP em uma porta livre e expõe `GetPort()` como único binding Wails. O React usa React Query para data fetching e Wails runtime events para atualizações de status do pipeline em tempo real.

---

## Arquitetura

### Entrypoint desktop

`cmd/desktop/main.go` inicializa o Wails com a `App` struct. `cmd/api/main.go` continua intacto para uso standalone.

`App.OnStartup`:
1. Abre o banco SQLite
2. Inicializa repositórios, services, handlers (mesmo código de `cmd/api`)
3. Sobe `http.ListenAndServe` em porta livre (`net.Listen("tcp", ":0")`) numa goroutine
4. Guarda a porta em `app.port`
5. Configura `orchestrator.notifyFn` para emitir `runtime.EventsEmit(ctx, "pipeline:status", payload)`

`App.OnShutdown`: fecha o DB.

`App.GetPort() int`: único binding exposto ao React.

### Pipeline status via Wails events

`Orchestrator` ganha campo `notifyFn func(meetingID, status string)` (nil-safe). Chamado a cada transição de status dentro de `RunCapturePipeline` e `RunAIPipeline`.

Em `cmd/desktop/app.go`, o callback emite:
```json
{ "meeting_id": "...", "status": "transcribing" }
```
via `runtime.EventsEmit(ctx, "pipeline:status", payload)`.

React escuta com `window.runtime.EventsOn("pipeline:status", handler)` e invalida o cache React Query daquela meeting.

Em `cmd/api/main.go`, `notifyFn` permanece nil — sem mudança de comportamento.

---

## File map

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `cmd/desktop/main.go` | criar | Entrypoint Wails |
| `cmd/desktop/app.go` | criar | App struct: lifecycle, GetPort binding, notifyFn setup |
| `cmd/desktop/app_test.go` | criar | Testes do App struct |
| `internal/services/orchestrator.go` | modificar | Adicionar campo `notifyFn`, chamar em cada transição de status |
| `internal/services/orchestrator_test.go` | modificar | Teste do notifyFn |
| `wails.json` | criar | Config Wails (nome, versão, frontend dir) |
| `frontend/` | criar | Projeto React via `wails generate` |
| `frontend/src/App.tsx` | criar | Layout raiz, boot GetPort, EventsOn |
| `frontend/src/components/layout/Sidebar.tsx` | criar | Lista de themes com badge |
| `frontend/src/components/layout/MeetingList.tsx` | criar | Lista de meetings filtrada |
| `frontend/src/components/layout/MeetingDetail.tsx` | criar | Painel direito com abas |
| `frontend/src/components/layout/Toolbar.tsx` | criar | Botão gravar + pipeline status badge |
| `frontend/src/components/recording/RecordingModal.tsx` | criar | Modal criar+iniciar gravação |
| `frontend/src/hooks/useApi.ts` | criar | Fetch wrapper com base URL dinâmica |
| `frontend/src/hooks/useThemes.ts` | criar | React Query themes |
| `frontend/src/hooks/useMeetings.ts` | criar | React Query meetings |
| `frontend/src/hooks/useMeeting.ts` | criar | React Query meeting detalhe |
| `frontend/src/hooks/usePipeline.ts` | criar | Wails EventsOn pipeline:status |

---

## Layout

```
┌─────────────────────────────────────────────────────────────┐
│  ⏺ Gravar Nova Reunião          [status badge pipeline]     │  ← Toolbar
├──────────┬──────────────────┬───────────────────────────────┤
│ Themes   │ Meetings         │ Meeting Detail                │
│          │                  │                               │
│ • Eng    │ • Daily 28/04 ✓  │  Daily 28/04                  │
│ • Produto│ • Sprint Review  │  Status: completed            │
│ • Geral  │ • 1:1 João       │                               │
│          │                  │  [Transcrição][Resumo][Pontos] │
│ + Novo   │ + Nova Meeting   │  [Tarefas]                    │
│          │                  │                               │
│          │                  │  ▶ Start  ■ Stop              │
└──────────┴──────────────────┴───────────────────────────────┘
```

---

## Componentes

### Toolbar
- Botão "⏺ Gravar Nova Reunião" — abre `RecordingModal`
- `PipelineStatusBadge` — mostra status da meeting em andamento (atualizado via evento Wails); invisível quando nenhuma meeting está em `recording`/`transcribing`/`processing`

### Sidebar
- Lista de themes com badge de contagem de meetings
- Clique num theme filtra `MeetingList`
- "Todos" selecionado por padrão (sem filtro)
- Botão "+" abre inline form para criar theme

### MeetingList
- Lista de meetings do theme selecionado
- Cada item: título, data formatada, badge de status colorido
- Cores dos badges: `recording`=vermelho, `transcribing`/`processing`=amarelo, `completed`=verde, `failed`=vermelho escuro, `pending`=cinza
- Botão "+ Nova Meeting" abre inline form (título + theme)
- Clique seleciona e abre detalhe

### MeetingDetail
- Header: título, data, status badge, botões de controle de gravação
  - Botão **▶ Start**: visível se `status ∈ {pending, failed}`
  - Botão **■ Stop**: visível se `status = recording`
  - Botão **⟳ Reprocessar**: visível se `status ∈ {failed, completed}` e transcript não vazio
- 4 abas:
  - **Transcrição**: textarea editável; salva via `PUT /meetings/{id}` com debounce de 1s; se vazio mostra placeholder "Nenhuma transcrição ainda"
  - **Resumo**: texto do summary; botão "Gerar" chama `POST /summary/generate`
  - **Pontos-chave**: lista ordenada; cada item editável inline; botão "Gerar"
  - **Tarefas**: lista com checkbox de concluído, assignee, due_date; botão "Gerar"
- Se nenhuma meeting selecionada: empty state "Selecione uma reunião"

### RecordingModal
- Campos: título (obrigatório), theme (select dos themes existentes)
- Ao confirmar: `POST /meetings` → `POST /meetings/{id}/start`
- Fecha e seleciona a meeting criada automaticamente
- Erro 503 (audio-service down): toast "Serviço de áudio indisponível"
- Erro 409 (conflito): toast "Já existe uma gravação em andamento"

---

## Estado global

```typescript
// App.tsx
const [port, setPort] = useState<number>(0)           // boot via GetPort()
const [selectedThemeId, setSelectedThemeId] = useState<string | null>(null)
const [selectedMeetingId, setSelectedMeetingId] = useState<string | null>(null)
```

React Query keys:
- `["themes"]`
- `["meetings", themeId]`
- `["meeting", id]`

`usePipeline` hook:
```typescript
// Escuta eventos Wails e invalida cache da meeting afetada
window.runtime.EventsOn("pipeline:status", ({ meeting_id }) => {
  queryClient.invalidateQueries({ queryKey: ["meeting", meeting_id] })
  queryClient.invalidateQueries({ queryKey: ["meetings"] })
})
```

---

## `useApi.ts`

```typescript
let baseURL = ""

export function initApi(port: number) {
  baseURL = `http://localhost:${port}`
}

export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${baseURL}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  })
  if (!res.ok) throw new Error(`${res.status}`)
  return res.json()
}
```

---

## `cmd/desktop/app.go`

```go
type App struct {
    ctx    context.Context
    db     *sql.DB
    port   int
    server *http.Server
}

func NewApp() *App { return &App{} }

func (a *App) OnStartup(ctx context.Context) {
    a.ctx = ctx
    // abre DB, cria repos/services/handlers igual ao cmd/api
    // net.Listen(":0") para porta livre
    // configura orchestrator.NotifyFn
    // go a.server.Serve(listener)
}

func (a *App) OnShutdown(ctx context.Context) {
    a.server.Shutdown(ctx)
    a.db.Close()
}

func (a *App) GetPort() int { return a.port }
```

---

## `internal/services/orchestrator.go` — mudança

```go
type Orchestrator struct {
    // ... campos existentes ...
    notifyFn func(meetingID, status string)  // nil-safe
}

func (o *Orchestrator) SetNotifyFn(fn func(meetingID, status string)) {
    o.notifyFn = fn
}

func (o *Orchestrator) notify(meetingID, status string) {
    if o.notifyFn != nil {
        o.notifyFn(meetingID, status)
    }
}
```

Chamado após cada `updateStatus` bem-sucedido dentro de `RunCapturePipeline` e `RunAIPipeline`.

---

## Testes

### `cmd/desktop/app_test.go`

- `TestApp_GetPort` — após `OnStartup`, `GetPort()` retorna porta > 0
- `TestApp_HTTPServerResponds` — `GET http://localhost:{port}/health` retorna 200
- `TestApp_Shutdown` — `OnShutdown` fecha sem erro

### `internal/services/orchestrator_test.go` (adição)

- `TestOrchestrator_NotifyFn_CalledOnStatusChange` — passa `notifyFn` fake, roda `RunCapturePipeline`, verifica que foi chamado com os status esperados na ordem correta: `transcribing` → `processing` → `completed`

---

## Dependências Go adicionais

```
github.com/wailsapp/wails/v2
```

## Dependências frontend (além do scaffold Wails+React)

```
@tanstack/react-query
@radix-ui/react-* (via shadcn)
tailwindcss
class-variance-authority
lucide-react
```

---

## Decisões de design

- **Porta aleatória:** evita conflito com `cmd/api` rodando em paralelo durante desenvolvimento. React descobre via `GetPort()` no boot.
- **`notifyFn` nil-safe:** `cmd/api` continua sem eventos; apenas `cmd/desktop` configura o callback.
- **React Query como cache:** invalidação automática ao receber evento Wails elimina polling e gerenciamento manual de estado.
- **Debounce na edição de transcrição:** evita `PUT` a cada keystroke — 1s após parar de digitar.
- **Wails v2:** estável. Wails v3 ainda em alpha.
- **Sem testes de componentes React nesta fatia:** smoke test manual cobre o fluxo. Testes de componente ficam para Fatia 9 se necessário.
- **`cmd/api` intacto:** uso standalone via Postman continua funcionando sem mudança.
