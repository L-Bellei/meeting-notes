# Resiliência e acessibilidade do frontend — Design

## Contexto

A auditoria da Etapa 5 encontrou superfícies de modal inconsistentes. `CardDetailModal`
era o único com `role="dialog"`, nome acessível e contenção de foco; Search, Settings,
Recording, Stop confirmation, Theme edit, Manual card e Column settings não expunham
semântica comum nem focus trap. A confirmação de remoção de coluna abre um segundo
overlay dentro de Column settings. Também foram confirmados estados sem feedback:
`SearchModal` descarta erros do hook e `CardDetailModal` deixa o painel vazio se a busca
falha. `index.html` declara `lang="en"` apesar de a interface ser em português.

Um teste React Testing Library foi executado antes da mudança de comportamento: falhou
por não encontrar o diálogo nomeado de busca e por não encontrar um alerta quando a
requisição de busca falha.

## Objetivos

- Compartilhar comportamento acessível entre todos os diálogos reais: nome/descrição,
  foco inicial, ciclo de Tab, Escape, restauração de foco e fechamento por backdrop
  apenas onde já era permitido.
- Dar nomes acessíveis a campos e botões icon-only, preservando a interação visual.
- Evitar traps conflitantes na confirmação de remoção de coluna, mantendo a confirmação
  dentro do diálogo existente.
- Mostrar carregamento/erro recuperável em busca e detalhe de card, e anunciar progresso
  de inicialização para tecnologia assistiva.
- Declarar idioma e título corretos do documento.
- Adicionar testes de componente focados na semântica, teclado, restauração de foco e erro.

## Fora de escopo

- Não alterar contratos HTTP, dados, mutations ou regras de negócio.
- Não reestruturar toda a tela de Settings nem otimizar chunks sem medição.
- Não adicionar um sistema de testes end-to-end ou mocks de repositórios.
- Não mudar o comportamento de áreas portaled que não são diálogos, como AudioPlayer.

## Decisões de implementação

- Criar um `Dialog` compartilhado baseado em portal e atributos ARIA nativos. Cada uso
  fornece um `aria-labelledby`, classes de overlay/painel e opção explícita de backdrop.
- O diálogo guarda e restaura o elemento previamente focado, prioriza descendente com
  `autoFocus`, mantém Tab/Shift+Tab dentro do painel e trata Escape sem impedir um
  comportamento customizado, como cancelar a edição de notas.
- Usar um único diálogo para edição de colunas e sua confirmação de exclusão; a pergunta
  de confirmação passa a ser um grupo inline no painel.
- Search anuncia carregamento, vazio e erro. Card detail anuncia carregamento, expõe erro
  e oferece retry explícito. O LoadingScreen usa uma região viva não intrusiva.
- Atualizar `lang` para `pt-BR` e o título para Meeting Notes.
- Não otimizar o bundle nesta etapa sem uma medição que mostre benefício.

## Validação

- Reproduções pré-implementação: os dois testes de SearchModal falham (diálogo sem nome
  e erro de busca invisível).
- Após implementar: executar a suíte Vitest, build TypeScript/Vite e lint; validar Tab,
  Shift+Tab, Escape/backdrop e restauração de foco nos testes. Executar o restante das
  suítes somente se a alteração exigir integração com backend.
