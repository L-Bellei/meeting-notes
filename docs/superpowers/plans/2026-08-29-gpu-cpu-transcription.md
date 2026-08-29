# GPU/CPU na transcrição — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** O usuário escolhe o device de transcrição (Auto/GPU/CPU) nas Configurações; o instalador embarca as DLLs de CUDA; fallback GPU→CPU por chamada, sem estado pegajoso.

**Architecture:** `whisper_device` viaja por transcrição no `POST /transcribe` (como `language`). O `Transcriber` vira cache de modelos por device (`self._models`), resolve o efetivo a cada chamada e faz fallback sem mutar a resolução. O scan de GPU é calculado uma vez no lifespan e exposto no `/health`, repassado pelo `GET /health` do Go até a UI.

**Tech Stack:** Python (FastAPI, faster-whisper, ctranslate2), Go 1.22 (chi), SQLite (migration 019), React 19 + TS, PyInstaller.

**Spec:** `docs/superpowers/specs/2026-08-29-gpu-cpu-transcription-design.md`

## Global Constraints

- Sem comentários no código, salvo WHY não-óbvio (convenção do CLAUDE.md); os comentários presentes nos blocos de código deste plano são deliberados.
- Os dois entry points (`cmd/api/main.go`, `cmd/desktop/app.go`) mudam juntos quando a mudança os afeta.
- Setting novo: `whisper_device` ∈ {`auto`, `cuda`, `cpu`}, default `auto` (migration 019).
- Timeout do `/transcribe` no Go: **4 horas** (`4 * time.Hour`).
- `/health` do audio-service ganha `gpu_available: bool`, `gpu_name: string|null`, `gpu_vram_mb: int|null`; `device` passa a ser "device efetivo da última transcrição".
- Fallback: falha com efetivo `cuda` → warning com a causa → retenta em CPU **na mesma chamada**; a chamada seguinte re-resolve `auto` (retenta CUDA). Modelo CPU do fallback fica em cache.
- No fallback/CPU o compute type é **sempre `int8`** (comportamento atual preservado).
- Testes Python nunca carregam `WhisperModel` real (patch ativo durante o corpo do teste — Task 1).
- pytest roda de `audio-service/` com `.venv\Scripts\python.exe -m pytest -q`.

---

### Task 1: Consertar o foot-gun do harness de `test_transcriber.py`

**Files:**
- Modify: `audio-service/tests/test_transcriber.py:11-29`

**Interfaces:**
- Produces: `_make_transcriber(tmp_path, device, compute_type)` vira **context manager** (uso: `with _make_transcriber(tmp_path) as t:`); a fixture `transcriber` faz yield **dentro** do with. Task 2 reescreve partes deste arquivo assumindo essa forma.

Hoje `_make_transcriber` sai do `patch("transcriber.WhisperModel", ...)` antes de retornar: um teste cujo mock lance dentro do `try` de `transcribe()` chamaria o `WhisperModel` real (download de GB). Com o refactor da Task 2 (criação lazy de modelos durante `transcribe`), isso deixa de ser latente.

- [ ] **Step 1: Reescrever o helper e a fixture** (substituir as linhas 11-29):

```python
from contextlib import contextmanager


@contextmanager
def _make_transcriber(tmp_path, device="cuda", compute_type="int8_float16"):
    """Patches ficam ativos durante o corpo do teste: um mock que lança dentro
    de transcribe() jamais pode alcançar o WhisperModel real (download de GB)."""
    fake_model = MagicMock()
    with patch("transcriber.WhisperModel", return_value=fake_model) as mock_cls, \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_resolve_device_compute", return_value=(device, compute_type)):
        t = Transcriber(
            model_name="medium",
            device=device,
            compute_type=compute_type,
            recordings_dir=tmp_path,
        )
        t._fake_model = fake_model
        t._mock_cls = mock_cls
        yield t


@pytest.fixture
def transcriber(tmp_path):
    with _make_transcriber(tmp_path) as t:
        yield t
```

- [ ] **Step 2: Rodar a suíte** — `cd audio-service; .venv\Scripts\python.exe -m pytest -q` — Expected: 41 passed (nenhum teste muda de resultado; só a janela do patch muda).

- [ ] **Step 3: Prova do conserto** — adicionar UM teste que teria disparado o foot-gun antes:

```python
def test_fixture_patch_active_during_test_body(transcriber, tmp_path):
    """Regressão do harness: WhisperModel deve continuar mockado no corpo do teste."""
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    transcriber._fake_model.transcribe.side_effect = RuntimeError("boom cuda")
    with pytest.raises(RuntimeError):
        # device é cuda: o except tenta recarregar em CPU — que DEVE bater no mock,
        # não no WhisperModel real. side_effect abaixo prova que bateu no mock.
        transcriber._mock_cls.side_effect = RuntimeError("segundo load também falha")
        transcriber.transcribe(wav)
    assert transcriber._mock_cls.call_count >= 1
```

- [ ] **Step 4: Rodar e ver passar** — `pytest -q` → 42 passed.

- [ ] **Step 5: Commit**

```bash
git add audio-service/tests/test_transcriber.py
git commit -m "test: patch do WhisperModel ativo durante o corpo do teste"
```

---

### Task 2: `Transcriber` — device por chamada, cache de modelos, scan e fallback sem mutação

**Files:**
- Modify: `audio-service/transcriber.py`
- Modify: `audio-service/tests/test_transcriber.py`

**Interfaces:**
- Consumes: forma context-manager de `_make_transcriber` (Task 1).
- Produces (Task 3 depende): `Transcriber.transcribe(path, language=None, device="auto") -> TranscribeResult` com campo novo `device: str` (efetivo usado); atributos `gpu_available: bool`, `gpu_name: str|None`, `gpu_vram_mb: int|None`, `device` (efetivo da última transcrição; no boot, o carregado), `model_loaded`, `model_name`. `_resolve_device_compute` **deixa de existir** (substituído por `_scan_gpu` + `_effective_device` + `_compute_for`).

- [ ] **Step 1: Reescrever os testes primeiro.** Trocar, em TODOS os pontos do arquivo, `patch.object(Transcriber, "_resolve_device_compute", return_value=(device, compute_type))` por `patch.object(Transcriber, "_scan_gpu", return_value=(device == "cuda", None, None))` (o helper da Task 1 inclusive). Remover os `t._model = gpu_model` (atributo morre). Reescrever os testes de fallback e adicionar os novos:

```python
def test_transcribe_uses_cpu_when_device_cpu_requested(transcriber, tmp_path):
    """Mesmo com GPU disponível, device=cpu força CPU."""
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock(); info.language = "pt"; info.duration = 1.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    result = transcriber.transcribe(wav, device="cpu")

    assert result.device == "cpu"
    # segundo load (cpu) aconteceu além do load do boot (cuda)
    assert transcriber._mock_cls.call_count == 2
    transcriber._mock_cls.assert_called_with("medium", device="cpu", compute_type="int8")


def test_transcribe_model_cache_reuses_per_device(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock(); info.language = "pt"; info.duration = 1.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    transcriber.transcribe(wav)
    transcriber.transcribe(wav)

    assert transcriber._mock_cls.call_count == 1  # só o load do boot


def test_transcribe_fallback_does_not_stick(tmp_path):
    """Falha em CUDA cai para CPU NA CHAMADA; a próxima chamada retenta CUDA."""
    calls = []

    def model_factory(name, device, compute_type):
        m = MagicMock()
        if device == "cuda":
            def flaky(*a, **k):
                calls.append("cuda")
                if len([c for c in calls if c == "cuda"]) == 1:
                    raise RuntimeError("CUDA failed with error out of memory")
                info = MagicMock(); info.language = "pt"; info.duration = 1.0
                return (iter([]), info)
            m.transcribe.side_effect = flaky
        else:
            def ok(*a, **k):
                calls.append("cpu")
                info = MagicMock(); info.language = "pt"; info.duration = 1.0
                return (iter([]), info)
            m.transcribe.side_effect = ok
        return m

    with patch("transcriber.WhisperModel", side_effect=model_factory), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_scan_gpu", return_value=(True, "RTX", 4096)):
        t = Transcriber("medium", "auto", "auto", tmp_path)
        wav = tmp_path / "rec.wav"
        wav.write_bytes(b"fake")

        r1 = t.transcribe(wav)   # cuda falha -> cpu
        r2 = t.transcribe(wav)   # retenta cuda -> sucesso

    assert r1.device == "cpu"
    assert r2.device == "cuda"
    assert calls == ["cuda", "cpu", "cuda"]


def test_scan_exposed_on_attributes(tmp_path):
    with patch("transcriber.WhisperModel", return_value=MagicMock()), \
         patch.object(Transcriber, "_setup_dll_paths"), \
         patch.object(Transcriber, "_scan_gpu", return_value=(True, "NVIDIA GeForce RTX 2050", 4096)):
        t = Transcriber("medium", "auto", "auto", tmp_path)
    assert t.gpu_available is True
    assert t.gpu_name == "NVIDIA GeForce RTX 2050"
    assert t.gpu_vram_mb == 4096
    assert t.device == "cuda"
```

