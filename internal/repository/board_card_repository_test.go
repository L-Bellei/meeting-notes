package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

func createTestMeeting(t *testing.T, meetingRepo *repository.MeetingRepository, id, title string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	m := &models.Meeting{
		ID:        id,
		Title:     title,
		StartedAt: &now,
		Status:    models.StatusPending,
	}
	if err := meetingRepo.Create(ctx, m); err != nil {
		t.Fatalf("createTestMeeting %s: %v", id, err)
	}
}

func TestBoardCardRepository_CreateSequentialNumbers(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-1", "Meeting One")
	createTestMeeting(t, meetingRepo, "meeting-2", "Meeting Two")

	card1, err := cardRepo.Create(ctx, "meeting-1", "col-backlog", "", 1000)
	if err != nil {
		t.Fatalf("Create card1: %v", err)
	}
	if card1.Number != 1 {
		t.Errorf("card1.Number = %d, want 1", card1.Number)
	}

	card2, err := cardRepo.Create(ctx, "meeting-2", "col-backlog", "", 2000)
	if err != nil {
		t.Fatalf("Create card2: %v", err)
	}
	if card2.Number != 2 {
		t.Errorf("card2.Number = %d, want 2", card2.Number)
	}
}

func TestBoardCardRepository_DuplicateMeetingIDReturnsErrDuplicate(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-dup", "Meeting Dup")

	_, err := cardRepo.Create(ctx, "meeting-dup", "col-backlog", "", 1000)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err = cardRepo.Create(ctx, "meeting-dup", "col-backlog", "", 2000)
	if !errors.Is(err, repository.ErrDuplicate) {
		t.Errorf("duplicate Create err = %v, want ErrDuplicate", err)
	}
}

