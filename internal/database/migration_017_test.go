package database

import "testing"

func TestMigration017_ClearsOnlyDescriptionIdenticalToSummary(t *testing.T) {
	db, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	seed := []struct {
		meetingID, cardID, summary, description string
		number                                  int
	}{
		{"m-copy", "c-copy", "resumo alfa", "resumo alfa", 1},
		{"m-edit", "c-edit", "resumo beta", "anotação minha", 2},
	}
	for _, s := range seed {
		if _, err := db.Exec(
			`INSERT INTO meetings (id, title, status) VALUES (?, ?, 'processed')`,
			s.meetingID, s.meetingID); err != nil {
			t.Fatalf("seed meeting %s: %v", s.meetingID, err)
		}
		if _, err := db.Exec(
			`INSERT INTO summaries (id, meeting_id, content, model_used) VALUES (?, ?, ?, 'test')`,
			"s-"+s.meetingID, s.meetingID, s.summary); err != nil {
			t.Fatalf("seed summary %s: %v", s.meetingID, err)
		}
		if _, err := db.Exec(
			`INSERT INTO board_cards
			   (id, meeting_id, column_id, number, position, description, source, updated_at, created_at)
			 VALUES (?, ?, 'col-backlog', ?, ?, ?, 'meeting', '2020-01-01 00:00:00', '2020-01-01 00:00:00')`,
			s.cardID, s.meetingID, s.number, float64(s.number)*1000, s.description); err != nil {
			t.Fatalf("seed card %s: %v", s.cardID, err)
		}
	}

	// modernc.org/sqlite reformata qualquer coluna DATETIME para RFC3339 a cada
	// leitura, independente do texto original gravado — por isso comparamos o
	// valor antes/depois da migration em vez de um literal fixo.
	var updatedAtBefore string
	if err := db.QueryRow(`SELECT updated_at FROM board_cards WHERE id = 'c-copy'`).Scan(&updatedAtBefore); err != nil {
		t.Fatalf("read updated_at before: %v", err)
	}

	// No Open a migration roda antes de existir dado algum, então testar o efeito
	// exige reexecutá-la. Ela é um UPDATE idempotente, então isso é legítimo.
	stmt, err := migrationsFS.ReadFile("migrations/017_card_description_annotations.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.Exec(string(stmt)); err != nil {
		t.Fatalf("exec migration: %v", err)
	}

	var got string
	if err := db.QueryRow(`SELECT description FROM board_cards WHERE id = 'c-copy'`).Scan(&got); err != nil {
		t.Fatalf("read c-copy: %v", err)
	}
	if got != "" {
		t.Errorf("descrição idêntica ao resumo deveria ter sido limpa, got %q", got)
	}

	if err := db.QueryRow(`SELECT description FROM board_cards WHERE id = 'c-edit'`).Scan(&got); err != nil {
		t.Fatalf("read c-edit: %v", err)
	}
	if got != "anotação minha" {
		t.Errorf("descrição divergente deveria ser preservada, got %q", got)
	}

	var updatedAtAfter string
	if err := db.QueryRow(`SELECT updated_at FROM board_cards WHERE id = 'c-copy'`).Scan(&updatedAtAfter); err != nil {
		t.Fatalf("read updated_at after: %v", err)
	}
	if updatedAtAfter != updatedAtBefore {
		t.Errorf("updated_at não deveria mudar, before %q, after %q", updatedAtBefore, updatedAtAfter)
	}
}
