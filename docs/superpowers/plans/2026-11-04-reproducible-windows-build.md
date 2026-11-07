# Build Windows reproduzível — Plano

## Objetivo

Corrigir detecção de pré-requisitos e bloquear artefatos incompletos no caminho
canônico `build.ps1`, preservando a geração do instalador e do executável portátil.

## Alterações

1. **Preflight**
   - Resolver Wails, Go e npm e reportar ausências de forma objetiva.
   - Resolver NSIS pelo `PATH` ou pelo local padrão; acrescentar o diretório ao
     `PATH` somente no processo atual.
   - Validar o executável principal do bundle PyInstaller e o whisper.cpp dentro
     dele, com mensagens reproduzíveis se estiverem ausentes.
   - Adicionar `-PreflightOnly`, sem alterar arquivos ou executar build/testes.

2. **Empacotamento completo**
   - Manter cópia do bundle e smoke test antes de qualquer coleta nos dois modos.
   - Exercitar `whisper-cli.exe --help` para verificar carregamento das DLLs do
     bundle antes do smoke test HTTP do serviço.
   - Fazer `-NoNSIS` pular somente a chamada NSIS e gerar um ZIP portátil com
     `meeting-notes.exe` e `audio-service/` adjacentes.
   - Criar o artefato sob nome temporário, validar tamanho/conteúdo e então mover
     ao nome final, sem expor ZIP parcial.

3. **Documentação**
   - Atualizar `README.md` para apontar ao caminho canônico da raiz.
   - Ajustar `CLAUDE.md` com o preflight sem build e o contrato de `-NoNSIS`.
   - Registrar decisão, resultado e próximo passo nos arquivos `.claude/` e no
     roadmap.

## Verificação

- Rodar `build.ps1 -PreflightOnly` no ambiente atual: deve reconhecer NSIS instalado
  fora do `PATH`, verificar assets e não alterar `wails.json`.
- Rodar preflight em cópia temporária sem os assets: deve falhar com instruções
  acionáveis antes de testar/compilar, inclusive com `-NoNSIS`.
- Confirmar que `-NoNSIS` não desativa os gates de bundle e smoke por inspeção do
  fluxo e validação apropriada; não usar artefatos parciais como entrega.
- Executar testes Go e Python pertinentes, `go vet ./...` e `git diff --check`.
- Não publicar artefatos nem iniciar geração de uma release.
