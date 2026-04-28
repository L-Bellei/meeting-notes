package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"meeting-notes/internal/models"
)

type SummaryRepository struct {
	db *sql.DB
}

func NewSummaryRepository(db *sql.DB) *SummaryRepository {
	return &SummaryRepository{db: db}
}

func (r *SummaryRepository) GetByMeetingID(ctx context.Context, meetingID string) (*models.Summary, error) {
	var s models.Summary
	var createdAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, meeting_id, content, model_used, input_tokens, output_tokens, created_at FROM summaries WHERE meeting_id = ?`,
		meetingID,
	).Scan(&s.ID, &s.MeetingID, &s.Content, &s.ModelUsed, &s.InputTokens, &s.OutputTokens, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get summary: %w", err)
	}
	if s.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SummaryRepository) Upsert(ctx context.Context, s *models.Summary) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO summaries (id, meeting_id, content, model_used, input_tokens, output_tokens, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(meeting_id) DO UPDATE SET
		   id = excluded.id,
		   content = excluded.content,
		   model_used = excluded.model_used,
		   input_tokens = excluded.input_tokens,
		   output_tokens = excluded.output_tokens,
		   created_at = excluded.created_at`,
		s.ID, s.MeetingID, s.Content, s.ModelUsed, s.InputTokens, s.OutputTokens,
		s.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert summary: %w", err)
	}
	return nil
}

func (r *SummaryRepository) Delete(ctx context.Context, meetingID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM summaries WHERE meeting_id = ?`, meetingID)
	if err != nil {
		return fmt.Errorf("delete summary: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete summary rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
