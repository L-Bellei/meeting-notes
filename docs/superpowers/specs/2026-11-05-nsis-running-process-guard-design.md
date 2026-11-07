# Proteção do instalador NSIS contra processos ativos

## Contexto

O template customizado `cmd/desktop/build/windows/installer/project.nsi` executa
`taskkill /F` para o executável configurado, `meeting-notes.exe` e `audio-service.exe`
sempre que o instalador inicia. Isso encerra o app à força, mesmo sem consentimento,
e pode interromper uma gravação ou o processamento de uma reunião. O backlog também
registra que uma atualização com o app aberto já deixou arquivos e versão em estados
misturados.

Uma reprodução isolada com um processo descartável chamado `Meeting Notes.exe`
confirmou que o comando atual termina o processo.

## Objetivo

Impedir que instalação, atualização ou desinstalação substitua/remova arquivos enquanto
o Meeting Notes ou seu serviço de áudio estiver em execução, sem finalizar processos
automaticamente.

## Decisões e limites

- Se um processo relacionado estiver ativo, o instalador interativo informa que o usuário
  deve fechar o Meeting Notes e aborta antes de desinstalar a versão anterior ou gravar
  arquivos da nova versão.
- Em execução silenciosa, aborta sem diálogo e não altera arquivos; o instalador retorna
  código de saída não zero.
- Se não for possível consultar a lista de processos, falha de forma fechada; não inicia
  instalação/desinstalação sem confirmar que os processos estão ausentes.
- Verificar os nomes usados pelo instalador atual (`${PRODUCT_EXECUTABLE}`,
  `meeting-notes.exe` e `audio-service.exe`) para cobrir o executável empacotado e o
  nome usado no desenvolvimento.
- Não usar `taskkill` nem encerrar o serviço automaticamente.
- Proteger também a desinstalação direta, além de upgrades, para não remover arquivos
  enquanto qualquer processo do produto está usando-os.
- Não alterar o caminho portátil ZIP, a versão do produto ou a lógica de upgrade quando
  nenhum processo está ativo.
- Uma janela de corrida ainda pode existir se o processo iniciar depois da checagem; o
  instalador repetirá a verificação imediatamente antes da extração/remoção.

## Validação

- Teste NSIS isolado: sem processos do produto, a instalação de teste completa e cria um
  marcador.
- Para cada nome protegido, executar um processo descartável com aquele nome; o instalador
  silencioso deve abortar, não criar o marcador e deixar o processo de teste vivo.
- Compilar um harness isolado com a macro compartilhada e preprocessar o script real com
  NSIS, sem gerar ou publicar uma release.