Os testes existentes de fallback (`test_transcribe_cuda_dll_error_falls_back_to_cpu`, `test_transcribe_cuda_oom_falls_back_to_cpu`, `test_transcribe_cpu_retry_failure_propagates`) são reescritos na mesma forma do `model_factory` acima (sem `t._model =`), e os asserts `assert t.device == "cpu"` passam a valer como "efetivo da última transcrição". `test_transcribe_error_on_cpu_propagates` usa `_scan_gpu` retornando `(False, None, None)`. `test_init_loads_model_and_sets_attributes` verifica `mock_cls.assert_called_once_with("medium", device="cuda", compute_type="int8_float16")` com `_scan_gpu=(True,...)` e `compute_type="int8_float16"` explícito.

- [ ] **Step 2: Rodar e ver falhar** — `pytest tests/test_transcriber.py -q` — FAIL (`_scan_gpu` não existe, `device=` kwarg não existe).

- [ ] **Step 3: Reescrever `transcriber.py`.** `TranscribeResult` ganha `device: str`. A classe:

```python
class Transcriber:
    def __init__(self, model_name, device, compute_type, recordings_dir):
        self.model_name = model_name
        self.default_device = device
        self.compute_type = compute_type
        self.recordings_dir = Path(recordings_dir).resolve()
        self._setup_dll_paths()
        self.gpu_available, self.gpu_name, self.gpu_vram_mb = self._scan_gpu()
        self._models: dict[str, "WhisperModel"] = {}
        effective = self._effective_device(device)
        self._get_model(effective)
        self.device = effective
        self.model_loaded = True

    def _effective_device(self, requested: str) -> str:
        if requested in (None, "", "auto", "cuda") and self.gpu_available:
            return "cuda"
        return "cpu"

    def _compute_for(self, device: str) -> str:
        if device == "cpu":
            # Fallback e CPU explícito sempre int8: um compute_type de GPU
            # (int8_float16) herdado quebraria o modelo de CPU.
            return "int8"
        return self.compute_type if self.compute_type != "auto" else "int8_float16"

    def _get_model(self, device: str):
        if device not in self._models:
            self._models[device] = WhisperModel(
                self.model_name, device=device, compute_type=self._compute_for(device)
            )
        return self._models[device]

    def _scan_gpu(self):
        available = False
        try:
            import ctranslate2
            if ctranslate2.get_cuda_device_count() > 0:
                # Verify the CUDA compute DLLs are actually loadable before committing to GPU
                import ctypes
                for dll in ("cublas64_12.dll", "cublas64_11.dll"):
                    try:
                        ctypes.CDLL(dll)
                        available = True
                        break
                    except OSError:
                        continue
        except Exception:
            pass
        name = None
        vram = None
        if available:
            try:
                import subprocess
                flags = 0x08000000 if sys.platform == "win32" else 0
                out = subprocess.run(
                    ["nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"],
                    capture_output=True, text=True, timeout=5, creationflags=flags,
                )
                first = out.stdout.strip().splitlines()[0]
                raw_name, raw_mem = first.rsplit(",", 1)
                name = raw_name.strip()
                vram = int(float(raw_mem.strip()))
            except Exception:
                pass
        return available, name, vram
```

`_setup_dll_paths` fica como está. `_resolve_device_compute` é removido. `transcribe`:

