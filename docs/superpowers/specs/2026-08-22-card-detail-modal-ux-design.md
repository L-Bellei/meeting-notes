# CardDetailModal — UI/UX

**Data:** 2026-08-22
**Status:** aprovado

## Problema

O `CardDetailModal` é a tela onde se lê e se age sobre um card do board. Dez problemas foram
levantados investigando o componente a pedido do usuário, e um décimo primeiro apareceu durante
a investigação — este último é de semântica de dado, não de UI, e é o que mais muda o desenho.

### O 11º achado: a descrição é uma cópia morta do resumo

`board_card_service.go:61-63` preenche a descrição de um card de reunião com
`sum.Content` — uma **cópia** do resumo, tirada no instante da criação e nunca ressincronizada.
O modal renderiza as duas coisas: a seção DESCRIÇÃO (a cópia) e a seção RESUMO (a fonte viva).

Medido no banco de dev, card #1:

```
len(description) = 1867
len(summary)     = 1867
identicos?       True
```

Consequências: metade da altura útil do modal é texto repetido, e "editar a descrição" de um
card de reunião significa editar uma cópia velha do resumo. O item 4 (edição sem afordância)
não é só falta de pista visual — a edição em si tem pouco sentido nesse caminho.

### Os dez achados de UI/UX

| # | Achado |
|---|---|
| 1 | Três áreas de scroll aninhadas: o corpo, a descrição (`max-h-56`) e o resumo (`max-h-40`). A caixa interna captura a roda do mouse. |
| 2 | Único modal do app que não fecha com `Escape`. Nenhum modal tem `role="dialog"`, `aria-modal` ou focus trap. |
| 3 | Confirm de exclusão de dois cliques cujo `confirmDelete` **nunca reseta**: um clique acidental arma a exclusão para qualquer clique posterior. Único feedback é a cor e um `title`. |
| 4 | Clicar em qualquer ponto da descrição troca a leitura formatada por um `textarea` com o texto cru, sem nenhuma afordância de que é clicável. |
| 5 | Checkboxes sem optimistic update. (`salvando...`/erro já entraram no PR #45.) |
| 6 | `card.tasks.length > 0` esconde a seção inteira: sem estado vazio e sem acesso ao "Gerar tasks" que já existe em `useGenerateTasks`. |
| 7 | `priority` e `assignee` vêm na resposta e o modal ignora. |
| 8 | Hierarquia invertida: o título da reunião é `text-sm`, o menor elemento da tela. O badge do tema herda a cor do tema — em vermelho, lê como erro. |
| 9 | `status` é texto morto. Mover de coluna exige fechar o modal e arrastar no board. |
| 10 | `w-[640px]` sem `max-w`: sangra fora da viewport em janela estreita. |

## Decisões

Todas tomadas com o usuário durante o brainstorm.

1. **Escopo:** os dez itens, mais o 11º.
2. **Descrição vira anotação própria.** Card novo de reunião nasce com descrição vazia; a seção
   passa a ser "Suas anotações" e o Resumo fica sendo a fonte viva da IA. Dois campos, dois
   propósitos.
3. **Cards existentes:** migration limpa a descrição **apenas** onde ela continua byte-a-byte
   igual ao resumo. Qualquer descrição editada fica intacta.
4. **`Escape`/a11y resolvem só neste modal.** O primitivo `Modal` compartilhado (que consertaria
   os outros cinco) vai para o BACKLOG: sem teste de render, migrar seis modais num PR é risco
   desproporcional.
5. **Mover de coluna:** `<select>` no header, no lugar do texto de status. Mesmo padrão de
   `<select>` que o modal já usa para associar reunião.
6. **Layout:** fluxo único, um só scroll, com "ver mais" nas seções longas.
7. **Header:** barra de cor do tema no topo do modal, header em uma linha com título dominante.
8. **Tasks vazias:** estado vazio com "Gerar tasks", e `has_transcript` no payload para o botão
   saber se a operação é possível antes de o usuário bater num 422.

### Fora de escopo

- Primitivo `Modal` compartilhado e migração dos outros cinco modais.
- Framework de teste no frontend (`vitest`) — débito registrado, decisão do usuário pendente.
- Qualquer mudança em `internal/ai`.
- Reordenar tasks, editar descrição de task, atribuir responsável pela UI.

## Arquitetura

### Estrutura de arquivos

`CardDetailModal.tsx` tem ~415 linhas hoje e cresceria bem além disso. A decomposição segue o
padrão que a aba de temas estabeleceu na v2.6.0 (arquivos focados, um por responsabilidade):

| Arquivo | Responsabilidade |
|---|---|
| `components/board/CardDetailModal.tsx` | Casca: portal, overlay, `Escape`, focus trap, atributos de a11y, barra de cor, composição das seções. **Detém** `editingNotes` e `confirmDelete` — o handler de `Escape` precisa de `editingNotes` para decidir entre cancelar a edição e fechar o modal, então o estado não pode viver dentro da seção. |
| `components/board/CardModalHeader.tsx` | `#1`, título, nome do tema, select de coluna, excluir, fechar. |
| `components/board/CardTasksSection.tsx` | Tasks de reunião (`TaskRow`) e tasks manuais, estado vazio, gerar tasks. |
| `components/board/CardNotesSection.tsx` | Anotações: leitura, botão de lápis, `textarea`, salvar/cancelar. Recebe `editing` e os callbacks da casca; não guarda o flag de edição. |
| `components/ui/ExpandableText.tsx` | O "ver mais": recebe `lines: number`, corta nessa altura via `line-clamp` e expande no lugar. Consumido por Resumo e Pontos-chave. |

`DescriptionView`/`tryParseStructured` (o renderizador de JSON estruturado) **saem**: com a
descrição virando anotação escrita pelo usuário, não há mais JSON para interpretar nesse campo.
O Resumo é texto puro.

### Backend

**Migration `017_card_description_annotations.sql`**

```sql
UPDATE board_cards
SET description = ''
WHERE source = 'meeting'
  AND meeting_id IS NOT NULL
  AND description <> ''
  AND description = (
    SELECT content FROM summaries WHERE summaries.meeting_id = board_cards.meeting_id
  );
```

`summaries` tem índice único em `meeting_id` (`002_unique_summary_meeting.sql`), então o
subselect devolve no máximo uma linha e o `UPDATE` é determinístico.

**A migration não toca `updated_at`.** O `updated_at` do card alimenta o tempo relativo exibido
no `KanbanCard` ("há 3d"). Isto é limpeza de sistema, não edição do usuário: bumpar faria todo
card parecer recém-mexido.

As migrations não são transacionais (débito conhecido), mas esta é um único `UPDATE`
idempotente — reexecutá-la é inócuo e uma falha no meio não deixa estado parcial perigoso.

**`board_card_service.go` — parar de copiar o resumo**

```go
// antes (linhas 61-63)
description := ""
if sum, err := s.summaryRepo.GetByMeetingID(ctx, meetingID); err == nil {
    description = sum.Content
}

// depois
description := ""
```

O `summaryRepo` continua sendo usado por `GetDetail`; só esta cópia sai.

**`has_transcript` no detalhe do card**

`models.BoardCardDetail` ganha o campo:

```go
HasTranscript bool `json:"has_transcript"`
```

`BoardCardService` já tem `meetingRepo`. Mas `MeetingRepository.GetByID` carrega o transcript
inteiro — potencialmente megabytes — só para saber se ele existe. Um método dedicado evita isso:

```go
func (r *MeetingRepository) HasTranscript(ctx context.Context, meetingID string) (bool, error)
// SELECT COUNT(*) FROM meetings
// WHERE id = ? AND transcript IS NOT NULL AND transcript != ''
```

`GetDetail` preenche `detail.HasTranscript` dentro do bloco `if detail.MeetingID != nil`,
tratando erro como `false` — o mesmo padrão tolerante que as outras buscas do método já usam.

## Comportamentos

### Escape, foco e a11y

- `role="dialog"`, `aria-modal="true"`, `aria-labelledby` apontando para o `id` do título.
- `Escape` fecha, via listener em `useEffect`, seguindo o padrão de `SearchModal.tsx:29`.
- **Com a edição de anotações aberta, `Escape` cancela a edição; só o segundo `Escape` fecha o
  modal.** Sem isso, um `Escape` distraído descarta texto digitado.
- Foco entra no modal ao abrir, `Tab` circula dentro dele, e o foco volta ao elemento anterior
  ao fechar.

### Confirmação de exclusão

Segue sendo dois cliques, mas: o rótulo **"Confirmar?"** aparece ao lado do ícone; o estado
reseta após 4 segundos; e reseta ao fechar o modal. Hoje `confirmDelete` nunca volta atrás.

### Header

- Barra de 3px na cor do tema no topo do modal; neutra em card manual ou sem tema. Reaproveita
  a linguagem visual da barra de cor da aba de temas.
- Uma linha: `#1` (`text-xs`, discreto) · título (`text-base font-semibold`, `truncate`, texto
  completo no `title`) · nome do tema (`text-xs`, recolhe em janela estreita) · select de
  coluna · excluir · fechar.
- **Nenhum badge com fundo colorido.** A cor do tema vive só na barra — é o que resolve o
  vermelho lendo como erro.
- O select usa `useColumns()` e `useMoveCard()`. Move para o **fim** da coluna destino:
  `position = max(position das cartas com aquele column_id) + 1000`, e `1000` quando a coluna
  está vazia — a mesma convenção de `LastPositionInColumn` no backend.
- A lista de cartas vem de `useCards()` sem argumento, que resolve para a chave
  `["board-cards", EMPTY_FILTERS]`. `useCardForMeeting` já faz exatamente isso, então é cache
  compartilhado e não uma busca nova. Chamar `useCards(filters)` com os filtros ativos do board
  seria errado: a carta destino pode estar filtrada para fora e o `max` sairia baixo.

### Corpo

- **Um único scroll**, no corpo. Nenhum `overflow-y` interno. Esta é a correção do item 1.
- Ordem das seções: **Tasks → Resumo → Pontos-chave → Suas anotações** — do que se age para o
  que se lê.
- Resumo usa `ExpandableText` com `lines={6}`; Pontos-chave com `lines={8}`. Abaixo do corte o botão não aparece.
- `w-[640px] max-w-[calc(100vw-2rem)]`.

### Tasks

- Optimistic update em `useUpdateTask`: `onMutate` grava o novo estado no cache e devolve o
  anterior, `onError` restaura, `onSettled` revalida.
- **O indicador `salvando...` sai** quando o optimistic update entra. `isPending` fica verdadeiro
  no mesmo instante do clique, então o rótulo apareceria em toda marcação, ao lado de um checkbox
  que já mudou de estado — ruído, não informação. A mensagem de erro do PR #45 **fica**: é ela
  que impede a falha de voltar a ser silenciosa.
- Cada linha mostra prioridade (alta/média/baixa) e responsável quando houver.
- Vazio: "Nenhuma task" mais o botão **Gerar tasks**, habilitado quando `has_transcript` é
  verdadeiro e explicando o motivo quando não.

### Anotações

- Botão de lápis explícito. Clicar no texto deixa de ser o gatilho de edição.
- Vazio mostra "Nada anotado ainda" com o lápis ao lado.
- Em card manual, `toggleTask`/`addTask`/`removeTask` continuam enviando `card.description`
  persistida, não o state local — a correção do PR #46. Reintroduzir o state local aqui traz o
  bug de volta.

## Verificação

**Go:** a migration ganha teste de caso idêntico (limpa) e de caso divergente (preserva), mais
verificação de que `updated_at` não muda. `HasTranscript` ganha teste de repositório, e
`has_transcript` entra nos testes de handler do board.

**Frontend:** sem framework de teste (débito no BACKLOG), a verificação é `npx tsc --noEmit`,
`npm run build` e roteiro manual. O roteiro tem de rodar na **janela nativa** depois de
reiniciar o `wails dev` — o HMR do vite não chega nela, e isso já fez uma correção correta ser
reportada como quebrada.

Roteiro manual mínimo: um scroll só de ponta a ponta; `Escape` fechando, e cancelando a edição
antes de fechar; excluir armando, mostrando "Confirmar?" e desarmando sozinho; mover de coluna
e ver o card mudar de lugar no board; marcar task e ver o contador do card acompanhar; card de
reunião sem tasks mostrando o estado vazio; janela estreita sem scroll horizontal.

## Riscos

- **A migration roda no banco do usuário no próximo launch.** É `UPDATE` sem `DROP`, restrito a
  descrições ainda idênticas ao resumo. No banco de dev afeta 1 card, 0 divergentes. O banco do
  app instalado não foi medido.
- **Perda de capacidade percebida:** quem editou a descrição de um card de reunião por cima da
  cópia mantém o texto (a migration preserva divergentes), mas a seção muda de nome. Aceito.
- **Sem teste de render**, a decomposição em cinco arquivos é a mudança mais arriscada do lote:
  é exatamente o tipo de refatoração que um teste de montagem protegeria. Os dois bugs
  corrigidos nos PRs #45 e #46 seriam pegos por um.
