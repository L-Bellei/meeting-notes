package database_test

import (
	"os"
	"testing"

	"meeting-notes/internal/database"
)

func TestOpen_CreatesTablesOnStartup(t *testing.T) {
	path := t.TempDir() + "/test.db"

	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	tables := []string{"themes", "meetings", "summaries", "key_points", "tasks"}
	for _, table := range tables {
		var name string
		row := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table)
		if err := row.Scan(&name); err != nil {
			t.Errorf("table %q not found after migration: %v", table, err)
		}
	}
}

func TestOpen_IsIdempotent(t *testing.T) {
	path := t.TempDir() + "/test.db"

	db1, err := database.Open(path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	db1.Close()

	db2, err := database.Open(path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	db2.Close()
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
