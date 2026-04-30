# Manual Board Cards — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Permitir criação de cards no board sem vínculo com reunião, com formulário de título + coluna + descrição, indicador visual de lápis e opção de associar a uma reunião depois.

**Architecture:** Migration 008 recria `board_cards` com `meeting_id` nullable e adiciona colunas `title`, `tasks` (JSON), `source`. O backend expande o repositório, service e handler com dois novos endpoints. O frontend adiciona `CreateManualCardModal`, botão "+" nas colunas, botão "Novo card" na toolbar e suporte a tasks manuais no `CardDetailModal`.

**Tech Stack:** Go 1.22+, SQLite (modernc), chi v5, React 19 + TypeScript, React Query v5, lucide-react, Tailwind CSS.

---

### Task 1: Migration 008

**Files:**
- Create: `internal/database/migrations/008_manual_cards.sql`

- [ ] **Step 1: Criar o arquivo de migration**

```sql
-- 008_manual_cards.sql
-- Recria board_cards com meeting_id nullable + novos campos
CREATE TABLE board_cards_new (
    id          TEXT PRIMARY KEY,
    meeting_id  TEXT REFERENCES meetings(id) ON DELETE CASCADE,
    column_id   TEXT NOT NULL REFERENCES board_columns(id),
    number      INTEGER NOT NULL UNIQUE,
    position    REAL NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    tasks       TEXT NOT NULL DEFAULT '[]',
    source      TEXT NOT NULL DEFAULT 'meeting',
    updated_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL
);

INSERT INTO board_cards_new
    SELECT id, meeting_id, column_id, number, position,
           '', description, '[]', 'meeting', updated_at, created_at
    FROM board_cards;

DROP TABLE board_cards;
ALTER TABLE board_cards_new RENAME TO board_cards;

CREATE INDEX idx_board_cards_column ON board_cards(column_id);
CREATE INDEX idx_board_cards_number ON board_cards(number);
CREATE UNIQUE INDEX idx_board_cards_meeting
    ON board_cards(meeting_id) WHERE meeting_id IS NOT NULL;
```

- [ ] **Step 2: Verificar que a migration roda sem erros**

Run: `cd F:/dev/meeting-notes && go test ./internal/repository/... -run TestBoardColumnRepository_ListSeedColumns -v`

Expected: `PASS` — `openBoardTestDB` abre um banco que roda todas as migrations incluindo a 008. Se falhar com "no such column", a migration tem um erro de sintaxe.

- [ ] **Step 3: Commit**

```bash
git add internal/database/migrations/008_manual_cards.sql
git commit -m "feat: add migration 008 — manual board cards schema"
```

---

### Task 2: Atualizar modelos Go

**Files:**
- Modify: `internal/models/models.go`

- [ ] **Step 1: Atualizar `BoardCard`, `BoardCardSummary` e `BoardCardDetail` em `internal/models/models.go`**

Substituir os três tipos e adicionar o método helper:

```go
type BoardCard struct {
	ID          string    `json:"id"`
	MeetingID   *string   `json:"meeting_id"`
	ColumnID    string    `json:"column_id"`
	Number      int       `json:"number"`
	Position    float64   `json:"position"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tasks       []string  `json:"tasks"`
	Source      string    `json:"source"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (c *BoardCard) DisplayTitle(meeting *Meeting) string {
	if c.MeetingID != nil && meeting != nil {
		return meeting.Title
	}
	return c.Title
}

type BoardCardSummary struct {
	ID           string       `json:"id"`
	MeetingID    *string      `json:"meeting_id"`
	ColumnID     string       `json:"column_id"`
	Number       int          `json:"number"`
	Position     float64      `json:"position"`
	Description  string       `json:"description"`
	Source       string       `json:"source"`
	UpdatedAt    time.Time    `json:"updated_at"`
	CreatedAt    time.Time    `json:"created_at"`
	MeetingTitle string       `json:"meeting_title"`
	ThemeID      *string      `json:"theme_id"`
	ThemeName    *string      `json:"theme_name"`
	ThemeColor   *string      `json:"theme_color"`
	Status       string       `json:"status"`
	TaskProgress TaskProgress `json:"task_progress"`
}

type BoardCardDetail struct {
	ID           string     `json:"id"`
	MeetingID    *string    `json:"meeting_id"`
	ColumnID     string     `json:"column_id"`
	Number       int        `json:"number"`
	Position     float64    `json:"position"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Source       string     `json:"source"`
	ManualTasks  []string   `json:"manual_tasks"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CreatedAt    time.Time  `json:"created_at"`
	Status       string     `json:"status"`
	MeetingTitle string     `json:"meeting_title"`
	ThemeID      *string    `json:"theme_id"`
	ThemeName    *string    `json:"theme_name"`
	ThemeColor   *string    `json:"theme_color"`
	Summary      *Summary   `json:"summary"`
	KeyPoints    []KeyPoint `json:"key_points"`
	Tasks        []Task     `json:"tasks"`
}
```

- [ ] **Step 2: Compilar para verificar erros de tipo**

Run: `cd F:/dev/meeting-notes && go build ./...`

Expected: erros de compilação nos arquivos que usam `BoardCard.MeetingID` como `string` — isso é esperado. Os erros indicam quais linhas precisam ser corrigidas nas tarefas seguintes. Se houver ZERO erros já, verificar se os tipos foram salvos corretamente.

- [ ] **Step 3: Commit**

```bash
git add internal/models/models.go
git commit -m "feat: update board card models — nullable meeting_id, source, title, tasks"
```

---

### Task 3: Atualizar repository — queries existentes

**Files:**
- Modify: `internal/repository/board_card_repository.go`
- Modify: `internal/repository/board_card_repository_test.go`

- [ ] **Step 1: Escrever os testes que devem passar após as mudanças**

Em `internal/repository/board_card_repository_test.go`, adicionar ao final do arquivo:

```go
func TestBoardCardRepository_MeetingIDIsPointer(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-ptr", "Meeting Pointer")

	card, err := cardRepo.Create(ctx, "meeting-ptr", "col-backlog", "", 1000)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if card.MeetingID == nil {
		t.Error("MeetingID should not be nil for a meeting card")
	}
	if *card.MeetingID != "meeting-ptr" {
		t.Errorf("MeetingID = %q, want 'meeting-ptr'", *card.MeetingID)
	}
	if card.Source != "meeting" {
		t.Errorf("Source = %q, want 'meeting'", card.Source)
	}
}

func TestBoardCardRepository_Update(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-upd", "Meeting Update")

	card, err := cardRepo.Create(ctx, "meeting-upd", "col-backlog", "original", 1000)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := cardRepo.Update(ctx, card.ID, "updated desc", []string{}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := cardRepo.GetByID(ctx, card.ID)
	if err != nil {
		t.Fatalf("GetByID after Update: %v", err)
	}
	if got.Description != "updated desc" {
		t.Errorf("Description = %q, want 'updated desc'", got.Description)
	}
}
```

- [ ] **Step 2: Rodar os novos testes para confirmar que falham**

Run: `cd F:/dev/meeting-notes && go test ./internal/repository/... -run "TestBoardCardRepository_MeetingIDIsPointer|TestBoardCardRepository_Update" -v`

Expected: FAIL — `cardRepo.Update` não existe ainda e `card.MeetingID` ainda é `string`.

- [ ] **Step 3: Atualizar `Create`, `GetByID`, `GetByMeetingID` em `board_card_repository.go`**

Substituir o método `Create` (linhas 122–160):

