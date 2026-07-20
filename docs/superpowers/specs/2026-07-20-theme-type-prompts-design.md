# Prompts personalizados por tipo de geração — Design

**Data:** 2026-07-20
**Tipo:** Feature (revisita a decisão de 2026-04-29 "customPrompt campo único", que foi um YAGNI deliberado)

## Problema

Hoje `Theme.CustomPrompt` (campo único) é enviado igual para os três geradores de IA (resumo, pontos-chave, tarefas). Não há como personalizar a instrução de um tipo sem afetar os outros. Ex.: querer um resumo em bullet points mas manter tarefas no formato padrão é impossível.

## Contexto atual

- `internal/models` → `Theme.CustomPrompt string` (json `custom_prompt`).
- AI clients (`internal/ai/{anthropic,openai,dynamic}_client.go`): `GenerateSummary/GenerateKeyPoints/GenerateTasks(ctx, transcript, notes, customPrompt)`; `buildInstruction(default, customPrompt)` retorna `customPrompt` se não-vazio, senão a instrução padrão embutida.
- Services (`SummaryService/KeyPointService/TaskService`): `Generate(ctx, meeting, customPrompt)`.
- 4 call sites resolvem o prompt do tema:
  - `internal/handlers/summary_handler.go`, `key_point_handler.go`, `task_handler.go`
  - `internal/services/orchestrator.go` (`maybeGenerate`, ~linhas 302-316) — os três de uma vez.
- Frontend: `ThemeEditModal.tsx` tem um textarea "Prompt personalizado"; tipo `Theme` em `useThemes.ts`.
- Última migration: `014_meeting_language.sql` → próxima é `015`.
- App single-user; dois entry points (`cmd/api`, `cmd/desktop`) compartilham services/repository.

## Estrutura escolhida

**Geral + 3 overrides por tipo.** Precedência ao gerar cada tipo: **específico → geral (`custom_prompt`) → default embutido**. Zero perda de dados; temas existentes continuam funcionando via fallback ao geral. (Alternativas descartadas: só 3 campos aposentando o geral — perderia a conveniência de "um prompt para tudo"; descartar o atual — perda de dados.)

## Design

### 1. Dados + migration

- Migration **`015_theme_type_prompts.sql`**: adiciona a `themes` três colunas `TEXT NOT NULL DEFAULT ''`:
  - `custom_summary_prompt`, `custom_key_points_prompt`, `custom_tasks_prompt`.
- `custom_prompt` permanece (o "geral"). Novas colunas nascem `''` → fallback ao geral; nenhuma migração de dados necessária.
- `Theme` struct ganha `CustomSummaryPrompt`, `CustomKeyPointsPrompt`, `CustomTasksPrompt` (`string`, json `custom_summary_prompt` etc.).

### 2. Resolução de precedência (no model)

Centralizada num helper no `Theme`, para os 4 call sites não duplicarem a lógica:

```go
type PromptKind int

const (
    PromptSummary PromptKind = iota
    PromptKeyPoints
    PromptTasks
)

func (t *Theme) PromptFor(kind PromptKind) string {
    var specific string
    switch kind {
    case PromptSummary:
        specific = t.CustomSummaryPrompt
    case PromptKeyPoints:
        specific = t.CustomKeyPointsPrompt
    case PromptTasks:
        specific = t.CustomTasksPrompt
    }
    if specific != "" {
        return specific
    }
    return t.CustomPrompt
}
```

O último degrau (*resolvido → default embutido*) já é coberto por `buildInstruction` nos AI clients. **Assinaturas de AI client e services permanecem inalteradas** (continuam recebendo um único `customPrompt string`).

### 3. Call sites

Trocar `theme.CustomPrompt` por `theme.PromptFor(<kind>)`:
- `summary_handler.go` → `PromptFor(models.PromptSummary)`
- `key_point_handler.go` → `PromptFor(models.PromptKeyPoints)`
- `task_handler.go` → `PromptFor(models.PromptTasks)`
- `orchestrator.go` (`maybeGenerate`): cada `Generate` recebe o `PromptFor` do seu tipo (resolve o tema uma vez, chama `PromptFor` por tipo).

### 4. Repository

`theme_repository.go`: incluir as 3 colunas em SELECT (todas as queries que montam `Theme`), INSERT, UPDATE e no scan.

### 5. Frontend

- `ThemeEditModal.tsx`: manter o textarea atual rotulado como **"Prompt geral"**, e adicionar 3 textareas abaixo — **Resumo**, **Pontos-chave**, **Tarefas** — com hint: *"vazio → usa o prompt geral; geral vazio → usa o padrão"*.
- `useThemes.ts`: tipo `Theme` + payloads de create/update ganham `custom_summary_prompt`, `custom_key_points_prompt`, `custom_tasks_prompt`.
- Estado do `ThemeEditModal` (useState) para os 3 novos campos, populado a partir do tema em edição.

## Testes

**Go:**
- `Theme.PromptFor` (unit): específico vence quando presente; cai no geral quando específico vazio; retorna `""` quando ambos vazios (para o AI client cair no default). Um caso por `PromptKind`.
- `theme_repository`: round-trip das 3 colunas (SQLite via `t.TempDir()`), incluindo valores vazios (default).
- Orchestrator (opcional, se barato): via `fakeAI` capturando o `customPrompt` recebido por tipo, confirmar que um tema com `custom_summary_prompt` setado e `custom_tasks_prompt` vazio faz o summary usar o específico e as tasks caírem no geral.

**Frontend:** sem test runner no projeto → cobertura por `tsc --noEmit` + `npm run build`.

## Fora de escopo

- Templating/variáveis no prompt.
- Prompt por reunião (fica no tema).
- Renomear/reordenar o campo geral no schema.
