# Correção dos bugs conhecidos — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Corrigir os cinco bugs conhecidos registrados no backlog, cujas causas já foram localizadas com arquivo e linha.

**Architecture:** Duas tasks separadas por ciclo de teste, não por tema: a do Python tem teste automatizado de verdade (pytest com `WhisperModel` mockado), a do frontend não tem harness e se verifica por typecheck + build + exercício ao vivo. Um reviewer pode reprovar uma sem tocar na outra.

**Tech Stack:** Python 3.12 + pytest + faster-whisper (mockado nos testes); React 19 + TypeScript + `@dnd-kit/core`.

**Spec:** não há spec — estes são defeitos já diagnosticados, registrados em `.claude/BACKLOG.md` (seção "Bugs conhecidos") com arquivo e linha. As duas decisões de comportamento foram tomadas pelo usuário em 2026-08-21 e estão nas Global Constraints abaixo.

## Global Constraints

- **Decisão do usuário (fallback de GPU):** se o device é `cuda` e a transcrição levanta **qualquer** exceção, recarregar em CPU e tentar de novo, registrando o erro original em log. Segue a decisão já registrada de que "a transcrição é o ativo primário" (DECISIONS, 2026-06-05). Custo aceito: um erro real falha duas vezes antes de ser reportado.
- **Decisão do usuário (menu no scroll):** rolar a lista **fecha** o menu de ações do tema. Não implementar reposicionamento acompanhando a linha.
- Não tocar em `internal/` (Go) — nenhum destes bugs é de backend Go.
- O frontend **não tem infra de teste** (sem vitest, sem testing-library) e introduzir uma está fora do escopo deste plano. Verificação de frontend é `npx tsc --noEmit` + `npm run build` + checagem ao vivo. Nunca reportar isso como cobertura de teste.
- Testes Python rodam de `audio-service/` com `python -m pytest`. As dependências estão instaladas no Python global (não há venv).
- Sem comentários no código, salvo quando o WHY é não-óbvio.
- Textos de UI em pt-BR.

---

### Task 1: Fallback amplo de GPU para CPU no transcriber

**Files:**
- Modify: `audio-service/transcriber.py:118-134`
- Modify: `audio-service/tests/test_transcriber.py:170-188`

**Interfaces:**
- Produces: comportamento de `Transcriber.transcribe`: em `self.device == "cuda"`, qualquer exceção da inferência recarrega o modelo em CPU (`WhisperModel(self.model_name, device="cpu", compute_type="int8")`), define `self.device = "cpu"` e repete a transcrição uma vez. Em `self.device == "cpu"`, exceções propagam. Uma falha na retentativa em CPU também propaga.

O bug: hoje o fallback só dispara para erros de DLL (`"dll"`, `"cublas"`, `"cudnn"`, `"library"`, `"not found"`, `"cannot be loaded"`). Um out-of-memory da GPU não casa com nenhum e cai no `raise`, marcando a reunião como FAILED em vez de transcrever mais devagar.

- [ ] **Step 1: Reescrever o teste que codifica o comportamento antigo**

`test_transcribe_non_dll_error_propagates` (linhas 170-188) afirma que um erro não-DLL na GPU propaga — exatamente o que esta task inverte. **Substituir** esse teste pelos três abaixo. Note que os dois primeiros usam `side_effect` na chamada de `transcribe` (erro imediato), diferente do teste de DLL existente que usa um gerador que levanta na iteração; ambos os caminhos passam pelo mesmo `except`.

