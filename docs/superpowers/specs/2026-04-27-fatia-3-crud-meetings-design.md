# Spec: Fatia 3 — CRUD Meetings

## Objetivo

Implementar CRUD completo de reuniões (`/api/meetings`) seguindo o mesmo padrão 3 camadas da Fatia 2. Inclui filtros por `theme_id` e `status` no endpoint de listagem, e um response enriquecido no endpoint de detalhe com arrays aninhados vazios (a serem preenchidos na Fatia 4).

---

## Arquitetura

Mesmo padrão: `repository` → `service` → `handler`. Sem novas abstrações.

### File Map

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `internal/repository/meeting_repository.go` | criar | SQL CRUD + filtros opcionais em `List` |
| `internal/repository/meeting_repository_test.go` | criar | Testes TDD com SQLite real |
| `internal/services/meeting_service.go` | criar | Validação, UUID, defaults |
| `internal/services/meeting_service_test.go` | criar | Testes TDD com SQLite real |
| `internal/handlers/meeting_handler.go` | criar | HTTP handlers + DTOs |
| `internal/handlers/meeting_handler_test.go` | criar | Testes TDD com httptest |
| `cmd/api/main.go` | modificar | Registrar rotas `/api/meetings` |

---

## Endpoints

```
GET    /api/meetings           listar reuniões (filtros opcionais: theme_id, status)
POST   /api/meetings           criar reunião → 201
GET    /api/meetings/{id}      detalhe com summary/key_points/tasks vazios → 200
PUT    /api/meetings/{id}      atualizar metadados → 200
DELETE /api/meetings/{id}      deletar → 204
```

---

## Camada: Repository

**Assinaturas:**

```go
func (r *MeetingRepository) List(ctx context.Context, themeID, status string) ([]models.Meeting, error)
func (r *MeetingRepository) Create(ctx context.Context, m *models.Meeting) error
func (r *MeetingRepository) GetByID(ctx context.Context, id string) (*models.Meeting, error)
func (r *MeetingRepository) Update(ctx context.Context, m *models.Meeting) error
func (r *MeetingRepository) Delete(ctx context.Context, id string) error
```

**Filtros em `List`:** strings opcionais. Se não vazias, adicionam cláusula `WHERE` na query. Ambos combinados usam `AND`. Ordem: `started_at DESC`.

**Erros sentinela:** reutiliza `ErrNotFound` já definido no pacote `repository`. `ErrDuplicate` não se aplica (sem UNIQUE constraint em meetings).

**`created_at` e `started_at`:** escaneados como `string`, convertidos com `parseTime` (já implementado no tema repository).

---

## Camada: Service

**Assinaturas:**

```go
func (s *MeetingService) List(ctx context.Context, themeID, status string) ([]models.Meeting, error)
func (s *MeetingService) Create(ctx context.Context, title, themeID, status string, startedAt *time.Time) (*models.Meeting, error)
func (s *MeetingService) GetByID(ctx context.Context, id string) (*models.Meeting, error)
func (s *MeetingService) Update(ctx context.Context, id, title, themeID, status string, startedAt *time.Time, durationSeconds int, transcript string) (*models.Meeting, error)
func (s *MeetingService) Delete(ctx context.Context, id string) error
```

**Validação e defaults:**

- `title` vazio → `*ValidationError` (reutiliza tipo do pacote `services`)
- `status` inválido → `*ValidationError`; valores válidos: `pending`, `recording`, `transcribing`, `processing`, `completed`, `failed`
- `startedAt` nil → default `time.Now().UTC()`
- `status` vazio em Create → default `"pending"`
- `theme_id` não é validado contra a tabela `themes` (FK nullable com SET NULL)
- UUID gerado no service

---

## Camada: Handler

**DTOs de request:**

```go
type createMeetingRequest struct {
    Title     string  `json:"title"`
    ThemeID   string  `json:"theme_id"`
    StartedAt *string `json:"started_at"`
    Status    string  `json:"status"`
}

type updateMeetingRequest struct {
    Title           string  `json:"title"`
    ThemeID         string  `json:"theme_id"`
    StartedAt       *string `json:"started_at"`
    Status          string  `json:"status"`
    DurationSeconds int     `json:"duration_seconds"`
    Transcript      string  `json:"transcript"`
}
```

**DTO de response para `GET /api/meetings/{id}`:**

```go
type meetingDetailResponse struct {
    models.Meeting
    Summary   *models.Summary   `json:"summary"`
    KeyPoints []models.KeyPoint `json:"key_points"`
    Tasks     []models.Task     `json:"tasks"`
}
```

Na Fatia 3: `Summary: nil`, `KeyPoints: []models.KeyPoint{}`, `Tasks: []models.Task{}`.

**`GET /api/meetings` (list):** retorna `[]models.Meeting` simples — sem arrays aninhados. Lê query params `theme_id` e `status` via `r.URL.Query().Get(...)`.

**Mapeamento de erros → HTTP:**

| Erro | Status |
|---|---|
| `*ValidationError` (title vazio, status inválido) | 422 |
| `repository.ErrNotFound` | 404 |
| JSON inválido no body | 400 |
| Erro de DB | 500 |

**`started_at` no request:** recebido como `*string` em ISO 8601, parseado com `time.Parse(time.RFC3339, ...)` no handler antes de passar ao service. Se parse falhar → 400. Em `PUT`, se `started_at` for omitido (nil), o service preserva o valor existente — mesmo comportamento do `color` no theme service.

---

## HTTP Status Codes

| Operação | Sucesso | Erros possíveis |
|---|---|---|
| GET /api/meetings | 200 | 500 |
| POST /api/meetings | 201 | 400, 422, 500 |
| GET /api/meetings/{id} | 200 | 404, 500 |
| PUT /api/meetings/{id} | 200 | 400, 404, 422, 500 |
| DELETE /api/meetings/{id} | 204 | 404, 500 |

---

## Testes

Mesmo padrão da Fatia 2: SQLite real via `database.Open(t.TempDir() + "/test.db")`, sem mocks.

**Repository:** Create, GetByID, List (vazio, com dados, com filtros), Update, Update_NotFound, Delete, Delete_NotFound, GetByID_NotFound.

**Service:** Create, Create_TitleRequired, Create_InvalidStatus, Create_DefaultStatus, Create_DefaultStartedAt, GetByID, GetByID_NotFound, Update, Update_TitleRequired, Delete, List, List_FilterByTheme, List_FilterByStatus.

**Handler:** List_Empty, Create, Create_TitleRequired, Create_InvalidStatus, GetByID, GetByID_NotFound, Update, Update_NotFound, Delete, Delete_NotFound.

---

## Decisões de design

- `theme_id` não é validado contra a tabela `themes`: a FK é nullable com `ON DELETE SET NULL`, então um ID inexistente não quebra a constraint (SQLite não enforça FKs por padrão sem `PRAGMA foreign_keys = ON`). Validar aumentaria complexidade sem benefício para uso local.
- `parseTime` já implementado em `theme_repository.go` — o meeting repository reimplementa localmente (sem extrair para shared utility) para manter os pacotes independentes. Extração pode ser feita depois se um terceiro repository surgir.
- O DTO `meetingDetailResponse` fica no pacote `handlers` (unexported), não em `models`, porque é uma preocupação de apresentação HTTP.
