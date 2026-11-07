# Reorganizar temas com teclado

## Contexto

O reparent de temas na barra lateral aceita apenas arraste com ponteiro. Isso impede que
usuários que navegam por teclado reorganizem a hierarquia, apesar dos demais controles de
tema já serem botões acessíveis.

## Objetivo

Permitir mover os mesmos temas que já podem ser arrastados com mouse, mantendo intactas as
regras de hierarquia e a API atual.

## Experiência

- Cada tema movível terá um botão dedicado e nomeado para iniciar o arraste; os listeners de
  ponteiro deixam de ocupar a linha inteira.
- Espaço ou Enter inicia o movimento; as setas percorrem os destinos; Espaço, Enter ou Tab
  solta; Escape cancela.
- O leitor de tela recebe instruções e anúncios em português para início, destino, conclusão
  ou cancelamento.
- A zona de raiz e os temas de primeiro nível permanecem como destinos; temas que já têm
  subcategorias continuam não movíveis.
- Restrições de hierarquia continuam validadas pelo mesmo endpoint; nenhuma mudança de API,
  persistência ou sensor do Board faz parte do escopo.

## Validação

- Testes de UI verificam o handle focável, instruções acessíveis e movimento por teclado até
  a zona de raiz ou um tema de primeiro nível, além do cancelamento.
- `npm --prefix frontend run lint`, `npm --prefix frontend test` e
  `npm --prefix frontend run build`.
