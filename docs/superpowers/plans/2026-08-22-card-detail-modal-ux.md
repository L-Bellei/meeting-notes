# CardDetailModal UI/UX — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Corrigir os dez problemas de UI/UX do `CardDetailModal` e o décimo primeiro achado — a descrição de um card de reunião ser uma cópia morta do resumo.

**Architecture:** Três mudanças pequenas no Go (migration idempotente, parar de copiar o resumo na criação do card, e um `has_transcript` no payload do detalhe) e a decomposição do modal de 415 linhas em uma casca mais quatro componentes focados, seguindo o padrão que a aba de temas estabeleceu na v2.6.0. O corpo passa a ter um único scroll.

**Tech Stack:** Go 1.22+, chi v5, modernc/sqlite (sem CGO); React 19 + TypeScript, Tailwind CSS v4, React Query v5.

**Spec:** `docs/superpowers/specs/2026-08-22-card-detail-modal-ux-design.md`

**Branch:** `feat/card-detail-modal-ux` (já criada, a partir de `master` em `5d44982`)

## Global Constraints

- **Sem comentários no código, salvo quando o WHY é não-óbvio.** Convenção do projeto (`CLAUDE.md`).
- **Todo texto de UI em pt-BR.**
- **Não tocar `internal/ai`.**
- **A migration não altera `updated_at`.** Esse campo alimenta o tempo relativo do `KanbanCard`; bumpar faria todo card parecer recém-mexido por uma limpeza de sistema.
- **Não reintroduzir o state local em `toggleTask`/`addTask`/`removeTask`.** Eles enviam `card.description` (o valor persistido). Enviar o state local regride o bug corrigido no PR #46.
- **`line-clamp` com valor dinâmico não funciona no Tailwind** — o JIT não vê classe montada em runtime. Use `style` inline com `WebkitLineClamp`.
- **Frontend sem framework de teste.** Verificação é `npx tsc --noEmit`, `npm run build` e roteiro manual. Adicionar `vitest` está **fora de escopo**.
- **O HMR do vite não chega à janela nativa do `wails dev`.** Antes de qualquer verificação manual, mate o processo (`SingleInstanceLock`) e reinicie o `wails dev`. O `hmr update` no log é o vite emitindo, não o webview aplicando.
- **`master` é protegido** — a integração é por PR.
- Os dois entry points (`cmd/api` e `cmd/desktop`) devem permanecer em sincronia. **Este plano não adiciona rotas**, então nenhum dos dois muda.

---

## Estrutura de arquivos

| Arquivo | Responsabilidade | Task |
|---|---|---|
| `internal/database/migrations/017_card_description_annotations.sql` | Limpa a descrição só onde ainda é igual ao resumo | 1 |
| `internal/database/migration_017_test.go` | Testa o SQL real da migration (pacote `database`, para alcançar `migrationsFS`) | 1 |
| `internal/services/board_card_service.go` | Para de copiar o resumo na criação; preenche `HasTranscript` no detalhe | 1, 2 |
| `internal/repository/meeting_repository.go` | `HasTranscript`, sem carregar o transcript inteiro | 2 |
| `internal/models/models.go` | Campo `HasTranscript` em `BoardCardDetail` | 2 |
| `frontend/src/components/ui/ExpandableText.tsx` | "ver mais": corta em `lines` e expande no lugar | 3 |
| `frontend/src/components/board/CardDetailModal.tsx` | Casca: portal, overlay, `Escape`, focus trap, a11y, barra de cor, um scroll, composição. Detém `editingNotes` e `confirmDelete`. | 4, 5, 6, 7 |
| `frontend/src/components/board/CardModalHeader.tsx` | `#1`, título, tema, select de coluna, excluir, fechar | 5 |
| `frontend/src/components/board/CardTasksSection.tsx` | Tasks de reunião e manuais, estado vazio, gerar tasks | 6 |
| `frontend/src/components/board/CardNotesSection.tsx` | Anotações: leitura, lápis, `textarea`, salvar/cancelar | 7 |
| `frontend/src/hooks/useMeeting.ts` | Optimistic update em `useUpdateTask` | 8 |
| `frontend/src/hooks/useBoard.ts` | Tipo `BoardCardDetail` ganha `has_transcript` | 2 |

---

## Task 1: Migration 017 e parar de copiar o resumo

**Files:**
- Create: `internal/database/migrations/017_card_description_annotations.sql`
- Create: `internal/database/migration_017_test.go`
- Modify: `internal/services/board_card_service.go:61-64`
- Modify: `internal/services/board_card_service_test.go:14-28` (extrair helper que também devolve o `*sql.DB`)

**Interfaces:**
- Consumes: nada de tasks anteriores.
- Produces: `newTestBoardCardServiceWithDB(t *testing.T) (*services.BoardCardService, *sql.DB)` no arquivo de teste, usado pela Task 2.

- [ ] **Step 1: Escrever o teste da migration que falha**

Crie `internal/database/migration_017_test.go`. Ele fica no pacote `database` (não `database_test`) porque precisa alcançar a variável não-exportada `migrationsFS`.

```go
package database

import "testing"

func TestMigration017_ClearsOnlyDescriptionIdenticalToSummary(t *testing.T) {
	db, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	seed := []struct {
		meetingID, cardID, summary, description string
		number                                  int
	}{
		{"m-copy", "c-copy", "resumo alfa", "resumo alfa", 1},
		{"m-edit", "c-edit", "resumo beta", "anotação minha", 2},
	}
	for _, s := range seed {
		if _, err := db.Exec(
			`INSERT INTO meetings (id, title, status) VALUES (?, ?, 'processed')`,
			s.meetingID, s.meetingID); err != nil {
			t.Fatalf("seed meeting %s: %v", s.meetingID, err)
		}
		if _, err := db.Exec(
			`INSERT INTO summaries (id, meeting_id, content, model_used) VALUES (?, ?, ?, 'test')`,
			"s-"+s.meetingID, s.meetingID, s.summary); err != nil {
			t.Fatalf("seed summary %s: %v", s.meetingID, err)
		}
		if _, err := db.Exec(
			`INSERT INTO board_cards
			   (id, meeting_id, column_id, number, position, description, source, updated_at, created_at)
			 VALUES (?, ?, 'col-backlog', ?, ?, ?, 'meeting', '2020-01-01 00:00:00', '2020-01-01 00:00:00')`,
			s.cardID, s.meetingID, s.number, float64(s.number)*1000, s.description); err != nil {
			t.Fatalf("seed card %s: %v", s.cardID, err)
		}
	}

	// No Open a migration roda antes de existir dado algum, então testar o efeito
	// exige reexecutá-la. Ela é um UPDATE idempotente, então isso é legítimo.
	stmt, err := migrationsFS.ReadFile("migrations/017_card_description_annotations.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.Exec(string(stmt)); err != nil {
		t.Fatalf("exec migration: %v", err)
	}

	var got string
	if err := db.QueryRow(`SELECT description FROM board_cards WHERE id = 'c-copy'`).Scan(&got); err != nil {
		t.Fatalf("read c-copy: %v", err)
	}
	if got != "" {
		t.Errorf("descrição idêntica ao resumo deveria ter sido limpa, got %q", got)
	}

	if err := db.QueryRow(`SELECT description FROM board_cards WHERE id = 'c-edit'`).Scan(&got); err != nil {
		t.Fatalf("read c-edit: %v", err)
	}
	if got != "anotação minha" {
		t.Errorf("descrição divergente deveria ser preservada, got %q", got)
	}

	var updatedAt string
	if err := db.QueryRow(`SELECT updated_at FROM board_cards WHERE id = 'c-copy'`).Scan(&updatedAt); err != nil {
		t.Fatalf("read updated_at: %v", err)
	}
	if updatedAt != "2020-01-01 00:00:00" {
		t.Errorf("updated_at não deveria mudar, got %q", updatedAt)
	}
}
```

