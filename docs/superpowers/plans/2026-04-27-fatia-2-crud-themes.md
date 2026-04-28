# Fatia 2 — CRUD Themes: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implementar CRUD completo de temas com repositório, serviço, handlers HTTP e rotas registradas no servidor.

**Architecture:** Três camadas: `repository` (acesso ao banco), `services` (validação + UUID), `handlers` (HTTP + DTOs). Cada camada só conhece a camada imediatamente abaixo. Testes usam SQLite real em memória temporária — sem mocks. Context propagado em todas as chamadas de DB.

**Tech Stack:** Go std lib (`database/sql`, `net/http/httptest`), chi v5, google/uuid, modernc.org/sqlite

**Success Criteria:**
```
curl -s -X POST http://localhost:8080/api/themes \
  -H "Content-Type: application/json" \
  -d '{"name":"Engenharia","color":"#3b82f6"}' | jq .
# → {"id":"...","name":"Engenharia","description":"","color":"#3b82f6","created_at":"..."}

curl -s http://localhost:8080/api/themes | jq .
# → [{"id":"...","name":"Engenharia",...}]
```

---

## File Map

| Arquivo | Responsabilidade |
|---|---|
| `internal/repository/theme_repository.go` | SQL CRUD contra `themes` + erros sentinela |
| `internal/repository/theme_repository_test.go` | Testes TDD do repositório com DB real |
| `internal/services/theme_service.go` | Validação, UUID, default de cor, coordenação |
| `internal/services/theme_service_test.go` | Testes TDD do serviço com DB real |
| `internal/handlers/respond.go` | Helpers `writeJSON` / `writeError` compartilhados |
| `internal/handlers/theme_handler.go` | HTTP handlers + DTOs de request |
| `internal/handlers/theme_handler_test.go` | Testes TDD dos handlers com httptest |
| `cmd/api/main.go` | Modificar: registrar rotas `/api/themes` |

---

## Task 1: Theme Repository

**Files:**
- Create: `internal/repository/theme_repository.go`
- Create: `internal/repository/theme_repository_test.go`

O repositório traduz operações de domínio em SQL. Expõe erros sentinela (`ErrNotFound`, `ErrDuplicate`) para que as camadas acima não precisem saber nada sobre SQLite. O `created_at` é escaneado como `string` e convertido com `parseTime` para suportar múltiplos formatos de data que o SQLite pode retornar.

- [ ] **Step 1: Escrever os testes que vão falhar**

Crie `internal/repository/theme_repository_test.go`:

```go
package repository_test

import (
	"context"
	"errors"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

func openTestDB(t *testing.T) *repository.ThemeRepository {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return repository.NewThemeRepository(db)
}

func TestThemeRepository_CreateAndGetByID(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	theme := &models.Theme{
		ID:          "id-001",
		Name:        "Engenharia",
		Description: "Reuniões de eng",
		Color:       "#3b82f6",
	}
	if err := repo.Create(ctx, theme); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "id-001")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Engenharia" {
		t.Errorf("Name = %q, want %q", got.Name, "Engenharia")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestThemeRepository_Create_DuplicateName(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	theme := &models.Theme{ID: "id-001", Name: "Dup", Color: "#fff"}
	if err := repo.Create(ctx, theme); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	theme2 := &models.Theme{ID: "id-002", Name: "Dup", Color: "#000"}
	err := repo.Create(ctx, theme2)
	if !errors.Is(err, repository.ErrDuplicate) {
		t.Errorf("expected ErrDuplicate, got %v", err)
	}
}

func TestThemeRepository_List(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	themes, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(themes) != 0 {
		t.Errorf("expected 0 themes, got %d", len(themes))
	}

	repo.Create(ctx, &models.Theme{ID: "a", Name: "Zeta", Color: "#fff"})
	repo.Create(ctx, &models.Theme{ID: "b", Name: "Alpha", Color: "#000"})

	themes, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List after inserts: %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("expected 2 themes, got %d", len(themes))
	}
	if themes[0].Name != "Alpha" {
		t.Errorf("expected sorted by name, first = %q", themes[0].Name)
	}
}

func TestThemeRepository_Update(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	repo.Create(ctx, &models.Theme{ID: "id-001", Name: "Original", Color: "#fff"})

	got, _ := repo.GetByID(ctx, "id-001")
	got.Name = "Atualizado"
	got.Color = "#000"

	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, _ := repo.GetByID(ctx, "id-001")
	if updated.Name != "Atualizado" {
		t.Errorf("Name = %q, want %q", updated.Name, "Atualizado")
	}
}

func TestThemeRepository_Update_NotFound(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	err := repo.Update(ctx, &models.Theme{ID: "nope", Name: "X", Color: "#fff"})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestThemeRepository_Delete(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	repo.Create(ctx, &models.Theme{ID: "id-001", Name: "Para deletar", Color: "#fff"})
	if err := repo.Delete(ctx, "id-001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, "id-001")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestThemeRepository_Delete_NotFound(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestThemeRepository_GetByID_NotFound(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Rodar os testes para confirmar que falham**

```bash
go test ./internal/repository/... -v
```

Resultado esperado: erro de compilação — pacote `repository` não existe.

- [ ] **Step 3: Implementar `internal/repository/theme_repository.go`**

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"meeting-notes/internal/models"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("name already exists")
)

type ThemeRepository struct {
	db *sql.DB
}

func NewThemeRepository(db *sql.DB) *ThemeRepository {
	return &ThemeRepository{db: db}
}

func (r *ThemeRepository) List(ctx context.Context) ([]models.Theme, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, description, color, created_at FROM themes ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list themes: %w", err)
	}
	defer rows.Close()

	var themes []models.Theme
	for rows.Next() {
		var t models.Theme
		var createdAt string
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Color, &createdAt); err != nil {
			return nil, fmt.Errorf("scan theme: %w", err)
		}
		if t.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, err
		}
		themes = append(themes, t)
	}
	return themes, rows.Err()
}

func (r *ThemeRepository) Create(ctx context.Context, theme *models.Theme) error {
	if theme.CreatedAt.IsZero() {
		theme.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO themes (id, name, description, color, created_at) VALUES (?, ?, ?, ?, ?)`,
		theme.ID, theme.Name, theme.Description, theme.Color,
		theme.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrDuplicate
		}
		return fmt.Errorf("create theme: %w", err)
	}
	return nil
}

func (r *ThemeRepository) GetByID(ctx context.Context, id string) (*models.Theme, error) {
	var t models.Theme
	var createdAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, description, color, created_at FROM themes WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &t.Color, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get theme: %w", err)
	}
	if t.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *ThemeRepository) Update(ctx context.Context, theme *models.Theme) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE themes SET name = ?, description = ?, color = ? WHERE id = ?`,
		theme.Name, theme.Description, theme.Color, theme.ID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrDuplicate
		}
		return fmt.Errorf("update theme: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update theme rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ThemeRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM themes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete theme: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete theme rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}
```

- [ ] **Step 4: Rodar os testes**

```bash
go test ./internal/repository/... -v
```

Resultado esperado:
```
=== RUN   TestThemeRepository_CreateAndGetByID
--- PASS
=== RUN   TestThemeRepository_Create_DuplicateName
--- PASS
=== RUN   TestThemeRepository_List
--- PASS
=== RUN   TestThemeRepository_Update
--- PASS
=== RUN   TestThemeRepository_Update_NotFound
--- PASS
=== RUN   TestThemeRepository_Delete
--- PASS
=== RUN   TestThemeRepository_Delete_NotFound
--- PASS
=== RUN   TestThemeRepository_GetByID_NotFound
--- PASS
PASS
ok      meeting-notes/internal/repository
```

- [ ] **Step 5: Commit**

```bash
git add internal/repository/theme_repository.go internal/repository/theme_repository_test.go
git commit -m "feat: add theme repository with CRUD and sentinel errors"
```