```go
func (r *BoardCardRepository) Create(ctx context.Context, meetingID, columnID, description string, position float64) (*models.BoardCard, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var maxNum int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(number), 0) FROM board_cards`).Scan(&maxNum); err != nil {
		return nil, fmt.Errorf("get max number: %w", err)
	}

	now := time.Now().UTC()
	mid := meetingID
	card := &models.BoardCard{
		ID:          uuid.New().String(),
		MeetingID:   &mid,
		ColumnID:    columnID,
		Number:      maxNum + 1,
		Position:    position,
		Title:       "",
		Description: description,
		Tasks:       []string{},
		Source:      "meeting",
		UpdatedAt:   now,
		CreatedAt:   now,
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO board_cards (id, meeting_id, column_id, number, position, title, description, tasks, source, updated_at, created_at)
		 VALUES (?, ?, ?, ?, ?, '', ?, '[]', 'meeting', ?, ?)`,
		card.ID, card.MeetingID, card.ColumnID, card.Number, card.Position, card.Description,
		card.UpdatedAt.UTC().Format(time.RFC3339Nano),
		card.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create board card: %w", err)
	}
	return card, tx.Commit()
}
```

Substituir `GetByID` (linhas 162–182):

```go
func (r *BoardCardRepository) GetByID(ctx context.Context, id string) (*models.BoardCard, error) {
	var card models.BoardCard
	var updatedAt, createdAt, tasksJSON string
	var meetingID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, meeting_id, column_id, number, position, title, description, tasks, source, updated_at, created_at
		 FROM board_cards WHERE id = ?`, id,
	).Scan(&card.ID, &meetingID, &card.ColumnID, &card.Number, &card.Position,
		&card.Title, &card.Description, &tasksJSON, &card.Source, &updatedAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get board card: %w", err)
	}
	if meetingID.Valid {
		s := meetingID.String
		card.MeetingID = &s
	}
	if err := json.Unmarshal([]byte(tasksJSON), &card.Tasks); err != nil {
		card.Tasks = []string{}
	}
	var parseErr error
	if card.UpdatedAt, parseErr = parseTime(updatedAt); parseErr != nil {
		return nil, parseErr
	}
	if card.CreatedAt, parseErr = parseTime(createdAt); parseErr != nil {
		return nil, parseErr
	}
	return &card, nil
}
```

Adicionar `"encoding/json"` aos imports se ainda não estiver.

Substituir `GetByMeetingID` (linhas 184–204):

```go
func (r *BoardCardRepository) GetByMeetingID(ctx context.Context, meetingID string) (*models.BoardCard, error) {
	var card models.BoardCard
	var updatedAt, createdAt, tasksJSON string
	var mid sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, meeting_id, column_id, number, position, title, description, tasks, source, updated_at, created_at
		 FROM board_cards WHERE meeting_id = ?`, meetingID,
	).Scan(&card.ID, &mid, &card.ColumnID, &card.Number, &card.Position,
		&card.Title, &card.Description, &tasksJSON, &card.Source, &updatedAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get board card by meeting: %w", err)
	}
	if mid.Valid {
		s := mid.String
		card.MeetingID = &s
	}
	if err := json.Unmarshal([]byte(tasksJSON), &card.Tasks); err != nil {
		card.Tasks = []string{}
	}
	var parseErr error
	if card.UpdatedAt, parseErr = parseTime(updatedAt); parseErr != nil {
		return nil, parseErr
	}
	if card.CreatedAt, parseErr = parseTime(createdAt); parseErr != nil {
		return nil, parseErr
	}
	return &card, nil
}
```

- [ ] **Step 4: Atualizar `GetDetail` e `List` em `board_card_repository.go`**

Substituir `GetDetail` (linhas 206–254):

```go
func (r *BoardCardRepository) GetDetail(ctx context.Context, id string) (*models.BoardCardDetail, error) {
	var d models.BoardCardDetail
	var updatedAt, createdAt, tasksJSON string
	var meetingID, themeID, themeName, themeColor sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.meeting_id, c.column_id, c.number, c.position, c.title, c.description,
		       c.tasks, c.source, c.updated_at, c.created_at,
		       col.name,
		       COALESCE(m.title, c.title, ''),
		       m.theme_id,
		       t.name, t.color
		FROM board_cards c
		LEFT JOIN meetings m ON c.meeting_id = m.id
		JOIN board_columns col ON c.column_id = col.id
		LEFT JOIN themes t ON m.theme_id = t.id
		WHERE c.id = ?`, id,
	).Scan(
		&d.ID, &meetingID, &d.ColumnID, &d.Number, &d.Position, &d.Title, &d.Description,
		&tasksJSON, &d.Source, &updatedAt, &createdAt,
		&d.Status,
		&d.MeetingTitle, &themeID,
		&themeName, &themeColor,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get board card detail: %w", err)
	}
	if meetingID.Valid {
		s := meetingID.String
		d.MeetingID = &s
	}
	if err := json.Unmarshal([]byte(tasksJSON), &d.ManualTasks); err != nil {
		d.ManualTasks = []string{}
	}
	var parseErr error
	if d.UpdatedAt, parseErr = parseTime(updatedAt); parseErr != nil {
		return nil, parseErr
	}
	if d.CreatedAt, parseErr = parseTime(createdAt); parseErr != nil {
		return nil, parseErr
	}
	if themeID.Valid {
		s := themeID.String
		d.ThemeID = &s
	}
	if themeName.Valid {
		s := themeName.String
		d.ThemeName = &s
	}
	if themeColor.Valid {
		s := themeColor.String
		d.ThemeColor = &s
	}
	return &d, nil
}
```

Substituir `List` (linhas 33–120):

```go
func (r *BoardCardRepository) List(ctx context.Context, f BoardCardFilters) ([]models.BoardCardSummary, error) {
	q := `
		SELECT
			c.id, c.meeting_id, c.column_id, c.number, c.position, c.description,
			c.source, c.updated_at, c.created_at,
			COALESCE(m.title, c.title, '') as meeting_title,
			m.theme_id,
			t.name, t.color,
			col.name,
			CASE WHEN c.meeting_id IS NULL THEN json_array_length(c.tasks) ELSE COUNT(tk.id) END,
			CASE WHEN c.meeting_id IS NULL THEN 0 ELSE COALESCE(SUM(CASE WHEN tk.completed = 1 THEN 1 ELSE 0 END), 0) END
		FROM board_cards c
		LEFT JOIN meetings m ON c.meeting_id = m.id
		JOIN board_columns col ON c.column_id = col.id
		LEFT JOIN themes t ON m.theme_id = t.id
		LEFT JOIN tasks tk ON m.id = tk.meeting_id
		WHERE 1=1`

	var args []any
	if f.Title != "" {
		q += ` AND COALESCE(m.title, c.title, '') LIKE ?`
		args = append(args, "%"+f.Title+"%")
	}
	if f.Number != nil {
		q += ` AND c.number = ?`
		args = append(args, *f.Number)
	}
	if f.CreatedAfter != nil {
		q += ` AND c.created_at >= ?`
		args = append(args, f.CreatedAfter.UTC().Format(time.RFC3339Nano))
	}
	if f.CreatedBefore != nil {
		q += ` AND c.created_at <= ?`
		args = append(args, f.CreatedBefore.UTC().Format(time.RFC3339Nano))
	}
	if f.UpdatedAfter != nil {
		q += ` AND c.updated_at >= ?`
		args = append(args, f.UpdatedAfter.UTC().Format(time.RFC3339Nano))
	}
	if f.UpdatedBefore != nil {
		q += ` AND c.updated_at <= ?`
		args = append(args, f.UpdatedBefore.UTC().Format(time.RFC3339Nano))
	}
	q += ` GROUP BY c.id ORDER BY col.position, c.position`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list board cards: %w", err)
	}
	defer rows.Close()

	var cards []models.BoardCardSummary
	for rows.Next() {
		var card models.BoardCardSummary
		var updatedAt, createdAt string
		var meetingID, themeID, themeName, themeColor sql.NullString
		if err := rows.Scan(
			&card.ID, &meetingID, &card.ColumnID, &card.Number, &card.Position, &card.Description,
			&card.Source, &updatedAt, &createdAt,
			&card.MeetingTitle, &themeID,
			&themeName, &themeColor,
			&card.Status,
			&card.TaskProgress.Total, &card.TaskProgress.Completed,
		); err != nil {
			return nil, fmt.Errorf("scan board card: %w", err)
		}
		if meetingID.Valid {
			s := meetingID.String
			card.MeetingID = &s
		}
		var parseErr error
		if card.UpdatedAt, parseErr = parseTime(updatedAt); parseErr != nil {
			return nil, parseErr
		}
		if card.CreatedAt, parseErr = parseTime(createdAt); parseErr != nil {
			return nil, parseErr
		}
		if themeID.Valid {
			s := themeID.String
			card.ThemeID = &s
		}
		if themeName.Valid {
			s := themeName.String
			card.ThemeName = &s
		}
		if themeColor.Valid {
			s := themeColor.String
			card.ThemeColor = &s
		}
		cards = append(cards, card)
	}
	return cards, rows.Err()
}
```

- [ ] **Step 5: Renomear `UpdateDescription` → `Update` (aceita tasks também)**

Substituir `UpdateDescription` (linhas 256–269):

```go
func (r *BoardCardRepository) Update(ctx context.Context, id, description string, tasks []string) error {
	tasksJSON, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx,
		`UPDATE board_cards SET description = ?, tasks = ?, updated_at = ? WHERE id = ?`,
		description, string(tasksJSON), now, id,
	)
	if err != nil {
		return fmt.Errorf("update board card: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 6: Atualizar testes existentes que usam `MeetingID` como string**

Em `board_card_repository_test.go`, atualizar as comparações de `MeetingID`. Buscar por `got.MeetingID` e `card.MeetingID` e substituir conforme necessário:

```go
// TestBoardCardRepository_GetByID (linha ~90)
// Substituir:
//   if got.Description != "some description" {
// (sem mudança nessa linha, mas verificar MeetingID se usado)

// TestBoardCardRepository_GetByMeetingID (linha ~315)
// Substituir:
if got.MeetingID == nil || *got.MeetingID != "meeting-byid" {
    t.Errorf("MeetingID = %v, want 'meeting-byid'", got.MeetingID)
}

// TestBoardCardRepository_UpdateDescription (linha ~280) — esse teste agora testa Update
// Substituir toda a chamada:
if err := cardRepo.Update(ctx, card.ID, "updated", []string{}); err != nil {
    t.Fatalf("Update: %v", err)
}
```

- [ ] **Step 7: Rodar todos os testes do repositório**

Run: `cd F:/dev/meeting-notes && go test ./internal/repository/... -v 2>&1 | tail -30`

Expected: todos passam com `PASS`. Se houver falhas de compilação, verificar imports (`encoding/json` adicionado).

- [ ] **Step 8: Commit**

```bash
git add internal/repository/board_card_repository.go internal/repository/board_card_repository_test.go
git commit -m "feat: update board card repository — nullable meeting_id, LEFT JOIN, Update method"
```

---

### Task 4: Repository — CreateManual e LinkToMeeting

**Files:**
- Modify: `internal/repository/board_card_repository.go`
- Modify: `internal/repository/board_card_repository_test.go`

- [ ] **Step 1: Escrever os testes que devem falhar primeiro**

Adicionar ao final de `board_card_repository_test.go`:

```go
func TestBoardCardRepository_CreateManual(t *testing.T) {
	db := openBoardTestDB(t)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	card, err := cardRepo.CreateManual(ctx, "col-backlog", "Revisar proposta", "Detalhe", []string{"Task A", "Task B"}, 1000)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	if card.MeetingID != nil {
		t.Errorf("MeetingID should be nil, got %v", card.MeetingID)
	}
	if card.Source != "manual" {
		t.Errorf("Source = %q, want 'manual'", card.Source)
	}
	if card.Title != "Revisar proposta" {
		t.Errorf("Title = %q, want 'Revisar proposta'", card.Title)
	}
	if len(card.Tasks) != 2 || card.Tasks[0] != "Task A" {
		t.Errorf("Tasks = %v, want [Task A Task B]", card.Tasks)
	}
	if card.Number < 1 {
		t.Errorf("Number = %d, want >= 1", card.Number)
	}
}

func TestBoardCardRepository_CreateManualSequentialNumber(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-seq", "Meeting Seq")
	m1, err := cardRepo.Create(ctx, "meeting-seq", "col-backlog", "", 1000)
	if err != nil {
		t.Fatalf("Create meeting card: %v", err)
	}

	m2, err := cardRepo.CreateManual(ctx, "col-backlog", "Manual card", "", []string{}, 2000)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	if m2.Number != m1.Number+1 {
		t.Errorf("manual card number = %d, want %d", m2.Number, m1.Number+1)
	}
}

func TestBoardCardRepository_CreateManualDuplicateTitleAllowed(t *testing.T) {
	db := openBoardTestDB(t)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	_, err := cardRepo.CreateManual(ctx, "col-backlog", "Same title", "", []string{}, 1000)
	if err != nil {
		t.Fatalf("first CreateManual: %v", err)
	}
	_, err = cardRepo.CreateManual(ctx, "col-backlog", "Same title", "", []string{}, 2000)
	if err != nil {
		t.Fatalf("second CreateManual with same title: %v", err)
	}
}

func TestBoardCardRepository_LinkToMeeting(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-link", "Meeting Link")

	card, err := cardRepo.CreateManual(ctx, "col-backlog", "Manual", "", []string{}, 1000)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}

	if err := cardRepo.LinkToMeeting(ctx, card.ID, "meeting-link"); err != nil {
		t.Fatalf("LinkToMeeting: %v", err)
	}

	got, err := cardRepo.GetByID(ctx, card.ID)
	if err != nil {
		t.Fatalf("GetByID after link: %v", err)
	}
	if got.MeetingID == nil || *got.MeetingID != "meeting-link" {
		t.Errorf("MeetingID = %v, want 'meeting-link'", got.MeetingID)
	}
	if got.Source != "meeting" {
		t.Errorf("Source = %q, want 'meeting' after link", got.Source)
	}
}

func TestBoardCardRepository_LinkToMeeting_AlreadyLinked(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-already", "Meeting Already")
	meetingCard, err := cardRepo.Create(ctx, "meeting-already", "col-backlog", "", 1000)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	createTestMeeting(t, meetingRepo, "meeting-other", "Meeting Other")
	err = cardRepo.LinkToMeeting(ctx, meetingCard.ID, "meeting-other")
	if !errors.Is(err, repository.ErrDuplicate) {
		t.Errorf("LinkToMeeting on already-linked card = %v, want ErrDuplicate", err)
	}
}
```

- [ ] **Step 2: Rodar para confirmar falha**

Run: `cd F:/dev/meeting-notes && go test ./internal/repository/... -run "TestBoardCardRepository_CreateManual|TestBoardCardRepository_LinkToMeeting" -v 2>&1 | head -20`

Expected: FAIL — `cardRepo.CreateManual` e `cardRepo.LinkToMeeting` não existem.

- [ ] **Step 3: Implementar `CreateManual` e `LinkToMeeting`**

Adicionar ao final de `board_card_repository.go` (antes do `rebalanceIfNeeded`):

```go
func (r *BoardCardRepository) CreateManual(ctx context.Context, columnID, title, description string, tasks []string, position float64) (*models.BoardCard, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var maxNum int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(number), 0) FROM board_cards`).Scan(&maxNum); err != nil {
		return nil, fmt.Errorf("get max number: %w", err)
	}

	tasksJSON, err := json.Marshal(tasks)
	if err != nil {
		return nil, fmt.Errorf("marshal tasks: %w", err)
	}

	now := time.Now().UTC()
	card := &models.BoardCard{
		ID:          uuid.New().String(),
		MeetingID:   nil,
		ColumnID:    columnID,
		Number:      maxNum + 1,
		Position:    position,
		Title:       title,
		Description: description,
		Tasks:       tasks,
		Source:      "manual",
		UpdatedAt:   now,
		CreatedAt:   now,
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO board_cards (id, meeting_id, column_id, number, position, title, description, tasks, source, updated_at, created_at)
		 VALUES (?, NULL, ?, ?, ?, ?, ?, ?, 'manual', ?, ?)`,
		card.ID, card.ColumnID, card.Number, card.Position,
		card.Title, card.Description, string(tasksJSON),
		card.UpdatedAt.UTC().Format(time.RFC3339Nano),
		card.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("create manual card: %w", err)
	}
	return card, tx.Commit()
}

func (r *BoardCardRepository) LinkToMeeting(ctx context.Context, cardID, meetingID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx,
		`UPDATE board_cards SET meeting_id = ?, source = 'meeting', updated_at = ? WHERE id = ? AND meeting_id IS NULL`,
		meetingID, now, cardID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrDuplicate
		}
		return fmt.Errorf("link card to meeting: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var exists int
		_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM board_cards WHERE id = ?`, cardID).Scan(&exists)
		if exists == 0 {
			return ErrNotFound
		}
		return ErrDuplicate
	}
	return nil
}
```

- [ ] **Step 4: Rodar os novos testes**

Run: `cd F:/dev/meeting-notes && go test ./internal/repository/... -run "TestBoardCardRepository_CreateManual|TestBoardCardRepository_LinkToMeeting" -v`

Expected: todos passam com `PASS`.

- [ ] **Step 5: Rodar suite completa do repositório**

Run: `cd F:/dev/meeting-notes && go test ./internal/repository/... -v 2>&1 | grep -E "PASS|FAIL|---"`

Expected: todos `PASS`.

- [ ] **Step 6: Commit**

```bash
git add internal/repository/board_card_repository.go internal/repository/board_card_repository_test.go
git commit -m "feat: add CreateManual and LinkToMeeting to board card repository"
```

---

### Task 5: Atualizar service

**Files:**
- Modify: `internal/services/board_card_service.go`
- Create: `internal/services/board_card_service_test.go`

- [ ] **Step 1: Escrever os testes do service**

Criar `internal/services/board_card_service_test.go`:

```go
package services_test

