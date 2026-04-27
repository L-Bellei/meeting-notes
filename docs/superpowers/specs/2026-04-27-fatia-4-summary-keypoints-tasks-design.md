# Spec: Fatia 4 — Summary, Key Points & Tasks

## Objetivo

Implementar CRUD completo para os três sub-recursos de reuniões (`summary`, `key_points`, `tasks`) com geração via Anthropic API. Atualizar `GET /api/meetings/{id}` para retornar dados reais em vez de arrays vazios.

---

## Arquitetura

Mesmo padrão 3 camadas: `repository` → `service` → `handler`. Novo pacote `internal/ai/` com interface `AIClient` e implementação real `AnthropicClient`. Os services de geração recebem `AIClient` via injeção de dependência — testes usam um `fakeAIClient`.

### File Map

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `internal/ai/anthropic_client.go` | criar | Interface `AIClient` + implementação Anthropic |
| `internal/config/config.go` | modificar | Adicionar `AnthropicAPIKey` |
| `internal/repository/summary_repository.go` | criar | CRUD para summaries |
| `internal/repository/summary_repository_test.go` | criar | Testes TDD com SQLite real |
| `internal/repository/key_point_repository.go` | criar | CRUD para key_points |
| `internal/repository/key_point_repository_test.go` | criar | Testes TDD com SQLite real |
| `internal/repository/task_repository.go` | criar | CRUD para tasks |
| `internal/repository/task_repository_test.go` | criar | Testes TDD com SQLite real |
| `internal/services/summary_service.go` | criar | CRUD + generate, injeta AIClient |
| `internal/services/summary_service_test.go` | criar | Testes com fakeAIClient |
| `internal/services/key_point_service.go` | criar | CRUD + generate |
| `internal/services/key_point_service_test.go` | criar | Testes com fakeAIClient |
| `internal/services/task_service.go` | criar | CRUD + generate |
| `internal/services/task_service_test.go` | criar | Testes com fakeAIClient |
| `internal/handlers/summary_handler.go` | criar | HTTP handlers para summary |
| `internal/handlers/summary_handler_test.go` | criar | Testes com httptest |
| `internal/handlers/key_point_handler.go` | criar | HTTP handlers para key_points |
| `internal/handlers/key_point_handler_test.go` | criar | Testes com httptest |
| `internal/handlers/task_handler.go` | criar | HTTP handlers para tasks |
| `internal/handlers/task_handler_test.go` | criar | Testes com httptest |
| `internal/handlers/meeting_handler.go` | modificar | `GetByID` retorna dados reais |
| `cmd/api/main.go` | modificar | Registrar novas rotas |

---

## Endpoints

```
POST   /api/meetings/{id}/summary           criar summary → 201
GET    /api/meetings/{id}/summary           obter summary → 200 / 404
PUT    /api/meetings/{id}/summary           atualizar → 200
DELETE /api/meetings/{id}/summary           deletar → 204
POST   /api/meetings/{id}/summary/generate  gerar via AI → 201

GET    /api/meetings/{id}/key_points                listar → 200
POST   /api/meetings/{id}/key_points                criar → 201
PUT    /api/meetings/{id}/key_points/{kpId}         atualizar → 200
DELETE /api/meetings/{id}/key_points/{kpId}         deletar → 204
POST   /api/meetings/{id}/key_points/generate       gerar via AI → 201

GET    /api/meetings/{id}/tasks                     listar → 200
POST   /api/meetings/{id}/tasks                     criar → 201
PUT    /api/meetings/{id}/tasks/{taskId}            atualizar (incl. completed) → 200
DELETE /api/meetings/{id}/tasks/{taskId}            deletar → 204
POST   /api/meetings/{id}/tasks/generate            gerar via AI → 201
```

---

## Pacote AI

```go
// internal/ai/anthropic_client.go

type TaskSuggestion struct {
    Description string
    Assignee    string // vazio se não identificado
    Priority    string // "low", "medium", "high"
}

type AIClient interface {
    GenerateSummary(ctx context.Context, transcript string) (content string, inputTokens, outputTokens int, err error)
    GenerateKeyPoints(ctx context.Context, transcript string) (points []string, inputTokens, outputTokens int, err error)
    GenerateTasks(ctx context.Context, transcript string) (tasks []TaskSuggestion, inputTokens, outputTokens int, err error)
}

type AnthropicClient struct {
    apiKey string
    model  string // "claude-haiku-4-5-20251001"
}

func NewAnthropicClient(apiKey string) *AnthropicClient
```

Cada método envia o transcript via API Anthropic e pede retorno em JSON. O client faz parse do JSON antes de retornar ao service.

**Config:** `ANTHROPIC_API_KEY` adicionada ao `config.Load()`. Se ausente, o server sobe normalmente — os endpoints `/generate` retornam 503.

---

## Camada: Repository

### SummaryRepository

