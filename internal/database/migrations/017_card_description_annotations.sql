-- 017_card_description_annotations.sql
-- A descrição de um card de reunião era uma cópia do resumo, tirada na criação e
-- nunca ressincronizada. Ela passa a ser anotação do usuário, então a cópia sai —
-- mas só onde o texto ainda é idêntico ao resumo, preservando o que foi editado.
-- updated_at não é tocado de propósito: ele alimenta o tempo relativo do card.
UPDATE board_cards
SET description = ''
WHERE source = 'meeting'
  AND meeting_id IS NOT NULL
  AND description <> ''
  AND description = (
    SELECT content FROM summaries WHERE summaries.meeting_id = board_cards.meeting_id
  );
