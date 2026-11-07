package database_test

import (
	"database/sql"
	"testing"

	"meeting-notes/internal/database"
	_ "modernc.org/sqlite"
)

func TestOpen_FailedMigrationRollsBackAllStatements(t *testing.T) {
	path := t.TempDir() + "/test.db"
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"custom_summary_prompt", "custom_key_points_prompt", "custom_tasks_prompt"} {
		if _, err := db.Exec(`ALTER TABLE themes ADD COLUMN ` + column + ` TEXT NOT NULL DEFAULT ''`); err != nil {
			t.Fatalf("restore migration column %s: %v", column, err)
		}
	}
	if _, err := db.Exec(`CREATE VIEW migration_failure_guard AS SELECT custom_key_points_prompt FROM themes`); err != nil {
		t.Fatalf("create migration guard: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = '016_theme_single_prompt.sql'`); err != nil {
		t.Fatalf("reset migration record: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if reopened, err := database.Open(path); err == nil {
		reopened.Close()
		t.Fatal("Open succeeded although migration 016 must fail on the guarded column")
	}

	checkDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer checkDB.Close()
	var remaining int
	if err := checkDB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('themes') WHERE name = 'custom_summary_prompt'`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("first statement persisted after migration failure: custom_summary_prompt count = %d, want 1", remaining)
	}
	var applied int
	if err := checkDB.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = '016_theme_single_prompt.sql'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 0 {
		t.Fatalf("failed migration recorded as applied: count = %d, want 0", applied)
	}

	if _, err := checkDB.Exec(`DROP VIEW migration_failure_guard`); err != nil {
		t.Fatalf("drop migration guard: %v", err)
	}
	if err := checkDB.Close(); err != nil {
		t.Fatal(err)
	}
	retriedDB, err := database.Open(path)
	if err != nil {
		t.Fatalf("retry migration after removing guard: %v", err)
	}
	defer retriedDB.Close()
	if err := retriedDB.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = '016_theme_single_prompt.sql'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("retried migration record count = %d, want 1", applied)
	}
}
