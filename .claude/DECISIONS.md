# Decisões Arquiteturais

---

## [2026-05-02] Wails/OnStartup: GetPort() deve bloquear via channel, nunca retornar 0

**Contexto:** Wails v2 carrega o frontend (WebView2) concorrentemente com `OnStartup`. Se `GetPort()` for chamado antes de `a.port` ser definido em `OnStartup`, retorna 0. O frontend então faz poll em `localhost:0` indefinidamente até o timeout, exibindo erro falso de servidor.

**Alternativas:**
- Retornar `a.port` diretamente (race condition — era o bug)
- Polling com sleep em `GetPort()` (funciona mas tem data race sem sync)
- Channel `portReady chan struct{}` fechado logo após `net.Listen` ter sucesso

**Escolha:** `portReady chan struct{}` inicializado em `NewApp()`, fechado com `sync.Once` imediatamente após `a.port = ln.Addr().Port` (antes de `tray.Start()`). `GetPort()` bloqueia em `<-portReady` com timeout de 30 s.

**Justificativa:** O channel é fechado ANTES de `tray.Start()` para que o frontend não precise esperar o tray inicializar (Shell_NotifyIconW pode ser lento no primeiro boot). O `sync.Once` garante que todos os caminhos de erro em `OnStartup` também fecham o channel (via `defer signalPort()`), evitando deadlock.

Registro de decisões transversais ao projeto. Decisões específicas de cada feature estão nos planos do Superpowers correspondentes.

---

## [2026-05-02] faster-whisper: gerador lazy exige consumo dentro do bloco try

**Contexto:** `WhisperModel.transcribe()` retorna um gerador Python. A computação CUDA real ocorre ao iterar os segmentos (lazy), não na chamada a `transcribe()`. Erros de DLL CUDA (ex: `cublas64_12.dll not found`) surfaceiam apenas durante `join(seg.text for seg in segments)`, fora de qualquer `try` que envolva só o `transcribe()`.

**Alternativas:**
- Wrapping apenas de `transcribe()` — não captura erros lazy (era o bug)
- Consumir o gerador imediatamente com `list(segments)` antes do try — funciona mas perde o streaming
- Consumir o gerador **dentro** do bloco `try` junto com `transcribe()`

**Escolha:** `segments, info = self._model.transcribe(...)` e `text = " ".join(...)` no mesmo bloco `try`.

**Justificativa:** Qualquer integração com `faster-whisper` que precise capturar erros de inferência deve consumir o gerador dentro do try. Padrão a seguir em futuras alterações do `transcriber.py`.

---

## [2026-05-02] CUDA: detecção antecipada via ctypes antes de carregar o modelo

**Contexto:** `_resolve_device_compute` agora tenta `ctypes.CDLL("cublas64_12.dll")` antes de comprometer-se com device `cuda`. Se a DLL não estiver acessível, o device cai para `cpu` sem nunca instanciar o modelo GPU.

