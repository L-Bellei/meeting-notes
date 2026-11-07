package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"strings"
)

const searchLimit = 20

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
	quotedQ := `"` + strings.ReplaceAll(q, `"`, `""`) + `"*`
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT meeting_id, snippet(meetings_fts, -1, '<b>', '</b>', '...', 15) AS snippet
		 FROM meetings_fts
		 WHERE meetings_fts MATCH ?
		 ORDER BY rank
		 LIMIT %d`, searchLimit),
		quotedQ,
	)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.MeetingID, &result.Snippet); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		snippet := result.Snippet
		snippet = strings.ReplaceAll(snippet, "<b>", "\x00OPEN\x00")
		snippet = strings.ReplaceAll(snippet, "</b>", "\x00CLOSE\x00")
		snippet = html.EscapeString(snippet)
		snippet = strings.ReplaceAll(snippet, "\x00OPEN\x00", "<b>")
		snippet = strings.ReplaceAll(snippet, "\x00CLOSE\x00", "</b>")
		result.Snippet = snippet
		results = append(results, result)
	}
	return results, rows.Err()
}