func TestBoardCardRepository_GetByID(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-get", "Meeting Get")

	card, err := cardRepo.Create(ctx, "meeting-get", "col-backlog", "some description", 1000)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := cardRepo.GetByID(ctx, card.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Description != "some description" {
		t.Errorf("Description = %q, want 'some description'", got.Description)
	}
	if got.ColumnID != "col-backlog" {
		t.Errorf("ColumnID = %q, want col-backlog", got.ColumnID)
	}
}

func TestBoardCardRepository_GetByIDNotFound(t *testing.T) {
	db := openBoardTestDB(t)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	_, err := cardRepo.GetByID(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetByID err = %v, want ErrNotFound", err)
	}
}

func TestBoardCardRepository_Move(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-move", "Meeting Move")

	card, err := cardRepo.Create(ctx, "meeting-move", "col-backlog", "", 1000)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := cardRepo.Move(ctx, card.ID, "col-wip", 500); err != nil {
		t.Fatalf("Move: %v", err)
	}

	got, err := cardRepo.GetByID(ctx, card.ID)
	if err != nil {
		t.Fatalf("GetByID after Move: %v", err)
	}
	if got.ColumnID != "col-wip" {
		t.Errorf("ColumnID after Move = %q, want col-wip", got.ColumnID)
	}
	if got.Position != 500 {
		t.Errorf("Position after Move = %f, want 500", got.Position)
	}
}

func TestBoardCardRepository_MoveRebalance(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	// Create three meetings and cards in col-backlog
	for i, id := range []string{"meeting-rb-1", "meeting-rb-2", "meeting-rb-3"} {
		createTestMeeting(t, meetingRepo, id, id)
		_, err := cardRepo.Create(ctx, id, "col-backlog", "", float64((i+1)*1000))
		if err != nil {
			t.Fatalf("Create card %s: %v", id, err)
		}
	}

	// Get all cards in col-backlog to find card IDs
	cards, err := cardRepo.List(ctx, repository.BoardCardFilters{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Find first two cards in col-backlog
	var c1, c2 *models.BoardCardSummary
	for i := range cards {
		if cards[i].ColumnID == "col-backlog" {
			if c1 == nil {
				c1 = &cards[i]
			} else if c2 == nil {
				c2 = &cards[i]
				break
			}
		}
	}
	if c1 == nil || c2 == nil {
		t.Fatal("expected at least 2 cards in col-backlog")
	}

	// Move c2 to same position as c1 to trigger rebalance (position gap < 1e-9)
	if err := cardRepo.Move(ctx, c2.ID, "col-backlog", c1.Position); err != nil {
		t.Fatalf("Move for rebalance: %v", err)
	}

	// After rebalance, positions should be well-spaced (multiples of 1000)
	cards2, err := cardRepo.List(ctx, repository.BoardCardFilters{})
	if err != nil {
		t.Fatalf("List after rebalance: %v", err)
	}
	var prev *float64
	for _, card := range cards2 {
		if card.ColumnID != "col-backlog" {
			continue
		}
		if prev != nil && card.Position-*prev < 1e-9 {
			t.Errorf("positions not rebalanced: consecutive positions %.10f and %.10f", *prev, card.Position)
		}
		p := card.Position
		prev = &p
	}
}

func TestBoardCardRepository_ListTitleFilter(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-alpha", "Alpha Meeting")
	createTestMeeting(t, meetingRepo, "meeting-beta", "Beta Meeting")

	_, err := cardRepo.Create(ctx, "meeting-alpha", "col-backlog", "", 1000)
	if err != nil {
		t.Fatalf("Create card alpha: %v", err)
	}
	_, err = cardRepo.Create(ctx, "meeting-beta", "col-backlog", "", 2000)
	if err != nil {
		t.Fatalf("Create card beta: %v", err)
	}

	// Filter by title "Alpha"
	cards, err := cardRepo.List(ctx, repository.BoardCardFilters{Title: "Alpha"})
	if err != nil {
		t.Fatalf("List with title filter: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 card with title filter 'Alpha', got %d", len(cards))
	}
	if cards[0].MeetingTitle != "Alpha Meeting" {
		t.Errorf("MeetingTitle = %q, want 'Alpha Meeting'", cards[0].MeetingTitle)
	}

	// No filter returns all
	all, err := cardRepo.List(ctx, repository.BoardCardFilters{})
	if err != nil {
		t.Fatalf("List without filter: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 cards without filter, got %d", len(all))
	}
}

func TestBoardCardRepository_Delete(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-del", "Meeting Del")

	card, err := cardRepo.Create(ctx, "meeting-del", "col-backlog", "", 1000)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := cardRepo.Delete(ctx, card.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = cardRepo.GetByID(ctx, card.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("after Delete GetByID err = %v, want ErrNotFound", err)
	}
}

func TestBoardCardRepository_DeleteNotFound(t *testing.T) {
	db := openBoardTestDB(t)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	err := cardRepo.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Delete err = %v, want ErrNotFound", err)
	}
}

func TestBoardCardRepository_UpdateDescription(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	createTestMeeting(t, meetingRepo, "meeting-desc", "Meeting Desc")

	card, err := cardRepo.Create(ctx, "meeting-desc", "col-backlog", "original", 1000)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := cardRepo.UpdateDescription(ctx, card.ID, "updated"); err != nil {
		t.Fatalf("UpdateDescription: %v", err)
	}

	got, err := cardRepo.GetByID(ctx, card.ID)
	if err != nil {
		t.Fatalf("GetByID after UpdateDescription: %v", err)
	}
	if got.Description != "updated" {
		t.Errorf("Description = %q, want 'updated'", got.Description)
	}
}

func TestBoardCardRepository_LastPositionInColumn(t *testing.T) {
	db := openBoardTestDB(t)
	meetingRepo := repository.NewMeetingRepository(db)
	cardRepo := repository.NewBoardCardRepository(db)
	ctx := context.Background()

	// Empty column
	pos, err := cardRepo.LastPositionInColumn(ctx, "col-backlog")
	if err != nil {
		t.Fatalf("LastPositionInColumn empty: %v", err)
	}
	if pos != 0 {
		t.Errorf("LastPositionInColumn empty = %f, want 0", pos)
	}

	createTestMeeting(t, meetingRepo, "meeting-pos", "Meeting Pos")
	_, err = cardRepo.Create(ctx, "meeting-pos", "col-backlog", "", 1500)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	pos2, err := cardRepo.LastPositionInColumn(ctx, "col-backlog")
	if err != nil {
		t.Fatalf("LastPositionInColumn: %v", err)
	}
	if pos2 != 1500 {
		t.Errorf("LastPositionInColumn = %f, want 1500", pos2)
	}
}
