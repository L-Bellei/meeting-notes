package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"meeting-notes/internal/models"
)

type KeyPointRepository struct {
	db *sql.DB
}

func NewKeyPointRepository(db *sql.DB) *KeyPointRepository {
	return &KeyPointRepository{db: db}
}

func (r *KeyPointRepository) ListByMeetingID(ctx context.Context, meetingID string) ([]models.KeyPoint, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, meeting_id, position, content FROM key_points WHERE meeting_id = ? ORDER BY position ASC`,
		meetingID,
	)
	if err != nil {
		return nil, fmt.Errorf("list key points: %w", err)
	}
	defer rows.Close()

	kps := []models.KeyPoint{}
	for rows.Next() {
		var kp models.KeyPoint
		if err := rows.Scan(&kp.ID, &kp.MeetingID, &kp.Position, &kp.Content); err != nil {
			return nil, fmt.Errorf("scan key point: %w", err)
		}
		kps = append(kps, kp)
	}
	return kps, rows.Err()
}

func (r *KeyPointRepository) Create(ctx context.Context, kp *models.KeyPoint) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO key_points (id, meeting_id, position, content) VALUES (?, ?, ?, ?)`,
		kp.ID, kp.MeetingID, kp.Position, kp.Content,
	)
	if err != nil {
		return fmt.Errorf("create key point: %w", err)
	}
	return nil
}

func (r *KeyPointRepository) GetByID(ctx context.Context, id string) (*models.KeyPoint, error) {
	var kp models.KeyPoint
	err := r.db.QueryRowContext(ctx,
		`SELECT id, meeting_id, position, content FROM key_points WHERE id = ?`, id,
	).Scan(&kp.ID, &kp.MeetingID, &kp.Position, &kp.Content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get key point: %w", err)
	}
	return &kp, nil
}

func (r *KeyPointRepository) Update(ctx context.Context, kp *models.KeyPoint) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE key_points SET position = ?, content = ? WHERE id = ?`,
		kp.Position, kp.Content, kp.ID,
	)
	if err != nil {
		return fmt.Errorf("update key point: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update key point rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *KeyPointRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM key_points WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete key point: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete key point rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *KeyPointRepository) DeleteByMeetingID(ctx context.Context, meetingID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM key_points WHERE meeting_id = ?`, meetingID)
	if err != nil {
		return fmt.Errorf("delete key points by meeting: %w", err)
	}
	return nil
}