```python
    def transcribe(self, path: Path, language: Optional[str] = None, device: str = "auto") -> TranscribeResult:
        resolved = Path(path).resolve()
        try:
            resolved.relative_to(self.recordings_dir)
        except ValueError:
            raise ValueError(f"path outside recordings dir: {path}")
        if not resolved.exists():
            raise ValueError(f"path does not exist: {path}")

        lang = None if language in (None, "", "auto") else language
        transcribe_kwargs = dict(
            language=lang,
            condition_on_previous_text=False,
            compression_ratio_threshold=1.8,
            repetition_penalty=1.1,
        )
        effective = self._effective_device(device)
        model = self._get_model(effective)
        try:
            segments, info = model.transcribe(str(resolved), **transcribe_kwargs)
            # Consume the generator inside the try block — errors from lazy CUDA ops surface here
            text = " ".join(seg.text.strip() for seg in segments).strip()
        except Exception as e:
            if effective != "cuda":
                raise
            # Transcrição é o ativo primário: qualquer falha na GPU vale uma
            # retentativa em CPU nesta chamada; a resolução NÃO fica pegajosa —
            # a próxima chamada re-resolve e retenta CUDA.
            import logging
            logging.warning("GPU inference failed (%s), retrying this call on CPU", e)
            effective = "cpu"
            model = self._get_model("cpu")
            segments, info = model.transcribe(str(resolved), **transcribe_kwargs)
            text = " ".join(seg.text.strip() for seg in segments).strip()
        self.device = effective
        return TranscribeResult(
            transcript=text,
            language=info.language,
            duration_seconds=info.duration,
            model=self.model_name,
            device=effective,
        )
```

Manter os comentários dos `transcribe_kwargs` originais (linhas "Prevents hallucination..." etc.) — foram omitidos acima por brevidade, mas devem permanecer no arquivo.

- [ ] **Step 4: Rodar e ver passar** — `pytest tests/test_transcriber.py -q`. Depois a suíte inteira: `pytest -q` (test_main.py ainda passa — a assinatura nova tem defaults).

- [ ] **Step 5: Commit**

```bash
git add audio-service/transcriber.py audio-service/tests/test_transcriber.py
git commit -m "feat: Transcriber com device por chamada, cache por device e fallback sem mutação"
```

---

### Task 3: API do audio-service — `/health` com scan e `device` no `/transcribe`

**Files:**
- Modify: `audio-service/main.py:37-51,89-102`
- Modify: `audio-service/tests/test_main.py`

**Interfaces:**
- Consumes: atributos/assinatura do `Transcriber` (Task 2).
- Produces (Tasks 4-5 dependem): `/health` responde também `gpu_available`, `gpu_name`, `gpu_vram_mb`; `TranscribeRequest` aceita `device: Optional[str] = "auto"`; resposta do `/transcribe` inclui `"device": result.device`.

- [ ] **Step 1: Testes primeiro** em `test_main.py` (seguir o padrão existente do arquivo — TestClient com transcriber/recorder mockados; ler o arquivo antes):

```python
def test_health_includes_gpu_scan_fields(client_with_mocks):
    client, recorder, transcriber = client_with_mocks
    transcriber.gpu_available = True
    transcriber.gpu_name = "NVIDIA GeForce RTX 2050"
    transcriber.gpu_vram_mb = 4096
    r = client.get("/health")
    body = r.json()
    assert body["gpu_available"] is True
    assert body["gpu_name"] == "NVIDIA GeForce RTX 2050"
    assert body["gpu_vram_mb"] == 4096


def test_transcribe_passes_device_and_returns_effective(client_with_mocks):
    client, recorder, transcriber = client_with_mocks
    transcriber.transcribe.return_value = TranscribeResult(
        transcript="oi", language="pt", duration_seconds=1.0, model="medium", device="cuda"
    )
    r = client.post("/transcribe", json={"path": "tmp/rec.wav", "device": "cuda"})
    assert r.status_code == 200
    assert r.json()["device"] == "cuda"
    args, kwargs = transcriber.transcribe.call_args
    assert kwargs.get("device") == "cuda" or (len(args) >= 3 and args[2] == "cuda")
```

(Adaptar o nome da fixture ao que `test_main.py` realmente usa; se não houver fixture equivalente, criar uma seguindo o padrão do arquivo.)

- [ ] **Step 2: Ver falhar; implementar** em `main.py`:

```python
class TranscribeRequest(BaseModel):
    path: str
    language: Optional[str] = None
    device: Optional[str] = "auto"
```

`/health` adiciona ao dict: `"gpu_available": transcriber.gpu_available, "gpu_name": transcriber.gpu_name, "gpu_vram_mb": transcriber.gpu_vram_mb`. `/transcribe` chama `transcriber.transcribe(Path(req.path), req.language, device=req.device or "auto")` e adiciona `"device": result.device` à resposta.

