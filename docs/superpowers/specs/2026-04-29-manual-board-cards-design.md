# Manual Board Cards — Design Spec

**Data:** 2026-04-29
**Grupo:** A (de 3 — ver backlog)
**Status:** Aprovado

---

## Contexto

O board atual recebe cards de duas formas: auto-add via tema após processamento de reunião, e botão "Adicionar ao Board" na tela de detalhe de reunião. Ambos vinculam o card a uma reunião existente.

Esta feature permite criar cards avulsos — sem vínculo inicial com nenhuma reunião — diretamente no board.

---

## Modelo de dados — Migration 008

A tabela `board_cards` é alterada via recriação (SQLite não suporta `ALTER COLUMN`):

```sql
CREATE TABLE board_cards_new (
    id          TEXT PRIMARY KEY,
    meeting_id  TEXT REFERENCES meetings(id) ON DELETE CASCADE,  -- nullable
    column_id   TEXT NOT NULL REFERENCES board_columns(id),
    number      INTEGER NOT NULL UNIQUE,
    position    REAL NOT NULL,
    title       TEXT NOT NULL DEFAULT '',        -- usado por cards manuais
    description TEXT NOT NULL DEFAULT '',
    tasks       TEXT NOT NULL DEFAULT '[]',      -- JSON array; usado por cards manuais
    source      TEXT NOT NULL DEFAULT 'meeting', -- 'meeting' | 'manual'
    updated_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL
);

INSERT INTO board_cards_new
    SELECT id, meeting_id, column_id, number, position,
           '', description, '[]', 'meeting', updated_at, created_at
    FROM board_cards;

DROP TABLE board_cards;
ALTER TABLE board_cards_new RENAME TO board_cards;

CREATE INDEX idx_board_cards_column  ON board_cards(column_id);
CREATE INDEX idx_board_cards_number  ON board_cards(number);
CREATE UNIQUE INDEX idx_board_cards_meeting
    ON board_cards(meeting_id) WHERE meeting_id IS NOT NULL;
```

**Campos adicionados:**
- `meeting_id` — torna-se nullable; unicidade garantida via índice parcial
- `title` — título do card manual (cards de reunião derivam o título via JOIN)
- `tasks` — JSON array de strings (`["Task A", "Task B"]`); cards de reunião usam as tasks da reunião
- `source` — `'meeting'` ou `'manual'`; controla renderização e regras de negócio

**Struct Go (`BoardCard`):**
```go
type BoardCard struct {
    ID          string
    MeetingID   *string   // nil para cards manuais
    ColumnID    string
    Number      int
    Position    float64
    Title       string    // usado quando MeetingID == nil
    Description string
    Tasks       []string  // usado quando MeetingID == nil
    Source      string    // "meeting" | "manual"
    UpdatedAt   time.Time
    CreatedAt   time.Time
}

func (c *BoardCard) DisplayTitle(meeting *Meeting) string {
    if c.MeetingID != nil && meeting != nil {
        return meeting.Title
    }
    return c.Title
}
```

---

## Backend

### Repository (`BoardCardRepository`)

- `Create(card BoardCard)` — sem mudança de assinatura; aceita `MeetingID` como `*string`
- `LinkToMeeting(cardID, meetingID string) error` — novo: UPDATE de `meeting_id` e `source = 'meeting'`
- `Update(card BoardCard)` — inclui `title`, `tasks`, `source` no SET

### Service (`BoardCardService`)

- `CreateManualCard(columnID, title, description string) (BoardCard, error)` — novo
  - Valida `title` não vazio
  - Gera UUID, define `source = "manual"`, `MeetingID = nil`
  - Chama `repo.Create`
- `LinkCardToMeeting(cardID, meetingID string) error` — novo
  - Valida que card existe e `source == "manual"`
  - Valida que reunião existe
  - Chama `repo.LinkToMeeting`

### Handler — novas rotas

```
POST  /api/board/cards/manual        → CreateManualCard
PATCH /api/board/cards/{id}/link     → LinkCardToMeeting
```

**Payload POST `/api/board/cards/manual`:**
```json
{
  "column_id":   "col-backlog",
  "title":       "Revisar proposta comercial",
  "description": "Verificar escopo e precificação"
}
```

**Payload PATCH `/api/board/cards/{id}/link`:**
```json
{ "meeting_id": "<uuid>" }
```

### Validações e error handling

| Condição | HTTP |
|---|---|
| `title` vazio | 400 |
| `column_id` inexistente | 404 |
| Card já tem `meeting_id` | 409 |
| `meeting_id` inexistente no link | 404 |

**Cascade delete:** Card associado a uma reunião (`source = 'meeting'`, `meeting_id` preenchido) é removido em cascata se a reunião for deletada. Card manual puro (`meeting_id IS NULL`) não é afetado por deleção de reuniões.

---

## Frontend

### Indicador visual no `KanbanCard`

Cards manuais (`source === 'manual'`) exibem um ícone de lápis (✏️ ou SVG equivalente) ao lado do número `#N`, no lugar onde ficaria o badge de tema. Ao ser associado a uma reunião, o ícone desaparece e o comportamento volta ao normal.

### Pontos de entrada

**1. Botão "+" no cabeçalho da coluna (`KanbanColumn.tsx`)**
- Ícone `+` no `col-header`, ao lado do nome e contador
- Abre `CreateManualCardModal` com `column_id` pré-preenchido (select desabilitado)

**2. Botão "Novo card" na toolbar do board (`BoardView.tsx`)**
- Ao lado de `BoardFilters`
- Abre `CreateManualCardModal` sem coluna pré-selecionada (select habilitado)

### `CreateManualCardModal` — novo componente

Formulário com:
- **Título** (input, obrigatório — botão "Criar card" desabilitado enquanto vazio)
- **Coluna** (select, pré-selecionado se aberto via coluna)
- **Descrição** (textarea, opcional)

Hook: `useCreateManualCard` → `POST /api/board/cards/manual`
Em sucesso: invalida query do board, fecha modal.
Em erro: toast de erro, modal permanece aberto.

### `CardDetailModal` — edição de tasks manuais

Para cards com `source === 'manual'`, as tasks exibidas e editadas no modal são lidas de `board_cards.tasks` (JSON array). A UI de edição de tasks é a mesma já existente para cards de reunião — sem novo componente necessário.

### `CardDetailModal` — associar a reunião

Exibido apenas para `source === 'manual'` sem `meeting_id`:
- Botão "Associar a reunião" abre selector searchable de reuniões existentes
- Ao confirmar: `PATCH /api/board/cards/{id}/link` → invalida queries do board
- Após associar: card passa a exibir título e tasks da reunião; ícone de lápis desaparece; `board_cards.tasks` é ignorado

---

## Fora de escopo

- Tasks editáveis na criação (adicionadas depois via `CardDetailModal`)
- Desassociar uma reunião após linkagem
- Cards manuais com cor/tema próprio
