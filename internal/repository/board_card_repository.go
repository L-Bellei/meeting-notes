package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/models"
)

type BoardCardRepository struct {
	db *sql.DB
}

func NewBoardCardRepository(db *sql.DB) *BoardCardRepository {
	return &BoardCardRepository{db: db}
}

type BoardCardFilters struct {
	Title         string
	Number        *int
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	UpdatedAfter  *time.Time
	UpdatedBefore *time.Time
}

func (r *BoardCardRepository) List(ctx context.Context, f BoardCardFilters) ([]models.BoardCardSummary, error) {
	q := `
		SELECT
			c.id, c.meeting_id, c.column_id, c.number, c.position, c.description,
			c.source, c.updated_at, c.created_at,
			COALESCE(m.title, c.title, '') as meeting_title,
			m.theme_id,
			t.name, t.color,
			col.name,
			CASE WHEN c.meeting_id IS NULL THEN json_array_length(c.tasks) ELSE COUNT(tk.id) END,
			CASE WHEN c.meeting_id IS NULL THEN 0 ELSE COALESCE(SUM(CASE WHEN tk.completed = 1 THEN 1 ELSE 0 END), 0) END
		FROM board_cards c
		LEFT JOIN meetings m ON c.meeting_id = m.id
		JOIN board_columns col ON c.column_id = col.id
		LEFT JOIN themes t ON m.theme_id = t.id
		LEFT JOIN tasks tk ON m.id = tk.meeting_id
		WHERE 1=1`

	var args []any
	if f.Title != "" {
		q += ` AND COALESCE(m.title, c.title, '') LIKE ?`
		args = append(args, "%"+f.Title+"%")
	}
	if f.Number != nil {
		q += ` AND c.number = ?`
		args = append(args, *f.Number)
	}
	if f.CreatedAfter != nil {
		q += ` AND c.created_at >= ?`
		args = append(args, f.CreatedAfter.UTC().Format(time.RFC3339Nano))
	}
	if f.CreatedBefore != nil {
		q += ` AND c.created_at <= ?`
		args = append(args, f.CreatedBefore.UTC().Format(time.RFC3339Nano))
	}
	if f.UpdatedAfter != nil {
		q += ` AND c.updated_at >= ?`
		args = append(args, f.UpdatedAfter.UTC().Format(time.RFC3339Nano))
	}
	if f.UpdatedBefore != nil {
		q += ` AND c.updated_at <= ?`
		args = append(args, f.UpdatedBefore.UTC().Format(time.RFC3339Nano))
	}
	q += ` GROUP BY c.id ORDER BY col.position, c.position`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list board cards: %w", err)
	}
	defer rows.Close()

	var cards []models.BoardCardSummary
	for rows.Next() {
		var card models.BoardCardSummary
		var updatedAt, createdAt string
		var meetingID, themeID, themeName, themeColor sql.NullString
		if err := rows.Scan(
			&card.ID, &meetingID, &card.ColumnID, &card.Number, &card.Position, &card.Description,
			&card.Source, &updatedAt, &createdAt,
			&card.MeetingTitle, &themeID,
			&themeName, &themeColor,
			&card.Status,
			&card.TaskProgress.Total, &card.TaskProgress.Completed,
		); err != nil {
			return nil, fmt.Errorf("scan board card: %w", err)
		}
		if meetingID.Valid {
			s := meetingID.String
			card.MeetingID = &s
		}
		var parseErr error
		if card.UpdatedAt, parseErr = parseTime(updatedAt); parseErr != nil {
			return nil, parseErr
		}
		if card.CreatedAt, parseErr = parseTime(createdAt); parseErr != nil {
			return nil, parseErr
		}
		if themeID.Valid {
			s := themeID.String
			card.ThemeID = &s
		}
		if themeName.Valid {
			s := themeName.String
			card.ThemeName = &s
		}
		if themeColor.Valid {
			s := themeColor.String
			card.ThemeColor = &s
		}
		cards = append(cards, card)
	}
	return cards, rows.Err()
}