- [ ] **Step 3: Rodar e ver passar** — `pytest -q` (suíte completa).

- [ ] **Step 4: Commit**

```bash
git add audio-service/main.py audio-service/tests/test_main.py
git commit -m "feat: /health com scan de GPU e device por chamada no /transcribe"
```

---

### Task 4: Go — client de áudio (device, health, timeout 4h) e orchestrator

**Files:**
- Modify: `internal/audio/client.go`
- Modify: `internal/audio/client_test.go`
- Modify: `internal/services/orchestrator.go:204-208,409-413`
- Modify: fakes/stubs: `internal/services/orchestrator_test.go`, `internal/services/transcription_checks_test.go` (assinatura nova)

**Interfaces:**
- Produces (Task 5-6 dependem): `HealthResponse` ganha `GPUAvailable bool `json:"gpu_available"``, `GPUName string `json:"gpu_name"``, `GPUVRAMMB int `json:"gpu_vram_mb"``; interface `Client.Transcribe(ctx context.Context, path, language, device string) (*TranscribeResponse, error)`; `TranscribeResponse` ganha `Device string `json:"device"``; `transcribeClient.Timeout = 4 * time.Hour`.

- [ ] **Step 1: Testes primeiro** em `client_test.go` (seguir o padrão do arquivo — httptest server):

```go
func TestTranscribe_SendsDeviceAndParsesEffective(t *testing.T) {
	var receivedBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Write([]byte(`{"transcript":"olá","language":"pt","duration_seconds":1.0,"model":"medium","device":"cuda"}`))
	}))
	defer srv.Close()
	c := NewHTTPClient(srv.URL)
	got, err := c.Transcribe(context.Background(), "tmp/rec-1.wav", "pt", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if receivedBody["device"] != "auto" {
		t.Fatalf("device no request = %q, want auto", receivedBody["device"])
	}
	if got.Device != "cuda" {
		t.Fatalf("Device = %q, want cuda", got.Device)
	}
}

func TestHealth_ParsesGPUFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","state":"idle","loopback_available":true,"model_loaded":true,"model_name":"medium","device":"cuda","gpu_available":true,"gpu_name":"RTX 2050","gpu_vram_mb":4096}`))
	}))
	defer srv.Close()
	c := NewHTTPClient(srv.URL)
	h, err := c.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !h.GPUAvailable || h.GPUName != "RTX 2050" || h.GPUVRAMMB != 4096 {
		t.Fatalf("gpu fields: %+v", h)
	}
}

func TestTranscribeClient_TimeoutIsFourHours(t *testing.T) {
	c := NewHTTPClient("http://x")
	if c.transcribeClient.Timeout != 4*time.Hour {
		t.Fatalf("timeout = %v, want 4h", c.transcribeClient.Timeout)
	}
}
```

- [ ] **Step 2: Ver falhar** — `go test ./internal/audio/ -v` (não compila: assinatura).

- [ ] **Step 3: Implementar** em `client.go`: campos novos no `HealthResponse` e `TranscribeResponse`; assinatura `Transcribe(ctx context.Context, path, language, device string)`; body `map[string]string{"path": path, "language": language, "device": device}`; `transcribeClient: &http.Client{Timeout: 4 * time.Hour}` com o comentário:

```go
		// 4h: cobre tentativa de GPU queimada a meio da transcrição + reprocesso
		// inteiro em CPU (1,3× tempo real) em reuniões longas — ver spec 2026-08-29.
		transcribeClient: &http.Client{Timeout: 4 * time.Hour},
```

- [ ] **Step 4: Orchestrator** — nos DOIS call sites (linhas ~204-208 e ~409-413), o bloco que lê `whisper_language` passa a ler também o device:

```go
	whisperLang := ""
	whisperDevice := "auto"
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		whisperLang = s["whisper_language"]
		if d := s["whisper_device"]; d != "" {
			whisperDevice = d
		}
	}
	trResp, err := o.audio.Transcribe(ctx, stopResp.Path, whisperLang, whisperDevice)
