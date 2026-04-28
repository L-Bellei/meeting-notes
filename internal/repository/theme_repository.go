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

var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("name already exists")
)

type ThemeRepository struct {
	db *sql.DB
}

func NewThemeRepository(db *sql.DB) *ThemeRepository {
	return &ThemeRepository{db: db}
}

func (r *ThemeRepository) List(ctx context.Context) ([]models.Theme, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, parent_id, name, description, color, created_at FROM themes ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list themes: %w", err)
	}
	defer rows.Close()

	var themes []models.Theme
	for rows.Next() {
		t, err := scanTheme(rows)
		if err != nil {
			return nil, fmt.Errorf("scan theme: %w", err)
		}
		themes = append(themes, *t)
	}
	return themes, rows.Err()
}

func (r *ThemeRepository) Create(ctx context.Context, theme *models.Theme) error {
	if theme.CreatedAt.IsZero() {
		theme.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO themes (id, parent_id, name, description, color, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		theme.ID, theme.ParentID, theme.Name, theme.Description, theme.Color,
		theme.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrDuplicate
		}
		return fmt.Errorf("create theme: %w", err)
	}
	return nil
}

func (r *ThemeRepository) GetByID(ctx context.Context, id string) (*models.Theme, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, parent_id, name, description, color, created_at FROM themes WHERE id = ?`, id)
	t, err := scanTheme(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get theme: %w", err)
	}
	return t, nil
}

// ChildIDs returns the IDs of all direct children of the given theme.
func (r *ThemeRepository) ChildIDs(ctx context.Context, parentID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM themes WHERE parent_id = ?`, parentID)
	if err != nil {
		return nil, fmt.Errorf("child theme ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *ThemeRepository) Update(ctx context.Context, theme *models.Theme) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE themes SET parent_id = ?, name = ?, description = ?, color = ? WHERE id = ?`,
		theme.ParentID, theme.Name, theme.Description, theme.Color, theme.ID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrDuplicate
		}
		return fmt.Errorf("update theme: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update theme rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ThemeRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM themes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete theme: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete theme rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type themeScanner interface {
	Scan(dest ...any) error
}

func scanTheme(row themeScanner) (*models.Theme, error) {
	var t models.Theme
	var parentID sql.NullString
	var createdAt string
	if err := row.Scan(&t.ID, &parentID, &t.Name, &t.Description, &t.Color, &createdAt); err != nil {
		return nil, err
	}
	if parentID.Valid {
		v := parentID.String
		t.ParentID = &v
	}
	var err error
	if t.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &t, nil
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}
