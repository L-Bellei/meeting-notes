package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type SearchResult struct {
	MeetingID string
	Snippet   string
}

type SearchRepository struct {
	db *sql.DB
}

func NewSearchRepository(db *sql.DB) *SearchRepository {
	return &SearchRepository{db: db}
}

func (r *SearchRepository) Search(ctx context.Context, q string) ([]SearchResult, error) {
	if q == "" {
		return nil, errors.New("query must not be empty")
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT meeting_id, snippet(meetings_fts, -1, '<b>', '</b>', '...', 15) AS snippet
		 FROM meetings_fts
		 WHERE meetings_fts MATCH ?
		 ORDER BY rank
		 LIMIT 20`,
		q,
	)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.MeetingID, &r.Snippet); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (r *SearchRepository) UpsertMeeting(ctx context.Context, meetingID, title, transcript, summary, keyPoints, tasks string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM meetings_fts WHERE meeting_id = ?`, meetingID); err != nil {
		return fmt.Errorf("delete from fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		meetingID, title, transcript, summary, keyPoints, tasks,
	); err != nil {
		return fmt.Errorf("insert into fts: %w", err)
	}
	return tx.Commit()
}

func (r *SearchRepository) DeleteMeeting(ctx context.Context, meetingID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM meetings_fts WHERE meeting_id = ?`, meetingID)
	return err
}