```go
func (r *SummaryRepository) GetByMeetingID(ctx context.Context, meetingID string) (*models.Summary, error)
func (r *SummaryRepository) Upsert(ctx context.Context, s *models.Summary) error  // INSERT OR REPLACE
func (r *SummaryRepository) Delete(ctx context.Context, meetingID string) error
```

`Upsert` usa `INSERT OR REPLACE INTO summaries` — simplifica tanto o create manual quanto o generate (ambos substituem). `GetByMeetingID` retorna `ErrNotFound` se não existir.

### KeyPointRepository

```go
func (r *KeyPointRepository) ListByMeetingID(ctx context.Context, meetingID string) ([]models.KeyPoint, error)
func (r *KeyPointRepository) Create(ctx context.Context, kp *models.KeyPoint) error
func (r *KeyPointRepository) GetByID(ctx context.Context, id string) (*models.KeyPoint, error)
func (r *KeyPointRepository) Update(ctx context.Context, kp *models.KeyPoint) error
func (r *KeyPointRepository) Delete(ctx context.Context, id string) error
func (r *KeyPointRepository) DeleteByMeetingID(ctx context.Context, meetingID string) error
```

`ListByMeetingID` ordena por `position ASC`. `DeleteByMeetingID` usado pelo generate para substituir todos.

### TaskRepository

```go
func (r *TaskRepository) ListByMeetingID(ctx context.Context, meetingID string) ([]models.Task, error)
func (r *TaskRepository) Create(ctx context.Context, t *models.Task) error
func (r *TaskRepository) GetByID(ctx context.Context, id string) (*models.Task, error)
func (r *TaskRepository) Update(ctx context.Context, t *models.Task) error
func (r *TaskRepository) Delete(ctx context.Context, id string) error
func (r *TaskRepository) DeleteByMeetingID(ctx context.Context, meetingID string) error
```

`ListByMeetingID` ordena por `created_at ASC`. `completed` armazenado como `INTEGER` (0/1) no SQLite, convertido para `bool` no scan.

**Erros sentinela:** reutiliza `ErrNotFound` do pacote `repository`.

---

## Camada: Service

### SummaryService

```go
func NewSummaryService(repo *SummaryRepository, ai AIClient) *SummaryService
func (s *SummaryService) Get(ctx context.Context, meetingID string) (*models.Summary, error)
func (s *SummaryService) Upsert(ctx context.Context, meetingID, content, modelUsed string) (*models.Summary, error)
func (s *SummaryService) Delete(ctx context.Context, meetingID string) error
func (s *SummaryService) Generate(ctx context.Context, meeting *models.Meeting) (*models.Summary, error)
```

`Upsert` valida `content` não vazio → `*ValidationError`. UUID gerado no service. `Generate` valida `meeting.Transcript != nil && *meeting.Transcript != ""` → `*ValidationError`; chama `ai.GenerateSummary`; faz Upsert do resultado. `ai` pode ser `nil` se `ANTHROPIC_API_KEY` ausente — nesse caso `Generate` retorna erro sentinel `ErrAINotConfigured`.

### KeyPointService

```go
func NewKeyPointService(repo *KeyPointRepository, ai AIClient) *KeyPointService
func (s *KeyPointService) List(ctx context.Context, meetingID string) ([]models.KeyPoint, error)
func (s *KeyPointService) Create(ctx context.Context, meetingID, content string, position int) (*models.KeyPoint, error)
func (s *KeyPointService) Update(ctx context.Context, id, content string, position int) (*models.KeyPoint, error)
func (s *KeyPointService) Delete(ctx context.Context, id string) error
func (s *KeyPointService) Generate(ctx context.Context, meeting *models.Meeting) ([]models.KeyPoint, error)
```

`Create/Update` valida `content` não vazio → `*ValidationError`. `Generate` deleta todos os key_points existentes da meeting e cria os novos retornados pela AI. `position` atribuído sequencialmente (0, 1, 2, ...).

### TaskService

```go
func NewTaskService(repo *TaskRepository, ai AIClient) *TaskService
func (s *TaskService) List(ctx context.Context, meetingID string) ([]models.Task, error)
func (s *TaskService) Create(ctx context.Context, meetingID, description string, assignee *string, dueDate *time.Time, priority string) (*models.Task, error)
func (s *TaskService) Update(ctx context.Context, id, description string, assignee *string, dueDate *time.Time, priority string, completed bool) (*models.Task, error)
func (s *TaskService) Delete(ctx context.Context, id string) error
func (s *TaskService) Generate(ctx context.Context, meeting *models.Meeting) ([]models.Task, error)
```

`Create/Update` valida `description` não vazio → `*ValidationError`. `priority` válido: `"low"`, `"medium"`, `"high"`; default `"medium"` em Create se vazio. `Generate` deleta todas as tasks existentes e cria as novas. `completed` default `false` nas tasks geradas por AI.

**`ErrAINotConfigured`:** novo sentinel em `internal/services/errors.go` (ou em cada service file). Mapeado para 503 no handler.

