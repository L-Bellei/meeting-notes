package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"meeting-notes/internal/models"
)

type MeetingRepository struct {
	db *sql.DB
}

func NewMeetingRepository(db *sql.DB) *MeetingRepository {
	return &MeetingRepository{db: db}
}

func (r *MeetingRepository) List(ctx context.Context, themeID, status string) ([]models.Meeting, error) {
	query := `SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, created_at FROM meetings`
	var args []any
	var conditions []string

	if themeID != "" {
		conditions = append(conditions, "theme_id = ?")
		args = append(args, themeID)
	}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY started_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}
	defer rows.Close()

	var meetings []models.Meeting
	for rows.Next() {
		m, err := scanMeeting(rows)
		if err != nil {
			return nil, fmt.Errorf("scan meeting: %w", err)
		}
		meetings = append(meetings, *m)
	}
	return meetings, rows.Err()
}

func (r *MeetingRepository) Create(ctx context.Context, m *models.Meeting) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	var startedAt *string
	if m.StartedAt != nil {
		s := m.StartedAt.UTC().Format(time.RFC3339Nano)
		startedAt = &s
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO meetings (id, theme_id, title, started_at, duration_seconds, status, transcript, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ThemeID, m.Title, startedAt, m.DurationSeconds, string(m.Status), m.Transcript,
		m.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create meeting: %w", err)
	}
	return nil
}

func (r *MeetingRepository) GetByID(ctx context.Context, id string) (*models.Meeting, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, created_at FROM meetings WHERE id = ?`, id,
	)
	m, err := scanMeeting(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting: %w", err)
	}
	return m, nil
}

func (r *MeetingRepository) Update(ctx context.Context, m *models.Meeting) error {
	var startedAt *string
	if m.StartedAt != nil {
		s := m.StartedAt.UTC().Format(time.RFC3339Nano)
		startedAt = &s
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE meetings SET theme_id = ?, title = ?, started_at = ?, duration_seconds = ?, status = ?, transcript = ? WHERE id = ?`,
		m.ThemeID, m.Title, startedAt, m.DurationSeconds, string(m.Status), m.Transcript, m.ID,
	)
	if err != nil {
		return fmt.Errorf("update meeting: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update meeting rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MeetingRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM meetings WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete meeting: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete meeting rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type meetingScanner interface {
	Scan(dest ...any) error
}

func scanMeeting(row meetingScanner) (*models.Meeting, error) {
	var m models.Meeting
	var themeID sql.NullString
	var startedAt sql.NullString
	var duration sql.NullInt64
	var transcript sql.NullString
	var createdAt string
	var status string

	err := row.Scan(&m.ID, &themeID, &m.Title, &startedAt, &duration, &status, &transcript, &createdAt)
	if err != nil {
		return nil, err
	}

	if themeID.Valid {
		v := themeID.String
		m.ThemeID = &v
	}
	if startedAt.Valid {
		t, err := parseMeetingTime(startedAt.String)
		if err != nil {
			return nil, err
		}
		m.StartedAt = &t
	}
	if duration.Valid {
		d := int(duration.Int64)
		m.DurationSeconds = &d
	}
	if transcript.Valid {
		v := transcript.String
		m.Transcript = &v
	}
	m.Status = models.MeetingStatus(status)
	if m.CreatedAt, err = parseMeetingTime(createdAt); err != nil {
		return nil, err
	}
	return &m, nil
}

func parseMeetingTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}
