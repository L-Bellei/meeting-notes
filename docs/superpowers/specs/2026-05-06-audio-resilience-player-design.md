# Audio Resilience, Player & Transcription Diagnostics — Design Spec

**Data:** 2026-05-06
**Status:** Aprovado para implementação

---

## Contexto

Após o fix do Whisper (v2.2.5), a transcrição passou a falhar com `status=failed` sem transcript. O caminho do WAV nunca é persistido no banco, então a app perde a referência ao arquivo e não há como fazer retry. Este spec adiciona resiliência ao pipeline de transcrição e player de áudio com visualizador de espectro.

---

## Objetivo

1. **Resiliência de transcrição** — nunca perder o áudio gravado; sempre permitir retry de transcrição enquanto o WAV existir.
2. **Diagnóstico de falhas** — mostrar a mensagem de erro real do Whisper na UI em vez de apenas "failed".
3. **Validação preventiva** — verificar saúde do modelo e integridade do WAV antes de tentar transcrever.
4. **Player de áudio** — widget flutuante com controles e visualizador de espectro (design opção B aprovado).
5. **Opção keep_audio** — configuração global para preservar o WAV após transcrição bem-sucedida.

---

## Decisões de Arquitetura

**Approach A — WAV permanece no dir do audio-service.**
O arquivo fica em `recordings/` do audio-service. O Go backend armazena o path no banco e serve o arquivo diretamente via HTTP. Retry chama `/transcribe` no audio-service (o arquivo já está no recordings dir). Não há cópia nem movimentação de arquivos.

**Keep/delete policy:**
- Após **falha** na transcrição: WAV **nunca** é deletado (independente de `keep_audio`), para que retry seja sempre possível.
- Após **sucesso** na transcrição: deletar WAV somente se `keep_audio = false`. Se `keep_audio = true`, manter indefinidamente.

---

## Schema — Migrations

### Migration 011 — `audio_path` e `error_message` em meetings

```sql
ALTER TABLE meetings ADD COLUMN audio_path TEXT;
ALTER TABLE meetings ADD COLUMN error_message TEXT;
```

`audio_path` é setado imediatamente após `StopRecording` e limpo se o WAV for deletado com sucesso (quando `keep_audio=false` após transcrição bem-sucedida).

`error_message` é sobrescrito a cada tentativa de transcrição, guardando o último erro.

### Migration 012 — setting `keep_audio`

Inserir `keep_audio` na tabela de settings com valor padrão `false`.

---

## Backend

### Orchestrator — `RunCapturePipeline` (revisado)

```
1. StopRecording()           → obter stopResp.Path
2. Salvar audio_path = stopResp.Path no banco         ← NOVO (antes de qualquer falha)
3. Pre-flight: health check do audio-service           ← NOVO
   - GET /health → model_loaded
   - Se false: salvar error_message, marcar failed, return
4. Pre-flight: validar WAV                             ← NOVO
   - Verificar que arquivo existe e size > 10 KB
   - Se inválido: salvar error_message, marcar failed, return
5. Transcrever
   - Se erro: salvar error_message no banco, marcar failed, return (WAV mantido)
6. Salvar transcript, duration; marcar status=processing
7. Deletar WAV somente se keep_audio = false           ← MODIFICADO
   - Se deletado com sucesso: limpar audio_path no banco
8. Gerar AI (se auto_generate)
9. Marcar completed
```

### Novo endpoint — `POST /api/meetings/{id}/retranscribe`

Disponível apenas quando `meeting.audio_path != nil` e `meeting.status == failed`.

```
1. Verificar que audio_path existe no banco
2. Verificar que o arquivo existe em disco
   - Se não existe: retornar 409 com mensagem clara ("arquivo de áudio não encontrado")
3. Spawnar goroutine com RunRetranscribePipeline(meetingID, audio_path)
   - Marcar status=transcribing
   - Pre-flight checks (igual ao pipeline principal)
   - Transcrever
   - Se erro: salvar error_message, marcar failed
   - Se sucesso: salvar transcript, deletar WAV se keep_audio=false, prosseguir para AI
```

### Novo endpoint — `GET /api/meetings/{id}/audio`

Serve o arquivo WAV diretamente do path em `meeting.audio_path`.

- Retorna 404 se `audio_path` é nulo ou arquivo não existe em disco.
- Usa `http.ServeContent` com headers de Range para suporte a seek nativo do `<audio>`.
- Content-Type: `audio/wav`.

### Settings — `keep_audio`

Novo campo booleano no model de Settings. Default: `false`. Lido durante o pipeline para decidir se deleta o WAV após transcrição bem-sucedida.

### Pre-flight checks — `internal/services/transcription_checks.go`

```go
func checkAudioServiceHealth(ctx context.Context, audioClient AudioClient) error
func validateWAVFile(path string) error  // size > 10KB, arquivo existe
```

Retornam erros descritivos em português para serem salvos em `error_message`.

---

## Frontend

### Settings — toggle `keep_audio`

