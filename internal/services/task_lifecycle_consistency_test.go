package services_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTaskLifecycleFixture(t *testing.T, aiClient ai.AIClient) (*taskLifecycleFixture, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	meetingRepo := repository.NewMeetingRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	transcript := "A equipe vai revisar o plano."
	now := time.Now().UTC()
	meeting := &models.Meeting{
		ID: "m-1", Title: "Planejamento", StartedAt: &now,
		Status: models.StatusCompleted, Transcript: &transcript,
	}
	if err := meetingRepo.Create(ctx, meeting); err != nil {
		t.Fatal(err)
	}
	return &taskLifecycleFixture{
		DB: db, Meeting: meeting, Meetings: meetingRepo, Tasks: taskRepo, Search: searchRepo,
		Service: services.NewTaskService(taskRepo, aiClient),
	}, ctx
}

type taskLifecycleFixture struct {
	DB       *sql.DB
	Meeting  *models.Meeting
	Meetings *repository.MeetingRepository
	Tasks    *repository.TaskRepository
	Search   *repository.SearchRepository
	Service  *services.TaskService
}

func TestTaskService_GenerateKeepsExistingTasksWhenReplacementFails(t *testing.T) {
	fake := &fakeAI{tasks: []ai.TaskSuggestion{{Description: "Generated replacement", Priority: "medium"}}}
	fixture, ctx := newTaskLifecycleFixture(t, fake)
	if err := fixture.Tasks.Create(ctx, &models.Task{
		ID: "existing", MeetingID: fixture.Meeting.ID, Description: "Existing manual task",
		Priority: models.PriorityHigh,
	}); err != nil {
		t.Fatal(err)
	}
	assertSearchResultCount(t, fixture.Search, ctx, "manual", 1)
	if _, err := fixture.DB.Exec(`CREATE TRIGGER reject_generated_task BEFORE INSERT ON tasks
		WHEN NEW.description = 'Generated replacement'
		BEGIN SELECT RAISE(ABORT, 'forced task insert failure'); END`); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.Service.Generate(ctx, fixture.Meeting, ""); err == nil {
		t.Fatal("Generate succeeded despite the forced task insert failure")
	}
	tasks, err := fixture.Tasks.ListByMeetingID(ctx, fixture.Meeting.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != "existing" {
		t.Fatalf("tasks after failed replacement = %+v, want the original task unchanged", tasks)
	}
	assertSearchResultCount(t, fixture.Search, ctx, "manual", 1)
	assertSearchResultCount(t, fixture.Search, ctx, "generated", 0)
}

func TestTaskService_MutationsKeepSearchIndexInSync(t *testing.T) {
	fixture, ctx := newTaskLifecycleFixture(t, nil)
	task, err := fixture.Service.Create(ctx, fixture.Meeting.ID, "Review deployment plan", nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	assertSearchResultCount(t, fixture.Search, ctx, "deployment", 1)

	if _, err := fixture.Service.Update(ctx, task.ID, "Review release checklist", nil, nil, "", false); err != nil {
		t.Fatal(err)
	}
	assertSearchResultCount(t, fixture.Search, ctx, "deployment", 0)
	assertSearchResultCount(t, fixture.Search, ctx, "checklist", 1)

	if err := fixture.Service.Delete(ctx, task.ID); err != nil {
		t.Fatal(err)
	}
	assertSearchResultCount(t, fixture.Search, ctx, "checklist", 0)
}

func TestTaskService_CreateRollsBackWhenSearchIndexIsUnavailable(t *testing.T) {
	fixture, ctx := newTaskLifecycleFixture(t, nil)
	if _, err := fixture.DB.Exec(`DROP TABLE meetings_fts`); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.Service.Create(ctx, fixture.Meeting.ID, "Review deployment plan", nil, nil, ""); err == nil {
		t.Fatal("Create succeeded while the search index was unavailable")
	}
	tasks, err := fixture.Tasks.ListByMeetingID(ctx, fixture.Meeting.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("tasks after failed index update = %+v, want no mutation", tasks)
	}
}

func TestMeetingService_UpdateRollsBackWhenSearchIndexUpdateFails(t *testing.T) {
	fixture, ctx := newTaskLifecycleFixture(t, nil)
	meetingService := services.NewMeetingService(fixture.Meetings, nil)
	if _, err := fixture.DB.Exec(`DROP TABLE meetings_fts`); err != nil {
		t.Fatal(err)
	}

	if _, err := meetingService.Update(ctx, fixture.Meeting.ID, "Rejected title", nil, "", nil, nil, nil, nil); err == nil {
		t.Fatal("Update succeeded despite the missing search index")
	}
	meeting, err := fixture.Meetings.GetByID(ctx, fixture.Meeting.ID)
	if err != nil {
		t.Fatal(err)
	}
	if meeting.Title != "Planejamento" {
		t.Fatalf("meeting title after failed index update = %q, want original title", meeting.Title)
	}
}

func assertSearchResultCount(t *testing.T, repo *repository.SearchRepository, ctx context.Context, query string, want int) {
	t.Helper()
	results, err := repo.Search(ctx, query)
	if err != nil {
		t.Fatalf("Search(%q): %v", query, err)
	}
	if len(results) != want {
		t.Fatalf("Search(%q) returned %d results, want %d", query, len(results), want)
	}
}