func (r *BoardCardRepository) Create(ctx context.Context, meetingID, columnID, description string, position float64) (*models.BoardCard, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var maxNum int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(number), 0) FROM board_cards`).Scan(&maxNum); err != nil {
		return nil, fmt.Errorf("get max number: %w", err)
	}

	now := time.Now().UTC()
	card := &models.BoardCard{
		ID:          uuid.New().String(),
		MeetingID:   &meetingID,
		ColumnID:    columnID,
		Number:      maxNum + 1,
		Position:    position,
		Title:       "",
		Description: description,
		Tasks:       []string{},
		Source:      "meeting",
		UpdatedAt:   now,
		CreatedAt:   now,
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO board_cards (id, meeting_id, column_id, number, position, title, description, tasks, source, updated_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, '[]', ?, ?, ?)`,
		card.ID, card.MeetingID, card.ColumnID, card.Number, card.Position,
		card.Title, card.Description, card.Source,
		card.UpdatedAt.UTC().Format(time.RFC3339Nano),
		card.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create board card: %w", err)
	}
	return card, tx.Commit()
}

func (r *BoardCardRepository) GetByID(ctx context.Context, id string) (*models.BoardCard, error) {
	var card models.BoardCard
	var updatedAt, createdAt, tasksJSON string
	var meetingID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, meeting_id, column_id, number, position, title, description, tasks, source, updated_at, created_at
		 FROM board_cards WHERE id = ?`, id,
	).Scan(&card.ID, &meetingID, &card.ColumnID, &card.Number, &card.Position,
		&card.Title, &card.Description, &tasksJSON, &card.Source, &updatedAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get board card: %w", err)
	}
	if meetingID.Valid {
		s := meetingID.String
		card.MeetingID = &s
	}
	if err := json.Unmarshal([]byte(tasksJSON), &card.Tasks); err != nil {
		card.Tasks = []string{}
	}
	var parseErr error
	if card.UpdatedAt, parseErr = parseTime(updatedAt); parseErr != nil {
		return nil, parseErr
	}
	if card.CreatedAt, parseErr = parseTime(createdAt); parseErr != nil {
		return nil, parseErr
	}
	return &card, nil
}

func (r *BoardCardRepository) GetByMeetingID(ctx context.Context, meetingID string) (*models.BoardCard, error) {
	var card models.BoardCard
	var updatedAt, createdAt, tasksJSON string
	var mid sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, meeting_id, column_id, number, position, title, description, tasks, source, updated_at, created_at
		 FROM board_cards WHERE meeting_id = ?`, meetingID,
	).Scan(&card.ID, &mid, &card.ColumnID, &card.Number, &card.Position,
		&card.Title, &card.Description, &tasksJSON, &card.Source, &updatedAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get board card by meeting: %w", err)
	}
	if mid.Valid {
		s := mid.String
		card.MeetingID = &s
	}
	if err := json.Unmarshal([]byte(tasksJSON), &card.Tasks); err != nil {
		card.Tasks = []string{}
	}
	var parseErr error
	if card.UpdatedAt, parseErr = parseTime(updatedAt); parseErr != nil {
		return nil, parseErr
	}
	if card.CreatedAt, parseErr = parseTime(createdAt); parseErr != nil {
		return nil, parseErr
	}
	return &card, nil
}

func (r *BoardCardRepository) GetDetail(ctx context.Context, id string) (*models.BoardCardDetail, error) {
	var d models.BoardCardDetail
	var updatedAt, createdAt, tasksJSON string
	var meetingID, themeID, themeName, themeColor sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.meeting_id, c.column_id, c.number, c.position, c.title, c.description,
		       c.tasks, c.source, c.updated_at, c.created_at,
		       col.name,
		       COALESCE(m.title, c.title, ''),
		       m.theme_id,
		       t.name, t.color
		FROM board_cards c
		LEFT JOIN meetings m ON c.meeting_id = m.id
		JOIN board_columns col ON c.column_id = col.id
		LEFT JOIN themes t ON m.theme_id = t.id
		WHERE c.id = ?`, id,
	).Scan(
		&d.ID, &meetingID, &d.ColumnID, &d.Number, &d.Position, &d.Title, &d.Description,
		&tasksJSON, &d.Source, &updatedAt, &createdAt,
		&d.Status,
		&d.MeetingTitle, &themeID,
		&themeName, &themeColor,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get board card detail: %w", err)
	}
	if meetingID.Valid {
		s := meetingID.String
		d.MeetingID = &s
	}
	if err := json.Unmarshal([]byte(tasksJSON), &d.ManualTasks); err != nil {
		d.ManualTasks = []string{}
	}
	var parseErr error
	if d.UpdatedAt, parseErr = parseTime(updatedAt); parseErr != nil {
		return nil, parseErr
	}
	if d.CreatedAt, parseErr = parseTime(createdAt); parseErr != nil {
		return nil, parseErr
	}
	if themeID.Valid {
		s := themeID.String
		d.ThemeID = &s
	}
	if themeName.Valid {
		s := themeName.String
		d.ThemeName = &s
	}
	if themeColor.Valid {
		s := themeColor.String
		d.ThemeColor = &s
	}
	return &d, nil
}

func (r *BoardCardRepository) Update(ctx context.Context, id, description string, tasks []string) error {
	tasksJSON, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx,
		`UPDATE board_cards SET description = ?, tasks = ?, updated_at = ? WHERE id = ?`,
		description, string(tasksJSON), now, id,
	)
	if err != nil {
		return fmt.Errorf("update board card: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BoardCardRepository) Move(ctx context.Context, id, columnID string, position float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx,
		`UPDATE board_cards SET column_id = ?, position = ?, updated_at = ? WHERE id = ?`,
		columnID, position, now, id,
	)
	if err != nil {
		return fmt.Errorf("move board card: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	if err := r.rebalanceIfNeeded(ctx, tx, columnID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *BoardCardRepository) rebalanceIfNeeded(ctx context.Context, tx *sql.Tx, columnID string) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT position FROM board_cards WHERE column_id = ? ORDER BY position`, columnID)
	if err != nil {
		return fmt.Errorf("check rebalance: %w", err)
	}
	var prev *float64
	needs := false
	for rows.Next() {
		var pos float64
		if err := rows.Scan(&pos); err != nil {
			rows.Close()
			return err
		}
		if prev != nil && pos-*prev < 1e-9 {
			needs = true
			break
		}
		prev = &pos
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if !needs {
		return nil
	}
	return r.rebalanceColumn(ctx, tx, columnID)
}

