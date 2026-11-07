# Build Windows reproduzível — Design

## Contexto

A Etapa 4 do roadmap exige um preflight que explique dependências ausentes e gates
que comprovem os componentes empacotados. A inspeção e as reproduções no checkout
confirmaram dois defeitos no `build.ps1`:

1. `makensis.exe` instalado no diretório padrão do NSIS não é detectado quando esse
   diretório não está no `PATH`; o script interrompe o preflight embora a ferramenta
   exista na máquina.
2. `-NoNSIS` contorna as verificações, cópia e smoke test do audio-service. Uma
   reprodução isolada concluiu com sucesso e coletou um `.exe` de zero bytes sem
   nenhum bundle de áudio.

## Objetivos

- Explicar e validar os pré-requisitos necessários antes de iniciar testes ou build.
- Reconhecer o NSIS no `PATH` ou em sua localização padrão, tornando-o disponível
  aos comandos Wails sem alterar configurações persistentes da máquina.
- Exigir o bundle executável do audio-service e seu `whisper-cli.exe` empacotado
  tanto para instalador quanto para o executável portátil.
- Executar a cópia e o smoke test do serviço em ambos os modos; `-NoNSIS` deve
  omitir apenas a geração do instalador.
- Executar `whisper-cli.exe --help` a partir do bundle copiado, validando que o
  executável e suas DLLs carregam sem depender de GPU ou modelo.
- Distribuir o modo portátil como ZIP contendo o executável Wails e a pasta
  `audio-service` adjacente, exigida pelo launcher em produção.
- Oferecer `-PreflightOnly` para verificar prontidão sem alterar versão, testar ou
  compilar.
- Recusar a coleta de artefatos vazios.

## Fora de escopo

- Não reinstalar ferramentas, baixar dependências ou compilar o bundle PyInstaller
  ou o whisper.cpp automaticamente.
- Não executar nem publicar uma release como parte desta etapa.
- Não alterar a arquitetura dos componentes, os requisitos de GPU/modelos ou o
  comportamento da aplicação.
- Não exigir CMake, Vulkan SDK ou Visual Studio para a build normal quando o
  bundle Vulkan já está presente; esses recursos só são necessários para regenerá-lo.

## Decisões de implementação

- O preflight verifica Wails, Go e npm, além dos assets de áudio. NSIS é obrigatório
  apenas no modo instalador. Ausência de assets aborta com os caminhos e comandos
  canônicos de reconstrução, sem instalar substitutos.
- Para NSIS, primeiro usar `Get-Command makensis`; caso não exista, procurar o
  caminho padrão de instalação e acrescentar sua pasta ao `PATH` apenas no processo
  atual. Caso não seja encontrado, indicar `-NoNSIS` como alternativa sem NSIS,
  mantendo os gates do audio-service.
- O preflight roda antes de persistir `-Version`, executar testes ou compilar.
  `-PreflightOnly` termina imediatamente após as verificações.
- A cópia e o smoke test do audio-service ocorrem depois da build limpa e antes da
  geração NSIS/coleta. O modo portátil percorre os mesmos gates e gera um ZIP
  autônomo, com a estrutura de arquivos que o app espera.
- Antes do smoke `/health`, o build executa `whisper-cli.exe --help` no bundle
  copiado. Isso testa o carregamento do binário/DLLs, sem alegar validação do
  desempenho Vulkan ou de uma transcrição em GPU.
- A coleta exige que o arquivo final exista e tenha tamanho maior que zero; o ZIP
  também é inspecionado para confirmar os dois executáveis esperados.
- O artefato é criado em arquivo temporário no mesmo diretório e movido ao nome
  final somente após validação, preservando eventual release anterior em caso de falha.

## Validação

- A reprodução do NSIS instalado fora do `PATH` falhou no preflight atual.
- Um harness isolado com Wails simulado reproduziu sucesso de `-NoNSIS` sem bundle
  e coleta de um executável vazio.
- Após a implementação: validar `-PreflightOnly` sem modificar `wails.json`, cobrir
  o caso de NSIS instalado fora do `PATH`, validar erro claro quando faltam assets e
  executar os testes Go/Python relevantes. A build/installer completo só deve ser
  executado quando for necessário validar uma entrega de release.
