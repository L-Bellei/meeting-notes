# Fatia 1 — Fundação Go: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Criar a fundação Go do projeto: módulo, config, database com migrations automáticas, models de domínio e endpoint `/health` funcional.

**Architecture:** Servidor HTTP Go com chi router, SQLite via driver puro (sem CGO), configuração via `.env`, migrations SQL aplicadas no startup. Estrutura `internal/` para encapsulamento real dos pacotes.

**Tech Stack:** Go 1.22+, chi v5, modernc.org/sqlite, godotenv, google/uuid

**Success Criteria:** `go run ./cmd/api` sobe na porta 8080, SQLite criado com schema completo, `curl http://localhost:8080/health` retorna `{"status":"ok","database":"ok"}`.

---

## File Map

| Arquivo | Responsabilidade |
|---|---|
| `go.mod` / `go.sum` | Definição do módulo e dependências travadas |
| `migrations/001_initial.sql` | Schema SQL completo (todas as tabelas + índices) |
| `internal/config/config.go` | Struct `Config` + `Load()` que lê `.env` e env vars |
| `internal/models/models.go` | Structs de domínio: Theme, Meeting, Summary, KeyPoint, Task |
| `internal/database/database.go` | `Open()` que abre SQLite e aplica migrations |
| `internal/database/database_test.go` | Testes para Open() e migration |
| `cmd/api/main.go` | Entry point: carrega config, abre DB, registra rotas, sobe servidor |
| `.env.example` | Template de variáveis de ambiente |
| `.env` | Arquivo local (gitignored) com valores reais |
| `.gitignore` | Ignora `.env`, binários, `*.db` |

---

## Task 1: Inicializar módulo Go e estrutura de pastas

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `.env.example`
- Create: `.env`

- [ ] **Step 1: Criar o módulo Go**

No terminal, dentro de `F:\dev\meeting-notes`:

```bash
go mod init meeting-notes
```

Resultado esperado: arquivo `go.mod` criado com `module meeting-notes` e `go 1.22` (ou superior).

- [ ] **Step 2: Adicionar dependências**

```bash
go get github.com/go-chi/chi/v5@latest
go get github.com/go-chi/cors@latest
go get github.com/google/uuid@latest
go get github.com/joho/godotenv@latest
go get modernc.org/sqlite@latest
```

Cada comando atualiza `go.mod` e `go.sum`.

- [ ] **Step 3: Criar estrutura de pastas**

```bash
mkdir -p cmd/api internal/config internal/database internal/handlers internal/models internal/repository internal/services internal/anthropic internal/audio migrations audio-service frontend
```

- [ ] **Step 4: Criar `.gitignore`**

Conteúdo final:

```gitignore
.env
*.db
*.db-shm
*.db-wal
meeting-notes.exe
meeting-notes
/audio-service/.venv/
__pycache__/
*.pyc
```

- [ ] **Step 5: Criar `.env.example`**

```env
HTTP_PORT=8080
DATABASE_PATH=./meeting-notes.db
ANTHROPIC_API_KEY=sk-ant-xxx
ANTHROPIC_MODEL=claude-sonnet-4-6
AUDIO_SERVICE_URL=http://localhost:8765
MAX_TOKENS=4096
WHISPER_MODEL=medium
WHISPER_DEVICE=cuda
WHISPER_COMPUTE_TYPE=int8_float16
```

- [ ] **Step 6: Criar `.env` local**

```env
HTTP_PORT=8080
DATABASE_PATH=./meeting-notes.db
ANTHROPIC_API_KEY=sk-ant-xxx
ANTHROPIC_MODEL=claude-sonnet-4-6
AUDIO_SERVICE_URL=http://localhost:8765
MAX_TOKENS=4096
WHISPER_MODEL=medium
WHISPER_DEVICE=cuda
WHISPER_COMPUTE_TYPE=int8_float16
```

- [ ] **Step 7: Verificar que o módulo compila vazio**

```bash
go build ./...
```

Resultado esperado: sem output (nenhum pacote ainda, mas o comando não falha).

- [ ] **Step 8: Commit**

```bash
git init
git add go.mod go.sum .gitignore .env.example
git commit -m "chore: initialize Go module with dependencies"
```

