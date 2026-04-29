# Meeting Notes

Aplicativo desktop para Windows que grava reuniões, transcreve o áudio automaticamente e gera resumos, pontos-chave e tarefas usando inteligência artificial.

---

## Visão geral

Meeting Notes captura o áudio do sistema ou microfone durante uma reunião, transcreve a fala via Whisper e passa o conteúdo para um modelo de linguagem (Claude ou GPT-4) que produz:

- **Resumo** em 2–3 parágrafos
- **Pontos-chave** extraídos da conversa
- **Tarefas** com responsável e prioridade

Tudo fica armazenado localmente em SQLite. Nenhum dado é enviado a servidores externos além das chamadas de API aos provedores de IA configurados.

---

## Funcionalidades

| Recurso | Descrição |
|---|---|
| Gravação de áudio | Captura loopback do sistema ou entrada de microfone |
| Transcrição Whisper | Modelos `tiny` → `large`, suporte a PT / EN / ES / auto |
| Geração por IA | Resumo, pontos-chave e tarefas via Anthropic ou OpenAI |
| Auto-geração | Dispara os três itens automaticamente ao parar a gravação |
| Temas | Categorize reuniões em temas com cor e descrição |
| Notas em Markdown | Editor com preview e toolbar de formatação |
| Configurações | Provedor de IA, chaves de API, modelo, idioma e Whisper — tudo via modal, sem editar `.env` |
| Transcrição manual | Cole ou edite transcrições sem precisar gravar |
| Reprocessar | Regenere resumo/pontos/tarefas de uma reunião já existente |
| Persistência total | SQLite local em `%AppData%\Meeting Notes\` |

---

## Arquitetura

```
┌─────────────────────────────────────┐
│         Wails Desktop App           │
│  React + TypeScript (frontend)      │
│  Go HTTP server (backend)           │
└────────────┬────────────────────────┘
             │ HTTP (localhost:porta aleatória)
             │
      ┌──────▼──────┐        ┌──────────────────┐
      │  Go API     │        │  Python Service  │
      │  chi router │◄──────►│  FastAPI         │
      │  SQLite     │        │  Whisper (GPU)   │
      │  AI clients │        │  PyAudioWPatch   │
      └─────────────┘        └──────────────────┘
```

### Backend Go

Estruturado em camadas:

```
cmd/
  desktop/       → Wails app (router embutido, porta aleatória)
  api/           → Servidor HTTP standalone (porta 8080)

internal/
  config/        → Carregamento de variáveis de ambiente e .env
  database/      → Conexão SQLite + runner de migrations
  models/        → Structs de domínio
  repository/    → Acesso ao banco (CRUD por entidade)
  services/      → Regras de negócio (Orchestrator, SummaryService, …)
  handlers/      → Handlers HTTP (chi)
  ai/            → DynamicAIClient → AnthropicClient / OpenAIClient
  audio/         → Cliente HTTP para o serviço Python
```

### Pipeline de processamento

Ao parar uma gravação o `Orchestrator` executa em goroutine:

```
StopRecording (audio service)
  → Transcribe (Whisper)
  → [se auto_generate = true] GenerateSummary + GenerateKeyPoints + GenerateTasks
  → status = completed
```

Notificações de status são emitidas via eventos Wails em tempo real para o frontend.

### Frontend React

```
src/
  hooks/        → useApi, useMeetings, useMeeting, useSettings, usePipeline, …
  components/
    layout/     → Toolbar, Sidebar, MeetingList, MeetingDetail
    recording/  → RecordingModal
    settings/   → SettingsModal
    ui/         → Button, Spinner, componentes reutilizáveis
