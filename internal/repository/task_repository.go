package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"meeting-notes/internal/models"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) ListByMeetingID(ctx context.Context, meetingID string) ([]models.Task, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, meeting_id, description, assignee, due_date, priority, completed, created_at FROM tasks WHERE meeting_id = ? ORDER BY created_at ASC`,
		meetingID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := []models.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, *t)
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) Create(ctx context.Context, t *models.Task) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	var dueDate *string
	if t.DueDate != nil {
		s := t.DueDate.UTC().Format(time.RFC3339Nano)
		dueDate = &s
	}
	completed := 0
	if t.Completed {
		completed = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tasks (id, meeting_id, description, assignee, due_date, priority, completed, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.MeetingID, t.Description, t.Assignee, dueDate, string(t.Priority), completed,
		t.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (r *TaskRepository) GetByID(ctx context.Context, id string) (*models.Task, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, meeting_id, description, assignee, due_date, priority, completed, created_at FROM tasks WHERE id = ?`, id,
	)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

func (r *TaskRepository) Update(ctx context.Context, t *models.Task) error {
	var dueDate *string
	if t.DueDate != nil {
		s := t.DueDate.UTC().Format(time.RFC3339Nano)
		dueDate = &s
	}
	completed := 0
	if t.Completed {
		completed = 1
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET description = ?, assignee = ?, due_date = ?, priority = ?, completed = ? WHERE id = ?`,
		t.Description, t.Assignee, dueDate, string(t.Priority), completed, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update task rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete task rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TaskRepository) DeleteByMeetingID(ctx context.Context, meetingID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE meeting_id = ?`, meetingID)
	if err != nil {
		return fmt.Errorf("delete tasks by meeting: %w", err)
	}
	return nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(row taskScanner) (*models.Task, error) {
	var t models.Task
	var assignee sql.NullString
	var dueDate sql.NullString
	var createdAt string
	var completedInt int64
	var priority string

	err := row.Scan(&t.ID, &t.MeetingID, &t.Description, &assignee, &dueDate, &priority, &completedInt, &createdAt)
	if err != nil {
		return nil, err
	}

	if assignee.Valid {
		v := assignee.String
		t.Assignee = &v
	}
	if dueDate.Valid {
		parsed, err := parseTime(dueDate.String)
		if err != nil {
			return nil, err
		}
		t.DueDate = &parsed
	}
	t.Priority = models.TaskPriority(priority)
	t.Completed = completedInt != 0
	if t.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &t, nil
}