```

(No segundo call site o path é `audioPath`.) Atualizar os fakes de `orchestrator_test.go` e `transcription_checks_test.go` para a assinatura nova (retornos inalterados).

- [ ] **Step 5: Suíte** — `go build ./... && go test ./internal/audio/ ./internal/services/ -v` — PASS. Depois `go test ./...`.

- [ ] **Step 6: Commit**

```bash
git add internal/audio internal/services
git commit -m "feat: device por transcrição no client de áudio, health com GPU e timeout de 4h"
```

---

### Task 5: Setting `whisper_device`, migration 019, health mirrors do Go e log do processo filho

**Files:**
- Create: `internal/database/migrations/019_whisper_device.sql`
- Modify: `internal/services/settings_service.go` (mapa `validSettings`)
- Modify: `internal/services/settings_service_test.go`
- Modify: `cmd/desktop/app.go:143-154` (mirror do /health) e `cmd/api/main.go:94-107` (idem)
- Modify: `cmd/desktop/app.go:317-327` (redirect de stdout/stderr do bundled)
- Modify: `frontend/src/hooks/useSettings.ts` (tipo `Settings` + `WRITABLE_SETTINGS` ganham `whisper_device`)

**Interfaces:**
- Consumes: `HealthResponse` com campos GPU (Task 4).
- Produces (Task 6 depende): `GET /health` do Go responde `gpu_available`, `gpu_name`, `gpu_vram_mb`, `device` além de `model_loaded`; setting `whisper_device` aceito no PUT.

- [ ] **Step 1: Teste da whitelist primeiro** (padrão dos testes existentes do arquivo):

```go
func TestSettingsService_Update_WhisperDeviceValidValues(t *testing.T) {
	svc := newTestSettingsService(t)
	for _, v := range []string{"auto", "cuda", "cpu"} {
		if err := svc.Update(context.Background(), map[string]string{"whisper_device": v}); err != nil {
			t.Fatalf("%s: %v", v, err)
		}
	}
	if err := svc.Update(context.Background(), map[string]string{"whisper_device": "tpu"}); err == nil {
		t.Fatal("tpu deveria ser rejeitado")
	}
}
```

(Usar o helper de construção que o arquivo já tem; se o nome difere de `newTestSettingsService`, seguir o existente.)

- [ ] **Step 2: Ver falhar; implementar**: no mapa `validSettings`, adicionar `"whisper_device": validateEnum("auto", "cuda", "cpu"),` ao lado de `whisper_model`. Migration `019_whisper_device.sql`:

```sql
INSERT OR IGNORE INTO settings (key, value) VALUES ('whisper_device', 'auto');
```

- [ ] **Step 3: Health mirrors.** Em `cmd/desktop/app.go:143-154` (e o equivalente em `cmd/api/main.go`, que também tem `database`):

```go
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"status":        "ok",
			"model_loaded":  false,
			"gpu_available": false,
			"gpu_name":      nil,
			"gpu_vram_mb":   nil,
			"device":        "",
		}
		if h, err := audioClient.Health(r.Context()); err == nil {
			resp["model_loaded"] = h.ModelLoaded
			resp["gpu_available"] = h.GPUAvailable
			resp["gpu_name"] = h.GPUName
			resp["gpu_vram_mb"] = h.GPUVRAMMB
			resp["device"] = h.Device
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	})
```

(No `cmd/api`, preservar o campo `database` existente dentro de `resp`.)

- [ ] **Step 4: Log do filho com destino.** No ramo do bundled em `cmd/desktop/app.go` (linhas ~317-327), logo após montar o `exec.Command`:

```go
			// O bundle é console app (v2.7.1): sem redirect, warnings do fallback
			// de GPU evaporam no app empacotado — ver DECISIONS 2026-08-28/29.
			if cacheDir, err := os.UserCacheDir(); err == nil {
				logPath := filepath.Join(cacheDir, "meeting-notes", "audio-service.log")
				if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err == nil {
					if fi, statErr := os.Stat(logPath); statErr == nil && fi.Size() > 5*1024*1024 {
						os.Remove(logPath)
					}
					if f, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); openErr == nil {
						c.Stdout = f
						c.Stderr = f
					}
				}
			}
```

- [ ] **Step 5: `useSettings.ts`** — adicionar `whisper_device: string` ao tipo `Settings` e `"whisper_device"` ao array `WRITABLE_SETTINGS`.

- [ ] **Step 6: Suíte** — `go test ./... && go build ./...`; `cd frontend; npx tsc --noEmit` — PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/database/migrations/019_whisper_device.sql internal/services cmd frontend/src/hooks/useSettings.ts
git commit -m "feat: setting whisper_device, health do Go com scan de GPU e log do audio-service"
```

