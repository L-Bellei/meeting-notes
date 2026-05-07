# Loading Screen Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Substituir o spinner inline por uma tela de carregamento completa (logo + spinner + checklist) que valida servidor HTTP, modelo de transcrição e chave de API antes de abrir a UI principal.

**Architecture:** O backend expõe `/health` com `model_loaded` e um novo `GET /api/ai/health` para validação da chave. O frontend exibe `LoadingScreen` enquanto `ready = false`, executando três checks sequenciais no `useEffect` de startup do `App.tsx`. O `LoadingScreen` aceita um array de checks com status e renderiza os estados visualmente.

**Tech Stack:** Go 1.22, chi v5, Anthropic SDK Go, React 19 + TypeScript, Tailwind CSS

---

## File Map

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `internal/ai/validate.go` | Criar | Função `Ping` que testa a chave da API configurada |
| `internal/handlers/ai_health_handler.go` | Criar | Handler para `GET /api/ai/health` |
| `internal/handlers/ai_health_handler_test.go` | Criar | Testes do handler |
| `cmd/desktop/app.go` | Modificar | Incluir `model_loaded` em `/health`; registrar `/api/ai/health` |
| `cmd/api/main.go` | Modificar | Idem |
| `frontend/src/assets/iconv2.png` | Copiar | Ícone da aplicação para bundle do frontend |
| `frontend/src/components/ui/LoadingScreen.tsx` | Criar | Componente visual da tela de carregamento |
| `frontend/src/App.tsx` | Modificar | Lógica de startup com os três checks |

---

## Task 1 — `internal/ai/validate.go` + `GET /api/ai/health`

**Files:**
- Create: `internal/ai/validate.go`
- Create: `internal/handlers/ai_health_handler.go`
- Create: `internal/handlers/ai_health_handler_test.go`
- Modify: `cmd/desktop/app.go` (linha 131 e linha 199)
- Modify: `cmd/api/main.go` (linha correspondente à rota `/api/logs`)

- [ ] **Step 1: Escrever o teste do handler**

Crie `internal/handlers/ai_health_handler_test.go`:

```go
package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"meeting-notes/internal/handlers"
)

func TestAIHealthHandler_NotConfigured(t *testing.T) {
	h := handlers.NewAIHealthHandler(func(_ context.Context) (bool, error) {
		return false, nil
	})
	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequest(http.MethodGet, "/api/ai/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["configured"] != false {
		t.Fatalf("want configured=false, got %v", body["configured"])
	}
}

func TestAIHealthHandler_ValidKey(t *testing.T) {
	h := handlers.NewAIHealthHandler(func(_ context.Context) (bool, error) {
		return true, nil
	})
	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequest(http.MethodGet, "/api/ai/health", nil))
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["configured"] != true || body["valid"] != true {
		t.Fatalf("want configured=true valid=true, got %v", body)
	}
}

func TestAIHealthHandler_InvalidKey(t *testing.T) {
	h := handlers.NewAIHealthHandler(func(_ context.Context) (bool, error) {
		return true, errors.New("authentication_error")
	})
	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequest(http.MethodGet, "/api/ai/health", nil))
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["configured"] != true || body["valid"] != false {
		t.Fatalf("want configured=true valid=false, got %v", body)
	}
	if body["error"] == nil {
		t.Fatal("want error field")
	}
}
```

- [ ] **Step 2: Rodar o teste para confirmar que falha**

```bash
cd F:/dev/meeting-notes
go test ./internal/handlers/ -run TestAIHealth -v
```

Esperado: `FAIL` com "undefined: handlers.NewAIHealthHandler"

- [ ] **Step 3: Criar `internal/ai/validate.go`**

```go
package ai

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Ping verifica se o provedor de IA configurado tem uma chave válida.
// Retorna (false, nil) quando nenhuma chave está configurada.
// Retorna (true, nil) quando a chave é válida.
// Retorna (true, err) quando a chave existe mas a validação falha.
func Ping(ctx context.Context, settings SettingsReader) (configured bool, err error) {
	m, err := settings.GetAll(ctx)
	if err != nil {
		return false, err
	}
	provider := m["ai_provider"]
	switch provider {
	case "anthropic":
		key := m["anthropic_api_key"]
		if key == "" {
			return false, nil
		}
		model := m["anthropic_model"]
		if model == "" {
			model = "claude-haiku-4-5-20251001"
		}
		c := anthropic.NewClient(option.WithAPIKey(key))
		_, pingErr := c.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: 1,
			Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("hi"))},
		})
		return true, pingErr
	case "openai":
		key := m["openai_api_key"]
		if key == "" {
			return false, nil
		}
		return true, nil
	default:
		return false, nil
	}
}
```