---

## Camada: Handler

### DTOs de request

```go
// summary
type createSummaryRequest struct {
    Content   string `json:"content"`
    ModelUsed string `json:"model_used"`
}

// key_point
type createKeyPointRequest struct {
    Position int    `json:"position"`
    Content  string `json:"content"`
}
type updateKeyPointRequest struct {
    Position int    `json:"position"`
    Content  string `json:"content"`
}

// task
type createTaskRequest struct {
    Description string  `json:"description"`
    Assignee    *string `json:"assignee"`
    DueDate     *string `json:"due_date"` // RFC3339, parseado no handler → 400 se inválido
    Priority    string  `json:"priority"`
}
type updateTaskRequest struct {
    Description string  `json:"description"`
    Assignee    *string `json:"assignee"`
    DueDate     *string `json:"due_date"`
    Priority    string  `json:"priority"`
    Completed   bool    `json:"completed"`
}
```

### Mapeamento de erros → HTTP

| Erro | Status |
|---|---|
| `*ValidationError` | 422 |
| `repository.ErrNotFound` | 404 |
| JSON inválido no body | 400 |
| `due_date` inválido (não RFC3339) | 400 |
| `ErrAINotConfigured` | 503 |
| Erro Anthropic API | 502 |
| Erro de DB | 500 |

### `/generate` handlers

Cada handler de generate:
1. Lê o `{id}` da meeting via chi
2. Busca a meeting completa (para acessar o transcript) — 404 se não existe
3. Chama o service `Generate(ctx, meeting)`
4. Retorna 201 com o(s) recurso(s) criado(s)

Para isso, os handlers de summary, key_point e task precisam de acesso ao `MeetingService` (ou ao `MeetingRepository` diretamente). Solução: injetar o `MeetingRepository` nos services de generate, ou passar o `MeetingService` para os handlers. **Opção adotada:** os handlers de generate recebem também o `*services.MeetingService` para buscar a meeting — sem nova abstração.

### `GET /api/meetings/{id}` atualizado

`MeetingHandler.GetByID` passa a chamar também `SummaryRepository.GetByMeetingID`, `KeyPointRepository.ListByMeetingID` e `TaskRepository.ListByMeetingID` para popular o `MeetingDetailResponse`. Se summary não existir → `nil`. Arrays sempre retornam `[]` nunca `null`. Para evitar acoplamento excessivo, o `MeetingHandler` recebe os três repositories via construtor.

---

## HTTP Status Codes

| Operação | Sucesso | Erros possíveis |
|---|---|---|
| GET summary | 200 | 404, 500 |
| POST/PUT summary | 201/200 | 400, 422, 500 |
| DELETE summary | 204 | 404, 500 |
| POST summary/generate | 201 | 422, 502, 503, 500 |
| GET key_points | 200 | 500 |
| POST key_point | 201 | 400, 422, 500 |
| PUT key_point | 200 | 400, 404, 422, 500 |
| DELETE key_point | 204 | 404, 500 |
| POST key_points/generate | 201 | 422, 502, 503, 500 |
| GET tasks | 200 | 500 |
| POST task | 201 | 400, 422, 500 |
| PUT task | 200 | 400, 404, 422, 500 |
| DELETE task | 204 | 404, 500 |
| POST tasks/generate | 201 | 422, 502, 503, 500 |

---

## Testes

Mesmo padrão das fatias anteriores: SQLite real via `database.Open(t.TempDir() + "/test.db")`, sem mocks de DB.

**fakeAIClient:** struct que implementa `AIClient`, retorna dados fixos. Definido nos arquivos de teste dos services e handlers — não é exportado.

**Repository tests:** CRUD + ListByMeetingID + DeleteByMeetingID. Todos pré-seedam uma meeting (e theme quando necessário) para satisfazer FK constraints.

**Service tests:** CRUD + validações + Generate com fakeAIClient + Generate sem transcript (→ ValidationError) + Generate com AI nil (→ ErrAINotConfigured).

**Handler tests:** happy paths + erros principais (404, 422, 400) + generate com fakeAIClient injetado.

---

## Decisões de design

- `SummaryRepository.Upsert` usa `INSERT OR REPLACE` — elimina a necessidade de lógica create-vs-update no service, já que summary é único por meeting.
- `Generate` substitui (não acumula) — evita duplicatas e é o comportamento mais previsível ao re-gerar.
- `ErrAINotConfigured` permite subir o servidor sem a API key configurada, degradando gracefully apenas os endpoints de geração.
- Os handlers de generate recebem `*services.MeetingService` para buscar a meeting — preferível a duplicar a lógica de lookup em cada service.
- `due_date` parseado no handler (RFC3339), igual ao `started_at` da Fatia 3.
- `completed` em `tasks` armazenado como `INTEGER` no SQLite (0/1) e convertido para `bool` no scan — padrão SQLite para booleanos.
