# Reorganização de temas por teclado — Plano

**Status: concluído em 2026-11-06.**

## Objetivo

Adicionar suporte a teclado e nomes/instruções acessíveis ao reparent de temas, sem mudar
comportamento do Board nem regras de hierarquia.

## Etapas

1. Criar testes de regressão para o estado atual: ausência de handle focável/instruções e
   impossibilidade de completar reparent por teclado.
2. Adicionar `KeyboardSensor` ao `DndContext` da Sidebar e um botão de arraste dedicado em
   cada tema movível.
3. Fornecer instruções e anúncios em português; manter alvos, mensagens de erro e validação
   do endpoint.
4. Executar os testes de UI, ESLint e build TypeScript/Vite; revisar o diff.
5. Atualizar decisão, changelog, backlog e estado do projeto.

## Critérios de aceite

- Temas movíveis podem ser arrastados e soltos via teclado, com foco e retorno de foco claros.
- Temas com subcategorias continuam indisponíveis para reparent.
- A árvore, o endpoint e o comportamento de arraste por mouse permanecem compatíveis.
- A ação é anunciada com texto em português por tecnologia assistiva.