import (
	"context"
	"errors"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTestBoardCardService(t *testing.T) *services.BoardCardService {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	cardRepo := repository.NewBoardCardRepository(db)
	columnRepo := repository.NewBoardColumnRepository(db)
	meetingRepo := repository.NewMeetingRepository(db)
	summaryRepo := repository.NewSummaryRepository(db)
	keyPointRepo := repository.NewKeyPointRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	return services.NewBoardCardService(cardRepo, columnRepo, meetingRepo, summaryRepo, keyPointRepo, taskRepo)
}

func TestBoardCardService_CreateManualCard(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	card, err := svc.CreateManualCard(ctx, "col-backlog", "Revisar proposta", "Detalhes")
	if err != nil {
		t.Fatalf("CreateManualCard: %v", err)
	}
	if card.Source != "manual" {
		t.Errorf("Source = %q, want 'manual'", card.Source)
	}
	if card.MeetingID != nil {
		t.Errorf("MeetingID should be nil")
	}
	if card.Title != "Revisar proposta" {
		t.Errorf("Title = %q, want 'Revisar proposta'", card.Title)
	}
}

func TestBoardCardService_CreateManualCard_EmptyTitle(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	_, err := svc.CreateManualCard(ctx, "col-backlog", "", "desc")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for empty title, got %T: %v", err, err)
	}
}

