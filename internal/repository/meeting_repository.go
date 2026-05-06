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

// ListFilters holds all optional filters for listing meetings.
type ListFilters struct {
	ThemeIDs     []string // empty = no filter; multiple = OR match
	Status       string
	Q            string // title LIKE %q%
	StartedAfter string // RFC3339 date
	StartedBefore string // RFC3339 date
}

func (r *MeetingRepository) List(ctx context.Context, f ListFilters) ([]models.Meeting, error) {
	query := `SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, notes, audio_path, error_message, created_at FROM meetings`
	var args []any
	var conditions []string

	if len(f.ThemeIDs) == 1 {
		conditions = append(conditions, "theme_id = ?")
		args = append(args, f.ThemeIDs[0])
	} else if len(f.ThemeIDs) > 1 {
		placeholders := strings.Repeat("?,", len(f.ThemeIDs))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, "theme_id IN ("+placeholders+")")
		for _, id := range f.ThemeIDs {
			args = append(args, id)
		}
	}
	if f.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, f.Status)
	}
	if f.Q != "" {
		conditions = append(conditions, "title LIKE ?")
		args = append(args, "%"+f.Q+"%")
	}
	if f.StartedAfter != "" {
		conditions = append(conditions, "started_at >= ?")
		args = append(args, f.StartedAfter)
	}
	if f.StartedBefore != "" {
		conditions = append(conditions, "started_at <= ?")
		args = append(args, f.StartedBefore)
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
	if meetings == nil {
		meetings = []models.Meeting{}
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
		`INSERT INTO meetings (id, theme_id, title, started_at, duration_seconds, status, transcript, notes, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ThemeID, m.Title, startedAt, m.DurationSeconds, string(m.Status), m.Transcript, m.Notes,
		m.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create meeting: %w", err)
	}
	return nil
}

func (r *MeetingRepository) GetByID(ctx context.Context, id string) (*models.Meeting, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, notes, audio_path, error_message, created_at FROM meetings WHERE id = ?`, id,
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
		`UPDATE meetings SET theme_id = ?, title = ?, started_at = ?, duration_seconds = ?, status = ?, transcript = ?, notes = ?, audio_path = ?, error_message = ? WHERE id = ?`,
		m.ThemeID, m.Title, startedAt, m.DurationSeconds, string(m.Status), m.Transcript, m.Notes, m.AudioPath, m.ErrorMessage, m.ID,
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

func (r *MeetingRepository) GetRecording(ctx context.Context) (*models.Meeting, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, notes, audio_path, error_message, created_at
     FROM meetings WHERE status = 'recording' LIMIT 1`,
	)
	m, err := scanMeeting(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get recording meeting: %w", err)
	}
	return m, nil
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
	var notes sql.NullString
	var audioPath sql.NullString
	var errorMessage sql.NullString
	var createdAt string
	var status string

	err := row.Scan(&m.ID, &themeID, &m.Title, &startedAt, &duration, &status,
		&transcript, &notes, &audioPath, &errorMessage, &createdAt)
	if err != nil {
		return nil, err
	}

	if themeID.Valid {
		v := themeID.String
		m.ThemeID = &v
	}
	if startedAt.Valid {
		t, err := parseTime(startedAt.String)
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
	if notes.Valid {
		v := notes.String
		m.Notes = &v
	}
	if audioPath.Valid {
		v := audioPath.String
		m.AudioPath = &v
	}
	if errorMessage.Valid {
		v := errorMessage.String
		m.ErrorMessage = &v
	}
	m.Status = models.MeetingStatus(status)
	if m.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &m, nil
}
