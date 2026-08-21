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

func TestOpen_ThemesHasSingleCustomPrompt(t *testing.T) {
	path := t.TempDir() + "/test.db"

	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	rows, err := db.Query("PRAGMA table_info(themes)")
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols[name] = true
	}

	if !cols["custom_prompt"] {
		t.Error("custom_prompt column missing")
	}
	for _, gone := range []string{"custom_summary_prompt", "custom_key_points_prompt", "custom_tasks_prompt"} {
		if cols[gone] {
			t.Errorf("column %q should have been dropped by migration 016", gone)
		}
	}
}

func TestOpen_SeedsSidebarPinnedSetting(t *testing.T) {
	path := t.TempDir() + "/test.db"

	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	var value string
	row := db.QueryRow("SELECT value FROM settings WHERE key = ?", "sidebar_pinned")
	if err := row.Scan(&value); err != nil {
		t.Fatalf("sidebar_pinned setting not found after migration: %v", err)
	}
	if value != "false" {
		t.Errorf("sidebar_pinned = %q, want %q", value, "false")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