func TestBoardCardService_CreateManualCard_InvalidColumn(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	_, err := svc.CreateManualCard(ctx, "col-nonexistent", "Title", "")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound for invalid column, got %v", err)
	}
}

func TestBoardCardService_Update(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	card, err := svc.CreateManualCard(ctx, "col-backlog", "Title", "original desc")
	if err != nil {
		t.Fatalf("CreateManualCard: %v", err)
	}

	updated, err := svc.Update(ctx, card.ID, "updated desc", []string{"Task 1"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "updated desc" {
		t.Errorf("Description = %q, want 'updated desc'", updated.Description)
	}
}

func TestBoardCardService_Update_NotFound(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	_, err := svc.Update(ctx, "nonexistent-id", "desc", []string{})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBoardCardService_CreateManualCard_UsesFirstColumnWhenEmpty(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	card, err := svc.CreateManualCard(ctx, "", "Title", "")
	if err != nil {
		t.Fatalf("CreateManualCard with empty column: %v", err)
	}
	if card.ColumnID == "" {
		t.Error("ColumnID should not be empty when no column provided")
	}
}

// LinkCardToMeeting requires a real meeting, so we use SQL directly via a helper DB.
func TestBoardCardService_LinkCardToMeeting_NotFound(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	card, err := svc.CreateManualCard(ctx, "col-backlog", "Manual", "")
	if err != nil {
		t.Fatalf("CreateManualCard: %v", err)
	}

	err = svc.LinkCardToMeeting(ctx, card.ID, "nonexistent-meeting")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent meeting, got %v", err)
	}
}

func TestBoardCardService_LinkCardToMeeting_CardNotFound(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	err := svc.LinkCardToMeeting(ctx, "nonexistent-card", "any-meeting")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent card, got %v", err)
	}
}

var _ = models.BoardCard{}
```

- [ ] **Step 2: Rodar testes para confirmar falha**

Run: `cd F:/dev/meeting-notes && go test ./internal/services/... -run "TestBoardCardService" -v 2>&1 | head -30`

Expected: FAIL — `svc.CreateManualCard`, `svc.Update`, `svc.LinkCardToMeeting` não existem ainda.

- [ ] **Step 3: Atualizar `board_card_service.go`**

Substituir `UpdateDescription` e adicionar os métodos novos. O arquivo completo após as mudanças:

```go
package services

import (
	"context"
	"errors"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type BoardCardService struct {
	cardRepo     *repository.BoardCardRepository
	columnRepo   *repository.BoardColumnRepository
	meetingRepo  *repository.MeetingRepository
	summaryRepo  *repository.SummaryRepository
	keyPointRepo *repository.KeyPointRepository
	taskRepo     *repository.TaskRepository
}

func NewBoardCardService(
	cardRepo *repository.BoardCardRepository,
	columnRepo *repository.BoardColumnRepository,
	meetingRepo *repository.MeetingRepository,
	summaryRepo *repository.SummaryRepository,
	keyPointRepo *repository.KeyPointRepository,
	taskRepo *repository.TaskRepository,
) *BoardCardService {
	return &BoardCardService{cardRepo, columnRepo, meetingRepo, summaryRepo, keyPointRepo, taskRepo}
}

func (s *BoardCardService) List(ctx context.Context, f repository.BoardCardFilters) ([]models.BoardCardSummary, error) {
	cards, err := s.cardRepo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	if cards == nil {
		return []models.BoardCardSummary{}, nil
	}
	return cards, nil
}

func (s *BoardCardService) Create(ctx context.Context, meetingID, columnID string) (*models.BoardCard, error) {
	if _, err := s.meetingRepo.GetByID(ctx, meetingID); err != nil {
		return nil, err
	}
	if columnID == "" {
		cols, err := s.columnRepo.List(ctx)
		if err != nil {
			return nil, err
		}
		if len(cols) == 0 {
			return nil, &ValidationError{Message: "no columns exist; create a column first"}
		}
		columnID = cols[0].ID
	} else {
		if _, err := s.columnRepo.GetByID(ctx, columnID); err != nil {
			return nil, err
		}
	}

	description := ""
	if sum, err := s.summaryRepo.GetByMeetingID(ctx, meetingID); err == nil {
		description = sum.Content
	}

	lastPos, err := s.cardRepo.LastPositionInColumn(ctx, columnID)
	if err != nil {
		return nil, err
	}
	return s.cardRepo.Create(ctx, meetingID, columnID, description, lastPos+1000)
}

func (s *BoardCardService) CreateManualCard(ctx context.Context, columnID, title, description string) (*models.BoardCard, error) {
	if title == "" {
		return nil, &ValidationError{Message: "title is required"}
	}
	if columnID == "" {
		cols, err := s.columnRepo.List(ctx)
		if err != nil {
			return nil, err
		}
		if len(cols) == 0 {
			return nil, &ValidationError{Message: "no columns exist; create a column first"}
		}
		columnID = cols[0].ID
	} else {
		if _, err := s.columnRepo.GetByID(ctx, columnID); err != nil {
			return nil, err
		}
	}
	lastPos, err := s.cardRepo.LastPositionInColumn(ctx, columnID)
	if err != nil {
		return nil, err
	}
	return s.cardRepo.CreateManual(ctx, columnID, title, description, []string{}, lastPos+1000)
}

func (s *BoardCardService) LinkCardToMeeting(ctx context.Context, cardID, meetingID string) error {
	if _, err := s.cardRepo.GetByID(ctx, cardID); err != nil {
		return err
	}
	if _, err := s.meetingRepo.GetByID(ctx, meetingID); err != nil {
		return err
	}
	err := s.cardRepo.LinkToMeeting(ctx, cardID, meetingID)
	if errors.Is(err, repository.ErrDuplicate) {
		return err
	}
	return err
}

func (s *BoardCardService) GetDetail(ctx context.Context, id string) (*models.BoardCardDetail, error) {
	detail, err := s.cardRepo.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	if detail.MeetingID != nil {
		if sum, err := s.summaryRepo.GetByMeetingID(ctx, *detail.MeetingID); err == nil {
			detail.Summary = sum
		}
		kps, err := s.keyPointRepo.ListByMeetingID(ctx, *detail.MeetingID)
		if err == nil {
			detail.KeyPoints = kps
		}
		tasks, err := s.taskRepo.ListByMeetingID(ctx, *detail.MeetingID)
		if err == nil {
			detail.Tasks = tasks
		}
	}
	if detail.KeyPoints == nil {
		detail.KeyPoints = []models.KeyPoint{}
	}
	if detail.Tasks == nil {
		detail.Tasks = []models.Task{}
	}
	if detail.ManualTasks == nil {
		detail.ManualTasks = []string{}
	}
	return detail, nil
}

func (s *BoardCardService) Update(ctx context.Context, id, description string, tasks []string) (*models.BoardCard, error) {
	if err := s.cardRepo.Update(ctx, id, description, tasks); err != nil {
		return nil, err
	}
	return s.cardRepo.GetByID(ctx, id)
}

func (s *BoardCardService) Move(ctx context.Context, id, columnID string, position float64) error {
	if _, err := s.columnRepo.GetByID(ctx, columnID); err != nil {
		return err
	}
	return s.cardRepo.Move(ctx, id, columnID, position)
}

func (s *BoardCardService) Delete(ctx context.Context, id string) error {
	return s.cardRepo.Delete(ctx, id)
}

func (s *BoardCardService) FindByMeetingID(ctx context.Context, meetingID string) (*models.BoardCard, error) {
	card, err := s.cardRepo.GetByMeetingID(ctx, meetingID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	return card, err
}
```

- [ ] **Step 4: Rodar os testes do service**

Run: `cd F:/dev/meeting-notes && go test ./internal/services/... -run "TestBoardCardService" -v`

Expected: todos passam com `PASS`.

- [ ] **Step 5: Rodar suite completa**

Run: `cd F:/dev/meeting-notes && go test ./... 2>&1 | grep -E "FAIL|ok"`

Expected: `ok` para todos os pacotes; sem `FAIL`.

- [ ] **Step 6: Commit**

```bash
git add internal/services/board_card_service.go internal/services/board_card_service_test.go
git commit -m "feat: add CreateManualCard, LinkCardToMeeting, Update to BoardCardService"
```

---

### Task 6: Atualizar handler e registrar rotas

**Files:**
- Modify: `internal/handlers/board_handler.go`
- Modify: `cmd/api/main.go`
- Modify: `cmd/desktop/app.go`

- [ ] **Step 1: Atualizar `UpdateCard`, adicionar `CreateManualCard` e `LinkCardToMeeting` em `board_handler.go`**

Substituir `UpdateCard` (linhas 218–237):

```go
func (h *BoardHandler) UpdateCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Description string   `json:"description"`
		Tasks       []string `json:"tasks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Tasks == nil {
		req.Tasks = []string{}
	}
	card, err := h.cardSvc.Update(r.Context(), id, req.Description, req.Tasks)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "card not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update card")
		return
	}
	writeJSON(w, http.StatusOK, card)
}
```

Adicionar ao final de `board_handler.go`:

```go
func (h *BoardHandler) CreateManualCard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ColumnID    string `json:"column_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	card, err := h.cardSvc.CreateManualCard(r.Context(), req.ColumnID, req.Title, req.Description)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusBadRequest, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "column not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create manual card")
		return
	}
	writeJSON(w, http.StatusCreated, card)
}

func (h *BoardHandler) LinkCardToMeeting(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		MeetingID string `json:"meeting_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.cardSvc.LinkCardToMeeting(r.Context(), id, req.MeetingID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "card or meeting not found")
			return
		}
		if errors.Is(err, repository.ErrDuplicate) {
			writeError(w, http.StatusConflict, "card already linked to a meeting")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to link card to meeting")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Registrar as novas rotas em `cmd/api/main.go`**

Localizar o bloco `r.Route("/api/board", ...)` (linhas 129–141) e adicionar as duas novas rotas:

```go
r.Route("/api/board", func(r chi.Router) {
    r.Get("/columns", boardHandler.ListColumns)
    r.Post("/columns", boardHandler.CreateColumn)
    r.Patch("/columns/reorder", boardHandler.ReorderColumns)
    r.Put("/columns/{id}", boardHandler.UpdateColumn)
    r.Delete("/columns/{id}", boardHandler.DeleteColumn)
    r.Get("/cards", boardHandler.ListCards)
    r.Post("/cards", boardHandler.CreateCard)
    r.Post("/cards/manual", boardHandler.CreateManualCard)   // nova
    r.Get("/cards/{id}", boardHandler.GetCard)
    r.Put("/cards/{id}", boardHandler.UpdateCard)
    r.Delete("/cards/{id}", boardHandler.DeleteCard)
    r.Patch("/cards/{id}/move", boardHandler.MoveCard)
    r.Patch("/cards/{id}/link", boardHandler.LinkCardToMeeting) // nova
})
```

**Atenção:** `r.Post("/cards/manual", ...)` deve vir ANTES de `r.Get("/cards/{id}", ...)` para evitar conflito com o parâmetro `{id}` capturando `"manual"`.

- [ ] **Step 3: Registrar as mesmas rotas em `cmd/desktop/app.go`**

Localizar o mesmo bloco em `cmd/desktop/app.go` (linhas 151–162) e aplicar as mesmas duas adições na mesma ordem.

- [ ] **Step 4: Compilar os dois entry points**

Run: `cd F:/dev/meeting-notes && go build ./cmd/api/... && go build ./cmd/desktop/...`

Expected: `exit status 0` sem mensagens de erro.

- [ ] **Step 5: Rodar suite completa**

Run: `cd F:/dev/meeting-notes && go test ./... 2>&1 | grep -E "FAIL|ok"`

Expected: `ok` para todos os pacotes.

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/board_handler.go cmd/api/main.go cmd/desktop/app.go
git commit -m "feat: add CreateManualCard and LinkCardToMeeting handler and routes"
```

---

### Task 7: Frontend — tipos e hooks

**Files:**
- Modify: `frontend/src/hooks/useBoard.ts`

- [ ] **Step 1: Atualizar interfaces e adicionar hooks em `frontend/src/hooks/useBoard.ts`**

Substituir `BoardCardSummary`, `BoardCardDetail` e adicionar os dois novos hooks. O arquivo completo:

```typescript
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "./useApi"

export interface TaskProgress { total: number; completed: number }

export interface BoardCardSummary {
  id: string
  meeting_id: string | null
  column_id: string
  number: number
  position: number
  description: string
  source: string
  updated_at: string
  created_at: string
  meeting_title: string
  theme_id: string | null
  theme_name: string | null
  theme_color: string | null
  status: string
  task_progress: TaskProgress
}

export interface BoardCardDetail {
  id: string
  meeting_id: string | null
  column_id: string
  number: number
  position: number
  title: string
  description: string
  source: string
  manual_tasks: string[]
  updated_at: string
  created_at: string
  status: string
  meeting_title: string
  theme_id: string | null
  theme_name: string | null
  theme_color: string | null
  summary: { id: string; content: string; model_used: string } | null
  key_points: Array<{ id: string; content: string; position: number; meeting_id: string }>
  tasks: Array<{ id: string; description: string; completed: boolean; priority: string; assignee: string | null; meeting_id: string }>
}

export interface BoardCardFilters {
  title?: string
  number?: number
  created_after?: string
  created_before?: string
  updated_after?: string
  updated_before?: string
}

export const EMPTY_FILTERS: BoardCardFilters = {}

export function useCards(filters: BoardCardFilters = EMPTY_FILTERS) {
  const params = new URLSearchParams()
  if (filters.title) params.set("title", filters.title)
  if (filters.number != null) params.set("number", String(filters.number))
  if (filters.created_after) params.set("created_after", filters.created_after)
  if (filters.created_before) params.set("created_before", filters.created_before)
  if (filters.updated_after) params.set("updated_after", filters.updated_after)
  if (filters.updated_before) params.set("updated_before", filters.updated_before)
  const qs = params.toString()
  return useQuery({
    queryKey: ["board-cards", filters],
    queryFn: () => api<BoardCardSummary[]>(`/api/board/cards${qs ? "?" + qs : ""}`),
  })
}

export function useCardDetail(id: string | null) {
  return useQuery({
    queryKey: ["board-card", id],
    queryFn: () => api<BoardCardDetail>(`/api/board/cards/${id}`),
    enabled: !!id,
  })
}

export function useCreateCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ meeting_id, column_id }: { meeting_id: string; column_id?: string }) =>
      api<{ id: string; number: number }>("/api/board/cards", {
        method: "POST",
        body: JSON.stringify({ meeting_id, column_id }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["board-cards"] })
      qc.invalidateQueries({ queryKey: ["board-columns"] })
    },
  })
}

export function useCreateManualCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ column_id, title, description }: { column_id: string; title: string; description?: string }) =>
      api<{ id: string; number: number }>("/api/board/cards/manual", {
        method: "POST",
        body: JSON.stringify({ column_id, title, description: description ?? "" }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["board-cards"] })
      qc.invalidateQueries({ queryKey: ["board-columns"] })
    },
  })
}

export function useLinkCardToMeeting() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ cardId, meetingId }: { cardId: string; meetingId: string }) =>
      api<void>(`/api/board/cards/${cardId}/link`, {
        method: "PATCH",
        body: JSON.stringify({ meeting_id: meetingId }),
      }),
    onSuccess: (_data, { cardId }) => {
      qc.invalidateQueries({ queryKey: ["board-cards"] })
      qc.invalidateQueries({ queryKey: ["board-card", cardId] })
    },
  })
}

export function useDeleteCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api<void>(`/api/board/cards/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["board-cards"] })
      qc.invalidateQueries({ queryKey: ["board-columns"] })
    },
  })
}