func (r *BoardCardRepository) rebalanceColumn(ctx context.Context, tx *sql.Tx, columnID string) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM board_cards WHERE column_id = ? ORDER BY position`, columnID)
	if err != nil {
		return fmt.Errorf("list for rebalance: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE board_cards SET position = ? WHERE id = ?`, float64((i+1)*1000), id,
		); err != nil {
			return fmt.Errorf("rebalance card %s: %w", id, err)
		}
	}
	return nil
}

func (r *BoardCardRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM board_cards WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete board card: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BoardCardRepository) LastPositionInColumn(ctx context.Context, columnID string) (float64, error) {
	var pos sql.NullFloat64
	err := r.db.QueryRowContext(ctx,
		`SELECT MAX(position) FROM board_cards WHERE column_id = ?`, columnID,
	).Scan(&pos)
	if err != nil {
		return 0, fmt.Errorf("last position: %w", err)
	}
	return pos.Float64, nil
}

func (r *BoardCardRepository) CreateManual(ctx context.Context, columnID, title, description string, tasks []string, position float64) (*models.BoardCard, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var maxNum int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(number), 0) FROM board_cards`).Scan(&maxNum); err != nil {
		return nil, fmt.Errorf("get max number: %w", err)
	}

	tasksJSON, err := json.Marshal(tasks)
	if err != nil {
		return nil, fmt.Errorf("marshal tasks: %w", err)
	}

	now := time.Now().UTC()
	card := &models.BoardCard{
		ID:          uuid.New().String(),
		MeetingID:   nil,
		ColumnID:    columnID,
		Number:      maxNum + 1,
		Position:    position,
		Title:       title,
		Description: description,
		Tasks:       tasks,
		Source:      "manual",
		UpdatedAt:   now,
		CreatedAt:   now,
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO board_cards (id, meeting_id, column_id, number, position, title, description, tasks, source, updated_at, created_at)
		 VALUES (?, NULL, ?, ?, ?, ?, ?, ?, 'manual', ?, ?)`,
		card.ID, card.ColumnID, card.Number, card.Position,
		card.Title, card.Description, string(tasksJSON),
		card.UpdatedAt.UTC().Format(time.RFC3339Nano),
		card.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("create manual card: %w", err)
	}
	return card, tx.Commit()
}

func (r *BoardCardRepository) LinkToMeeting(ctx context.Context, cardID, meetingID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx,
		`UPDATE board_cards SET meeting_id = ?, source = 'meeting', updated_at = ? WHERE id = ? AND meeting_id IS NULL`,
		meetingID, now, cardID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrDuplicate
		}
		return fmt.Errorf("link card to meeting: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var exists int
		_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM board_cards WHERE id = ?`, cardID).Scan(&exists)
		if exists == 0 {
			return ErrNotFound
		}
		return ErrDuplicate
	}
	return nil
}