```python
def test_transcribe_cuda_oom_falls_back_to_cpu(tmp_path):
    """Falta de VRAM na GPU recarrega em CPU e transcreve, em vez de falhar a reunião."""
    cpu_seg = MagicMock()
    cpu_seg.text = "fallback"
    cpu_info = MagicMock()
    cpu_info.language = "pt"
    cpu_info.duration = 3.0

    cpu_model = MagicMock()
    cpu_model.transcribe.return_value = (iter([cpu_seg]), cpu_info)

    gpu_model = MagicMock()
    gpu_model.transcribe.side_effect = RuntimeError("CUDA failed with error out of memory")

    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")

    with patch("transcriber.WhisperModel", side_effect=[gpu_model, cpu_model]) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=("cuda", "int8_float16")):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
        t._model = gpu_model

        result = t.transcribe(wav)

    assert result.transcript == "fallback"
    assert t.device == "cpu"
    mock_cls.assert_called_with("medium", device="cpu", compute_type="int8")


def test_transcribe_error_on_cpu_propagates(tmp_path):
    """Em CPU não há para onde cair: o erro propaga em vez de recarregar em loop."""
    cpu_model = MagicMock()
    cpu_model.transcribe.side_effect = ValueError("invalid audio format")

    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")

    with patch("transcriber.WhisperModel", return_value=cpu_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=("cpu", "int8")):
        t = Transcriber("medium", "cpu", "int8", tmp_path)
        t._model = cpu_model

        with pytest.raises(ValueError, match="invalid audio format"):
            t.transcribe(wav)

    assert t.device == "cpu"
    assert mock_cls.call_count == 1


def test_transcribe_cpu_retry_failure_propagates(tmp_path):
    """Se a retentativa em CPU também falha, o erro sobe — nada é engolido."""
    gpu_model = MagicMock()
    gpu_model.transcribe.side_effect = RuntimeError("CUDA failed with error out of memory")

    cpu_model = MagicMock()
    cpu_model.transcribe.side_effect = ValueError("corrupt wav")

    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")

    with patch("transcriber.WhisperModel", side_effect=[gpu_model, cpu_model]), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=("cuda", "int8_float16")):
        t = Transcriber("medium", "cuda", "int8_float16", tmp_path)
        t._model = gpu_model

        with pytest.raises(ValueError, match="corrupt wav"):
            t.transcribe(wav)

    assert t.device == "cpu"
```

Manter `test_transcribe_cuda_dll_error_falls_back_to_cpu` (linhas 134-168) **sem alteração**: erro de DLL na GPU continua caindo para CPU, agora pelo caminho genérico.

- [ ] **Step 2: Rodar para ver falhar**

Run: `cd audio-service && python -m pytest tests/test_transcriber.py -v`
Expected: `test_transcribe_cuda_oom_falls_back_to_cpu` e `test_transcribe_cpu_retry_failure_propagates` FALHAM com `RuntimeError: CUDA failed with error out of memory` propagando (o `except` atual não reconhece OOM). `test_transcribe_error_on_cpu_propagates` já passa — o `except` exige `device == "cuda"`.

- [ ] **Step 3: Ampliar o fallback**

Em `audio-service/transcriber.py`, substituir o bloco `except` (linhas 122-131) por:

```python
        except Exception as e:
            if self.device != "cuda":
                raise
            # Transcrição é o ativo primário: qualquer falha na GPU (DLL ausente,
            # falta de VRAM, driver) vale uma retentativa em CPU antes de desistir.
            import logging
            logging.warning("GPU inference failed (%s), reloading model on CPU", e)
            self._model = WhisperModel(self.model_name, device="cpu", compute_type="int8")
            self.device = "cpu"
            segments, info = self._model.transcribe(str(resolved), **transcribe_kwargs)
            text = " ".join(seg.text.strip() for seg in segments).strip()
```

A variável `err = str(e).lower()` e a tupla de keywords saem por completo. O comentário fica porque o WHY (por que tentar de novo em vez de falhar) não é óbvio pelo código.

- [ ] **Step 4: Rodar para ver passar**

Run: `cd audio-service && python -m pytest tests/test_transcriber.py -v`
Expected: PASS em todos, incluindo o teste de DLL preexistente.

- [ ] **Step 5: Suíte Python completa**

Run: `cd audio-service && python -m pytest -q`
Expected: PASS. Nenhum outro teste depende da lista de keywords (confirmar com `grep -rn "cublas\|cannot be loaded" tests/`).

- [ ] **Step 6: Commit**

```bash
git add audio-service/transcriber.py audio-service/tests/test_transcriber.py
git commit -m "fix: fall back to CPU on any GPU inference failure, not just DLL errors"
```

---

### Task 2: Quatro correções de frontend

**Files:**
- Modify: `frontend/src/components/sidebar/Sidebar.tsx` (mensagem de exclusão em pt-BR; `onDragCancel`)
- Modify: `frontend/src/components/sidebar/ThemeRowMenu.tsx` (fechar no scroll)
- Modify: `frontend/src/components/layout/Toolbar.tsx` (hamburger só na view de Reuniões + rótulo acessível)

**Interfaces:**
- Consumes: `Toolbar` já recebe a prop `activeView: "meetings" | "board"` — nenhuma prop nova é necessária.
- Produces: nenhuma interface nova; são quatro correções locais.

- [ ] **Step 1: Mensagem de exclusão em pt-BR**