---

## Task 2: Theme Service

**Files:**
- Create: `internal/services/theme_service.go`
- Create: `internal/services/theme_service_test.go`

O serviço encapsula: validação de `name` obrigatório, default de `color`, geração de UUID, e coordenação com o repositório. Define `ValidationError` para que o handler possa distinguir erros de negócio de erros de infraestrutura.

- [ ] **Step 1: Escrever os testes que vão falhar**

Crie `internal/services/theme_service_test.go`:

```go
package services_test

import (
	"context"
	"errors"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTestService(t *testing.T) *services.ThemeService {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return services.NewThemeService(repository.NewThemeRepository(db))
}

func TestThemeService_Create(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	theme, err := svc.Create(ctx, "Produto", "Reuniões de produto", "#8b5cf6")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if theme.ID == "" {
		t.Error("ID should be set")
	}
	if theme.Name != "Produto" {
		t.Errorf("Name = %q", theme.Name)
	}
	if theme.Color != "#8b5cf6" {
		t.Errorf("Color = %q", theme.Color)
	}
	if theme.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestThemeService_Create_DefaultColor(t *testing.T) {
	svc := newTestService(t)

	theme, err := svc.Create(context.Background(), "Sem cor", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if theme.Color != "#6366f1" {
		t.Errorf("default color = %q, want %q", theme.Color, "#6366f1")
	}
}

func TestThemeService_Create_NameRequired(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.Create(context.Background(), "", "", "")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestThemeService_Create_DuplicateName(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.Create(ctx, "Dup", "", "")
	_, err := svc.Create(ctx, "Dup", "", "")
	if !errors.Is(err, repository.ErrDuplicate) {
		t.Errorf("expected ErrDuplicate, got %v", err)
	}
}

func TestThemeService_GetByID(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "Eng", "", "")
	got, err := svc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch")
	}
}

func TestThemeService_GetByID_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.GetByID(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestThemeService_Update(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "Original", "", "")
	updated, err := svc.Update(ctx, created.ID, "Novo Nome", "nova desc", "#ff0000")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Novo Nome" {
		t.Errorf("Name = %q", updated.Name)
	}
	if updated.Color != "#ff0000" {
		t.Errorf("Color = %q", updated.Color)
	}
}

func TestThemeService_Update_NameRequired(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "Original", "", "")
	_, err := svc.Update(ctx, created.ID, "", "", "")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestThemeService_Delete(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "Para deletar", "", "")
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := svc.GetByID(ctx, created.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestThemeService_List(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.Create(ctx, "B", "", "")
	svc.Create(ctx, "A", "", "")

	themes, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("expected 2, got %d", len(themes))
	}
	if themes[0].Name != "A" {
		t.Errorf("expected sorted, got %q first", themes[0].Name)
	}
}
```

- [ ] **Step 2: Rodar os testes para confirmar que falham**

```bash
go test ./internal/services/... -v
```

Resultado esperado: erro de compilação — pacote `services` não existe.

- [ ] **Step 3: Implementar `internal/services/theme_service.go`**

```go
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type ThemeService struct {
	repo *repository.ThemeRepository
}

func NewThemeService(repo *repository.ThemeRepository) *ThemeService {
	return &ThemeService{repo: repo}
}

func (s *ThemeService) List(ctx context.Context) ([]models.Theme, error) {
	return s.repo.List(ctx)
}

func (s *ThemeService) Create(ctx context.Context, name, description, color string) (*models.Theme, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	if color == "" {
		color = "#6366f1"
	}
	t := &models.Theme{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Color:       color,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *ThemeService) GetByID(ctx context.Context, id string) (*models.Theme, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ThemeService) Update(ctx context.Context, id, name, description, color string) (*models.Theme, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Name = name
	t.Description = description
	if color != "" {
		t.Color = color
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *ThemeService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

var _ = fmt.Sprintf
```

> **Nota:** A linha `var _ = fmt.Sprintf` evita "imported and not used" se `fmt` não for usada. Remova se não precisar — o compilador vai avisar.