```

Gerenciamento de estado via **React Query v5** — todas as chamadas REST são queries/mutations com invalidação automática de cache.

---

## Stack de tecnologias

### Go (backend)

| Biblioteca | Versão | Uso |
|---|---|---|
| [Wails v2](https://wails.io) | v2 | Framework desktop (webview + Go) |
| [chi](https://github.com/go-chi/chi) | v5 | Router HTTP |
| [anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) | v1.38 | Cliente Anthropic Claude |
| [openai-go](https://github.com/openai/openai-go) | v1.12 | Cliente OpenAI GPT |
| [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) | v1.50 | SQLite puro Go (sem CGO) |
| [godotenv](https://github.com/joho/godotenv) | — | Leitura de arquivos `.env` |
| [google/uuid](https://github.com/google/uuid) | — | Geração de IDs |
| [go-chi/cors](https://github.com/go-chi/cors) | — | Middleware CORS |

### React (frontend)

| Biblioteca | Versão | Uso |
|---|---|---|
| React | 19 | UI framework |
| TypeScript | 6 | Tipagem estática |
| Vite | 8 | Build tool |
| [@tanstack/react-query](https://tanstack.com/query) | v5 | Server state / cache |
| [Tailwind CSS](https://tailwindcss.com) | v4 | Estilização |
| [lucide-react](https://lucide.dev) | — | Ícones |
| [react-markdown](https://github.com/remarkjs/react-markdown) | — | Renderização Markdown |
| [remark-gfm](https://github.com/remarkjs/remark-gfm) | — | GitHub Flavored Markdown |

### Python (serviço de áudio)

| Biblioteca | Versão | Uso |
|---|---|---|
| [FastAPI](https://fastapi.tiangolo.com) | 0.115 | API REST do serviço de áudio |
| [faster-whisper](https://github.com/SYSTRAN/faster-whisper) | 1.0.3 | Transcrição (CTranslate2 + Whisper) |
| [PyAudioWPatch](https://github.com/s0d3s/PyAudioWPatch) | 0.2.12 | Captura loopback WASAPI (Windows) |
| [uvicorn](https://www.uvicorn.org) | 0.32 | Servidor ASGI |
| nvidia-cudnn-cu12 / cublas | — | Aceleração CUDA para Whisper |

---

## Banco de dados

SQLite em `%AppData%\Meeting Notes\meeting-notes.db`. Migrations aplicadas automaticamente na inicialização.

```
themes          id, name, description, color, parent_id, created_at
meetings        id, theme_id, title, started_at, duration_seconds,
                status, transcript, notes, created_at
summaries       id, meeting_id, content, model_used, input_tokens,
                output_tokens, created_at
key_points      id, meeting_id, position, content
tasks           id, meeting_id, description, assignee, due_date,
                priority, completed, created_at
settings        key TEXT PRIMARY KEY, value TEXT
```

**Settings padrão**

| Chave | Padrão | Descrição |
|---|---|---|
| `user_name` | `""` | Nome exibido nas reuniões |
| `ai_provider` | `anthropic` | Provedor ativo (`anthropic` ou `openai`) |
| `anthropic_api_key` | `""` | Chave de API Anthropic |
| `anthropic_model` | `claude-sonnet-4-6` | Modelo Claude |
| `openai_api_key` | `""` | Chave de API OpenAI |
| `openai_model` | `gpt-4o` | Modelo OpenAI |
| `auto_generate` | `true` | Gerar automaticamente após gravar |
| `whisper_language` | `pt` | Idioma de transcrição |
| `whisper_model` | `medium` | Tamanho do modelo Whisper |

---

## API REST

### Health

| Método | Rota | Descrição |
|---|---|---|
| GET | `/health` | Status do servidor e banco de dados |

### Temas

| Método | Rota | Descrição |
|---|---|---|
| GET | `/api/themes` | Listar temas |
| POST | `/api/themes` | Criar tema |
| GET | `/api/themes/{id}` | Buscar tema |
| PUT | `/api/themes/{id}` | Atualizar tema |
| DELETE | `/api/themes/{id}` | Remover tema |

### Reuniões

| Método | Rota | Descrição |
|---|---|---|
| GET | `/api/meetings` | Listar reuniões (filtro por `?theme_id=`) |
| POST | `/api/meetings` | Criar reunião |
| GET | `/api/meetings/{id}` | Buscar reunião com detalhes |
| PUT | `/api/meetings/{id}` | Atualizar reunião |
| DELETE | `/api/meetings/{id}` | Remover reunião |
| POST | `/api/meetings/{id}/start` | Iniciar gravação |
| POST | `/api/meetings/{id}/stop` | Parar gravação e iniciar pipeline |
| POST | `/api/meetings/{id}/process` | Reprocessar com IA |
| POST | `/api/meetings/{id}/transcript` | Definir transcrição manualmente |

### Resumo / Pontos-chave / Tarefas

Padrão idêntico para `/summary`, `/key_points` e `/tasks`:

```
GET    /api/meetings/{id}/summary
POST   /api/meetings/{id}/summary
PUT    /api/meetings/{id}/summary
DELETE /api/meetings/{id}/summary
POST   /api/meetings/{id}/summary/generate   ← chama a IA
```

### Configurações

| Método | Rota | Descrição |
|---|---|---|
| GET | `/api/settings` | Retornar todas as configurações |
| PUT | `/api/settings` | Atualizar configurações (parcial) |

---

## Pré-requisitos

### Para executar o instalador

- Windows 10 / 11 (64-bit)
- GPU NVIDIA com drivers CUDA ≥ 12.4 (recomendado para Whisper; funciona em CPU mas é mais lento)

### Para desenvolvimento

- [Go](https://go.dev) 1.22+
- [Node.js](https://nodejs.org) 20+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2
- [Python](https://python.org) 3.11+
- [NSIS](https://nsis.sourceforge.io) (para gerar instalador)

---

## Configuração do ambiente de desenvolvimento

### 1. Clonar o repositório

```bash
git clone <url-do-repo>
cd meeting-notes
```

### 2. Variáveis de ambiente

Crie um arquivo `.env` na raiz do projeto:

```env
ANTHROPIC_API_KEY=sk-ant-api03-...
ANTHROPIC_MODEL=claude-sonnet-4-6
AUDIO_SERVICE_URL=http://localhost:8765
HTTP_PORT=8080
WHISPER_LANGUAGE=pt
WHISPER_MODEL=medium
WHISPER_DEVICE=cuda
WHISPER_COMPUTE_TYPE=int8_float16
```

> Em produção (instalador), o `.env` fica em `%AppData%\Meeting Notes\.env`.

### 3. Serviço de áudio (Python)

```bash
cd audio-service
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
uvicorn main:app --port 8765
```

### 4. Executar em modo de desenvolvimento

```bash
# Na raiz do projeto
wails dev
```

O Wails inicia o servidor Go e o frontend Vite simultaneamente com hot-reload.

### 5. Executar apenas o servidor API (sem Wails)

```bash
go run ./cmd/api/...
```

---

## Build e instalador

```powershell
# Adicionar NSIS ao PATH (se necessário)
$env:PATH += ";C:\Program Files (x86)\NSIS"

