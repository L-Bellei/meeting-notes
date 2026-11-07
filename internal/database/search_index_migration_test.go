package database_test

import (
	"context"
	"path/filepath"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/repository"
)

func TestOpen_RebuildsSearchIndexWithoutChangingSourceData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, trigger := range []string{
		"meetings_fts_meeting_insert",
		"meetings_fts_meeting_update",
		"meetings_fts_meeting_delete",
		"meetings_fts_summary_insert",
		"meetings_fts_summary_update",
		"meetings_fts_summary_delete",
		"meetings_fts_key_point_insert",
		"meetings_fts_key_point_update",
		"meetings_fts_key_point_delete",
		"meetings_fts_task_insert",
		"meetings_fts_task_update",
		"meetings_fts_task_delete",
	} {
		if _, err := db.Exec(`DROP TRIGGER ` + trigger); err != nil {
			t.Fatalf("drop trigger %s: %v", trigger, err)
		}
	}
	if _, err := db.Exec(`DROP VIEW meeting_search_content`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = '022_atomic_search_index.sql'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO meetings (id, title, status) VALUES ('m-1', 'Current title', 'completed')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO summaries (id, meeting_id, content, model_used) VALUES ('s-1', 'm-1', 'Current summary', 'test')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO key_points (id, meeting_id, position, content) VALUES ('k-1', 'm-1', 0, 'Current point')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO tasks (id, meeting_id, description) VALUES ('t-1', 'm-1', 'Current task')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM meetings_fts`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
		VALUES ('m-1', 'Stale title', '', '', '', 'Stale task'), ('orphan', 'Orphan entry', '', '', '', '')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = database.Open(path)
	if err != nil {
		t.Fatalf("reapply search-index migration: %v", err)
	}
	defer db.Close()

	searchRepo := repository.NewSearchRepository(db)
	for query, want := range map[string]int{
		"current": 1,
		"summary": 1,
		"point":   1,
		"task":    1,
		"stale":   0,
		"orphan":  0,
	} {
		results, err := searchRepo.Search(context.Background(), query)
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		if len(results) != want {
			t.Errorf("search %q returned %d results, want %d", query, len(results), want)
		}
	}

	for table, want := range map[string]int{"meetings": 1, "summaries": 1, "key_points": 1, "tasks": 1} {
		var got int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Errorf("%s count = %d, want %d", table, got, want)
		}
	}
}
