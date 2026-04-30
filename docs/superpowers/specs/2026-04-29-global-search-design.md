# Busca Global — Design Spec

**Data:** 2026-04-29
**Grupo:** B (de 3 — ver backlog)
**Status:** Aprovado

---

## Contexto

A lista de reuniões já tem busca por título (`GET /api/meetings?q=`) e filtros por status e data. Esta feature adiciona busca full-text em todos os campos de conteúdo — transcrição, resumo, key_points e tasks — acessível via modal global (`Ctrl+K`).

---

## Modelo de dados — Migration 009

### Virtual table FTS5

```sql
CREATE VIRTUAL TABLE meetings_fts USING fts5(
    meeting_id UNINDEXED,
    title,
    transcript,
    summary,
    key_points,
    tasks,
    tokenize = 'unicode61'
);
```

- `meeting_id` — UUID da reunião, armazenado mas não indexado para busca
- `title` — título da reunião
- `transcript` — transcrição completa
- `summary` — conteúdo do resumo gerado
- `key_points` — todos os key_points concatenados com `\n`
- `tasks` — todas as descriptions de tasks concatenadas com `\n`
- `tokenize = 'unicode61'` — suporte a acentuação e caracteres não-ASCII

### População inicial

```sql
INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
SELECT
    m.id,
    m.title,
    COALESCE(m.transcript, ''),
    COALESCE(s.content, ''),
    COALESCE((
        SELECT GROUP_CONCAT(kp.content, char(10))
        FROM key_points kp WHERE kp.meeting_id = m.id
    ), ''),
    COALESCE((
        SELECT GROUP_CONCAT(t.description, char(10))
        FROM tasks t WHERE t.meeting_id = m.id
    ), '')
FROM meetings m
LEFT JOIN summaries s ON s.meeting_id = m.id;
```

---

## Backend

### Repository (`SearchRepository`)

Arquivo: `internal/repository/search_repository.go`

```go
type SearchResult struct {
    MeetingID string
    Snippet   string
}

func (r *SearchRepository) Search(ctx context.Context, q string) ([]SearchResult, error)

func (r *SearchRepository) UpsertMeeting(ctx context.Context,
    meetingID, title, transcript, summary, keyPoints, tasks string) error

func (r *SearchRepository) DeleteMeeting(ctx context.Context, meetingID string) error
```

**Query de busca:**
```sql
SELECT meeting_id, snippet(meetings_fts, -1, '<b>', '</b>', '...', 15) as snippet
FROM meetings_fts
WHERE meetings_fts MATCH ?
ORDER BY rank
LIMIT 20
```

**Upsert:**
```sql
INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT DO UPDATE SET
    title = excluded.title,
    transcript = excluded.transcript,
    summary = excluded.summary,
    key_points = excluded.key_points,
    tasks = excluded.tasks;
```

Nota: FTS5 não suporta `ON CONFLICT` nativo — o upsert é feito via `DELETE` + `INSERT` em transação.

**Delete:**
```sql
DELETE FROM meetings_fts WHERE meeting_id = ?
```

### Service (`SearchService`)

Arquivo: `internal/services/search_service.go`

```go
type SearchResultItem struct {
    MeetingID    string    `json:"meeting_id"`
    MeetingTitle string    `json:"meeting_title"`
    Snippet      string    `json:"snippet"`
    StartedAt    *time.Time `json:"started_at"`
    Status       string    `json:"status"`
}

func (s *SearchService) Search(ctx context.Context, q string) ([]SearchResultItem, error)
```

- Retorna erro de validação se `q` for vazio ou menor que 2 caracteres
- Chama `searchRepo.Search` e faz JOIN com meetings para obter `meeting_title`, `started_at`, `status`
- Retorna `[]SearchResultItem{}` (slice vazio, não nil) se nenhum resultado

### Sincronização do índice FTS

O `SearchRepository.UpsertMeeting` é chamado pelo service layer em:

1. **`MeetingService`** — após qualquer update que altere `title`, `transcript` ou `notes`  
   (notes não são indexadas por ora, mas o upsert é chamado mesmo assim para consistência)
2. **`OrchestratorService`** — ao final do pipeline de processamento, após salvar summary, key_points e tasks; o `SetNotifyFn` existente é expandido ou um segundo callback é adicionado

O `SearchRepository.DeleteMeeting` é chamado em `MeetingService.Delete`.

O `SearchRepository` é injetado no `MeetingService` e no `OrchestratorService` via construtor.

### Handler (`SearchHandler`)

Arquivo: `internal/handlers/search_handler.go`

```
GET /api/search?q=<termo>
```

- Retorna 400 se `q` ausente ou menor que 2 caracteres
- Retorna `[]SearchResultItem` (200) — array vazio se sem resultados
- Rota registrada em `cmd/api/main.go` e `cmd/desktop/app.go`

---

## Frontend

### Hook `useSearch`

Arquivo: `frontend/src/hooks/useSearch.ts`

```typescript
export interface SearchResultItem {
  meeting_id: string
  meeting_title: string
  snippet: string        // pode conter <b>termo</b>
  started_at: string | null
  status: string
}

export function useSearch(q: string) {
  return useQuery({
    queryKey: ["search", q],
    queryFn: () => api<SearchResultItem[]>(`/api/search?q=${encodeURIComponent(q)}`),
    enabled: q.trim().length >= 2,
    staleTime: 0,
  })
}
```

### Componente `SearchModal`

Arquivo: `frontend/src/components/search/SearchModal.tsx`

- Portal renderizado em `document.body`
- Input com debounce de 200ms
- Lista de resultados: título da reunião + snippet com HTML renderizado (`dangerouslySetInnerHTML` — conteúdo vem exclusivamente do backend controlado)
- Fecha com `Esc` ou clique no backdrop
- Estado de loading e "nenhum resultado" tratados
- Ao clicar num resultado: fecha modal, seleciona a reunião passando o termo `q` como highlight via React state

### Ativação global

- `keydown` listener em `App.tsx` detecta `Ctrl+K` → abre `SearchModal`
- Ícone de lupa (`Search` do lucide-react) adicionado na toolbar do `MeetingList` — também abre o modal
- Estado `searchOpen: boolean` em `App.tsx`

### Highlight no detalhe da reunião

- `MeetingDetail` recebe prop opcional `highlightQuery?: string`
- Função utilitária `highlightText(text: string, query: string): string` — envolve ocorrências em `<mark>` (case-insensitive)
- Aplicada em: título, transcrição, resumo, key_points e tasks
- Se `highlightQuery` ausente, comportamento atual preservado sem mudança

---

## Fora de escopo

- Busca nas `notes` do usuário
- Busca em cards do board
- Histórico de buscas recentes
- Filtros dentro do modal de busca (status, data)