`Sidebar.tsx:228` mostra `err.message`, que vem cru do backend em inglês (`"theme not found"` ou `"failed to delete theme"`, de `internal/handlers/theme_handler.go:122,125`). O arquivo já tem o padrão certo para isso em `MOVE_ERROR_MESSAGES` + `moveErrorMessage` (linhas 14-25). Adicionar o par equivalente, logo abaixo de `moveErrorMessage`:

```tsx
const DELETE_ERROR_MESSAGES: Record<string, string> = {
  "theme not found": "Tema não encontrado — talvez já tenha sido excluído.",
}

function deleteErrorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : ""
  return DELETE_ERROR_MESSAGES[raw] ?? "Não foi possível excluir o tema."
}
```

E na linha 228, trocar

```tsx
                    setDeleteError(err instanceof Error ? err.message : "Não foi possível excluir o tema.")
```

por

```tsx
                    setDeleteError(deleteErrorMessage(err))
```

- [ ] **Step 2: `onDragCancel` no DndContext**

Um arraste cancelado (Escape, ou o ponteiro saindo da janela) hoje deixa `activeId` preenchido, o que mantém `droppable` falso naquela linha até o próximo arraste. Em `Sidebar.tsx:180-183`, acrescentar o handler ao lado dos que já existem:

```tsx
        <DndContext
          sensors={sensors}
          onDragStart={e => setActiveId(String(e.active.id))}
          onDragEnd={e => { setActiveId(null); handleDragEnd(e) }}
          onDragCancel={() => setActiveId(null)}
        >
```

- [ ] **Step 3: Fechar o menu ao rolar a lista**

O menu é `fixed`, posicionado a partir de um rect capturado na abertura (`ThemeRowMenu.tsx:37`), então rolar a lista o deixa fora de lugar. Decisão do usuário: fechar. Eventos de `scroll` **não borbulham**, então o listener precisa ser em fase de captura para pegar o scroll do container interno. No `useEffect` de `ThemeRowMenu.tsx` (linhas 17-34), acrescentar dentro do mesmo efeito:

```tsx
    document.addEventListener("scroll", onClose, true)
```

e no cleanup:

```tsx
      document.removeEventListener("scroll", onClose, true)
```

Não trocar `onClose` por um wrapper: ele já é a única coisa a fazer, e a dependência `[onClose, anchor]` do efeito já está correta.

- [ ] **Step 4: Hamburger só na view de Reuniões**

Na view Board o painel de temas não é renderizado, mas o hamburger continua alternando `sidebarOpen` — mexendo no estado que o "recolhe no Board e retoma ao voltar" depende. O `Ctrl+B` já foi corrigido (`App.tsx:139`); o botão não.

Em `frontend/src/components/layout/Toolbar.tsx`, trocar o botão (linhas 20-22) por uma versão condicional e com rótulo:

```tsx
      {activeView === "meetings" && (
        <Button variant="ghost" size="icon" onClick={onToggleSidebar} title="Mostrar/ocultar temas (Ctrl+B)" aria-label="Mostrar ou ocultar o painel de temas">
          <Menu size={18} />
        </Button>
      )}
```

**Tradeoff aceito:** esconder o botão desloca "Meeting Notes" ~40px para a esquerda ao entrar no Board. A alternativa (deixar o botão sempre visível e desabilitado) mantém o layout estável mas cria um controle permanentemente morto naquela view, o que é pior. Se o deslocamento incomodar na verificação ao vivo, reportar em vez de improvisar outra solução.

- [ ] **Step 5: Typecheck e build**

Run: `cd frontend && npx tsc --noEmit`
Expected: sem erros.

Run: `cd frontend && npm run build`
Expected: build conclui (o aviso de chunk size é preexistente e conhecido).

- [ ] **Step 6: Commit**

```bash
git add frontend/src
git commit -m "fix: pt-BR delete error, drag cancel, menu on scroll, hamburger on board view"
```

---

## Final verification

- [ ] `cd audio-service && python -m pytest -q` verde.
- [ ] `go vet ./...` limpo e `go test ./...` verde (nada de Go foi tocado — é confirmação de que nada quebrou por tabela).
- [ ] `cd frontend && npx tsc --noEmit` limpo; `npm run build` conclui.
- [ ] Verificação ao vivo (controller, com `wails dev`): abrir o menu `⋯` e rolar a lista → fecha; entrar no Board → hamburger desaparece; voltar para Reuniões → hamburger volta e o painel retoma o estado anterior; iniciar um arraste e cancelar com Escape → o próximo arraste funciona normalmente na mesma linha.
- [ ] Atualizar `.claude/BACKLOG.md` removendo os cinco bugs corrigidos da seção "Bugs conhecidos".