export function useUpdateCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, description, tasks }: { id: string; description: string; tasks?: string[] }) =>
      api<{ id: string }>(`/api/board/cards/${id}`, {
        method: "PUT",
        body: JSON.stringify({ description, tasks: tasks ?? [] }),
      }),
    onSuccess: (_data, { id }) => {
      qc.invalidateQueries({ queryKey: ["board-cards"] })
      qc.invalidateQueries({ queryKey: ["board-card", id] })
    },
  })
}

export function useCardForMeeting(meetingId: string | null) {
  const result = useQuery({
    queryKey: ["board-cards", EMPTY_FILTERS],
    queryFn: () => api<BoardCardSummary[]>("/api/board/cards"),
    enabled: !!meetingId,
  })
  return {
    ...result,
    data: meetingId ? result.data?.find(c => c.meeting_id === meetingId) : undefined,
  }
}

export function useMoveCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, column_id, position }: { id: string; column_id: string; position: number }) =>
      api<void>(`/api/board/cards/${id}/move`, {
        method: "PATCH",
        body: JSON.stringify({ column_id, position }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board-cards"] }),
    onError: () => qc.invalidateQueries({ queryKey: ["board-cards"] }),
  })
}
```

- [ ] **Step 2: Verificar que o TypeScript compila**

Run: `cd F:/dev/meeting-notes/frontend && npx tsc --noEmit 2>&1 | head -30`

Expected: zero erros de tipo. Se houver erros em componentes que usam `meeting_id` como `string` não-nullable, corrigir nas tarefas seguintes.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/useBoard.ts
git commit -m "feat: update useBoard types and add useCreateManualCard, useLinkCardToMeeting hooks"
```