# Build com versão específica
.\build.ps1 -Version "0.2.0"

# Build pulando os testes
.\build.ps1 -Version "0.2.0" -SkipTests

# Build sem NSIS (apenas o .exe portátil)
.\build.ps1 -Version "0.2.0" -NoNSIS
```

O artefato é gerado em `dist/meeting-notes-{VERSION}-windows-amd64-installer.exe`.

---

## Testes

```bash
# Go — todos os pacotes
go test ./...

# Frontend — checar TypeScript
cd frontend && npm run build
```

---

## Configurações da aplicação

As configurações podem ser alteradas pela interface gráfica (**ícone de engrenagem** na toolbar) ou diretamente no banco de dados.

**Provedores de IA suportados:**
- **Anthropic**: `claude-sonnet-4-6`, `claude-opus-4-7`, `claude-haiku-4-5`
- **OpenAI**: `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`

**Modelos Whisper disponíveis:**

| Modelo | Velocidade | Precisão |
|---|---|---|
| `tiny` | ⚡⚡⚡⚡⚡ | ★☆☆☆☆ |
| `base` | ⚡⚡⚡⚡ | ★★☆☆☆ |
| `small` | ⚡⚡⚡ | ★★★☆☆ |
| `medium` | ⚡⚡ | ★★★★☆ |
| `large` | ⚡ | ★★★★★ |

> Trocar o modelo Whisper requer reiniciar o serviço de áudio.

---

## Estrutura do projeto

```
meeting-notes/
├── audio-service/           # Serviço Python (gravação + Whisper)
│   ├── main.py              # FastAPI app
│   ├── recorder.py          # Captura de áudio (WASAPI loopback)
│   ├── transcriber.py       # Wrapper faster-whisper
│   └── requirements.txt
├── cmd/
│   ├── api/main.go          # Servidor HTTP standalone
│   └── desktop/
│       ├── app.go           # Ciclo de vida Wails + roteamento
│       └── wails.json       # Configuração do build Wails
├── frontend/
│   └── src/
│       ├── App.tsx
│       ├── hooks/           # useApi, useSettings, useMeetings, usePipeline, …
│       └── components/
│           ├── layout/      # Toolbar, Sidebar, MeetingList, MeetingDetail
│           ├── recording/   # RecordingModal
│           ├── settings/    # SettingsModal
│           └── ui/          # Componentes reutilizáveis
├── internal/
│   ├── ai/                  # DynamicAIClient, AnthropicClient, OpenAIClient
│   ├── audio/               # Cliente HTTP para o serviço Python
│   ├── config/              # Config + carregamento de .env
│   ├── database/            # Conexão SQLite + migrations (001–005)
│   ├── handlers/            # Handlers HTTP (meetings, themes, settings, …)
│   ├── models/              # Meeting, Theme, Summary, KeyPoint, Task
│   ├── repository/          # Camada de acesso ao banco
│   └── services/            # Orchestrator, MeetingService, SettingsService, …
├── docs/                    # Specs e planos de implementação
├── build.ps1                # Script de build PowerShell
├── go.mod
└── .env.example
```

---

## Versionamento

| Versão | Principais mudanças |
|---|---|
| **0.2.0** | Configurações via modal (provedor IA, chaves, Whisper), suporte a OpenAI, auto-geração ao gravar, DynamicAIClient |
| **0.1.1** | Fix: banco de dados criado em `%AppData%` em vez de diretório da instalação |
| **0.1.0** | Lançamento inicial: gravação, transcrição, geração com Claude, temas, notas em Markdown |