Na verdade, `fmt` não é usada diretamente neste arquivo. Remova o import de `fmt` e a linha `var _ = fmt.Sprintf`. O arquivo correto fica:

```go
package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type ThemeService struct {
	repo *repository.ThemeRepository
}

func NewThemeService(repo *repository.ThemeRepository) *ThemeService {
	return &ThemeService{repo: repo}
}

func (s *ThemeService) List(ctx context.Context) ([]models.Theme, error) {
	return s.repo.List(ctx)
}

func (s *ThemeService) Create(ctx context.Context, name, description, color string) (*models.Theme, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	if color == "" {
		color = "#6366f1"
	}
	t := &models.Theme{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Color:       color,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *ThemeService) GetByID(ctx context.Context, id string) (*models.Theme, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ThemeService) Update(ctx context.Context, id, name, description, color string) (*models.Theme, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Name = name
	t.Description = description
	if color != "" {
		t.Color = color
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *ThemeService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: Rodar os testes**

```bash
go test ./internal/services/... -v
```

Resultado esperado: todos os testes PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/services/theme_service.go internal/services/theme_service_test.go
git commit -m "feat: add theme service with validation and UUID generation"
```

---

## Task 3: Respond Helpers + Theme Handler

**Files:**
- Create: `internal/handlers/respond.go`
- Create: `internal/handlers/theme_handler.go`
- Create: `internal/handlers/theme_handler_test.go`

O handler decodifica o JSON do request, chama o serviço, e escreve a resposta. O `respond.go` centraliza o `Content-Type` e o encoding JSON para todos os handlers atuais e futuros.

- [ ] **Step 1: Escrever os testes que vão falhar**

Crie `internal/handlers/theme_handler_test.go`:

```go
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTestThemeHandler(t *testing.T) *handlers.ThemeHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	repo := repository.NewThemeRepository(db)
	svc := services.NewThemeService(repo)
	return handlers.NewThemeHandler(svc)
}

func withChiID(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestThemeHandler_List_Empty(t *testing.T) {
	h := newTestThemeHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/themes", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var result []models.Theme
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("expected empty list, got %d", len(result))
	}
}

func TestThemeHandler_Create(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"name":"Produto","description":"desc","color":"#8b5cf6"}`
	req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var theme models.Theme
	json.NewDecoder(w.Body).Decode(&theme)
	if theme.ID == "" {
		t.Error("ID should be set")
	}
	if theme.Name != "Produto" {
		t.Errorf("Name = %q", theme.Name)
	}
}

func TestThemeHandler_Create_NameRequired(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"description":"sem nome"}`
	req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestThemeHandler_Create_Duplicate(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"name":"Dup"}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.Create(w, req)
		if i == 1 && w.Code != http.StatusConflict {
			t.Errorf("second create status = %d, want 409", w.Code)
		}
	}
}

func TestThemeHandler_GetByID(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"name":"Eng"}`
	reqC := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Theme
	json.NewDecoder(wC.Body).Decode(&created)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/themes/"+created.ID, nil), created.ID)
	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestThemeHandler_GetByID_NotFound(t *testing.T) {
	h := newTestThemeHandler(t)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/themes/nope", nil), "nope")
	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestThemeHandler_Update(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"name":"Original"}`
	reqC := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Theme
	json.NewDecoder(wC.Body).Decode(&created)

	updateBody := `{"name":"Atualizado","color":"#ff0000"}`
	req := withChiID(
		httptest.NewRequest(http.MethodPut, "/api/themes/"+created.ID, bytes.NewBufferString(updateBody)),
		created.ID,
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var updated models.Theme
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Name != "Atualizado" {
		t.Errorf("Name = %q", updated.Name)
	}
}

func TestThemeHandler_Delete(t *testing.T) {
	h := newTestThemeHandler(t)

	body := `{"name":"Para deletar"}`
	reqC := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(body))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Theme
	json.NewDecoder(wC.Body).Decode(&created)

	req := withChiID(
		httptest.NewRequest(http.MethodDelete, "/api/themes/"+created.ID, nil),
		created.ID,
	)
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestThemeHandler_Delete_NotFound(t *testing.T) {
	h := newTestThemeHandler(t)

	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/themes/nope", nil), "nope")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
```

- [ ] **Step 2: Rodar os testes para confirmar que falham**

```bash
go test ./internal/handlers/... -v
```

Resultado esperado: erro de compilação — pacotes `handlers` não existem.

- [ ] **Step 3: Criar `internal/handlers/respond.go`**

```go
package handlers

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
```

- [ ] **Step 4: Criar `internal/handlers/theme_handler.go`**

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type ThemeHandler struct {
	svc *services.ThemeService
}

func NewThemeHandler(svc *services.ThemeService) *ThemeHandler {
	return &ThemeHandler{svc: svc}
}

type createThemeRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

type updateThemeRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

func (h *ThemeHandler) List(w http.ResponseWriter, r *http.Request) {
	themes, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list themes")
		return
	}
	if themes == nil {
		themes = []models.Theme{}
	}
	writeJSON(w, http.StatusOK, themes)
}

