# Whisper "auto" — detecção real de idioma + exibição

**Data:** 2026-07-20
**Tipo:** Fix + pequena feature (persistir + exibir idioma detectado)

## Problema

Com `whisper_language = "auto"`, o áudio é sempre transcrito como português, nunca detectando o idioma de verdade. A cadeia atual:

1. **Go** (`internal/services/orchestrator.go`): quando `whisper_language == "auto"` (ou vazio), `whisperLang` fica `""`.
2. **Go client** (`internal/audio/client.go`): serializa `{"language": ""}` — envia string vazia, não omite o campo. O Python recebe `""`, nunca `None`.
3. **Python** (`audio-service/transcriber.py`): `lang = language or self.default_language` → `"" or "pt"` → `"pt"`. Sempre cai no default, nunca auto-detecta.

Para o faster-whisper detectar, é preciso passar `language=None` ao `model.transcribe()`; ele então detecta e retorna o idioma em `info.language`.

Além disso, o idioma retornado (`info.language`) hoje é **descartado**: o orchestrator só usa `.Transcript`; o model `Meeting` não tem campo de idioma.

## Contexto relevante

- A IA já é agnóstica de idioma: todos os prompts em `internal/ai/*_client.go` dizem *"in the same language as the content"*. Corrigir a transcrição já faz resumo/pontos-chave/tasks saírem no idioma certo — **nenhuma mudança de IA é necessária**.
- Setting `whisper_language` aceita `pt | en | es | auto` (`internal/services/settings_service.go`).
- Há dois caminhos de transcrição no orchestrator (pipeline normal e retranscribe) — ambos precisam do mesmo tratamento.
- Última migration: `013_meeting_keep_audio.sql` → próxima é `014`.
- App single-user; dois entry points (`cmd/api`, `cmd/desktop`) compartilham services/repository.

## Abordagem escolhida

**Normalizar no Python** (dono do modelo). Go encaminha o valor cru da setting (incluindo `"auto"`); o Python mapeia `"auto"`/`""`/`None` → `language=None`. Alternativas descartadas: normalizar no Go (insuficiente sozinho — o `or default_language` do Python permaneceria); remover `default_language` de vez (muda comportamento standalone sem ganho aqui).

## Design

### 1. Correção da detecção (backend)

**Python — `audio-service/transcriber.py`** (`transcribe`):
- Substituir `lang = language or self.default_language` por normalização explícita:
  - `lang = None if language in (None, "", "auto") else language`
- Passar `language=lang` ao modelo (None ⇒ faster-whisper detecta).
- `info.language` continua retornando o idioma real usado/detectado.

**Python — `audio-service/main.py`**: `WHISPER_LANGUAGE` (env `default_language`) passa a default `"auto"` em vez de `"pt"`, para o bug não reaparecer no uso standalone. `default_language` deixa de ser fallback silencioso.

**Go — `internal/services/orchestrator.go`** (ambos os caminhos, ~linha 204 e ~410):
- Passar `s["whisper_language"]` direto (inclusive `"auto"`) em vez de zerar quando `== "auto"`.
- `internal/audio/client.go` não muda (continua mandando o campo `language`).

### 2. Persistir o idioma

- Migration **`014_meeting_language.sql`**: `ALTER TABLE meetings ADD COLUMN language TEXT;` (nullable).
- `internal/models/meeting.go`: `Language *string \`json:"language"\`` na struct `Meeting`.
- `internal/repository`: incluir `language` nos SELECT/INSERT/UPDATE e no scan da `Meeting` (todas as queries que montam a struct).
- `internal/services/orchestrator.go`: após transcrever, `m.Language = &trResp.Language` (ambos os caminhos), persistido no update que já grava o transcript.

### 3. Exibir o idioma (frontend)

- `frontend/src/…` tipo `Meeting`: adicionar `language?: string`.
- Helper `languageLabel(code: string): string` com mapa amigável (`pt→Português`, `en→Inglês`, `es→Espanhol`, `fr→Francês`, `de→Alemão`, `it→Italiano`, `ja→Japonês`, `zh→Chinês` — conjunto pequeno dos comuns) e **fallback `code.toUpperCase()`** para qualquer outro.
- `frontend/src/components/layout/MeetingDetail.tsx` (cluster de badges ~linha 198): novo `<Badge>` ao lado do status, renderizado **somente quando `meeting.language`** existir. Rótulo = `languageLabel(meeting.language)`.

### 4. Edge cases

- Reuniões antigas (`language` NULL) → sem badge. Natural, sem migration de dados.
- Idioma forçado (pt/en/es) → `info.language` ecoa o forçado; badge mostra o mesmo. Consistente.
- Idioma detectado fora do mapa (ex.: `ja`) → badge mostra o código em maiúsculo (`JA`).

## Testes (TDD)

**Python (`audio-service/tests/test_transcriber.py`):**
- `"auto"` → `model.transcribe` chamado com `language=None`.
- `""` e `None` → idem (`language=None`).
- código explícito (`"en"`) → repassado como `language="en"`.
- Atualizar/remover `test_transcribe_uses_default_language_when_none_provided` (trava o comportamento errado atual).

**Go:**
- Orchestrator: `whisper_language="auto"` ⇒ `Transcribe` recebe `"auto"`; `m.Language` persistido a partir de `trResp.Language`.
- Repository: round-trip de `language` (nullable) em SQLite em memória (`t.TempDir()`), incluindo NULL.

**Frontend:**
- `languageLabel`: mapeados retornam o nome; desconhecido retorna código em maiúsculo.

## Fora de escopo

- Override manual de idioma no retranscribe (fica para backlog).
- Bandeiras/emoji no badge.
- Detecção de múltiplos idiomas na mesma reunião.