---

### Task 8: KanbanCard — indicador visual de lápis

**Files:**
- Modify: `frontend/src/components/board/KanbanCard.tsx`

- [ ] **Step 1: Adicionar o ícone de lápis para cards manuais**

Substituir o conteúdo de `KanbanCard.tsx`:

```tsx
import { useSortable } from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import { Pencil } from "lucide-react"
import type { BoardCardSummary } from "../../hooks/useBoard"

interface Props {
  card: BoardCardSummary
  onClick: () => void
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins <= 0) return "agora"
  if (mins < 60) return `há ${mins}m`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `há ${hours}h`
  return `há ${Math.floor(hours / 24)}d`
}

export function KanbanCard({ card, onClick }: Props) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: card.id,
    data: { columnId: card.column_id, position: card.position },
  })
  const style = { transform: CSS.Transform.toString(transform), transition }
  const { total, completed } = card.task_progress

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      className={`bg-card border border-border rounded-md p-3 cursor-grab active:cursor-grabbing hover:border-primary/50 transition-colors select-none ${isDragging ? "opacity-50" : ""}`}
      onClick={onClick}
    >
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-muted-foreground">#{card.number}</span>
          {card.source === "manual" && (
            <Pencil size={10} className="text-muted-foreground/60" />
          )}
        </div>
        {card.source !== "manual" && card.theme_color && (
          <span
            className="text-xs px-1.5 py-0.5 rounded-full"
            style={{ background: card.theme_color + "22", color: card.theme_color }}
          >
            {card.theme_name}
          </span>
        )}
      </div>
      <p className="text-sm font-medium text-foreground mb-1 line-clamp-1">{card.meeting_title}</p>
      {card.description && (
        <p className="text-xs text-muted-foreground line-clamp-2 mb-2">{card.description}</p>
      )}
      <div className="flex items-center gap-2">
        {total > 0 && (
          <div className="flex gap-1 flex-wrap">
            {Array.from({ length: Math.min(total, 10) }, (_, i) => (
              <div
                key={i}
                className={`w-2 h-2 rounded-full ${i < completed ? "bg-green-500" : "bg-muted-foreground/30"}`}
              />
            ))}
            {total > 10 && <span className="text-xs text-muted-foreground">+{total - 10}</span>}
          </div>
        )}
        <span className="text-xs text-muted-foreground ml-auto">{relativeTime(card.created_at)}</span>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Verificar TypeScript**

Run: `cd F:/dev/meeting-notes/frontend && npx tsc --noEmit 2>&1 | head -20`

Expected: zero erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/board/KanbanCard.tsx
git commit -m "feat: add pencil indicator for manual cards in KanbanCard"
```

---

### Task 9: KanbanColumn — botão "+" na coluna

**Files:**
- Modify: `frontend/src/components/board/KanbanColumn.tsx`

- [ ] **Step 1: Atualizar `KanbanColumn.tsx` com o botão "+"**

```tsx
import { SortableContext, verticalListSortingStrategy } from "@dnd-kit/sortable"
import { useDroppable } from "@dnd-kit/core"
import { Plus } from "lucide-react"
import type { BoardColumn } from "../../hooks/useBoardColumns"
import type { BoardCardSummary } from "../../hooks/useBoard"
import { KanbanCard } from "./KanbanCard"

interface Props {
  column: BoardColumn
  cards: BoardCardSummary[]
  onCardClick: (id: string) => void
  onAddCard?: (columnId: string) => void
}

export function KanbanColumn({ column, cards, onCardClick, onAddCard }: Props) {
  const { setNodeRef } = useDroppable({ id: column.id })

  return (
    <div className="flex flex-col bg-muted/30 rounded-lg w-64 flex-shrink-0 min-h-0">
      <div className="flex items-center justify-between px-3 py-2.5 border-b border-border flex-shrink-0">
        <span className="text-sm font-medium">{column.name}</span>
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded-full">
            {cards.length}
          </span>
          {onAddCard && (
            <button
              onClick={e => { e.stopPropagation(); onAddCard(column.id) }}
              className="text-muted-foreground hover:text-foreground transition-colors p-0.5 rounded"
              title="Novo card"
            >
              <Plus size={14} />
            </button>
          )}
        </div>
      </div>
      <div ref={setNodeRef} className="flex flex-col gap-2 p-2 overflow-y-auto flex-1 min-h-16">
        <SortableContext items={cards.map(c => c.id)} strategy={verticalListSortingStrategy}>
          {cards.map(card => (
            <KanbanCard key={card.id} card={card} onClick={() => onCardClick(card.id)} />
          ))}
        </SortableContext>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Verificar TypeScript**

Run: `cd F:/dev/meeting-notes/frontend && npx tsc --noEmit 2>&1 | head -20`

Expected: zero erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/board/KanbanColumn.tsx
git commit -m "feat: add '+' button to KanbanColumn header"
```

---

### Task 10: CreateManualCardModal — novo componente

**Files:**
- Create: `frontend/src/components/board/CreateManualCardModal.tsx`

- [ ] **Step 1: Criar o componente**

