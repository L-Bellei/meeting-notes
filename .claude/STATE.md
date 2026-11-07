# Estado do Projeto — 2026-11-07

## Sessão
- **`master` (`8d82b4a`)** contém o PR #59 e as melhorias de engenharia.
- **v2.10.1 publicada** com o instalador `meeting-notes-2.10.1-windows-amd64-installer.exe`.
- **v2.11.0 publicada** com o instalador `meeting-notes-2.11.0-windows-amd64-installer.exe`
  (648,6 MB; SHA-256 `65E4C7E03BF17922B6CA2938984F201A3B4ED87E0D56FEFD5A3BBAE3F3DDD541`).
- Etapas 1–5, proteção NSIS e reparent de temas por teclado estão mergeados em `master`
  pelo PR #59.
- Os logs locais vazios do StackQL foram preservados fora do repositório, em `files/stackql-logs/`.

## Fase Superpowers

**Etapas 1–5, proteção NSIS e reparent de temas por teclado concluídos** pelo ciclo brainstorm → plan → implement → review → finish.

## Próximo passo imediato

Iniciar o brainstorm da feature de notificações nativas do Windows quando o processamento de
uma reunião termina; ela consta como acordada no backlog, mas ainda não foi especificada.

## Release v2.11.0

- PR #59 mergeado em `8d82b4a`; tag `v2.11.0` e release publicadas no GitHub.
- Build canônico `build.ps1` passou; `whisper-cli.exe --help` e o `/health` HTTP 200 do
  audio-service empacotado passaram (43 s). Instalador reporta FileVersion/ProductVersion 2.11.0.
- Validação adicional: `go test ./...`, 99 testes Python, ESLint, 11 testes frontend,
  build frontend e `git diff --check` passaram.
- A build emitiu o aviso conhecido de diferença entre Wails CLI 2.17.1 e runtime 2.12.0;
  a compilação e o smoke test concluíram com sucesso.

## Reorganização de temas por teclado

- Temas movíveis agora têm um handle de arraste com nome acessível e foco por teclado; temas
  com subcategorias continuam sem handle.
- `KeyboardSensor` percorre somente os destinos válidos (raiz e temas de primeiro nível) com
  as setas para cima/baixo. Espaço/Enter iniciam e soltam; Tab solta; Escape cancela.
- Instruções e anúncios de início, destino, soltura e cancelamento estão em português. O
  `PointerSensor`, a mutation e as regras da API permaneceram inalterados.
- Validação: 4 testes focados e 11 testes frontend passaram; lint e build passaram. Build ainda
  avisa que o chunk JavaScript excede 500 kB. `git diff --check` passou.
- Spec e plano: `docs/superpowers/specs/2026-11-06-keyboard-theme-reparent-design.md` e
  `docs/superpowers/plans/2026-11-06-keyboard-theme-reparent.md`.

## Etapa 1 — consistência do pipeline

- Auditoria reproduziu duas falhas: reprocessamento duplicado durante geração de IA e sobrescrita
  de alterações recentes por persistência de snapshot obsoleto.
- Implementados reserva por reunião, transições condicionais, gravação por campos, rejeição de
  alterações concorrentes em entradas/resultados gerados e cancelamento/junção dos pipelines no
  encerramento dos dois entry points.
- Encerramento preserva áudio disponível e mantém operações interrompidas em estado recuperável;
  indexação de busca deixou de usar goroutines não rastreadas.
- `go test ./...` passou; `npm run build` passou (com aviso preexistente de chunk acima de 500 kB).
- Não houve alteração de etapas posteriores nem commit.

## Etapa 2 — migrations e ciclo de vida do áudio

- Cada migration e seu registro no ledger agora são aplicados em uma transação. Teste induz falha
  após um statement, confirma rollback do schema/ledger e confirma que a migration pode ser tentada
  novamente.
- Migration aditiva 021 persiste `recording_id`; o ID percorre repositório, cliente de áudio e
  orchestrator para recuperação restrita à gravação correspondente. Arquivos inválidos ou
  desconhecidos não são excluídos nem associados automaticamente.
- O gravador preserva estado após falha ao parar, permite recuperar arquivo validado por UUID e o
  audio-service tenta finalizar a gravação no shutdown. Limpeza do áudio primeiro remove a
  referência persistida e só então remove o arquivo.
- Validação: `go test ./...`, `go vet ./...` e `pytest tests -q` em `audio-service` (99 testes)
  passaram. Build do frontend não foi repetido nesta etapa; a Etapa 2 não alterou a UI.