---

## Task 2: Config module

**Files:**
- Create: `internal/config/config.go`

O módulo `config` lê variáveis de ambiente (com fallback para `.env`) e expõe uma struct tipada. Sem testes nesta tarefa: as variáveis de ambiente dependem do ambiente — testar aqui seria testar o sistema operacional, não o nosso código. A integração fica coberta pelo teste de database na Task 4.

- [ ] **Step 1: Criar `internal/config/config.go`**

```go
package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort        string
	DatabasePath    string
	AnthropicAPIKey string
	AnthropicModel  string
	AudioServiceURL string
	MaxTokens       string
	WhisperModel    string
	WhisperDevice   string
	WhisperComputeType string
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		HTTPPort:        getEnv("HTTP_PORT", "8080"),
		DatabasePath:    getEnv("DATABASE_PATH", "./meeting-notes.db"),
		AnthropicAPIKey: getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicModel:  getEnv("ANTHROPIC_MODEL", "claude-sonnet-4-6"),
		AudioServiceURL: getEnv("AUDIO_SERVICE_URL", "http://localhost:8765"),
		MaxTokens:       getEnv("MAX_TOKENS", "4096"),
		WhisperModel:    getEnv("WHISPER_MODEL", "medium"),
		WhisperDevice:   getEnv("WHISPER_DEVICE", "cuda"),
		WhisperComputeType: getEnv("WHISPER_COMPUTE_TYPE", "int8_float16"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

> **Nota Go:** `godotenv.Load()` retorna erro se `.env` não existe — o `_` descarta esse erro intencionalmente. Em produção não haverá `.env`, só variáveis de ambiente reais. O `getEnv` garante o fallback.

- [ ] **Step 2: Verificar que compila**

```bash
go build ./internal/config/...
```

Resultado esperado: sem output.

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add config module with env loading"
```

---

## Task 3: Models de domínio

**Files:**
- Create: `internal/models/models.go`

As structs representam as entidades do banco. Sem lógica de negócio aqui — só tipos e tags JSON.

- [ ] **Step 1: Criar `internal/models/models.go`**

```go
package models

import "time"

type Theme struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	CreatedAt   time.Time `json:"created_at"`
}

type MeetingStatus string

const (
	StatusPending      MeetingStatus = "pending"
	StatusRecording    MeetingStatus = "recording"
	StatusTranscribing MeetingStatus = "transcribing"
	StatusProcessing   MeetingStatus = "processing"
	StatusCompleted    MeetingStatus = "completed"
	StatusFailed       MeetingStatus = "failed"
)

type Meeting struct {
	ID              string        `json:"id"`
	ThemeID         *string       `json:"theme_id"`
	Title           string        `json:"title"`
	StartedAt       *time.Time    `json:"started_at"`
	DurationSeconds *int          `json:"duration_seconds"`
	Status          MeetingStatus `json:"status"`
	Transcript      *string       `json:"transcript"`
	CreatedAt       time.Time     `json:"created_at"`
}

type Summary struct {
	ID           string    `json:"id"`
	MeetingID    string    `json:"meeting_id"`
	Content      string    `json:"content"`
	ModelUsed    string    `json:"model_used"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CreatedAt    time.Time `json:"created_at"`
}

type KeyPoint struct {
	ID        string `json:"id"`
	MeetingID string `json:"meeting_id"`
	Position  int    `json:"position"`
	Content   string `json:"content"`
}

type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