- [ ] **Step 4: Criar `internal/handlers/ai_health_handler.go`**

```go
package handlers

import (
	"context"
	"net/http"
)

type AIHealthHandler struct {
	ping func(ctx context.Context) (configured bool, err error)
}

func NewAIHealthHandler(ping func(context.Context) (bool, error)) *AIHealthHandler {
	return &AIHealthHandler{ping: ping}
}

func (h *AIHealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	configured, err := h.ping(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"configured": true,
			"valid":      false,
			"error":      err.Error(),
		})
		return
	}
	if !configured {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "valid": true})
}
```

- [ ] **Step 5: Rodar os testes para confirmar que passam**

```bash
go test ./internal/handlers/ -run TestAIHealth -v
```

Esperado: 3 testes PASS

- [ ] **Step 6: Atualizar `/health` em `cmd/desktop/app.go`**

Localizar a linha 131 (closure do `/health`) e substituir:

```go
// ANTES:
r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    fmt.Fprint(w, `{"status":"ok"}`)
})

// DEPOIS (audioClient já está em escopo na linha 78):
r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    modelLoaded := false
    if h, err := audioClient.Health(r.Context()); err == nil {
        modelLoaded = h.ModelLoaded
    }
    writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "model_loaded": modelLoaded})
})
```

Remover o `fmt.Fprint` — `writeJSON` já escreve o corpo. Verificar se `"fmt"` ainda é usado em outro lugar do arquivo; se não, remover do import.

- [ ] **Step 7: Registrar `/api/ai/health` em `cmd/desktop/app.go`**

Logo após a linha `r.Get("/api/logs", logHandler.List)`:

```go
aiHealthHandler := handlers.NewAIHealthHandler(func(ctx context.Context) (bool, error) {
    return ai.Ping(ctx, settingsRepo)
})
r.Get("/api/ai/health", aiHealthHandler.Check)
```

Verificar que `"meeting-notes/internal/ai"` está nos imports do arquivo (já deve estar).

- [ ] **Step 8: Aplicar as mesmas alterações em `cmd/api/main.go`**

Mesmo padrão: atualizar `/health` para incluir `model_loaded` e registrar `GET /api/ai/health`. Localizar o `/health` em `cmd/api/main.go` e fazer as mesmas substituições dos Steps 6 e 7.

- [ ] **Step 9: Build para verificar compilação**

```bash
cd F:/dev/meeting-notes
go build ./cmd/desktop/... && go build ./cmd/api/...
```

Esperado: sem erros de compilação.

- [ ] **Step 10: Rodar todos os testes Go**

```bash
go test ./internal/...
```

Esperado: todos passam.

- [ ] **Step 11: Commit**

```bash
git add internal/ai/validate.go internal/handlers/ai_health_handler.go internal/handlers/ai_health_handler_test.go cmd/desktop/app.go cmd/api/main.go
git commit -m "feat: add ai.Ping, GET /api/ai/health, model_loaded in /health"
```

---

## Task 2 — `LoadingScreen` component

**Files:**
- Copy: `frontend/src/assets/iconv2.png`
- Create: `frontend/src/components/ui/LoadingScreen.tsx`

- [ ] **Step 1: Copiar o ícone para o bundle do frontend**

```powershell
Copy-Item "F:\dev\meeting-notes\iconv2.png" "F:\dev\meeting-notes\frontend\src\assets\iconv2.png" -Force
```

- [ ] **Step 2: Criar `frontend/src/components/ui/LoadingScreen.tsx`**