Nova linha em `SettingsModal.tsx` (nova seção "Áudio"):

```
[ ] Guardar áudio das reuniões
    Preserva o arquivo de áudio após transcrição para reprodução posterior.
    Desativado: áudio deletado após transcrição bem-sucedida.
```

### MeetingDetail — header (quando `audio_path` existe)

Adicionar ícone 🔊 como `IconButton` no header da reunião. Clique alterna o estado `playerOpen` (context ou useState local).

### MeetingDetail — estado `failed` (revisado)

Quando `status === "failed"`:

1. **Mensagem de erro** — mostrar `meeting.error_message` abaixo do header em destaque vermelho (substituir o estado genérico atual).
2. **Botão retry** — "Tentar transcrever novamente" visível somente quando `meeting.audio_path` não é nulo. Dispara `POST /api/meetings/{id}/retranscribe`.

### Componente `AudioPlayer`

**Localização:** `frontend/src/components/ui/AudioPlayer.tsx`

Widget flutuante posicionado `fixed bottom-4 right-4 z-40`. Props:

```ts
interface AudioPlayerProps {
  meetingId: string
  meetingTitle: string
  onClose: () => void
}
```

Internamente usa `<audio>` com `src={apiUrl}/api/meetings/{meetingId}/audio` e controla via `useRef<HTMLAudioElement>`.

**Controles (centralizados):**
- Barra de seek clicável com thumb e preenchimento proporcional ao progresso
- Tempo atual / duração total
- Botão −15s, botão Play/Pause (principal, maior), botão +15s — todos alinhados ao centro
- Seção inferior separada por divisor: `AudioSpectrumVisualizer`

**Comportamento:**
- Persist entre navegação de reuniões (manter em contexto global ou portal React)
- Fechar com X ou quando o áudio termina

### Componente `AudioSpectrumVisualizer`

**Localização:** `frontend/src/components/ui/AudioSpectrumVisualizer.tsx`

Props:

```ts
interface AudioSpectrumVisualizerProps {
  audioRef: RefObject<HTMLAudioElement>
  playing: boolean
}
```

Usa Web Audio API:
```
AudioContext → createMediaElementSource(audioRef) → AnalyserNode → gainNode → destination
```

Canvas renderiza `AnalyserNode.getByteFrequencyData()` como barras verticais animadas (cores `#4f46e5` → `#818cf8`). `requestAnimationFrame` ativo somente quando `playing === true`.

**Nota de implementação:** `createMediaElementSource` deve ser chamado uma única vez por elemento de áudio (idempotente via ref). O nó de gain evita dupla saída de áudio ao conectar o analyser.

---

## Investigação — Falha de Transcrição (v2.2.5)

A causa provável é `vad_filter=True` no PyInstaller bundle: o modelo Silero VAD não está incluído no `.spec` atual (`excludes` lista `onnxruntime` mas não inclui explicitamente os dados do Silero). Como ação imediata durante a implementação: remover `vad_filter=True` do `transcriber.py` e adicionar `silero-vad` aos `datas` do spec apenas se testado e confirmado funcional no bundle. Os demais parâmetros (`condition_on_previous_text=False`, `repetition_penalty=1.1`, `compression_ratio_threshold=1.8`) devem ser mantidos.

---

## Testes

### Go

- `TestRetranscribeHandler_NoAudioPath` — 409 quando audio_path é nulo
- `TestRetranscribeHandler_FileMissing` — 409 quando arquivo não existe em disco
- `TestAudioServeHandler_NotFound` — 404 quando audio_path nulo
- `TestCheckAudioServiceHealth_ModelNotLoaded` — retorna erro descritivo
- `TestValidateWAVFile_TooSmall` — retorna erro para arquivos < 10 KB

### TypeScript

- `AudioPlayer` renderiza controles centralizados
- Botão retry visível somente quando `audio_path != null && status === "failed"`
- Botão retry não visível quando `audio_path === null`

---

## File Map

| Arquivo | Ação |
|---|---|
| `internal/database/migrations/011_audio_fields.sql` | Criar |
| `internal/database/migrations/012_keep_audio_setting.sql` | Criar |
| `internal/services/transcription_checks.go` | Criar |
| `internal/services/transcription_checks_test.go` | Criar |
| `internal/services/orchestrator.go` | Modificar |
| `internal/handlers/audio_serve_handler.go` | Criar |
| `internal/handlers/retranscribe_handler.go` | Criar |
| `internal/handlers/retranscribe_handler_test.go` | Criar |
| `cmd/desktop/app.go` | Modificar (registrar rotas) |
| `cmd/api/main.go` | Modificar (registrar rotas) |
| `audio-service/transcriber.py` | Modificar (remover vad_filter) |
| `frontend/src/components/ui/AudioPlayer.tsx` | Criar |
| `frontend/src/components/ui/AudioSpectrumVisualizer.tsx` | Criar |
| `frontend/src/components/layout/MeetingDetail.tsx` | Modificar |
| `frontend/src/components/settings/SettingsModal.tsx` | Modificar |