- [ ] **Step 2: Rodar o teste e confirmar que falha**

Run: `go test ./internal/database/ -run TestMigration017 -v`
Expected: FAIL em `read migration`, com `file does not exist` — o arquivo `.sql` ainda não existe.

- [ ] **Step 3: Criar a migration**

Crie `internal/database/migrations/017_card_description_annotations.sql`:

```sql
-- 017_card_description_annotations.sql
-- A descrição de um card de reunião era uma cópia do resumo, tirada na criação e
-- nunca ressincronizada. Ela passa a ser anotação do usuário, então a cópia sai —
-- mas só onde o texto ainda é idêntico ao resumo, preservando o que foi editado.
-- updated_at não é tocado de propósito: ele alimenta o tempo relativo do card.
UPDATE board_cards
SET description = ''
WHERE source = 'meeting'
  AND meeting_id IS NOT NULL
  AND description <> ''
  AND description = (
    SELECT content FROM summaries WHERE summaries.meeting_id = board_cards.meeting_id
  );
```

- [ ] **Step 4: Rodar o teste e confirmar que passa**

Run: `go test ./internal/database/ -run TestMigration017 -v`
Expected: PASS

- [ ] **Step 5: Extrair o helper de teste do service**

Em `internal/services/board_card_service_test.go`, adicione `"database/sql"` aos imports e substitua o helper existente por dois:

```go
func newTestBoardCardServiceWithDB(t *testing.T) (*services.BoardCardService, *sql.DB) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return services.NewBoardCardService(
		repository.NewBoardCardRepository(db),
		repository.NewBoardColumnRepository(db),
		repository.NewMeetingRepository(db),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db),
	), db
}

func newTestBoardCardService(t *testing.T) *services.BoardCardService {
	t.Helper()
	svc, _ := newTestBoardCardServiceWithDB(t)
	return svc
}
```

Os testes existentes continuam chamando `newTestBoardCardService` e não mudam.

- [ ] **Step 6: Escrever o teste que falha para a criação do card**

Adicione ao mesmo arquivo:

```go
func TestBoardCardService_Create_DoesNotCopySummaryIntoDescription(t *testing.T) {
	svc, db := newTestBoardCardServiceWithDB(t)
	ctx := context.Background()

	if _, err := db.Exec(
		`INSERT INTO meetings (id, title, status) VALUES ('m-1', 'Reunião', 'processed')`); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO summaries (id, meeting_id, content, model_used)
		 VALUES ('s-1', 'm-1', 'conteúdo do resumo', 'test')`); err != nil {
		t.Fatalf("seed summary: %v", err)
	}

	card, err := svc.Create(ctx, "m-1", "col-backlog")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if card.Description != "" {
		t.Errorf("Description = %q, want vazia — o resumo não deve ser copiado", card.Description)
	}
}
```

- [ ] **Step 7: Rodar e confirmar que falha**

Run: `go test ./internal/services/ -run TestBoardCardService_Create_DoesNotCopy -v`
Expected: FAIL com `Description = "conteúdo do resumo", want vazia`

- [ ] **Step 8: Parar de copiar o resumo**

Em `internal/services/board_card_service.go`, substitua as linhas 61-64:

```go
	description := ""
	if sum, err := s.summaryRepo.GetByMeetingID(ctx, meetingID); err == nil {
		description = sum.Content
	}
```

por:

```go
	description := ""