```tsx
import iconUrl from "../../assets/iconv2.png"
import { cn } from "../../lib/utils"
import { Spinner } from "./spinner"

export type CheckStatus = "hidden" | "pending" | "loading" | "done" | "error"

export interface LoadingCheck {
  label: string
  status: CheckStatus
  error?: string
}

interface LoadingScreenProps {
  checks: LoadingCheck[]
  fading: boolean
}

export function LoadingScreen({ checks, fading }: LoadingScreenProps) {
  return (
    <div
      className={cn(
        "fixed inset-0 z-50 flex flex-col items-center justify-center gap-7 bg-background transition-opacity duration-300",
        fading ? "opacity-0" : "opacity-100"
      )}
    >
      <div className="flex flex-col items-center gap-3">
        <img src={iconUrl} width={80} height={80} className="rounded-2xl" alt="Meeting Notes" />
        <span className="text-lg font-semibold tracking-tight">Meeting Notes</span>
      </div>

      <Spinner size={28} className="text-primary" />

      <div className="flex flex-col gap-3 min-w-[220px]">
        {checks.filter(c => c.status !== "hidden").map(check => (
          <div key={check.label} className="flex flex-col gap-1">
            <div className="flex items-center gap-2.5">
              <CheckIcon status={check.status} />
              <span className={cn("text-[13px]", labelClass(check.status))}>
                {check.label}
              </span>
            </div>
            {check.status === "error" && check.error && (
              <p className="ml-7 text-[11px] text-destructive">{check.error}</p>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

function CheckIcon({ status }: { status: CheckStatus }) {
  if (status === "done") {
    return (
      <div className="flex h-[18px] w-[18px] flex-shrink-0 items-center justify-center rounded-full bg-green-500">
        <svg width="10" height="10" viewBox="0 0 12 12">
          <polyline points="2,6 5,9 10,3" stroke="white" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </div>
    )
  }
  if (status === "loading") {
    return (
      <div className="flex h-[18px] w-[18px] flex-shrink-0 items-center justify-center rounded-full border-2 border-primary">
        <Spinner size={10} className="text-primary" />
      </div>
    )
  }
  if (status === "error") {
    return (
      <div className="flex h-[18px] w-[18px] flex-shrink-0 items-center justify-center rounded-full bg-destructive">
        <svg width="10" height="10" viewBox="0 0 12 12">
          <line x1="3" y1="3" x2="9" y2="9" stroke="white" strokeWidth="2" strokeLinecap="round" />
          <line x1="9" y1="3" x2="3" y2="9" stroke="white" strokeWidth="2" strokeLinecap="round" />
        </svg>
      </div>
    )
  }
  // pending
  return <div className="h-[18px] w-[18px] flex-shrink-0 rounded-full border-2 border-muted opacity-35" />
}

function labelClass(status: CheckStatus): string {
  switch (status) {
    case "done":    return "text-green-400"
    case "loading": return "text-primary/80"
    case "error":   return "text-destructive"
    default:        return "text-muted-foreground opacity-50"
  }
}
```

- [ ] **Step 3: Verificar tipos com TypeScript**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/assets/iconv2.png frontend/src/components/ui/LoadingScreen.tsx
git commit -m "feat: add LoadingScreen component with checklist UI"
```

---

## Task 3 — Lógica de startup em `App.tsx`

**Files:**
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Ler o arquivo atual**

Ler `frontend/src/App.tsx` completo para ter contexto preciso antes de editar.

- [ ] **Step 2: Substituir os imports e adicionar estado de checks**

No topo do componente `AppInner`, substituir:

```tsx
// ANTES — estado de startup (linhas 25-26):
const [ready, setReady] = useState(false)
const [startupError, setStartupError] = useState(false)
```

por:

```tsx
const [ready, setReady] = useState(false)
const [fadingOut, setFadingOut] = useState(false)
const [showLoading, setShowLoading] = useState(true)
const [checks, setChecks] = useState<import("../ui/LoadingScreen").LoadingCheck[]>([
  { label: "Servidor HTTP",           status: "loading" },
  { label: "Modelo de transcrição",   status: "pending" },
  { label: "Chave da API Anthropic",  status: "hidden"  },
])
```

Adicionar o import no topo do arquivo (junto aos outros imports de componentes):

```tsx
import { LoadingScreen } from "./components/ui/LoadingScreen"
```

- [ ] **Step 3: Substituir o `useEffect` de startup**

Substituir o `useEffect` que faz o polling do `/health` (atual, linhas ~45-56) pelo seguinte:

```tsx
useEffect(() => {
  let cancelled = false

  const upd = (index: number, patch: Partial<import("./components/ui/LoadingScreen").LoadingCheck>) =>
    setChecks(prev => prev.map((c, i) => i === index ? { ...c, ...patch } : c))

  GetPort().then(async port => {
    initApi(port)

    // Check 1 — Servidor HTTP
    const serverDeadline = Date.now() + 15_000
    let serverOk = false
    while (Date.now() < serverDeadline) {
      try {
        const res = await fetch(`http://localhost:${port}/health`)
        if (res.ok) { serverOk = true; break }
      } catch { /* aguarda */ }
      await new Promise(r => setTimeout(r, 500))
    }
    if (cancelled) return
    if (!serverOk) {
      upd(0, { status: "error", error: "Não foi possível conectar ao servidor. Tente reiniciar o app." })
      return
    }
    upd(0, { status: "done" })
    upd(1, { status: "loading" })

    // Check 2 — Modelo de transcrição
    const modelDeadline = Date.now() + 120_000
    let modelOk = false
    while (Date.now() < modelDeadline) {
      try {
        const res = await fetch(`http://localhost:${port}/health`)
        if (res.ok) {
          const h = await res.json() as { model_loaded?: boolean }
          if (h.model_loaded) { modelOk = true; break }
        }
      } catch { /* aguarda */ }
      await new Promise(r => setTimeout(r, 2_000))
    }
    if (cancelled) return
    if (!modelOk) {
      upd(1, { status: "error", error: "O modelo de transcrição demorou muito. Gravações funcionam, mas a transcrição pode falhar." })
      // não bloqueia — prossegue
    } else {
      upd(1, { status: "done" })
    }

    // Check 3 — Chave da API (condicional)
    try {
      const aiHealth = await fetch(`http://localhost:${port}/api/ai/health`).then(r => r.json()) as
        { configured: boolean; valid?: boolean; error?: string }

      if (aiHealth.configured) {
        if (!cancelled) upd(2, { status: "loading" })
        await new Promise(r => setTimeout(r, 0)) // flush render
        if (!cancelled) {
          if (aiHealth.valid) {
            upd(2, { status: "done" })
          } else {
            upd(2, { status: "error", error: aiHealth.error ?? "Chave inválida. Verifique nas configurações." })
          }
        }
      }
      // se !configured, o item permanece "hidden"
    } catch { /* silencioso — não bloqueia */ }

    if (!cancelled) setReady(true)
  })

  return () => { cancelled = true }
}, [])

