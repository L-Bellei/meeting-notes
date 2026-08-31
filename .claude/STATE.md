# Estado do Projeto — 2026-08-30 (fim de sessão)

## Sessão
- **Data:** 2026-08-30
- **`master` (`455893c`) = v2.10.0 publicada.** Feature da sessão: transcrição em **GPU AMD/Intel
  via whisper.cpp/Vulkan** como segundo motor (CUDA intacto para NVIDIA). PR #56, tag `v2.10.0`,
  instalador de 648 MB na Release. Homologado por tester externo antes do merge.
- **Worktree:** nenhum ativo. Branch `feat/vulkan-transcription` apagada no merge.

## Fase Superpowers

**Nenhum ciclo em andamento.** O último (`docs/superpowers/plans/2026-08-30-vulkan-transcription.md`,
spec `docs/superpowers/specs/2026-08-30-amd-gpu-vulkan-transcription-design.md`) foi completo:
10 tasks via Subagent-Driven Development (5 implementers + reviews por task em paralelo com
arquivos disjuntos), spike interativo do binário Vulkan, review final whole-branch (Fable) com
uma fix wave verificada. Workspace do SDD apagado; o registro é o git.

## Próximo passo imediato

Nenhum acordado. Candidatas no BACKLOG (Features futuras): **Notificações de pipeline** e
**Export** — ambas começam por `/superpowers:brainstorming`. Débito novo mais relevante:
**homologação em GPU AMD real** (a feature foi validada forçando Vulkan na RTX 2050 +
tester externo; ver BACKLOG).

## Estado de release

- **v2.10.0** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.10.0
  — instalador de **648 MB** (+17 MB vs v2.9.0: whisper-cli + DLLs ggml/Vulkan).
- **`master` está em paridade com a última release** (docs de sessão pendentes neste commit).
- Upgrade da v2.9.0: migration 020 converte `whisper_device` `cuda`→`gpu` — sem quebra.
- A corrida push→merge da v2.9.0 não se repetiu: o bump foi commitado ANTES de abrir o PR #56
  e conferido no HEAD pós-merge.

## Armadilhas de ambiente

Todas as anteriores no `CLAUDE.md` ("Rodando em dev") e DECISIONS. Novas desta sessão:
- **Máquina de build ganhou toolchain C++** (2026-08-30, via winget): CMake 4.4.3, Vulkan SDK
  1.4.357.0, VS Build Tools 2022 (VCTools). São pré-requisito do `fetch-whispercpp.ps1`.
  `VULKAN_SDK` e o CMake podem não estar no PATH de shells antigas — abrir shell nova.
- **NSIS não está no PATH** desta máquina: o `build.ps1` falha no pre-flight; prefixar
  `$env:Path += ";C:\Program Files (x86)\NSIS"`.
- **Testes Python são herméticos por construção**: com `whisper-cli.exe` em `vendor/` e o GGML
  no cache HF, qualquer teste que construa `Transcriber` sem patch de `find_whispercli` (ou sem
  `vulkan=` fake) volta a depender do host — foi um fix round real desta sessão (`49f0d1d`).
- `WHISPER_FORCE_BACKEND=vulkan` é só para dev/homologação; deixá-la setada na shell quebra
  testes de cadeia e muda o comportamento do app em dev.