```

`s.summaryRepo` continua em uso por `GetDetail`, então nenhum campo da struct fica órfão.

- [ ] **Step 9: Rodar a suíte inteira**

Run: `go vet ./... && go test ./...`
Expected: tudo `ok`. Se algum teste existente afirmava que a descrição vinha do resumo, ele estava codificando o bug — atualize-o para esperar vazio.

- [ ] **Step 10: Commit**

```bash
git add internal/database/migrations/017_card_description_annotations.sql internal/database/migration_017_test.go internal/services/board_card_service.go internal/services/board_card_service_test.go
git commit -m "feat: card description becomes user annotation, not a summary copy"
```

---

## Task 2: `has_transcript` no detalhe do card

**Files:**
- Modify: `internal/repository/meeting_repository.go` (novo método)
- Modify: `internal/repository/meeting_repository_test.go` (novo teste)
- Modify: `internal/models/models.go:147-167`
- Modify: `internal/services/board_card_service.go:73-101`
- Modify: `internal/services/board_card_service_test.go` (novo teste)
- Modify: `frontend/src/hooks/useBoard.ts:25-45`

**Interfaces:**
- Consumes: `newTestBoardCardServiceWithDB(t) (*services.BoardCardService, *sql.DB)` da Task 1.
- Produces: `MeetingRepository.HasTranscript(ctx context.Context, meetingID string) (bool, error)`; campo `HasTranscript bool` com tag `json:"has_transcript"` em `models.BoardCardDetail`; `has_transcript: boolean` na interface `BoardCardDetail` do frontend, consumido pela Task 6.

- [ ] **Step 1: Escrever o teste de repositório que falha**

Adicione a `internal/repository/meeting_repository_test.go`:

```go
func TestMeetingRepository_HasTranscript(t *testing.T) {
	repo := openMeetingTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	transcript := "texto transcrito"
	empty := ""

	cases := []struct {
		id         string
		transcript *string
		want       bool
	}{
		{"m-com", &transcript, true},
		{"m-vazio", &empty, false},
		{"m-nulo", nil, false},
	}
	for _, c := range cases {
		m := &models.Meeting{ID: c.id, Title: c.id, StartedAt: &now, Status: models.StatusPending, Transcript: c.transcript}
		if err := repo.Create(ctx, m); err != nil {
			t.Fatalf("Create %s: %v", c.id, err)
		}
		got, err := repo.HasTranscript(ctx, c.id)
		if err != nil {
			t.Fatalf("HasTranscript %s: %v", c.id, err)
		}
		if got != c.want {
			t.Errorf("HasTranscript(%s) = %v, want %v", c.id, got, c.want)
		}
	}

	got, err := repo.HasTranscript(ctx, "nao-existe")
	if err != nil {
		t.Fatalf("HasTranscript de id inexistente deveria devolver false sem erro: %v", err)
	}
	if got {
		t.Errorf("HasTranscript(nao-existe) = true, want false")
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `go test ./internal/repository/ -run TestMeetingRepository_HasTranscript -v`
Expected: FAIL na compilação — `repo.HasTranscript undefined`.

- [ ] **Step 3: Implementar `HasTranscript`**

Adicione a `internal/repository/meeting_repository.go`, depois de `GetByID`:

```go
// COUNT em vez de GetByID: só interessa a existência, e o transcript pode ter megabytes.
func (r *MeetingRepository) HasTranscript(ctx context.Context, meetingID string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM meetings WHERE id = ? AND transcript IS NOT NULL AND transcript != ''`,
		meetingID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
```

Se o campo do struct não se chamar `db`, use o nome que os outros métodos do arquivo usam.

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `go test ./internal/repository/ -run TestMeetingRepository_HasTranscript -v`
Expected: PASS

- [ ] **Step 5: Escrever o teste de service que falha**

Adicione a `internal/services/board_card_service_test.go`:

```go
func TestBoardCardService_GetDetail_ReportsHasTranscript(t *testing.T) {
	svc, db := newTestBoardCardServiceWithDB(t)
	ctx := context.Background()

	if _, err := db.Exec(
		`INSERT INTO meetings (id, title, status, transcript)
		 VALUES ('m-com', 'Com transcrição', 'processed', 'texto')`); err != nil {
		t.Fatalf("seed m-com: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO meetings (id, title, status) VALUES ('m-sem', 'Sem transcrição', 'pending')`); err != nil {
		t.Fatalf("seed m-sem: %v", err)
	}

	comCard, err := svc.Create(ctx, "m-com", "col-backlog")
	if err != nil {
		t.Fatalf("Create m-com: %v", err)
	}
	semCard, err := svc.Create(ctx, "m-sem", "col-backlog")
	if err != nil {
		t.Fatalf("Create m-sem: %v", err)
	}

	com, err := svc.GetDetail(ctx, comCard.ID)
	if err != nil {
		t.Fatalf("GetDetail m-com: %v", err)
	}
	if !com.HasTranscript {
		t.Error("HasTranscript = false para reunião com transcrição")
	}

	sem, err := svc.GetDetail(ctx, semCard.ID)
	if err != nil {
		t.Fatalf("GetDetail m-sem: %v", err)
	}
	if sem.HasTranscript {
		t.Error("HasTranscript = true para reunião sem transcrição")
	}
}

func TestBoardCardService_GetDetail_ManualCardHasNoTranscript(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	card, err := svc.CreateManualCard(ctx, "col-backlog", "Card manual", "")
	if err != nil {
		t.Fatalf("CreateManualCard: %v", err)
	}
	detail, err := svc.GetDetail(ctx, card.ID)
	if err != nil {
		t.Fatalf("GetDetail: %v", err)
	}
	if detail.HasTranscript {
		t.Error("card manual não tem reunião, então HasTranscript deve ser false")
	}
}
```

- [ ] **Step 6: Rodar e confirmar que falha**

Run: `go test ./internal/services/ -run TestBoardCardService_GetDetail -v`
Expected: FAIL na compilação — `com.HasTranscript undefined`.

- [ ] **Step 7: Adicionar o campo ao modelo**

Em `internal/models/models.go`, dentro de `BoardCardDetail`, depois de `Tasks`:

```go
	Tasks         []Task `json:"tasks"`
	HasTranscript bool   `json:"has_transcript"`
```

Realinhe as tags do struct como o `gofmt` exigir.

- [ ] **Step 8: Preencher no `GetDetail`**

Em `internal/services/board_card_service.go`, dentro do `if detail.MeetingID != nil` de `GetDetail`, adicione depois do bloco que carrega `tasks`:

```go
		if has, err := s.meetingRepo.HasTranscript(ctx, *detail.MeetingID); err == nil {
			detail.HasTranscript = has
		}
```

Erro tratado como `false` — o mesmo padrão tolerante que `Summary`, `KeyPoints` e `Tasks` já usam neste método. Card manual não entra no bloco, então fica `false` pelo zero-value.

- [ ] **Step 9: Rodar a suíte inteira**

Run: `go vet ./... && go test ./...`
Expected: tudo `ok`

- [ ] **Step 10: Espelhar no tipo do frontend**

Em `frontend/src/hooks/useBoard.ts`, na interface `BoardCardDetail`, depois de `tasks: Task[]`:

```ts
  tasks: Task[]
  has_transcript: boolean
```

- [ ] **Step 11: Verificar o frontend**

Run: `cd frontend && npx tsc --noEmit`
Expected: sem erros. O campo novo é obrigatório mas nenhum código o constrói à mão, então nada quebra.

- [ ] **Step 12: Commit**

```bash
git add internal/repository/meeting_repository.go internal/repository/meeting_repository_test.go internal/models/models.go internal/services/board_card_service.go internal/services/board_card_service_test.go frontend/src/hooks/useBoard.ts
git commit -m "feat: expose has_transcript on the board card detail"
```

---

## Task 3: `ExpandableText`

**Files:**
- Create: `frontend/src/components/ui/ExpandableText.tsx`

**Interfaces:**
- Consumes: `cn` de `../../lib/utils`.
- Produces: `<ExpandableText text={string} lines={number} className?={string} />`, consumido pela Task 4.

- [ ] **Step 1: Criar o componente**

O corte usa `style` inline, **não** classe Tailwind: `lines` é dinâmico e o JIT do Tailwind não gera classe montada em runtime. O botão só aparece quando o texto de fato transborda, medido comparando `scrollHeight` com `clientHeight`.

```tsx
import { useEffect, useRef, useState } from "react"
import { cn } from "../../lib/utils"

interface Props {
  text: string
  lines: number
  className?: string
}

export function ExpandableText({ text, lines, className }: Props) {
  const [expanded, setExpanded] = useState(false)
  const [overflows, setOverflows] = useState(false)
  const ref = useRef<HTMLParagraphElement>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    // Medido no elemento cortado: com o texto expandido scrollHeight == clientHeight
    // e a checagem daria falso negativo, então só medimos enquanto está cortado.
    if (expanded) return
    setOverflows(el.scrollHeight > el.clientHeight + 1)
  }, [text, lines, expanded])

  const clamp = expanded
    ? undefined
    : {
        display: "-webkit-box" as const,
        WebkitBoxOrient: "vertical" as const,
        WebkitLineClamp: lines,
        overflow: "hidden" as const,
      }

  return (
    <div>
      <p
        ref={ref}
        style={clamp}
        className={cn("text-sm text-muted-foreground whitespace-pre-wrap leading-relaxed", className)}
      >
        {text}
      </p>
      {(overflows || expanded) && (
        <button
          onClick={() => setExpanded(v => !v)}
          className="text-xs text-primary hover:underline mt-1"
        >
          {expanded ? "ver menos" : "ver mais"}
        </button>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Verificar**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: sem erros, build conclui.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ui/ExpandableText.tsx
git commit -m "feat: add ExpandableText with a measured ver mais toggle"
```

---

## Task 4: Casca do modal — um scroll, a11y, `Escape` em dois estágios, barra de cor

Esta task trabalha **dentro do arquivo atual**, sem extrair componentes ainda. As tasks 5 a 7 extraem depois. A ordem é essa para que cada task termine em algo verificável no app rodando.

**Files:**
- Modify: `frontend/src/components/board/CardDetailModal.tsx`

**Interfaces:**
- Consumes: `<ExpandableText text lines className? />` da Task 3.
- Produces: no `CardDetailModal`, o estado `editingNotes: boolean` (renomeado de `editing`) e `confirmDelete: boolean`, que as tasks 5 e 7 recebem por props. **Nenhum ref de textarea** — `noUnusedLocals` proíbe declarar aqui o que só seria usado adiante, e o `autoFocus` do `textarea` já resolve o foco.

- [ ] **Step 1: Renomear o estado de edição e adicionar o container do modal**

Renomeie `editing` para `editingNotes` em todas as ocorrências do arquivo (`useState`, `startEditing`, `cancelEditing`, `saveDescription`, e o JSX da seção de descrição).

Substitua o `div` do conteúdo do modal (hoje `className="bg-background border border-border rounded-lg w-[640px] max-h-[80vh] flex flex-col shadow-xl overflow-hidden"`) por:

```tsx
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="card-modal-title"
        className="bg-background border border-border rounded-lg w-[640px] max-w-[calc(100vw-2rem)] max-h-[80vh] flex flex-col shadow-xl overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        <div
          className="h-[3px] flex-shrink-0"
          style={{ background: (!isManual && card?.theme_color) || "#2a2a2a" }}
        />
```

O `#2a2a2a` é o `border` do tema do Tailwind (`tailwind.config.js`), usado como neutro em card manual ou sem tema.

Adicione `id="card-modal-title"` ao `h2` do título no header, para o `aria-labelledby` apontar para algo real.

- [ ] **Step 2: Adicionar `Escape` em dois estágios e focus trap**

Adicione o ref junto aos outros `useState` do componente:

```tsx
  const panelRef = useRef<HTMLDivElement>(null)
```

Só este. `tsconfig.app.json` tem `noUnusedLocals: true`, então declarar aqui um ref que
só é usado numa task posterior **quebra o build desta task**.

E este `useEffect` depois do `useEffect` existente:

```tsx
  useEffect(() => {
    if (!cardId) return
    const previouslyFocused = document.activeElement as HTMLElement | null

    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault()
        // Dois estágios: com a edição aberta, o primeiro Escape cancela a edição
        // em vez de fechar o modal e descartar o texto digitado.
        if (editingNotes) {
          cancelEditing()
          return
        }
        onClose()
        return
      }
      if (e.key !== "Tab") return
      const panel = panelRef.current
      if (!panel) return
      const focusable = panel.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
      )
      if (focusable.length === 0) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }

    window.addEventListener("keydown", onKey)
    panelRef.current?.querySelector<HTMLElement>("button, textarea, select")?.focus()
    return () => {
      window.removeEventListener("keydown", onKey)
      previouslyFocused?.focus()
    }
  }, [cardId, editingNotes, onClose])
```

`cancelEditing` é declarada como `function` dentro do componente, então está no escopo por hoisting.

- [ ] **Step 3: Resetar o estado ao trocar de card**

`BoardView.tsx:109` renderiza `<CardDetailModal cardId={selectedCardId} …>` **sem condição**:
o `return null` acontece dentro do componente, então ele nunca desmonta e o estado sobrevive
entre aberturas. O `useEffect` existente é keyed em `card?.id` e guardado por `if (card)`,
então não roda ao fechar, nem ao reabrir o **mesmo** card.

Isso é um risco pré-existente, não introduzido aqui: abrir o card A, clicar uma vez na
lixeira, fechar, abrir o card B e clicar na lixeira **exclui o B imediatamente**.

Adicione um `useEffect` keyed em `cardId` — que muda também quando vira `null`:

```tsx
  useEffect(() => {
    setEditingNotes(false)
    setConfirmDelete(false)
  }, [cardId])
```

- [ ] **Step 4: Remover todo scroll aninhado e reordenar as seções**

Três lugares perdem o scroll próprio — é a correção do item 1:

1. No `DescriptionView`, no `div` do caminho estruturado: remova `max-h-56 overflow-y-auto pr-1`.
2. No `DescriptionView`, no `p` do caminho de texto puro: remova `max-h-56 overflow-y-auto pr-1`.
3. Na seção "Resumo": troque o `p` inteiro (que tem `max-h-40 overflow-y-auto pr-1`) por `ExpandableText`.

Seção Resumo passa a ser:

```tsx
          {!isManual && card?.summary && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Resumo</h3>
              <ExpandableText text={card.summary.content} lines={6} />
            </section>
          )}
```

Seção Pontos-chave passa a usar `ExpandableText` por item, para que um ponto muito longo não estique o modal:

```tsx
          {!isManual && card && card.key_points.length > 0 && (
            <section>
              <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">Pontos-chave</h3>
              <ul className="space-y-1">
                {card.key_points.map(kp => (
                  <li key={kp.id} className="text-sm text-muted-foreground flex gap-2">
                    <span className="text-primary mt-0.5 flex-shrink-0">·</span>
                    <ExpandableText text={kp.content} lines={8} className="text-sm" />
                  </li>
                ))}
              </ul>
            </section>
          )}
```

Reordene as seções dentro do corpo para: **Tasks (reunião) → Tasks (manuais) → Resumo → Pontos-chave → Descrição → Associar a reunião**. O `div` do corpo mantém `flex-1 overflow-y-auto p-5 space-y-5` — ele é o único scroll que sobra.

Importe `ExpandableText` no topo:

```tsx
import { ExpandableText } from "../ui/ExpandableText"
```

E adicione `useRef` ao import de `react`.

- [ ] **Step 5: Verificar**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: sem erros, build conclui.

- [ ] **Step 6: Verificar na janela nativa**

Mate o `wails dev` (o `SingleInstanceLock` faz um segundo processo sair em silêncio) e reinicie-o a partir de `cmd/desktop`. O HMR do vite não chega à janela nativa — sem reiniciar você testa código velho.

Confira: a roda do mouse percorre o modal inteiro sem travar em nenhuma caixa interna; a barra de cor do tema aparece no topo; `Escape` fecha; com a descrição em edição, o primeiro `Escape` cancela a edição e o segundo fecha; `Tab` circula dentro do modal sem escapar para o board; e ao estreitar a janela o modal encolhe em vez de sangrar.

Confira também o reset: abra um card, clique **uma** vez na lixeira, feche o modal, abra
**outro** card e clique na lixeira — deve pedir confirmação de novo, não excluir.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/board/CardDetailModal.tsx
git commit -m "feat: single scroll, dialog a11y, two-stage Escape and theme color bar"
```

---

## Task 5: `CardModalHeader` — hierarquia, select de coluna, confirm de exclusão

**Files:**
- Create: `frontend/src/components/board/CardModalHeader.tsx`
- Modify: `frontend/src/components/board/CardDetailModal.tsx`

**Interfaces:**
- Consumes: `BoardCardDetail` de `../../hooks/useBoard`; `useColumns` de `../../hooks/useBoardColumns`; `useCards` e `useMoveCard` de `../../hooks/useBoard`; `confirmDelete` e os callbacks da casca.
- Produces: `<CardModalHeader card confirmDelete onDelete onClose />`.

- [ ] **Step 1: Criar o componente**

```tsx
import { X, Pencil, Trash2 } from "lucide-react"
import { Button } from "../ui/button"
import { useCards, useMoveCard, type BoardCardDetail } from "../../hooks/useBoard"
import { useColumns } from "../../hooks/useBoardColumns"
import { cn } from "../../lib/utils"

interface Props {
  card: BoardCardDetail
  confirmDelete: boolean
  onDelete: () => void
  onClose: () => void
}

export function CardModalHeader({ card, confirmDelete, onDelete, onClose }: Props) {
  const { data: columns = [] } = useColumns()
  // Sem filtro de propósito: com os filtros ativos do board a carta que define o
  // maior position pode estar filtrada para fora, e o card cairia no meio da coluna.
  const { data: cards = [] } = useCards()
  const moveCard = useMoveCard()

  function handleMove(columnID: string) {
    if (columnID === card.column_id) return
    const inTarget = cards.filter(c => c.column_id === columnID)
    const position = inTarget.length === 0
      ? 1000
      : Math.max(...inTarget.map(c => c.position)) + 1000
    moveCard.mutate({ id: card.id, column_id: columnID, position })
  }

  const isManual = card.source === "manual"

  return (
    <div className="flex items-center gap-3 px-5 py-4 border-b border-border flex-shrink-0">
      <span className="text-xs text-muted-foreground flex-shrink-0">#{card.number}</span>
      {isManual && <Pencil size={11} className="text-muted-foreground/60 flex-shrink-0" />}

      <h2 id="card-modal-title" className="text-base font-semibold flex-1 truncate" title={card.meeting_title}>
        {card.meeting_title}
      </h2>

      {!isManual && card.theme_name && (
        <span className="text-xs text-muted-foreground hidden sm:inline flex-shrink-0">
          {card.theme_name}
        </span>
      )}

      <select
        value={card.column_id}
        onChange={e => handleMove(e.target.value)}
        disabled={moveCard.isPending}
        aria-label="Mover para outra coluna"
        className="text-xs rounded-lg px-2 py-1 bg-input border border-border text-foreground focus:outline-none focus:ring-1 focus:ring-primary flex-shrink-0"
      >
        {columns.map(col => (
          <option key={col.id} value={col.id}>{col.name}</option>
        ))}
      </select>

      <button
        onClick={onDelete}
        aria-label={confirmDelete ? "Confirmar exclusão do card" : "Excluir card"}
        className={cn(
          "flex items-center gap-1 p-1 rounded transition-colors flex-shrink-0",
          confirmDelete
            ? "text-destructive bg-destructive/20"
            : "text-muted-foreground hover:text-destructive hover:bg-destructive/10",
        )}
      >
        <Trash2 size={14} />
        {confirmDelete && <span className="text-xs">Confirmar?</span>}
      </button>

      <Button variant="ghost" size="icon" onClick={onClose} aria-label="Fechar"><X size={16} /></Button>
    </div>
  )
}
```

O nome do tema usa `hidden sm:inline`: em janela estreita ele recolhe e a barra de cor no topo continua sinalizando o tema.

- [ ] **Step 2: Fazer o `confirmDelete` resetar**

Em `CardDetailModal.tsx`, substitua `handleDelete` por:

```tsx
  function handleDelete() {
    if (!confirmDelete) {
      setConfirmDelete(true)
      return
    }
    if (!cardId) return
    deleteCard.mutate(cardId, { onSuccess: onClose })
  }
```

e adicione este `useEffect`, que é a correção do item 3 — hoje `confirmDelete` nunca volta atrás:

```tsx
  useEffect(() => {
    if (!confirmDelete) return
    const timer = setTimeout(() => setConfirmDelete(false), 4000)
    return () => clearTimeout(timer)
  }, [confirmDelete])
```

O reset ao trocar ou fechar o card já foi resolvido no Step 3 da Task 4 — o modal **não**
desmonta, então ele depende daquele `useEffect` keyed em `cardId`. Não conte com desmontagem.

- [ ] **Step 3: Substituir o header inline pelo componente**

Remova todo o bloco `{/* Header */}` do `CardDetailModal.tsx` e coloque no lugar:

```tsx
        {isLoading && (
          <div className="px-5 py-4 border-b border-border flex-shrink-0">
            <span className="text-xs text-muted-foreground">Carregando...</span>
          </div>
        )}
        {card && (
          <CardModalHeader
            card={card}
            confirmDelete={confirmDelete}
            onDelete={handleDelete}
            onClose={onClose}
          />
        )}
```

Importe `CardModalHeader` e remova de `CardDetailModal.tsx` os imports que ficaram sem uso (`X`, `Trash2`, e `Pencil` se nenhuma outra parte do arquivo o usar) — `npx tsc --noEmit` acusa cada um.

- [ ] **Step 4: Verificar**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: sem erros, build conclui.

- [ ] **Step 5: Verificar na janela nativa**

Reinicie o `wails dev` (mate o processo primeiro). Confira: o título é o elemento mais proeminente do header e trunca com reticências num título longo, mostrando o texto completo no tooltip; nenhum badge colorido sobrou; o select mostra a coluna atual e mover leva o card para o **fim** da coluna escolhida no board; o primeiro clique na lixeira mostra "Confirmar?" e ele desaparece sozinho depois de ~4s; o segundo clique exclui.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/board/CardModalHeader.tsx frontend/src/components/board/CardDetailModal.tsx
git commit -m "feat: card modal header with column select and resetting delete confirm"
```

---

## Task 6: `CardTasksSection` — estado vazio, gerar tasks, prioridade e responsável

**Files:**
- Create: `frontend/src/components/board/CardTasksSection.tsx`
- Modify: `frontend/src/components/board/CardDetailModal.tsx`

**Interfaces:**
- Consumes: `has_transcript` da Task 2; `useUpdateTask` e `useGenerateTasks` de `../../hooks/useMeeting`; `BoardCardDetail` de `../../hooks/useBoard`.
- Produces: `<CardTasksSection card manualTasks onToggleManual onAddManual onRemoveManual />`.

- [ ] **Step 1: Criar o componente**

```tsx
import { useState } from "react"
import { Plus, Trash2 } from "lucide-react"
import { Button } from "../ui/button"
import { useUpdateTask, useGenerateTasks, type Task } from "../../hooks/useMeeting"
import type { BoardCardDetail } from "../../hooks/useBoard"
import { cn } from "../../lib/utils"

const PRIORITY_LABEL: Record<string, string> = { high: "alta", medium: "média", low: "baixa" }

interface Props {
  card: BoardCardDetail
  manualTasks: string[]
  onToggleManual: (index: number) => void
  onAddManual: (text: string) => void
  onRemoveManual: (index: number) => void
}

function parseManualTask(s: string): { text: string; done: boolean } {
  if (s.startsWith("[x] ")) return { text: s.slice(4), done: true }
  if (s.startsWith("[ ] ")) return { text: s.slice(4), done: false }
  return { text: s, done: false }
}

function TaskRow({ task, meetingId }: { task: Task; meetingId: string }) {
  const updateTask = useUpdateTask(meetingId, task.id)
  return (
    <div>
      <label className="flex items-start gap-2 cursor-pointer">
        <input
          type="checkbox"
          className="mt-0.5 accent-primary flex-shrink-0"
          checked={task.completed}
          onChange={e => updateTask.mutate({ ...task, completed: e.target.checked })}
        />
        <span className={cn("text-sm flex-1", task.completed && "line-through text-muted-foreground")}>
          {task.description}
        </span>
        {task.priority && (
          <span className={cn(
            "text-[10px] font-medium px-1 rounded mt-0.5 flex-shrink-0",
            task.priority === "high" ? "bg-destructive/15 text-destructive" :
            task.priority === "medium" ? "bg-yellow-500/15 text-yellow-600" :
            "bg-muted text-muted-foreground",
          )}>
            {PRIORITY_LABEL[task.priority] ?? task.priority}
          </span>
        )}
        {task.assignee && (
          <span className="text-[10px] text-muted-foreground/70 mt-0.5 flex-shrink-0">{task.assignee}</span>
        )}
      </label>
      {updateTask.isError && (
        <p className="text-xs text-destructive ml-6 mt-0.5">
          Falha ao salvar: {updateTask.error?.message ?? "erro desconhecido"}
        </p>
      )}
    </div>
  )
}

export function CardTasksSection({ card, manualTasks, onToggleManual, onAddManual, onRemoveManual }: Props) {
  const [newTask, setNewTask] = useState("")
  const generateTasks = useGenerateTasks(card.meeting_id ?? "")
  const isManual = card.source === "manual"

  if (isManual) {
    const done = manualTasks.filter(t => parseManualTask(t).done).length
    return (
      <section>
        <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">
          Tasks {manualTasks.length > 0 && `(${done}/${manualTasks.length})`}
        </h3>
        <div className="space-y-1.5 mb-2">
          {manualTasks.map((raw, i) => {
            const { text, done } = parseManualTask(raw)
            return (
              <div key={i} className="flex items-center gap-2 group">
                <input
                  type="checkbox"
                  className="accent-primary flex-shrink-0"
                  checked={done}
                  onChange={() => onToggleManual(i)}
                />
                <span className={cn("text-sm flex-1", done && "line-through text-muted-foreground")}>{text}</span>
                <button
                  onClick={() => onRemoveManual(i)}
                  aria-label="Remover task"
                  className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-all"
                >
                  <Trash2 size={13} />
                </button>
              </div>
            )
          })}
        </div>
        <div className="flex gap-2">
          <input
            className="flex-1 text-sm rounded-lg px-3 py-1.5 bg-input border border-border text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-primary"
            placeholder="Nova task..."
            value={newTask}
            onChange={e => setNewTask(e.target.value)}
            onKeyDown={e => {
              if (e.key !== "Enter" || !newTask.trim()) return
              onAddManual(newTask.trim())
              setNewTask("")
            }}
          />
          <Button
            size="sm"
            variant="ghost"
            disabled={!newTask.trim()}
            onClick={() => { onAddManual(newTask.trim()); setNewTask("") }}
          >
            <Plus size={14} />
          </Button>
        </div>
      </section>
    )
  }

  const done = card.tasks.filter(t => t.completed).length
  return (
    <section>
      <h3 className="text-xs font-medium text-muted-foreground uppercase mb-2">
        Tasks {card.tasks.length > 0 && `(${done}/${card.tasks.length})`}
      </h3>
      {card.tasks.length === 0 ? (
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground/70">Nenhuma task nesta reunião.</p>
          {card.has_transcript ? (
            <Button
              size="sm"
              variant="outline"
              disabled={generateTasks.isPending}
              onClick={() => generateTasks.mutate()}
            >
              {generateTasks.isPending ? "Gerando..." : "Gerar tasks"}
            </Button>
          ) : (
            <p className="text-xs text-muted-foreground/60">
              Gerar tasks precisa da transcrição da reunião.
            </p>
          )}
          {generateTasks.isError && (
            <p className="text-xs text-destructive">
              Falha ao gerar: {generateTasks.error?.message ?? "erro desconhecido"}
            </p>
          )}
        </div>
      ) : (
        <div className="space-y-1.5">
          {card.tasks.map(task => (
            <TaskRow key={task.id} task={task} meetingId={card.meeting_id ?? ""} />
          ))}
        </div>
      )}
    </section>
  )
}
```

- [ ] **Step 2: Consumir na casca**

Em `CardDetailModal.tsx`, remova as duas seções de tasks (a de cards manuais e a de reunião), o `TaskRow` do fim do arquivo, os helpers `parseManualTask`/`encodeManualTask` **não** — `encodeManualTask` continua sendo usado pelos handlers da casca. Mantenha os dois helpers na casca e deixe a cópia de `parseManualTask` no componente novo.

Substitua as seções por:

```tsx
          {card && (
            <CardTasksSection
              card={card}
              manualTasks={manualTasks}
              onToggleManual={toggleTask}
              onAddManual={addTask}
              onRemoveManual={removeTask}
            />
          )}
```

Ajuste `addTask` para receber o texto, em vez de ler o state que agora vive no componente filho:

```tsx
  function addTask(text: string) {
    if (!cardId || !card || !text) return
    const updated = [...manualTasks, encodeManualTask(text, false)]
    updateCard.mutate({ id: cardId, description: card.description, tasks: updated })
  }
```

`toggleTask` e `removeTask` **não mudam**: continuam enviando `card.description`, o valor persistido. Enviar o state local traria de volta o bug do PR #46.

Remova o `newTask`/`setNewTask` da casca e os imports que sobraram sem uso.

- [ ] **Step 3: Verificar**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: sem erros, build conclui.

- [ ] **Step 4: Verificar na janela nativa**

Reinicie o `wails dev`. Confira: marcar e desmarcar uma task de reunião persiste e o contador do card no board acompanha; prioridade e responsável aparecem quando existem; num card manual, adicionar por `Enter` e pelo botão funciona, e remover funciona; e — o teste que importa para o item 4 — **edite a descrição sem salvar, marque um checkbox, e confirme que a descrição não mudou**.

Para ver o estado vazio, use uma reunião sem tasks. Se todas tiverem tasks, apague-as por `DELETE /api/meetings/{id}/tasks/{taskId}` numa reunião de teste; com transcrição, o botão "Gerar tasks" aparece; sem, aparece a explicação.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/board/CardTasksSection.tsx frontend/src/components/board/CardDetailModal.tsx
git commit -m "feat: tasks section with empty state, generate action, priority and assignee"
```

---

## Task 7: `CardNotesSection` — anotações com lápis explícito

**Files:**
- Create: `frontend/src/components/board/CardNotesSection.tsx`
- Modify: `frontend/src/components/board/CardDetailModal.tsx`

**Interfaces:**
- Consumes: `editingNotes` e os callbacks da casca (Task 4).
- Produces: `<CardNotesSection value editing pending onChange onStartEditing onSave onCancel />`.

- [ ] **Step 1: Criar o componente**

```tsx
import { Pencil } from "lucide-react"
import { Button } from "../ui/button"

interface Props {
  value: string
  editing: boolean
  pending: boolean
  onChange: (v: string) => void
  onStartEditing: () => void
  onSave: () => void
  onCancel: () => void
}

export function CardNotesSection({
  value, editing, pending, onChange, onStartEditing, onSave, onCancel,
}: Props) {
  return (
    <section>
      <div className="flex items-center gap-2 mb-2">
        <h3 className="text-xs font-medium text-muted-foreground uppercase">Suas anotações</h3>
        {!editing && (
          <button
            onClick={onStartEditing}
            aria-label="Editar anotações"
            className="text-muted-foreground/60 hover:text-foreground transition-colors"
          >
            <Pencil size={12} />
          </button>
        )}
      </div>

      {editing ? (
        <div className="space-y-2">
          <textarea
            className="w-full text-sm bg-input border border-border rounded px-3 py-2 h-40 resize-none"
            value={value}
            onChange={e => onChange(e.target.value)}
            autoFocus
          />
          <div className="flex gap-2">
            <Button size="sm" onClick={onSave} disabled={pending}>
              {pending ? "Salvando..." : "Salvar"}
            </Button>
            <Button variant="ghost" size="sm" onClick={onCancel}>Cancelar</Button>
          </div>
        </div>
      ) : value ? (
        <p className="text-sm text-muted-foreground whitespace-pre-wrap leading-relaxed">{value}</p>
      ) : (
        <p className="text-sm italic text-muted-foreground/50">Nada anotado ainda</p>
      )}
    </section>
  )
}
```

- [ ] **Step 2: Consumir na casca e apagar o renderizador de JSON**

Em `CardDetailModal.tsx`:

Apague `DescriptionView`, `tryParseStructured` e a interface `StructuredDescription`. Com a descrição virando anotação escrita pelo usuário, não há mais JSON para interpretar nesse campo — o Resumo é texto puro e já usa `ExpandableText`.

Substitua a seção de descrição por:

```tsx
          <CardNotesSection
            value={description}
            editing={editingNotes}
            pending={updateCard.isPending}
            onChange={setDescription}
            onStartEditing={startEditing}
            onSave={saveDescription}
            onCancel={cancelEditing}
          />
```

Mova essa seção para **depois** de Pontos-chave, conforme a ordem da Task 4.

Remova os imports que sobraram sem uso (`Pencil` na casca, se nada mais o usar).

- [ ] **Step 3: Verificar**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: sem erros, build conclui.

- [ ] **Step 4: Verificar na janela nativa**

Reinicie o `wails dev`. Confira: num card de reunião a seção mostra "Suas anotações" e "Nada anotado ainda" (a migration limpou a cópia do resumo); clicar no **texto** não entra em edição, só o lápis; escrever, salvar e reabrir o modal mantém o texto; `Cancelar` descarta; e o primeiro `Escape` durante a edição cancela sem fechar o modal.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/board/CardNotesSection.tsx frontend/src/components/board/CardDetailModal.tsx
git commit -m "feat: notes section with an explicit pencil affordance"
```

---

## Task 8: Optimistic update no checkbox de task

**Files:**
- Modify: `frontend/src/hooks/useMeeting.ts:117-128`
- Modify: `frontend/src/components/board/CardTasksSection.tsx`

**Interfaces:**
- Consumes: `useUpdateTask(meetingId, taskId)` como está hoje.
- Produces: nada para tasks posteriores.

- [ ] **Step 1: Adicionar optimistic update**

Substitua `useUpdateTask` em `frontend/src/hooks/useMeeting.ts` por:

```ts
export function useUpdateTask(meetingId: string, taskId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Partial<Task>) =>
      api<Task>(`/api/meetings/${meetingId}/tasks/${taskId}`, { method: "PUT", body: JSON.stringify(data) }),
    onMutate: async (data) => {
      await qc.cancelQueries({ queryKey: ["board-card"] })
      const snapshots = qc.getQueriesData<{ tasks: Task[] }>({ queryKey: ["board-card"] })
      for (const [key, value] of snapshots) {
        if (!value?.tasks) continue
        qc.setQueryData(key, {
          ...value,
          tasks: value.tasks.map(t => (t.id === taskId ? { ...t, ...data } : t)),
        })
      }
      return { snapshots }
    },
    onError: (_err, _data, context) => {
      for (const [key, value] of context?.snapshots ?? []) {
        qc.setQueryData(key, value)
      }
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ["meeting", meetingId] })
      qc.invalidateQueries({ queryKey: ["board-card"] })
      qc.invalidateQueries({ queryKey: ["board-cards"] })
    },
  })
}
```

`onSettled` roda no sucesso **e** no erro, então as invalidações que hoje estão em `onSuccess`
cobrem os dois casos e o cache sempre reconverge com o servidor.

O tipo do snapshot é `{ tasks: Task[] }`, e não `BoardCardDetail`, de propósito: `useBoard.ts`
já importa `Task` daqui, então importar `BoardCardDetail` de lá fecharia um ciclo. Em runtime
o spread preserva o objeto inteiro — só a visão de tipo é estreita.

- [ ] **Step 2: Remover o indicador de pendência**

Em `CardTasksSection.tsx`, no `TaskRow`, não há indicador de pendência a remover se você seguiu a Task 6 — ela já foi escrita sem ele. Confirme que o `TaskRow` **não** renderiza `salvando...`: com optimistic update, `isPending` fica verdadeiro no mesmo instante do clique e o rótulo apareceria em toda marcação, ao lado de um checkbox que já mudou de estado.

A mensagem de erro **fica**. É ela que impede a falha de voltar a ser silenciosa — o modo exato em que o bug do PR #45 se esconde.

- [ ] **Step 3: Verificar**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: sem erros, build conclui.

- [ ] **Step 4: Verificar na janela nativa**

Reinicie o `wails dev`. Confira: o checkbox muda de estado **instantaneamente**, sem esperar a requisição; o contador `(n/12)` acompanha na hora; e o contador do card no Kanban atualiza logo depois.

Para conferir o rollback, pare o processo do audio-service não ajuda — a rota é do Go. Em vez disso, renomeie momentaneamente `taskId` na URL do hook para algo inexistente, marque uma task, confirme que o checkbox volta ao estado anterior **e** que a mensagem de erro aparece, e desfaça a alteração.

- [ ] **Step 5: Rodar a verificação completa e commitar**

```bash
cd frontend && npx tsc --noEmit && npm run build && cd .. && go vet ./... && go test ./...
git add frontend/src/hooks/useMeeting.ts frontend/src/components/board/CardTasksSection.tsx
git commit -m "feat: optimistic update on the task checkbox"
```

---

## Task 9: Documentação de continuidade

**Files:**
- Modify: `.claude/STATE.md`
- Modify: `.claude/BACKLOG.md`
- Modify: `.claude/CHANGELOG.md`
- Modify: `.claude/DECISIONS.md`

- [ ] **Step 1: Atualizar o BACKLOG**

Remova a entrada "UI/UX do CardDetailModal" de "Features futuras" — ela foi implementada. Adicione a "Débitos técnicos":

```markdown
- **Primitivo `Modal` compartilhado** — `CardDetailModal` ganhou `Escape`, `role="dialog"`, `aria-modal` e focus trap em 2026-08-22, mas os outros cinco modais (`SearchModal`, `SettingsModal`, `RecordingModal`, `CreateManualCardModal`, `ThemeEditModal`) seguem cada um com seu `Escape` e **nenhum** com `role="dialog"` nem focus trap. Extrair um componente `Modal` e migrar os seis foi deixado fora de escopo por ser risco desproporcional sem teste de render.
```

- [ ] **Step 2: Registrar a decisão**

Adicione a `.claude/DECISIONS.md`, com a data de 2026-08-22:

```markdown
## Descrição de card é anotação do usuário, não cópia do resumo

`BoardCardService.Create` copiava `summary.Content` para a descrição do card. A cópia
nunca ressincronizava, e o modal renderizava a cópia (DESCRIÇÃO) e a fonte viva (RESUMO)
uma embaixo da outra — byte-a-byte idênticas no card medido. A descrição passa a ser
anotação do usuário, vazia por padrão; o Resumo é a fonte da IA. A migration 017 limpa
**apenas** as descrições ainda idênticas ao resumo, preservando o que foi editado.
```

- [ ] **Step 3: Atualizar STATE e CHANGELOG**

No `STATE.md`, descreva o estado de `master` — não a branch do momento. As duas últimas sessões produziram um `STATE.md` desatualizado no instante do merge por prender-se à branch em curso.

No `CHANGELOG.md`, entrada nova no topo, depois do `---`, cobrindo: os dez itens de UI/UX, o 11º achado com a medição (`description` e `summary` idênticas, 1867 caracteres), a migration 017, a decomposição em cinco arquivos, e o `has_transcript`.

- [ ] **Step 4: Commit**

```bash
git add .claude/STATE.md .claude/BACKLOG.md .claude/CHANGELOG.md .claude/DECISIONS.md
git commit -m "docs: record the CardDetailModal UX work and the description decision"
```