// fade-out ao ficar pronto
useEffect(() => {
  if (!ready) return
  setFadingOut(true)
  const t = setTimeout(() => setShowLoading(false), 300)
  return () => clearTimeout(t)
}, [ready])
```

- [ ] **Step 4: Substituir o bloco de render da tela de carregamento**

Localizar e remover o bloco:

```tsx
if (!ready) {
  return (
    <div className="flex h-screen items-center justify-center flex-col gap-3 text-muted-foreground text-sm animate-fade-in">
      {startupError ? (
        <p className="text-destructive text-center px-8">
          Não foi possível conectar ao servidor. Tente reiniciar o app.
        </p>
      ) : (
        <>
          <Spinner size={24} className="text-primary" />
          Iniciando...
        </>
      )}
    </div>
  )
}
```

E no `return` principal do componente, antes do `<div className="flex flex-col h-screen...">`, adicionar:

```tsx
return (
  <>
    {showLoading && <LoadingScreen checks={checks} fading={fadingOut} />}
    {ready && (
      <div className="flex flex-col h-screen overflow-hidden bg-background">
        {/* ... conteúdo existente inalterado ... */}
      </div>
    )}
  </>
)
```

O `{ready && ...}` garante que o conteúdo principal só é montado após todos os checks.

- [ ] **Step 5: Verificar TypeScript**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 6: Testar manualmente com `wails dev`**

```bash
cd F:/dev/meeting-notes/cmd/desktop
# Matar audio-service.exe se estiver rodando:
# taskkill /F /IM audio-service.exe
wails dev
```

Verificar:
- Tela de carregamento aparece ao iniciar
- "Servidor HTTP" fica verde rapidamente
- "Modelo de transcrição" fica verde quando o modelo termina de carregar (pode demorar 30–60s na primeira vez)
- Se houver chave da API configurada, "Chave da API Anthropic" aparece e fica verde ou erro
- Se não houver chave, o terceiro item não aparece
- Após todos os checks, a tela faz fade-out e a UI principal aparece

- [ ] **Step 7: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "feat: loading screen startup checks in App.tsx"
```

---

## Verificação Final

```bash
# Go
go test ./internal/...

# TypeScript
cd frontend && npx tsc --noEmit

# Build desktop
cd cmd/desktop && go build ./...
```

Testar manualmente:
- Primeira inicialização: três checks aparecem sequencialmente
- Sem chave de API: apenas dois checks visíveis
- Chave inválida: terceiro check fica vermelho mas app abre normalmente
- Modelo demorando: segundo check fica vermelho com aviso, app abre normalmente
- Após tudo verde: fade-out suave para a tela principal
