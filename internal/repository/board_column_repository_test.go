package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/repository"
)

func openBoardTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestBoardColumnRepository_ListSeedColumns(t *testing.T) {
	db := openBoardTestDB(t)
	repo := repository.NewBoardColumnRepository(db)
	ctx := context.Background()

	cols, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 seed columns, got %d", len(cols))
	}
	// Verify order by position
	if cols[0].ID != "col-backlog" {
		t.Errorf("first column = %q, want col-backlog", cols[0].ID)
	}
	if cols[1].ID != "col-wip" {
		t.Errorf("second column = %q, want col-wip", cols[1].ID)
	}
	if cols[2].ID != "col-done" {
		t.Errorf("third column = %q, want col-done", cols[2].ID)
	}
}

func TestBoardColumnRepository_CRUD(t *testing.T) {
	db := openBoardTestDB(t)
	repo := repository.NewBoardColumnRepository(db)
	ctx := context.Background()

	// Create
	col, err := repo.Create(ctx, "Review")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if col.Name != "Review" {
		t.Errorf("Name = %q, want Review", col.Name)
	}
	if col.ID == "" {
		t.Error("ID should not be empty")
	}
	// Position should be > 3000 (after the 3 seed columns)
	if col.Position <= 3000 {
		t.Errorf("Position = %f, want > 3000", col.Position)
	}

	// GetByID
	got, err := repo.GetByID(ctx, col.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Review" {
		t.Errorf("GetByID Name = %q, want Review", got.Name)
	}

	// Update
	if err := repo.Update(ctx, col.ID, "In Review"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, err := repo.GetByID(ctx, col.ID)
	if err != nil {
		t.Fatalf("GetByID after Update: %v", err)
	}
	if got2.Name != "In Review" {
		t.Errorf("After Update Name = %q, want In Review", got2.Name)
	}

	// Delete
	if err := repo.Delete(ctx, col.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = repo.GetByID(ctx, col.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("after Delete GetByID err = %v, want ErrNotFound", err)
	}
}

func TestBoardColumnRepository_GetByIDNotFound(t *testing.T) {
	db := openBoardTestDB(t)
	repo := repository.NewBoardColumnRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetByID err = %v, want ErrNotFound", err)
	}
}

func TestBoardColumnRepository_UpdateNotFound(t *testing.T) {
	db := openBoardTestDB(t)
	repo := repository.NewBoardColumnRepository(db)
	ctx := context.Background()

	err := repo.Update(ctx, "nonexistent", "Name")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Update err = %v, want ErrNotFound", err)
	}
}

func TestBoardColumnRepository_DeleteNotFound(t *testing.T) {
	db := openBoardTestDB(t)
	repo := repository.NewBoardColumnRepository(db)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Delete err = %v, want ErrNotFound", err)
	}
}

func TestBoardColumnRepository_Reorder(t *testing.T) {
	db := openBoardTestDB(t)
	repo := repository.NewBoardColumnRepository(db)
	ctx := context.Background()

	// Initially: backlog=1000, wip=2000, done=3000
	// Reorder: done=500, backlog=1500, wip=2500
	items := []repository.ReorderItem{
		{ID: "col-done", Position: 500},
		{ID: "col-backlog", Position: 1500},
		{ID: "col-wip", Position: 2500},
	}
	if err := repo.Reorder(ctx, items); err != nil {
		t.Fatalf("Reorder: %v", err)
	}

	cols, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List after Reorder: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	// After reorder: done should be first
	if cols[0].ID != "col-done" {
		t.Errorf("first after reorder = %q, want col-done", cols[0].ID)
	}
	if cols[1].ID != "col-backlog" {
		t.Errorf("second after reorder = %q, want col-backlog", cols[1].ID)
	}
	if cols[2].ID != "col-wip" {
		t.Errorf("third after reorder = %q, want col-wip", cols[2].ID)
	}
}

func TestBoardColumnRepository_Count(t *testing.T) {
	db := openBoardTestDB(t)
	repo := repository.NewBoardColumnRepository(db)
	ctx := context.Background()

	n, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 3 {
		t.Errorf("Count = %d, want 3", n)
	}

	_, err = repo.Create(ctx, "Extra")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	n2, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count after Create: %v", err)
	}
	if n2 != 4 {
		t.Errorf("Count after Create = %d, want 4", n2)
	}
}

func TestBoardColumnRepository_CardCount(t *testing.T) {
	db := openBoardTestDB(t)
	repo := repository.NewBoardColumnRepository(db)
	ctx := context.Background()

	n, err := repo.CardCount(ctx, "col-backlog")
	if err != nil {
		t.Fatalf("CardCount: %v", err)
	}
	if n != 0 {
		t.Errorf("CardCount = %d, want 0", n)
	}
}