```tsx
import { useState } from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"
import { Button } from "../ui/button"
import { useCreateManualCard } from "../../hooks/useBoard"
import type { BoardColumn } from "../../hooks/useBoardColumns"

interface Props {
  columns: BoardColumn[]
  defaultColumnId?: string
  onClose: () => void
}

export function CreateManualCardModal({ columns, defaultColumnId, onClose }: Props) {
  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")
  const [columnId, setColumnId] = useState(defaultColumnId ?? columns[0]?.id ?? "")
  const createCard = useCreateManualCard()

  function handleSubmit() {
    if (!title.trim()) return
    createCard.mutate(
      { column_id: columnId, title: title.trim(), description },
      { onSuccess: onClose },
    )
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-background border border-border rounded-lg w-[420px] shadow-xl overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-border">
          <span className="font-semibold text-sm">Novo card</span>
          <Button variant="ghost" size="icon" onClick={onClose}><X size={16} /></Button>
        </div>

        <div className="px-5 py-4 space-y-4">
          <div>
            <label className="text-xs text-muted-foreground uppercase tracking-widest block mb-1">
              Título <span className="text-destructive">*</span>
            </label>
            <input
              className="w-full text-sm rounded-xl px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary bg-input border border-border text-foreground placeholder:text-muted-foreground/60"
              placeholder="Ex: Revisar proposta comercial"
              value={title}
              onChange={e => setTitle(e.target.value)}
              onKeyDown={e => e.key === "Enter" && handleSubmit()}
              autoFocus
            />
          </div>

          <div>
            <label className="text-xs text-muted-foreground uppercase tracking-widest block mb-1">
              Coluna
            </label>
            <select
              className="w-full text-sm rounded-xl px-3 py-2 focus:outline-none bg-input border border-border text-foreground"
              value={columnId}
              onChange={e => setColumnId(e.target.value)}
              disabled={!!defaultColumnId}
            >
              {columns.map(col => (
                <option key={col.id} value={col.id}>{col.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="text-xs text-muted-foreground uppercase tracking-widest block mb-1">
              Descrição
            </label>
            <textarea
              className="w-full text-sm rounded-xl px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary bg-input border border-border text-foreground placeholder:text-muted-foreground/60 resize-none"
              placeholder="Opcional..."
              rows={3}
              value={description}
              onChange={e => setDescription(e.target.value)}
            />
          </div>
        </div>

        <div className="flex gap-2 px-5 pb-5">
          <Button variant="ghost" className="flex-1" onClick={onClose}>Cancelar</Button>
          <Button
            className="flex-1"
            onClick={handleSubmit}
            disabled={!title.trim() || createCard.isPending}
          >
            {createCard.isPending ? "Criando..." : "Criar card"}
          </Button>
        </div>
      </div>
    </div>,
    document.body,
  )
}
```

- [ ] **Step 2: Verificar TypeScript**

Run: `cd F:/dev/meeting-notes/frontend && npx tsc --noEmit 2>&1 | head -20`

Expected: zero erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/board/CreateManualCardModal.tsx
git commit -m "feat: add CreateManualCardModal component"
```

---

### Task 11: BoardView — botão "Novo card" e wirear modal

**Files:**
- Modify: `frontend/src/components/board/BoardView.tsx`

- [ ] **Step 1: Atualizar `BoardView.tsx`**

```tsx
import { useState } from "react"
import { Plus, Settings2 } from "lucide-react"
import {
  DndContext, DragOverlay, PointerSensor, useSensor, useSensors,
  type DragEndEvent, type DragStartEvent,
} from "@dnd-kit/core"
import { Button } from "../ui/button"
import { useColumns } from "../../hooks/useBoardColumns"
import { useCards, useMoveCard, EMPTY_FILTERS, type BoardCardSummary, type BoardCardFilters } from "../../hooks/useBoard"
import { KanbanColumn } from "./KanbanColumn"
import { KanbanCard } from "./KanbanCard"
import { ColumnSettingsPanel } from "./ColumnSettingsPanel"
import { CardDetailModal } from "./CardDetailModal"
import { BoardFilters } from "./BoardFilters"
import { CreateManualCardModal } from "./CreateManualCardModal"
import { useQueryClient } from "@tanstack/react-query"

export function BoardView() {
  const { data: columns = [] } = useColumns()
  const [filters, setFilters] = useState<BoardCardFilters>(EMPTY_FILTERS)
  const { data: cards = [] } = useCards(filters)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [selectedCardId, setSelectedCardId] = useState<string | null>(null)
  const [activeCard, setActiveCard] = useState<BoardCardSummary | null>(null)
  const [createModalColumnId, setCreateModalColumnId] = useState<string | null>(null)
  const moveCard = useMoveCard()
  const qc = useQueryClient()

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))

  const cardsByColumn = columns.reduce<Record<string, BoardCardSummary[]>>((acc, col) => {
    acc[col.id] = cards.filter(c => c.column_id === col.id)
    return acc
  }, {})

  function onDragStart({ active }: DragStartEvent) {
    setActiveCard(cards.find(c => c.id === active.id) ?? null)
  }

  function onDragEnd({ active, over }: DragEndEvent) {
    setActiveCard(null)
    if (!over || active.id === over.id) return

    const card = cards.find(c => c.id === active.id)
    if (!card) return

    const targetColumnId = columns.find(col => col.id === over.id)
      ? (over.id as string)
      : (cards.find(c => c.id === over.id)?.column_id ?? card.column_id)

    const targetColumnCards = cards
      .filter(c => c.column_id === targetColumnId && c.id !== card.id)
      .sort((a, b) => a.position - b.position)

    const overCardIdx = targetColumnCards.findIndex(c => c.id === over.id)
    let newPosition: number
    if (overCardIdx === -1 || targetColumnCards.length === 0) {
      newPosition = (targetColumnCards[targetColumnCards.length - 1]?.position ?? 0) + 1000
    } else if (overCardIdx === 0) {
      newPosition = targetColumnCards[0].position / 2
    } else {
      newPosition = (targetColumnCards[overCardIdx - 1].position + targetColumnCards[overCardIdx].position) / 2
    }

    qc.setQueryData(["board-cards", filters], (old: BoardCardSummary[] | undefined) =>
      (old ?? []).map(c =>
        c.id === card.id ? { ...c, column_id: targetColumnId, position: newPosition } : c
      )
    )

    moveCard.mutate({ id: card.id, column_id: targetColumnId, position: newPosition })
  }

  return (
    <DndContext sensors={sensors} onDragStart={onDragStart} onDragEnd={onDragEnd}>
      <div className="flex flex-col flex-1 overflow-hidden">
        <div className="flex items-center gap-3 px-4 py-3 border-b border-border flex-shrink-0">
          <span className="font-semibold text-sm flex-1">Board</span>
          <BoardFilters filters={filters} onChange={setFilters} />
          <Button variant="ghost" size="sm" onClick={() => setCreateModalColumnId("")} className="gap-1.5">
            <Plus size={14} />
            Novo card
          </Button>
          <Button variant="ghost" size="icon" onClick={() => setSettingsOpen(true)}>
            <Settings2 size={16} />
          </Button>
        </div>
        <div className="flex flex-1 gap-4 p-4 overflow-x-auto overflow-y-hidden">
          {columns.map(col => (
            <KanbanColumn
              key={col.id}
              column={col}
              cards={cardsByColumn[col.id] ?? []}
              onCardClick={setSelectedCardId}
              onAddCard={colId => setCreateModalColumnId(colId)}
            />
          ))}
          {columns.length === 0 && (
            <div className="flex-1 flex items-center justify-center text-muted-foreground text-sm">
              Nenhuma coluna. Use o ícone de configurações para adicionar.
            </div>
          )}
        </div>
        <DragOverlay>
          {activeCard && <KanbanCard card={activeCard} onClick={() => {}} />}
        </DragOverlay>
      </div>
      {settingsOpen && <ColumnSettingsPanel onClose={() => setSettingsOpen(false)} />}
      <CardDetailModal cardId={selectedCardId} onClose={() => setSelectedCardId(null)} />
      {createModalColumnId !== null && columns.length > 0 && (
        <CreateManualCardModal
          columns={columns}
          defaultColumnId={createModalColumnId || undefined}
          onClose={() => setCreateModalColumnId(null)}
        />
      )}
    </DndContext>
  )
}
```

- [ ] **Step 2: Verificar TypeScript**

Run: `cd F:/dev/meeting-notes/frontend && npx tsc --noEmit 2>&1 | head -20`

Expected: zero erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/board/BoardView.tsx
git commit -m "feat: add Novo card button and wire CreateManualCardModal in BoardView"
```

---

### Task 12: CardDetailModal — tasks manuais e associar reunião

**Files:**
- Modify: `frontend/src/components/board/CardDetailModal.tsx`

- [ ] **Step 1: Atualizar `CardDetailModal.tsx`**

