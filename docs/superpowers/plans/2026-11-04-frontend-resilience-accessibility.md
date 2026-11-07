# Resiliência e acessibilidade do frontend — Plano

## Objetivo

Corrigir semântica, foco e estados silenciosos dos diálogos sem mudar contratos ou
regras de negócio, com cobertura de componente mínima e executável localmente.

## Alterações

1. **Testes de UI**
   - Configurar Vitest, jsdom e React Testing Library.
   - Manter o teste pré-correção de SearchModal para nome acessível e erro anunciado.
   - Cobrir focus trap/retorno de foco e Escape/backdrop no componente compartilhado.

2. **Diálogo compartilhado**
   - Implementar `Dialog` com portal, nome ARIA, foco inicial, Tab cíclico, Escape e
     retorno de foco.
   - Migrar Search, Settings, Recording, Stop confirmation, Theme edit, Manual card,
     Column settings e Card detail, preservando as opções de fechamento existentes e
     rotulando controles interativos.
   - Tornar a confirmação de exclusão de coluna inline, evitando diálogo aninhado.

3. **Estados e documento**
   - Apresentar carregamento/erro de busca e do detalhe de card com retry.
   - Anunciar checks da tela de carregamento.
   - Atualizar idioma e título do documento.

4. **Verificação e continuidade**
   - Rodar testes Vitest, build, lint, testes Go/Python se houver regressões de integração
     e `git diff --check`.
   - Atualizar `.claude/STATE.md`, `CHANGELOG.md`, `DECISIONS.md` e roadmap.

## Limites

- Não adicionar otimizações especulativas de bundle.
- Preservar todas as alterações das Etapas 1–4 e não criar commits.