**Alternativas:**
- Carregar modelo GPU e lidar com falha ao transcrever (fallback reativo — era o PR #23)
- Detectar antecipadamente (fallback proativo — PR #24)

**Escolha:** Detecção antecipada em `_resolve_device_compute` + fallback reativo em `transcribe()` como segunda linha de defesa.

**Justificativa:** Evita carregar modelo pesado na GPU para depois descobrir que as DLLs não estão disponíveis. O fallback reativo ainda existe para cobrir casos em que a DLL é carregável mas falha durante a inferência.

---

## [2026-04-29] Posicionamento de cards com float + rebalanceamento automático

**Contexto:** Ordem manual de cards dentro de colunas do kanban precisa ser persistida no SQLite sem renumeração constante.

**Alternativas:**
- Integer sequencial com renumeração ao mover (simples, mas O(n) writes)
- Float com inserção no meio sem renumeração (O(1) na maioria dos casos)

**Escolha:** Float. Threshold de rebalanceamento: gap < 1e-9 dentro de uma coluna dispara renumeração completa da coluna (1000, 2000, 3000...).

**Justificativa:** Operação de drag-and-drop é frequente; renumeração raramente ocorre na prática.

---

## [2026-04-29] customPrompt de tema substitui completamente o prompt padrão

**Contexto:** `Theme.CustomPrompt` é enviado para summary, key_points e tasks via o mesmo campo. Não há prompts separados por tipo de geração.

**Alternativas:**
- Campo único substituindo tudo (implementado)
- Campos separados por tipo (`custom_summary_prompt`, `custom_key_points_prompt`, `custom_tasks_prompt`)

**Escolha:** Campo único por ora (YAGNI).

**Justificativa:** Usuário único, uso pessoal. Se necessário, separar futuramente — schema suporta adição de colunas sem breaking change.

---

## [2026-04-29] Board é global, colunas são globais, numeração de cards é sequencial e imutável

**Contexto:** Design do kanban board.

**Escolha:** Um board global, colunas globais (não por tema), contador global de cards sequencial nunca reutilizado (estilo issue tracker).

**Justificativa:** App single-user, complexidade extra de boards por tema não agrega valor no uso atual.

---

## [2026-04-29] Seed de 3 colunas padrão na migration 007

**Contexto:** Board precisa de colunas para funcionar. Usuário pode não configurar nada logo após instalar.

**Escolha:** Migration 007 insere Backlog, Em Andamento, Concluído com IDs fixos (`col-backlog`, `col-wip`, `col-done`).

**Justificativa:** IDs fixos permitem idempotência se a migration rodar em banco existente.

---

## [2026-05-01] Win32 overlay: criação de janela e message loop no mesmo OS thread

**Contexto:** Ao criar uma janela Win32 em Go via goroutine, os eventos WM_* nunca chegam ao `GetMessage` se o loop roda em thread diferente da criação (Win32 thread affinity).

**Alternativas:**
- Criar janela na goroutine principal (inviável em Wails — bloquearia o runtime)
- Usar `PostThreadMessage` para despachar para outra thread (complexo)
- Criar janela e rodar o message loop na mesma goroutine fixada ao OS thread

**Escolha:** Goroutine dedicada com `runtime.LockOSThread()` / `defer runtime.UnlockOSThread()`. Canal `ready chan struct{}` sinaliza o HWND de volta ao chamador após `CreateWindowEx`.

**Justificativa:** Pattern simples, correto por especificação Win32, sem overhead extra. Qualquer janela Win32 criada em Go deve seguir este padrão.

---

## [2026-05-01] CUDA no audio-service: pré-load de DLLs + detecção via ctranslate2

**Contexto:** ctranslate2 usa `LoadLibrary` internamente e ignora `os.add_dll_directory`. Em Windows, `cublas64_12.dll` e DLLs do cudnn não são encontradas sem pré-carregamento explícito.

**Alternativas:**
- Adicionar DLLs ao PATH do sistema (requer configuração manual por máquina)
- Detectar CUDA via `torch.cuda.is_available()` (torch não está no venv do audio-service)
- Pré-carregar via `ctypes.CDLL` + detectar via `ctranslate2.get_cuda_device_count()`

**Escolha:** `_setup_dll_paths()` carrega todos os `.dll` de `nvidia.cudnn` e `nvidia.cublas` via `ctypes.CDLL` antes de instanciar `WhisperModel`. Detecção de GPU: `ctranslate2.get_cuda_device_count() > 0`.

**Justificativa:** Sem dependência de torch. Funciona em qualquer Windows com ou sem GPU NVIDIA. Em máquinas sem CUDA, os pacotes nvidia.* não estão instalados e o bloco é ignorado silenciosamente.

---

## [2026-04-29] Processo de build do installer

**Contexto:** `wails build` não encontra `makensis` no PATH por padrão.

**Escolha:** Comando de build completo para Windows:
```bash
cd cmd/desktop
PATH="$PATH:/c/Program Files (x86)/NSIS" wails build -nsis
cp "build/bin/Meeting Notes-amd64-installer.exe" "../../dist/meeting-notes-X.Y.Z-windows-amd64-installer.exe"
```

**Justificativa:** NSIS está instalado em `C:\Program Files (x86)\NSIS` mas não está no PATH padrão do bash.
