package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/models"
)

type BoardColumnRepository struct {
	db *sql.DB
}

func NewBoardColumnRepository(db *sql.DB) *BoardColumnRepository {
	return &BoardColumnRepository{db: db}
}

type ReorderItem struct {
	ID       string  `json:"id"`
	Position float64 `json:"position"`
}

func (r *BoardColumnRepository) List(ctx context.Context) ([]models.BoardColumnWithCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT col.id, col.name, col.position, col.created_at, COUNT(c.id) AS card_count
		FROM board_columns col
		LEFT JOIN board_cards c ON col.id = c.column_id
		GROUP BY col.id
		ORDER BY col.position`)
	if err != nil {
		return nil, fmt.Errorf("list board columns: %w", err)
	}
	defer rows.Close()
	var cols []models.BoardColumnWithCount
	for rows.Next() {
		var col models.BoardColumnWithCount
		var createdAt string
		if err := rows.Scan(&col.ID, &col.Name, &col.Position, &createdAt, &col.CardCount); err != nil {
			return nil, fmt.Errorf("scan board column: %w", err)
		}
		var parseErr error
		if col.CreatedAt, parseErr = parseTime(createdAt); parseErr != nil {
			return nil, parseErr
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

func (r *BoardColumnRepository) Create(ctx context.Context, name string) (*models.BoardColumn, error) {
	var maxPos sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `SELECT MAX(position) FROM board_columns`).Scan(&maxPos); err != nil {
		return nil, fmt.Errorf("get max position: %w", err)
	}
	col := &models.BoardColumn{
		ID:        uuid.New().String(),
		Name:      name,
		Position:  maxPos.Float64 + 1000,
		CreatedAt: time.Now().UTC(),
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO board_columns (id, name, position, created_at) VALUES (?, ?, ?, ?)`,
		col.ID, col.Name, col.Position, col.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("create board column: %w", err)
	}
	return col, nil
}

func (r *BoardColumnRepository) GetByID(ctx context.Context, id string) (*models.BoardColumn, error) {
	var col models.BoardColumn
	var createdAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, position, created_at FROM board_columns WHERE id = ?`, id,
	).Scan(&col.ID, &col.Name, &col.Position, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get board column: %w", err)
	}
	var parseErr error
	if col.CreatedAt, parseErr = parseTime(createdAt); parseErr != nil {
		return nil, parseErr
	}
	return &col, nil
}

func (r *BoardColumnRepository) Update(ctx context.Context, id, name string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE board_columns SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("update board column: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BoardColumnRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM board_columns WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete board column: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BoardColumnRepository) DeleteWithMove(ctx context.Context, id, moveTo string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`UPDATE board_cards SET column_id = ? WHERE column_id = ?`, moveTo, id,
	); err != nil {
		return fmt.Errorf("move cards: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM board_columns WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete column: %w", err)
	}
	return tx.Commit()
}

func (r *BoardColumnRepository) Reorder(ctx context.Context, items []ReorderItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	for _, item := range items {
		if _, err := tx.ExecContext(ctx,
			`UPDATE board_columns SET position = ? WHERE id = ?`, item.Position, item.ID,
		); err != nil {
			return fmt.Errorf("reorder column %s: %w", item.ID, err)
		}
	}
	return tx.Commit()
}

func (r *BoardColumnRepository) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM board_columns`).Scan(&n)
	return n, err
}

func (r *BoardColumnRepository) CardCount(ctx context.Context, columnID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM board_cards WHERE column_id = ?`, columnID,
	).Scan(&n)
	return n, err
}