- Spec e plano: `docs/superpowers/specs/2026-11-04-database-audio-lifecycle-design.md` e
  `docs/superpowers/plans/2026-11-04-database-audio-lifecycle.md`.

## Etapa 3 — lifecycle de tasks e indexação

- Testes reproduziram perda das tasks anteriores quando a inserção de uma substituição gerada falha,
  ausência de atualização da busca após CRUD manual de tasks e sucesso de alteração de reunião
  quando o índice FTS está indisponível.
- Substituição de tasks geradas agora é transacional. Migration 022 cria uma view da fonte canônica,
  triggers para meetings/summaries/key_points/tasks e reconstrói o FTS existente.
- Removidas atualizações best-effort redundantes no MeetingService/Orchestrator e a escrita direta
  arbitrária no SearchRepository. Erros FTS agora abortam a operação SQLite de origem.
- Testes verificam CRUD e busca, rollback diante de índice indisponível e rebuild que remove
  resultados obsoletos sem alterar os dados-fonte.
- Validação: `go test ./...` e `go vet ./...` passaram. Spec e plano em
  `docs/superpowers/specs/2026-11-04-task-index-consistency-design.md` e
  `docs/superpowers/plans/2026-11-04-task-index-consistency.md`.

## Etapa 4 — build e validação reproduzíveis

- Antes das alterações, o preflight rejeitou NSIS instalado em `C:\Program Files (x86)\NSIS`
  porque ele não estava no `PATH`. Harness isolado com Wails simulado confirmou que
  `-NoNSIS` podia coletar executável de zero bytes sem bundle de áudio.
- `build.ps1 -PreflightOnly` agora verifica Wails, Go, Node/npm, assets de áudio e NSIS
  quando necessário, sem alterar versão ou iniciar testes/build. O NSIS padrão é detectado
  e adicionado somente ao `PATH` do processo atual.
- Ambos os modos exigem o bundle PyInstaller e whisper-cli; depois da cópia, o build valida
  `whisper-cli.exe --help` (carregamento de DLLs, sem alegar teste de GPU) e o serviço
  empacotado respondendo HTTP 200 em `/health`. `-NoNSIS` agora gera ZIP portátil com
  `meeting-notes.exe` e `audio-service/`; coleta rejeita artefatos vazios e valida as entradas.
- A coleta monta o arquivo com nome temporário e só o move ao nome final após a validação.
- Verificações: preflight padrão passou e manteve `wails.json` inalterado; preflight isolado
  sem bundle abortou antes de criar `dist/`; layout do ZIP validado em temporário;
  `go test ./...`, `go vet ./...` e `pytest tests -q` (99 testes) passaram. Nenhuma build
  de installer ou publicação foi feita nesta etapa.
- Spec e plano: `docs/superpowers/specs/2026-11-04-reproducible-windows-build-design.md` e
  `docs/superpowers/plans/2026-11-04-reproducible-windows-build.md`.

## Etapa 5 — resiliência e acessibilidade do frontend

- Criado `Dialog` compartilhado e migrados Search, Settings, Recording, Stop confirmation,
  Theme edit, Manual card, Column settings e Card detail; todos têm nome acessível, foco inicial,
  contenção de Tab, Escape e restauração de foco. O fechamento por backdrop foi preservado apenas
  nos diálogos que já o permitiam.
- A confirmação de exclusão de coluna agora é inline, sem focus trap aninhado. Campos e botões
  relevantes receberam rótulos acessíveis; a busca, Settings e detalhe de card anunciam erros.
  Detalhe de card também oferece retry, e o carregamento inicial usa região viva.
- `frontend/index.html` declara `pt-BR` e título Meeting Notes. Adicionados Vitest, jsdom e
  React Testing Library; testes cobrem nome/erro de busca, erro/retry do card, teclado, Escape,
  backdrop e restauração de foco.
- Verificações: `npm --prefix frontend test` (7 testes) e `npm --prefix frontend run build`
  passaram. Build mantém o aviso de chunk acima de 500 kB. `git diff --check` passou.
- Na validação inicial desta etapa, ESLint reportou 39 erros e 1 warning; as correções posteriores
  estão registradas em “Lint do frontend” abaixo. As vulnerabilidades reportadas na instalação
  foram investigadas e resolvidas em “Auditoria de dependências do frontend”.
- Spec e plano: `docs/superpowers/specs/2026-11-04-frontend-resilience-accessibility-design.md`
  e `docs/superpowers/plans/2026-11-04-frontend-resilience-accessibility.md`.