```tsx
import { useState, useEffect } from "react"
import { createPortal } from "react-dom"
import { X, Pencil, Plus, Trash2 } from "lucide-react"
import { Button } from "../ui/button"
import { useCardDetail, useUpdateCard, useLinkCardToMeeting, type BoardCardDetail } from "../../hooks/useBoard"
import { useUpdateTask } from "../../hooks/useMeeting"
import { useMeetings } from "../../hooks/useMeetings"

interface Props {
  cardId: string | null
  onClose: () => void
}

type TaskItem = BoardCardDetail["tasks"][number]

export function CardDetailModal({ cardId, onClose }: Props) {
  const { data: card, isLoading } = useCardDetail(cardId)
  const updateCard = useUpdateCard()
  const linkCard = useLinkCardToMeeting()
  const [description, setDescription] = useState("")
  const [descriptionAtEditStart, setDescriptionAtEditStart] = useState("")
  const [editing, setEditing] = useState(false)
  const [newTask, setNewTask] = useState("")
  const [linkingMeeting, setLinkingMeeting] = useState(false)
  const [selectedMeetingId, setSelectedMeetingId] = useState("")
  const { data: meetings = [] } = useMeetings()

  useEffect(() => {
    if (card) {
      setDescription(card.description)
      setLinkingMeeting(false)
      setSelectedMeetingId("")
    }
  }, [card?.id])

  if (!cardId) return null

  const isManual = card?.source === "manual"
  const manualTasks = card?.manual_tasks ?? []

  function startEditing() {
    setDescriptionAtEditStart(description)
    setEditing(true)
  }

  function cancelEditing() {
    setDescription(descriptionAtEditStart)
    setEditing(false)
  }

  function saveDescription() {
    if (!cardId) return
    updateCard.mutate(
      { id: cardId, description, tasks: isManual ? manualTasks : [] },
      { onSuccess: () => setEditing(false) },
    )
  }

  function addTask() {
    if (!cardId || !newTask.trim()) return
    const updated = [...manualTasks, newTask.trim()]
    updateCard.mutate({ id: cardId, description, tasks: updated })
    setNewTask("")
  }

  function removeTask(index: number) {
    if (!cardId) return
    const updated = manualTasks.filter((_, i) => i !== index)
    updateCard.mutate({ id: cardId, description, tasks: updated })
  }

  function handleLink() {
    if (!cardId || !selectedMeetingId) return
    linkCard.mutate({ cardId, meetingId: selectedMeetingId }, { onSuccess: onClose })
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-background border border-border rounded-lg w-[640px] max-h-[80vh] flex flex-col shadow-xl overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center gap-3 px-5 py-4 border-b border-border flex-shrink-0">
          {isLoading && <span className="text-xs text-muted-foreground flex-1">Carregando...</span>}
          {card && (
            <>
              <div className="flex items-center gap-1.5">
                <span className="text-xs text-muted-foreground">#{card.number}</span>
                {isManual && <Pencil size={11} className="text-muted-foreground/60" />}
              </div>
              {!isManual && card.theme_name && (
                <span
                  className="text-xs px-1.5 py-0.5 rounded-full"
                  style={card.theme_color ? { background: card.theme_color + "22", color: card.theme_color } : undefined}
                >
                  {card.theme_name}
                </span>
              )}
              <h2 className="font-semibold text-sm flex-1">{card.meeting_title}</h2>
              <span className="text-xs text-muted-foreground">{card.status}</span>
            </>
          )}
          <Button variant="ghost" size="icon" onClick={onClose}><X size={16} /></Button>
        </div>

        <div className="flex-1 overflow-y-auto p-5 space-y-5">
          <section>
            <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Descrição</h3>
            {editing ? (
              <div className="space-y-2">
                <textarea
                  className="w-full text-sm bg-input border border-border rounded px-3 py-2 min-h-24 resize-y"
                  value={description}
                  onChange={e => setDescription(e.target.value)}
                  autoFocus
                />
                <div className="flex gap-2">
                  <Button size="sm" onClick={saveDescription}>Salvar</Button>
                  <Button variant="ghost" size="sm" onClick={cancelEditing}>Cancelar</Button>
                </div>
              </div>
            ) : (
              <p
                className="text-sm text-muted-foreground cursor-pointer hover:text-foreground transition-colors min-h-8"
                onClick={startEditing}
              >
                {description || <span className="italic">Clique para editar...</span>}
              </p>
            )}
          </section>

          {isManual && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Tasks</h3>
              <div className="space-y-1.5 mb-2">
                {manualTasks.map((task, i) => (
                  <div key={i} className="flex items-center gap-2 group">
                    <span className="text-sm text-foreground flex-1">{task}</span>
                    <button
                      onClick={() => removeTask(i)}
                      className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-all"
                    >
                      <Trash2 size={13} />
                    </button>
                  </div>
                ))}
              </div>
              <div className="flex gap-2">
                <input
                  className="flex-1 text-sm rounded-lg px-3 py-1.5 bg-input border border-border text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-primary"
                  placeholder="Nova task..."
                  value={newTask}
                  onChange={e => setNewTask(e.target.value)}
                  onKeyDown={e => e.key === "Enter" && addTask()}
                />
                <Button size="sm" variant="ghost" onClick={addTask} disabled={!newTask.trim()}>
                  <Plus size={14} />
                </Button>
              </div>
            </section>
          )}

          {!isManual && card && card.tasks.length > 0 && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">
                Tasks ({card.tasks.filter(t => t.completed).length}/{card.tasks.length})
              </h3>
              <div className="space-y-1.5">
                {card.tasks.map(task => (
                  <TaskRow key={task.id} task={task} meetingId={card.meeting_id ?? ""} />
                ))}
              </div>
            </section>
          )}

          {!isManual && card?.summary && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Resumo</h3>
              <p className="text-sm text-muted-foreground whitespace-pre-wrap">{card.summary.content}</p>
            </section>
          )}

          {!isManual && card && card.key_points.length > 0 && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Pontos-chave</h3>
              <ul className="space-y-1">
                {card.key_points.map(kp => (
                  <li key={kp.id} className="text-sm text-muted-foreground flex gap-2">
                    <span className="text-primary mt-0.5">·</span>
                    {kp.content}
                  </li>
                ))}
              </ul>
            </section>
          )}

          {isManual && !card?.meeting_id && (
            <section className="border-t border-border pt-4">
              {!linkingMeeting ? (
                <Button variant="ghost" size="sm" onClick={() => setLinkingMeeting(true)}>
                  Associar a uma reunião
                </Button>
              ) : (
                <div className="space-y-2">
                  <h3 className="text-xs font-medium text-muted-foreground uppercase">Associar a reunião</h3>
                  <select
                    className="w-full text-sm rounded-lg px-3 py-2 bg-input border border-border text-foreground focus:outline-none"
                    value={selectedMeetingId}
                    onChange={e => setSelectedMeetingId(e.target.value)}
                  >
                    <option value="">Selecionar reunião...</option>
                    {meetings.map(m => (
                      <option key={m.id} value={m.id}>{m.title}</option>
                    ))}
                  </select>
                  <div className="flex gap-2">
                    <Button size="sm" onClick={handleLink} disabled={!selectedMeetingId || linkCard.isPending}>
                      {linkCard.isPending ? "Associando..." : "Confirmar"}
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => { setLinkingMeeting(false); setSelectedMeetingId("") }}>
                      Cancelar
                    </Button>
                  </div>
                </div>
              )}
            </section>
          )}
        </div>
      </div>
    </div>,
    document.body,
  )
}

function TaskRow({ task, meetingId }: { task: TaskItem; meetingId: string }) {
  const updateTask = useUpdateTask(meetingId, task.id)
  return (
    <label className="flex items-start gap-2 cursor-pointer">
      <input
        type="checkbox"
        className="mt-0.5 accent-primary"
        checked={task.completed}
        onChange={e => updateTask.mutate({ completed: e.target.checked })}
      />
      <span className={`text-sm ${task.completed ? "line-through text-muted-foreground" : ""}`}>
        {task.description}
      </span>
    </label>
  )
}
```

- [ ] **Step 2: Verificar TypeScript completo**

Run: `cd F:/dev/meeting-notes/frontend && npx tsc --noEmit 2>&1`

Expected: zero erros.

- [ ] **Step 3: Rodar suite Go para garantir nenhuma regressão**

Run: `cd F:/dev/meeting-notes && go test ./... 2>&1 | grep -E "FAIL|ok"`

Expected: `ok` para todos os pacotes.

- [ ] **Step 4: Commit final**

```bash
git add frontend/src/components/board/CardDetailModal.tsx
git commit -m "feat: add manual tasks editor and link-to-meeting in CardDetailModal"
```