---

### Task 6: Frontend — seção de transcrição nas Configurações

**Files:**
- Create: `frontend/src/hooks/useAudioHealth.ts`
- Modify: `frontend/src/components/settings/SettingsModal.tsx` (seção Whisper/transcrição)

**Interfaces:**
- Consumes: `GET /health` do Go (Task 5) → `{status, model_loaded, gpu_available, gpu_name, gpu_vram_mb, device}`; setting `whisper_device` via `useSettings`/`useUpdateSettings`/`pickWritable` existentes.

- [ ] **Step 1: Hook** (seguir o padrão de `useAIConfigured.ts`/`useApi.ts` — ler antes):

```ts
import { useQuery } from "@tanstack/react-query"
import { api, useApiReady } from "./useApi"

export interface AudioHealth {
  status: string
  model_loaded: boolean
  gpu_available: boolean
  gpu_name: string | null
  gpu_vram_mb: number | null
  device: string
}

export function useAudioHealth() {
  const apiReady = useApiReady()
  return useQuery({
    queryKey: ["audio-health"],
    queryFn: () => api<AudioHealth>("/health"),
    enabled: apiReady,
    staleTime: 30_000,
  })
}
```

- [ ] **Step 2: UI.** Na seção de transcrição do `SettingsModal.tsx` (onde já vivem `whisper_model`/`whisper_language`), adicionar, seguindo o estilo/idioma do modal:
  - Linha de scan: se `gpu_available`: `GPU detectada: {gpu_name ?? "NVIDIA"}{gpu_vram_mb ? \` (${Math.round(gpu_vram_mb/1024)} GB)\` : ""}`; senão: `Nenhuma GPU NVIDIA — transcrição em CPU`.
  - Select `whisper_device`: `auto` → "Auto (recomendado)", `cuda` → "GPU", `cpu` → "CPU". Gravado pelo mesmo fluxo de save das outras settings (via `pickWritable`).
  - Linha do efetivo: `Última transcrição: {device === "cuda" ? "GPU" : "CPU"}` (ocultar se `device` vazio).

- [ ] **Step 3: Verificar** — `cd frontend; npx tsc --noEmit; npm run build` — limpos.

- [ ] **Step 4: Commit**

```bash
git add frontend/src
git commit -m "feat: seletor de device e scan de GPU nas Configurações"
```

---

### Task 7: PyInstaller — DLLs de CUDA no bundle

**Files:**
- Modify: `audio-service/build/pyinstaller/audio-service.spec:41-47` (o bloco de comentário que exclui nvidia.* é substituído)

**Interfaces:**
- Produces: bundle com `nvidia/cudnn` e `nvidia/cublas` (DLLs + `__init__.py` para o `importlib.import_module` do `_setup_dll_paths`). Task 8 mede e valida.

- [ ] **Step 1: Editar o spec.** Substituir o bloco de comentário das linhas 41-47 ("The nvidia.cudnn / nvidia.cublas GPU DLLs ... not a bug.") por:

```python
# GPU DLLs embarcadas por decisão de 2026-08-29 (revertendo 2026-08-21): o
# usuário escolhe o device e o instalador é autossuficiente. collect_all
# preserva nvidia/<pkg>/{bin,lib} como o _setup_dll_paths() do transcriber
# espera encontrar via importlib.
for pkg in ("nvidia.cudnn", "nvidia.cublas"):
    pkg_datas, pkg_binaries, pkg_hiddenimports = collect_all(pkg)
    datas += pkg_datas
    binaries += pkg_binaries
    hiddenimports += pkg_hiddenimports
```

- [ ] **Step 2: Rebuild do bundle** (demorado):

```powershell
cd audio-service
Remove-Item -Recurse -Force build\dist, build\work -ErrorAction SilentlyContinue
.venv\Scripts\python.exe -m PyInstaller build\pyinstaller\audio-service.spec --distpath build\dist --workpath build\work --noconfirm
```

- [ ] **Step 3: Verificações**: (a) `Get-ChildItem audio-service\build\dist\audio-service\_internal\nvidia -Recurse -Filter *.dll | Measure-Object -Sum Length` — soma na casa de ~1,6 GB; (b) smoke local: subir `audio-service.exe --port 8794` e `GET /health` deve responder com `gpu_available: true` **nesta máquina** (RTX 2050) e `device: "cuda"`.

