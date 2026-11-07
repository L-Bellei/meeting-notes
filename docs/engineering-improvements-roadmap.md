# Roadmap de melhorias de engenharia

**Status:** proposta de priorização, não é plano de implementação aprovado.
**Origem:** análise de engenharia e segurança registrada na sessão de 2026-11-04.
**Princípio:** confirmar cada hipótese no código e com testes antes de alterar comportamento. Cada etapa deve passar pelo workflow Superpowers: brainstorm → plan → implement → review → finish.

## Etapa 1 — Consistência e concorrência do pipeline

**Status:** concluída em 2026-11-04.

Investigar as transições de estado entre gravação, transcrição, geração de resumo/pontos-chave/tasks e falhas. Mapear concorrência, repetição de operações, cancelamento e encerramento do app; identificar onde resultados atrasados ou duplicados poderiam sobrescrever estado mais recente.

**Resultado esperado:** riscos reproduzíveis e priorizados, invariantes de estado definidos, testes de regressão e um plano de implementação com limites claros. Não presumir que os riscos apontados na auditoria ainda são bugs ativos.

## Etapa 2 — Migrations e ciclo de vida do áudio

**Status:** concluída em 2026-11-04.

Investigar a atomicidade e recuperação de migrations SQLite, incluindo o estado do banco após uma falha parcial. Rever também a finalização, recuperação e limpeza de gravações/arquivos temporários em falhas, reinício ou encerramento inesperado.

**Resultado esperado:** comportamento de falha documentado, estratégia de recuperação compatível com bancos existentes e testes para falha/interrupção. Evitar migrations destrutivas ou limpeza que possa apagar áudio válido.

## Etapa 3 — Lifecycle de tasks e indexação

**Status:** concluída em 2026-11-04.

Rastrear a criação e atualização de tasks derivadas das reuniões e a indexação/pesquisa dos dados. Verificar se falhas de persistência ou indexação são observáveis, recuperáveis e consistentes entre as telas e as fontes de dados.

**Resultado esperado:** contratos e estados de erro explícitos, caminho seguro de repetição/reconstrução quando aplicável e testes de integração que cubram consistência e falha.

## Etapa 4 — Build e validação reproduzíveis

**Status:** concluída em 2026-11-04.

Rever os pré-requisitos do build Windows, detecção de ferramentas/assets, bundle PyInstaller, binário whisper.cpp/Vulkan e validações do instalador. Diferenciar verificações que cabem em CI das que exigem hardware ou toolchain local; preservar `build.ps1` como caminho canônico da release.

**Resultado esperado:** preflight que explique dependências ausentes, instruções reproduzíveis e gates que comprovem o funcionamento dos componentes empacotados. Não publicar artefatos sem smoke test e não exigir reinstalação redundante de recursos já disponíveis.

**Resultado:** `build.ps1 -PreflightOnly` valida ferramentas/assets sem compilar e detecta NSIS no
local padrão. Ambos os formatos verificam o carregamento do whisper.cpp e executam smoke HTTP do
audio-service. `-NoNSIS` gera ZIP portátil completo; artefatos vazios são rejeitados. Validados
`go test ./...`, `go vet ./...`, 99 testes Python, preflight com e sem assets e layout do ZIP em
temporário. A coleta usa um nome temporário e só publica o nome final após validar o conteúdo.
Nenhum installer ou release foi gerado nesta etapa.

## Etapa 5 — Resiliência e acessibilidade do frontend

**Status:** concluída em 2026-11-04.

Reavaliar estados de erro/carregamento e a cobertura de testes de componentes. Investigar acessibilidade de modais e interações por teclado, além de otimizações de bundle apenas quando medidas justificarem a mudança.

**Resultado esperado:** melhorias priorizadas por impacto, testes que cubram regressões relevantes e validação manual das interações nativas quando testes automatizados não forem suficientes.

**Resultado:** todos os diálogos reais foram migrados para o componente compartilhado com
semântica ARIA, teclado e gerenciamento de foco; a confirmação de coluna deixou de criar overlay
aninhado. Busca e detalhe de card anunciam falha, e o detalhe permite retry. Os formulários e
controles relevantes receberam rótulos acessíveis; carregamento e idioma do documento foram
corrigidos. A suíte de UI passou (7 testes) e o build frontend passou. Após a etapa inicial,
ESLint foi corrigido sem desativar regras, e agora passa sem erros ou warnings. A auditoria de
dependências de desenvolvimento também foi resolvida com a atualização do Vitest; `npm audit`
e `npm audit --omit=dev` reportam zero vulnerabilidades. O build mantém o aviso de chunk
JavaScript acima de 500 kB. Specs e planos em `docs/superpowers/`.

## Sequenciamento e limites

- Avançar uma etapa por vez; não executar todas as etapas em um único lote.
- Usar as conclusões das Etapas 1–3 para determinar as correções necessárias, sem tratar a auditoria como especificação fechada.
- Cada etapa deve ter brainstorm/spec e plano próprio antes da implementação.
- Mudanças de banco, dados de usuário, contratos de API e comportamento de fallback exigem revisão explícita de compatibilidade.
- A entrega da release 2.10.1 é uma pendência separada deste roadmap.