func (h *ThemeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createThemeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	theme, err := h.svc.Create(r.Context(), req.Name, req.Description, req.Color)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrDuplicate) {
			writeError(w, http.StatusConflict, "theme name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create theme")
		return
	}
	writeJSON(w, http.StatusCreated, theme)
}

func (h *ThemeHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	theme, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "theme not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get theme")
		return
	}
	writeJSON(w, http.StatusOK, theme)
}

func (h *ThemeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateThemeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	theme, err := h.svc.Update(r.Context(), id, req.Name, req.Description, req.Color)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "theme not found")
			return
		}
		if errors.Is(err, repository.ErrDuplicate) {
			writeError(w, http.StatusConflict, "theme name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update theme")
		return
	}
	writeJSON(w, http.StatusOK, theme)
}

func (h *ThemeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "theme not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete theme")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 5: Rodar os testes**

```bash
go test ./internal/handlers/... -v
```

Resultado esperado: todos os testes PASS.

- [ ] **Step 6: Rodar todos os testes do projeto**

```bash
go test ./... -v
```

Resultado esperado: todos os testes dos três pacotes passam.

- [ ] **Step 7: Commit**

```bash
git add internal/handlers/respond.go internal/handlers/theme_handler.go internal/handlers/theme_handler_test.go
git commit -m "feat: add theme handler with JSON respond helpers"
```

---

## Task 4: Registrar rotas em main.go + validação curl

**Files:**
- Modify: `cmd/api/main.go`

Instanciar o handler e registrar as 5 rotas de temas no roteador chi.

- [ ] **Step 1: Modificar `cmd/api/main.go`**

O arquivo atual tem estas seções relevantes:

```go
// imports atuais:
import (
    "context"
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"

    "meeting-notes/internal/config"
    "meeting-notes/internal/database"
)

// em main(), após registrar o health handler:
r.Get("/health", healthHandler(db))
```

Adicionar os imports de `handlers`, `repository`, `services`:

```go
import (
    "context"
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"

    "meeting-notes/internal/config"
    "meeting-notes/internal/database"
    "meeting-notes/internal/handlers"
    "meeting-notes/internal/repository"
    "meeting-notes/internal/services"
)
```

E adicionar após `r.Get("/health", healthHandler(db))`:

```go
themeHandler := handlers.NewThemeHandler(
    services.NewThemeService(
        repository.NewThemeRepository(db),
    ),
)
r.Route("/api/themes", func(r chi.Router) {
    r.Get("/", themeHandler.List)
    r.Post("/", themeHandler.Create)
    r.Get("/{id}", themeHandler.GetByID)
    r.Put("/{id}", themeHandler.Update)
    r.Delete("/{id}", themeHandler.Delete)
})
```

O `main()` completo após a modificação:

```go
func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	r.Get("/health", healthHandler(db))

	themeHandler := handlers.NewThemeHandler(
		services.NewThemeService(
			repository.NewThemeRepository(db),
		),
	)
	r.Route("/api/themes", func(r chi.Router) {
		r.Get("/", themeHandler.List)
		r.Post("/", themeHandler.Create)
		r.Get("/{id}", themeHandler.GetByID)
		r.Put("/{id}", themeHandler.Update)
		r.Delete("/{id}", themeHandler.Delete)
	})

	log.Printf("server listening on :%s", cfg.HTTPPort)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

- [ ] **Step 2: Compilar**

```bash
go build ./...
```

Resultado esperado: sem output.

- [ ] **Step 3: Rodar o servidor e testar via curl**

Em um terminal:
```bash
go run ./cmd/api
```

Em outro terminal, executar em sequência:

```bash
# Criar tema
curl -s -X POST http://localhost:8080/api/themes \
  -H "Content-Type: application/json" \
  -d '{"name":"Engenharia","description":"Reuniões de eng","color":"#3b82f6"}' | jq .
```
Resultado esperado:
```json
{
  "id": "...",
  "name": "Engenharia",
  "description": "Reuniões de eng",
  "color": "#3b82f6",
  "created_at": "..."
}
```

```bash
# Listar temas
curl -s http://localhost:8080/api/themes | jq .
```
Resultado esperado: array com o tema criado.

```bash
# Buscar por ID (substitua <id> pelo ID retornado acima)
curl -s http://localhost:8080/api/themes/<id> | jq .
```

```bash
# Atualizar
curl -s -X PUT http://localhost:8080/api/themes/<id> \
  -H "Content-Type: application/json" \
  -d '{"name":"Eng Atualizado","color":"#10b981"}' | jq .
```

```bash
# Deletar
curl -s -X DELETE http://localhost:8080/api/themes/<id>
# Resultado esperado: sem body (204)
```

```bash
# Confirmar que foi deletado
curl -s http://localhost:8080/api/themes/<id> | jq .
# Resultado esperado: {"error":"theme not found"}
```

- [ ] **Step 4: Parar o servidor (Ctrl+C) e rodar todos os testes**

```bash
go test ./... -v
```

Resultado esperado: todos os testes dos 4 pacotes passam.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat: register theme routes in HTTP server"
```

---

## Self-Review

**Cobertura do spec (endpoints `/api/themes`):**
- [x] `GET /api/themes` — handler List, Task 3
- [x] `POST /api/themes` — handler Create, Task 3
- [x] `GET /api/themes/{id}` — handler GetByID, Task 3
- [x] `PUT /api/themes/{id}` — handler Update, Task 3
- [x] `DELETE /api/themes/{id}` — handler Delete, Task 3
- [x] Rotas registradas em main.go — Task 4
- [x] Repositório com SQLite real — Task 1
- [x] Serviço com UUID + validação — Task 2
- [x] Testes TDD em todas as camadas — Tasks 1, 2, 3

**HTTP status codes cobertos:**
- [x] 200 OK — GET /api/themes, GET /api/themes/{id}, PUT /api/themes/{id}
- [x] 201 Created — POST /api/themes
- [x] 204 No Content — DELETE /api/themes/{id}
- [x] 400 Bad Request — body JSON inválido
- [x] 404 Not Found — ID inexistente
- [x] 409 Conflict — nome duplicado
- [x] 422 Unprocessable Entity — validação (name vazio)
- [x] 500 Internal Server Error — erros de DB

**Checagem de placeholders:** Nenhum. Todo o código está completo.

**Consistência de tipos entre tasks:**
- `repository.ThemeRepository` → `services.ThemeService` → `handlers.ThemeHandler` ✓
- `repository.ErrNotFound`, `repository.ErrDuplicate` usados consistentemente em handler ✓
- `services.ValidationError` tipado com `errors.As` no handler ✓
- `models.Theme` usada em todas as camadas ✓
- `context.Context` propagado em todo o stack ✓