- [ ] **Step 4: Commit** (só o spec — o bundle é gitignorado):

```bash
git add audio-service/build/pyinstaller/audio-service.spec
git commit -m "feat: DLLs de CUDA no bundle do audio-service"
```

---

### Task 8: Experimento de corte das DLLs (INTERATIVO — máquina com GPU, sessão principal)

**Files:**
- Modify (condicional): `audio-service/build/pyinstaller/audio-service.spec` (filtro de binaries)
- Modify: `docs/superpowers/specs/2026-08-29-gpu-cpu-transcription-design.md` (registrar o resultado)

Critério de aceite (do spec): **uma transcrição real em `device: cuda`** contra o bundle podado. Requer uma gravação `.wav` real no diretório de recordings do bundle e a GPU desta máquina.

- [ ] **Step 1: Podar o bundle atual (sem mexer no spec ainda):** apagar de `build\dist\audio-service\_internal\nvidia\cudnn\bin` os arquivos `cudnn_engines_precompiled*.dll` e `cudnn_adv*.dll`. Medir a nova soma.

- [ ] **Step 2: Transcrição real em CUDA:** subir o exe podado com `RECORDINGS_DIR` apontando para um dir com um `.wav` real (reutilizar uma gravação do banco de dev), `POST /transcribe` com `{"path": "<wav>", "device": "cuda"}` e conferir: HTTP 200, `device: "cuda"` na resposta, transcript não-vazio, e **nenhum warning de fallback** no stdout.

- [ ] **Step 3a (passou):** codificar a poda no spec — após o loop do `collect_all` dos pacotes nvidia:

```python
# Medido em 2026-08-29: cudnn_engines_precompiled (562 MB) e cudnn_adv (230 MB)
# não são exercitados pelo faster-whisper; cortá-los leva o instalador de
# ~610 MB para ~390 MB. Validado com transcrição real em device=cuda.
binaries = [(dest, src, kind) for (dest, src, kind) in binaries
            if "cudnn_engines_precompiled" not in dest and "cudnn_adv" not in dest]
```

Rebuild completo pelo spec e repetir o Step 2 no bundle regenerado.

- [ ] **Step 3b (falhou):** restaurar o bundle completo (rebuild sem poda) e registrar no spec da feature que o corte foi testado e rejeitado, com o erro observado.

- [ ] **Step 4: Registrar o resultado** na seção "Empacotamento" do spec (instalador ~390 MB ou ~610 MB) e commitar spec + design doc:

```bash
git add audio-service/build/pyinstaller/audio-service.spec docs/superpowers/specs/2026-08-29-gpu-cpu-transcription-design.md
git commit -m "feat: resultado do experimento de corte das DLLs de CUDA"
```

---

### Task 9: Documentação e registro

**Files:**
- Modify: `.claude/DECISIONS.md` (entrada nova no topo)
- Modify: `.claude/BACKLOG.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: DECISIONS.md** — entrada `[2026-08-29] Instalador embarca CUDA; device de transcrição é escolha do usuário (reverte 2026-08-21)`: contexto (medições 3,2×; pedido do usuário), escolha (instalador único; `whisper_device` auto/cuda/cpu por chamada; fallback por chamada sem estado pegajoso; log do filho em `%LOCALAPPDATA%\meeting-notes\audio-service.log`), justificativa e trade-offs (tamanho do instalador conforme resultado da Task 8; ~465 MB inúteis em máquinas sem NVIDIA; timeout 4h).

- [ ] **Step 2: BACKLOG.md** — remover: o item grande "Instalador transcreve em CPU — empacotar GPU" (implementado; as observações abertas dele — downgrade permanente e log sem destino — foram resolvidas por esta feature), o item "GPU/CPU na transcrição" de Features futuras, e o item do foot-gun do harness (Task 1). Conferir se algum texto de outros itens referencia o CPU-only e ajustar.

- [ ] **Step 3: CLAUDE.md** — a frase "O bundle é **CPU-only** por decisão consciente (ver `.claude/DECISIONS.md`, 2026-08-21)" vira: "O bundle embarca CUDA desde 2026-08-29 (ver DECISIONS.md); o device é escolha do usuário nas Configurações (`whisper_device`), com fallback GPU→CPU por chamada."

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md .claude
git commit -m "docs: registrar a feature GPU/CPU e reverter a decisão CPU-only"
```
