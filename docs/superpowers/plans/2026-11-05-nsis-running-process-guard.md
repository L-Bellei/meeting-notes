# Proteção NSIS contra processos ativos — Plano

## Objetivo

Impedir instalações, upgrades e desinstalações com processos Meeting Notes ativos, sem
finalização forçada e sem alterar o fluxo quando o app está fechado.

## Etapas

1. Criar uma macro NSIS reutilizável que consulte `tasklist` através do plugin `nsExec`,
   detecte os executáveis protegidos e falhe fechada em caso de erro.
2. Integrar a proteção em `.onInit` e antes da extração do instalador, e em `un.onInit` e
   antes da remoção do desinstalador. Remover as chamadas `taskkill /F`.
3. Adicionar um teste de integração PowerShell que compile um instalador temporário usando
   a mesma macro e cubra execução normal, cada nome de processo, abort silencioso e
   preservação do processo.
4. Atualizar as decisões e documentação de continuidade/backlog após validação.
5. Revisar o diff, compilar o harness NSIS, preprocessar o script real e executar
   `git diff --check`. Não gerar artefato de release nem criar commit/tag.

## Resultado

Concluído: macro comum para installer/uninstaller, bloqueio em inicialização e antes da
escrita/remoção, sem encerramento forçado. O teste NSIS de integração passou com os três nomes
protegidos e o script real passou pelo preprocessor. Não foi gerado um instalador de release.

## Critérios de aceite

- Nenhum caminho de instalação ou remoção encerra o Meeting Notes/audio-service.
- Processos ativos bloqueiam antes de qualquer mutação de arquivos do produto.
- Modo silencioso bloqueia sem UI; o instalador retorna falha.
- Erro na consulta de processos bloqueia a operação.
- Sem processos ativos, o installer e uninstaller mantêm o fluxo funcional.