## Auditoria de dependências do frontend

- Auditoria inicial reportou 3 vulnerabilidades somente em dependências de desenvolvimento:
  2 críticas no `vitest`/`tinypool` e 1 moderada no `@vitest/mocker`. `npm audit --omit=dev`
  não encontrou vulnerabilidades.
- Com aprovação do usuário, `vitest` foi atualizado de 3.2.x para 5.1.0; os avisos ficaram
  resolvidos sem alterações de dependências de produção.
- Verificações após a atualização: `npm audit` e `npm audit --omit=dev` reportam zero
  vulnerabilidades; 7 testes de UI e build frontend passaram. O aviso de chunk >500 kB persiste.

## Lint do frontend

- ESLint foi zerado sem desativar regras: bindings Wails gerados foram excluídos; helpers de
  tarefas foram separados dos componentes; os componentes passaram a derivar estado de props/
  consultas em vez de sincronizá-lo por effects, e tipos/callbacks foram ajustados para remover
  `any` e leituras inválidas de refs.
- Verificações finais: `npm --prefix frontend run lint`, `npm --prefix frontend test`
  (7 testes), `npm --prefix frontend run build` e `git diff --check` passaram. O build mantém
  somente o aviso do chunk JavaScript acima de 500 kB.
- Roadmap concluído; nenhuma outra etapa está autorizada ou pendente. Não houve commit.

## Proteção do instalador NSIS

- Reprodução isolada com um executável descartável confirmou que o `taskkill /F` anterior
  encerrava o processo correspondente. As chamadas de encerramento forçado foram removidas.
- Instalador e desinstalador consultam `${PRODUCT_EXECUTABLE}`, `meeting-notes.exe` e
  `audio-service.exe`; com processo ativo, abortam antes da seção de escrita/remoção. O modo
  silencioso não mostra UI e o instalador retorna código não zero. Falhas na consulta são
  tratadas como bloqueio.
- Teste de integração `cmd/desktop/build/windows/installer/test-process-guard.ps1` passou:
  instalação/desinstalação silenciosas sem processos prosseguem; cada nome protegido bloqueia
  ambas e o processo sintético permanece ativo. `makensis /PPO` também processou o script real.
- Spec/plano: `docs/superpowers/specs/2026-11-05-nsis-running-process-guard-design.md` e
  `docs/superpowers/plans/2026-11-05-nsis-running-process-guard.md`. Não foi gerado artefato
  de release nem feito commit.

## Estado da release

- **v2.10.1 publicada:** https://github.com/L-Bellei/meeting-notes/releases/tag/v2.10.1
- Instalador: **648,6 MB**, SHA-256 `EB000EE239E4FFD28FFBD3B0E529EAF68F79A3FDB039F940854F870238711C6F`.
- `build.ps1` passou nos testes Go, no smoke test do `audio-service.exe` empacotado (`/health` HTTP 200 em 28s) e na geração NSIS. O instalador reporta FileVersion/ProductVersion `2.10.1`.
- A build falhava na análise de tipos do Wails CLI 2.12.0 (`golang.org/x/tools v0.30.0`) com Go 1.27.0, embora `go test ./cmd/desktop` passasse. Recompilar o mesmo CLI não resolveu.
- Wails CLI 2.17.1 (`x/tools v0.49.0`) passou a geração de bindings e a build. Foi instalado na máquina; o CLI 2.12.0 anterior foi preservado em backup fora do repositório.
- `go.mod` do app permanece em Wails v2.12.0. O CLI 2.17.1 emite aviso de diferença de versão, mas a build completa e o smoke passaram; não foi feito upgrade do runtime do app.

## Armadilhas de ambiente

- A máquina de build tem CMake, Vulkan SDK e VS Build Tools 2022, necessários para `fetch-whispercpp.ps1`.
- NSIS existe em `C:\Program Files (x86)\NSIS`, mas não fica no PATH padrão; `build.ps1` o detecta nesse caminho e o adiciona apenas ao processo de build.
- Para build com Go 1.27.0, usar Wails CLI 2.17.1 ou posterior; o CLI 2.12.0 com `x/tools` 0.30.0 falha na geração de bindings.
- Testes Python devem isolar `find_whispercli`/`vulkan` para não depender de binários e cache locais.
- `WHISPER_FORCE_BACKEND=vulkan` é somente para desenvolvimento/homologação; não deixá-la setada ao rodar testes de cadeia.