type Task struct {
	ID          string       `json:"id"`
	MeetingID   string       `json:"meeting_id"`
	Description string       `json:"description"`
	Assignee    *string      `json:"assignee"`
	DueDate     *time.Time   `json:"due_date"`
	Priority    TaskPriority `json:"priority"`
	Completed   bool         `json:"completed"`
	CreatedAt   time.Time    `json:"created_at"`
}
```

> **Nota Go:** Ponteiros (`*string`, `*time.Time`) representam campos NULLable do banco — distinção entre "ausente" e "string vazia". É o equivalente ao `Nullable<T>` do .NET.

- [ ] **Step 2: Verificar que compila**

```bash
go build ./internal/models/...
```

Resultado esperado: sem output.

- [ ] **Step 3: Commit**

```bash
git add internal/models/models.go
git commit -m "feat: add domain models"
```

---

## Task 4: Migration SQL

**Files:**
- Create: `migrations/001_initial.sql`

- [ ] **Step 1: Criar `migrations/001_initial.sql`**

```sql
CREATE TABLE IF NOT EXISTS themes (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    color       TEXT NOT NULL DEFAULT '#6366f1',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS meetings (
    id               TEXT PRIMARY KEY,
    theme_id         TEXT REFERENCES themes(id) ON DELETE SET NULL,
    title            TEXT NOT NULL,
    started_at       DATETIME,
    duration_seconds INTEGER,
    status           TEXT NOT NULL DEFAULT 'pending',
    transcript       TEXT,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS summaries (
    id            TEXT PRIMARY KEY,
    meeting_id    TEXT NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    content       TEXT NOT NULL,
    model_used    TEXT NOT NULL,
    input_tokens  INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS key_points (
    id         TEXT PRIMARY KEY,
    meeting_id TEXT NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    position   INTEGER NOT NULL,
    content    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    meeting_id  TEXT NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    assignee    TEXT,
    due_date    DATETIME,
    priority    TEXT NOT NULL DEFAULT 'medium',
    completed   INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_meetings_theme     ON meetings(theme_id);
CREATE INDEX IF NOT EXISTS idx_meetings_status    ON meetings(status);
CREATE INDEX IF NOT EXISTS idx_summaries_meeting  ON summaries(meeting_id);
CREATE INDEX IF NOT EXISTS idx_key_points_meeting ON key_points(meeting_id);
CREATE INDEX IF NOT EXISTS idx_tasks_meeting      ON tasks(meeting_id);
CREATE INDEX IF NOT EXISTS idx_tasks_completed    ON tasks(completed);
```

- [ ] **Step 2: Commit**

```bash
git add migrations/001_initial.sql
git commit -m "feat: add initial database migration"
```

---

## Task 5: Database module

**Files:**
- Create: `internal/database/database.go`
- Create: `internal/database/database_test.go`

O módulo abre a conexão SQLite e aplica todas as migrations em `migrations/` ordenadas por nome. Usa `embed` do Go para incluir os arquivos SQL no binário.

- [ ] **Step 1: Escrever o teste que ainda vai falhar**

Crie `internal/database/database_test.go`:

```go
package database_test

import (
	"os"
	"testing"

	"meeting-notes/internal/database"
)

func TestOpen_CreatesTablesOnStartup(t *testing.T) {
	path := t.TempDir() + "/test.db"

	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	tables := []string{"themes", "meetings", "summaries", "key_points", "tasks"}
	for _, table := range tables {
		var name string
		row := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table)
		if err := row.Scan(&name); err != nil {
			t.Errorf("table %q not found after migration: %v", table, err)
		}
	}
}

func TestOpen_IsIdempotent(t *testing.T) {
	path := t.TempDir() + "/test.db"

	db1, err := database.Open(path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	db1.Close()

	db2, err := database.Open(path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	db2.Close()
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
```

- [ ] **Step 2: Rodar o teste para confirmar que falha**

```bash
go test ./internal/database/... -v
```

Resultado esperado: erro de compilação `package meeting-notes/internal/database: cannot find package` — o pacote ainda não existe.

- [ ] **Step 3: Implementar `internal/database/database.go`**

```go
package database

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}
```

> **Nota Go:** `//go:embed migrations/*.sql` é uma diretiva do compilador. O `fs` embutido (`embed.FS`) faz os arquivos `.sql` entrarem no binário compilado — sem precisar distribuir arquivos separados.

**Atenção:** O `embed.FS` embute arquivos relativos ao arquivo `.go` que declara a diretiva. Por isso as migrations precisam estar em `internal/database/migrations/`, não na raiz `migrations/`.

- [ ] **Step 4: Copiar migrations para o local correto**

```bash
mkdir -p internal/database/migrations
cp migrations/001_initial.sql internal/database/migrations/001_initial.sql
```

> As migrations existem em dois lugares: `migrations/` (fonte canônica, versionada, usada por ferramentas de migração e documentação) e `internal/database/migrations/` (cópia embutida no binário Go). Em projetos maiores, um script de build sincroniza os dois — por ora, copiamos manualmente.

- [ ] **Step 5: Rodar os testes**

```bash
go test ./internal/database/... -v
```

Resultado esperado:
```
=== RUN   TestOpen_CreatesTablesOnStartup
--- PASS: TestOpen_CreatesTablesOnStartup (0.XXs)
=== RUN   TestOpen_IsIdempotent
--- PASS: TestOpen_IsIdempotent (0.XXs)
PASS
ok      meeting-notes/internal/database
```

- [ ] **Step 6: Commit**

```bash
git add internal/database/database.go internal/database/database_test.go internal/database/migrations/001_initial.sql
git commit -m "feat: add database module with embedded migrations"
```

---

## Task 6: Health handler e main.go

**Files:**
- Create: `cmd/api/main.go`

O `main.go` orquestra tudo: carrega config, abre o banco, registra rotas e sobe o servidor HTTP. O handler `/health` verifica que o banco responde.

- [ ] **Step 1: Criar `cmd/api/main.go`**

```go
package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"meeting-notes/internal/config"
	"meeting-notes/internal/database"
)

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

	log.Printf("server listening on :%s", cfg.HTTPPort)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		if err := db.Ping(); err != nil {
			dbStatus = "error"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"database": dbStatus,
		})
	}
}
```

> **Nota Go:** `chi.NewRouter()` é o equivalente ao `app.UseRouting()` do ASP.NET. `middleware.Logger` loga cada request. `http.HandlerFunc` é uma função que implementa a interface `http.Handler` — retornar uma closure aqui permite injetar dependências (o `db`) sem um struct.

- [ ] **Step 2: Compilar**

```bash
go build ./cmd/api/...
```

Resultado esperado: sem output (sem erros).

- [ ] **Step 3: Rodar o servidor**

```bash
go run ./cmd/api
```

Resultado esperado no terminal:
```
2026/04/27 XX:XX:XX server listening on :8080
```

- [ ] **Step 4: Testar o endpoint em outro terminal**

```bash
curl -s http://localhost:8080/health
```

Resultado esperado:
```json
{"database":"ok","status":"ok"}
```

- [ ] **Step 5: Parar o servidor (Ctrl+C) e confirmar que o arquivo `.db` foi criado**

```bash
ls -la meeting-notes.db
```

Resultado esperado: arquivo SQLite presente com alguns KB.

- [ ] **Step 6: Verificar o schema no banco**

```bash
go run ./cmd/api &
sleep 1
curl -s http://localhost:8080/health
```

Ou usar uma ferramenta SQLite (DB Browser for SQLite, DBeaver) para abrir `meeting-notes.db` e confirmar as 5 tabelas e 6 índices.

- [ ] **Step 7: Commit final**

```bash
git add cmd/api/main.go
git commit -m "feat: add HTTP server with health endpoint"
```

---

## Self-Review

**Cobertura do spec:**
- [x] `go.mod` com todas as dependências listadas — Task 1
- [x] `internal/config` com todas as vars de ambiente do `.env.example` — Task 2
- [x] `internal/models` com todas as entidades do modelo de dados — Task 3
- [x] `migrations/001_initial.sql` com todas as tabelas e índices — Task 4
- [x] `internal/database` com Open() + migrations automáticas — Task 5
- [x] `cmd/api/main.go` com `/health` — Task 6
- [x] Critério de pronto: `curl /health` retorna `{"status":"ok","database":"ok"}` — Task 6 Step 4

**Gaps identificados:**
- O spec menciona `go run ./cmd/api` sobe na porta 8080 ✓
- O spec menciona banco SQLite criado com schema ✓ (verificado no Step 5-6 da Task 6)
- Nenhuma funcionalidade de Fatia 2+ foi incluída — conforme esperado

**Checagem de placeholders:** Nenhum. Todos os steps têm código completo e comandos com saída esperada.

**Consistência de tipos:** `database.Open()` retorna `*sql.DB` — usado corretamente em `main.go` e nos testes. `config.Load()` retorna `*Config` — usado corretamente em `main.go`.
