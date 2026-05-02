# Loading Screen — Design

## Goal

Substituir o spinner inline atual por uma tela de carregamento completa que aguarda todos os recursos necessários antes de exibir a UI principal, eliminando o problema de primeira inicialização com dados vazios ou recursos indisponíveis.

## Architecture

A lógica de startup permanece em `App.tsx` mas é extraída para um componente dedicado `LoadingScreen`. As três verificações ocorrem sequencialmente após o servidor HTTP responder: primeiro valida o servidor, depois aguarda o modelo de transcrição, depois (condicionalmente) valida a chave da API. Cada verificação tem estado independente (`pending | loading | done | error`).

No backend, é adicionado um endpoint `GET /api/ai/health` que testa a chave Anthropic com uma chamada mínima à API.

## Componentes

### `frontend/src/assets/iconv2.png`
Cópia do ícone raiz (`/iconv2.png`) para dentro do bundle do frontend. Importado diretamente no `LoadingScreen`.

### `frontend/src/components/ui/LoadingScreen.tsx`
Componente full-screen com:
- Ícone da aplicação (`iconv2.png`, 80×80px, border-radius 16px)
- Wordmark "Meeting Notes" abaixo do ícone
- Spinner roxo (`Spinner` existente, tamanho 28)
- Checklist com até 3 itens renderizados sequencialmente

**Props:**
```ts
interface LoadingScreenProps {
  checks: LoadingCheck[]
}

interface LoadingCheck {
  label: string
  status: "pending" | "loading" | "done" | "error"
}
```

**Visual dos estados:**
- `pending`: círculo cinza vazio, texto `#888`, opacidade 35% — só aparece após o item anterior iniciar
- `loading`: spinner roxo dentro do círculo, texto roxo claro
- `done`: círculo verde preenchido com checkmark branco, texto verde
- `error`: círculo vermelho com ✕, texto vermelho — exibe mensagem de erro abaixo do item

### `frontend/src/App.tsx` — ajustes
Substituir o bloco `if (!ready)` atual pelo `<LoadingScreen checks={checks} />`.

Lógica de startup em `useEffect`:
1. `GetPort()` → `initApi(port)`
2. **Check 1 — Servidor HTTP:** poll `GET /health` a cada 500ms (timeout 15s) → quando 200, marca `done`
3. **Check 2 — Modelo de transcrição:** poll `GET /health` verificando `health.model_loaded === true` a cada 2s (timeout 120s, pois o modelo pode demorar ~30–60s na primeira carga)
4. **Check 3 — Chave da API (condicional):** chamar `GET /api/ai/health`; se `configured: false`, não renderiza o item; se `configured: true`, aguarda resposta e marca `done` ou `error`
5. Quando todos os checks ativos chegam a `done`, define `ready = true` e exibe a UI principal

Timeout de cada check gera estado `error` com mensagem específica. App não bloqueia indefinidamente.

### `internal/handlers/ai_health_handler.go` (novo)
`GET /api/ai/health` — responde:
```json
{ "configured": false }
```
Se não houver `anthropic_api_key` nas settings, ou:
```json
{ "configured": true, "valid": true }
{ "configured": true, "valid": false, "error": "invalid api key" }
```

A validação usa o cliente Anthropic existente (`ai.NewDynamicAIClient`) com uma chamada de contagem de tokens (`POST /v1/messages` com `max_tokens: 1` e prompt `"hi"`) — barato, rápido (~200ms), não gera conteúdo útil.

### `cmd/desktop/app.go` e `cmd/api/main.go`
Registrar rota `GET /api/ai/health` em ambos os entry points.

## Data Flow

```
App mounts
  → GetPort()
  → initApi(port)
  → [Check 1] poll /health → 200 OK → done
  → [Check 2] poll /health → model_loaded: true → done
  → [Check 3] GET /api/ai/health
      configured: false → skip (item oculto)
      configured: true, valid: true → done
      configured: true, valid: false → error (exibe aviso, não bloqueia)
  → setReady(true) → render AppInner
```

## Comportamento de erro

- **Servidor não responde em 15s:** estado `error` em Check 1, mensagem "Não foi possível conectar ao servidor. Tente reiniciar o app." Botão "Tentar novamente" reinicia o polling.
- **Modelo não carrega em 120s:** estado `error` em Check 2, mensagem "O modelo de transcrição demorou muito para carregar." App pode prosseguir (botão "Continuar mesmo assim") — gravação funciona, transcrição pode falhar.
- **Chave inválida:** estado `error` em Check 3, mensagem "Chave da API inválida. Verifique nas configurações." App prossegue — funcionalidades de IA ficarão indisponíveis.

## Transição

Quando `ready = true`, a `LoadingScreen` faz fade-out com `opacity: 0` em 300ms (`transition-opacity duration-300`) antes de desmontar, evitando flash brusco para a UI principal.

## Arquivos modificados

| Arquivo | Mudança |
|---|---|
| `frontend/src/assets/iconv2.png` | Copiar ícone do root do projeto |
| `frontend/src/components/ui/LoadingScreen.tsx` | Novo componente |
| `frontend/src/App.tsx` | Substituir bloco `if (!ready)` + expandir lógica de startup |
| `internal/handlers/ai_health_handler.go` | Novo handler |
| `cmd/desktop/app.go` | Registrar rota `/api/ai/health` |
| `cmd/api/main.go` | Registrar rota `/api/ai/health` |
