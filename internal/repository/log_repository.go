package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"meeting-notes/internal/models"
)

type LogRepository struct{ db *sql.DB }

func NewLogRepository(db *sql.DB) *LogRepository { return &LogRepository{db: db} }

func (r *LogRepository) Insert(ctx context.Context, level, component, message string, metadata *string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_logs (id, level, component, message, metadata) VALUES (?,?,?,?,?)`,
		uuid.New().String(), level, component, message, metadata)
	return err
}

func (r *LogRepository) List(ctx context.Context, limit int) ([]models.AppLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, level, component, message, metadata, created_at
         FROM app_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AppLog
	for rows.Next() {
		var l models.AppLog
		if err := rows.Scan(&l.ID, &l.Level, &l.Component, &l.Message, &l.Metadata, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
