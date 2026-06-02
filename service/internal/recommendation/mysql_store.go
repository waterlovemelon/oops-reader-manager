package recommendation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) FindByID(ctx context.Context, id int64) (*Recommendation, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, book_key, comment, status, scheduled_publish_at,
       created_by, updated_by, deleted_at, created_at, updated_at
FROM catalog_book_recommendations WHERE id = ?`, id)
	rec, err := scanRecommendation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find recommendation by id: %w", err)
	}
	return &rec, nil
}

func (s *MySQLStore) FindActiveBook(ctx context.Context, bookKey string) (*BookSnapshot, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT book_key, title, author, description, cover_storage_path
FROM catalog_books WHERE book_key = ? AND status <> 'deleted'`, bookKey)
	var snap BookSnapshot
	var author, description, coverPath sql.NullString
	if err := row.Scan(&snap.BookKey, &snap.Title, &author, &description, &coverPath); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find active book: %w", err)
	}
	snap.Author = author.String
	snap.Description = description.String
	snap.CoverPath = coverPath.String
	return &snap, nil
}

func (s *MySQLStore) List(ctx context.Context, input ListInput) ([]Recommendation, int, error) {
	if input.Limit < 1 {
		input.Limit = 20
	}
	if input.Offset < 0 {
		input.Offset = 0
	}
	where := "WHERE 1=1"
	args := []any{}
	if input.Status == "deleted" {
		where += " AND r.status = 'deleted'"
	} else if input.Status == "queued" {
		where += " AND r.status = 'active' AND r.deleted_at IS NULL AND r.scheduled_publish_at > NOW()"
	} else if input.Status == "published" {
		where += " AND r.status = 'active' AND r.deleted_at IS NULL AND r.scheduled_publish_at <= NOW()"
	} else {
		// Default: exclude deleted records
		where += " AND r.deleted_at IS NULL"
	}
	query := strings.TrimSpace(input.Query)
	if query != "" {
		where += " AND (LOWER(b.title) LIKE ? OR r.book_key LIKE ? OR LOWER(r.comment) LIKE ?)"
		like := "%" + strings.ToLower(query) + "%"
		args = append(args, like, like, like)
	}
	var total int
	countSQL := "SELECT COUNT(*) FROM catalog_book_recommendations r LEFT JOIN catalog_books b ON r.book_key = b.book_key " + where
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count recommendations: %w", err)
	}
	listArgs := append(append([]any{}, args...), input.Limit, input.Offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.book_key, r.comment, r.status, r.scheduled_publish_at,
       r.created_by, r.updated_by, r.deleted_at, r.created_at, r.updated_at
FROM catalog_book_recommendations r
LEFT JOIN catalog_books b ON r.book_key = b.book_key
`+where+`
ORDER BY r.scheduled_publish_at DESC, r.id DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list recommendations: %w", err)
	}
	defer rows.Close()
	var recs []Recommendation
	for rows.Next() {
		rec, err := scanRecommendationRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan recommendation: %w", err)
		}
		recs = append(recs, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate recommendations: %w", err)
	}
	return recs, total, nil
}

func (s *MySQLStore) Create(ctx context.Context, input CreateInput) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
INSERT INTO catalog_book_recommendations (book_key, comment, scheduled_publish_at, created_by)
VALUES (?, ?, ?, ?)`,
		input.BookKey, input.Comment, input.ScheduledPublishAt, nullableStr(input.CreatedBy),
	)
	if err != nil {
		return 0, fmt.Errorf("create recommendation: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

func (s *MySQLStore) Update(ctx context.Context, input UpdateInput) error {
	setClauses := []string{}
	args := []any{}
	if input.Comment != nil {
		setClauses = append(setClauses, "comment = ?")
		args = append(args, *input.Comment)
	}
	if input.ScheduledPublishAt != nil {
		setClauses = append(setClauses, "scheduled_publish_at = ?")
		args = append(args, *input.ScheduledPublishAt)
	}
	if len(setClauses) == 0 {
		return nil
	}
	setClauses = append(setClauses, "updated_by = ?")
	args = append(args, nullableStr(input.UpdatedBy))
	args = append(args, input.ID)
	_, err := s.db.ExecContext(ctx, `
UPDATE catalog_book_recommendations SET `+strings.Join(setClauses, ", ")+` WHERE id = ? AND deleted_at IS NULL`, args...)
	if err != nil {
		return fmt.Errorf("update recommendation: %w", err)
	}
	return nil
}

func (s *MySQLStore) SoftDelete(ctx context.Context, id int64, deletedBy string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE catalog_book_recommendations
SET status = 'deleted', deleted_at = NOW(), updated_by = ?
WHERE id = ? AND deleted_at IS NULL`,
		nullableStr(deletedBy), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete recommendation: %w", err)
	}
	return nil
}

type recScanner interface {
	Scan(dest ...any) error
}

func scanRecommendation(row recScanner) (Recommendation, error) {
	var rec Recommendation
	var updatedBy, createdBy sql.NullString
	var deletedAt sql.NullTime
	if err := row.Scan(
		&rec.ID, &rec.BookKey, &rec.Comment, &rec.Status, &rec.ScheduledPublishAt,
		&createdBy, &updatedBy, &deletedAt, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return Recommendation{}, err
	}
	rec.CreatedBy = createdBy.String
	rec.UpdatedBy = updatedBy.String
	if deletedAt.Valid {
		rec.DeletedAt = &deletedAt.Time
	}
	return rec, nil
}

func scanRecommendationRows(row *sql.Rows) (Recommendation, error) {
	return scanRecommendation(row)
}

func nullableStr(value string) any {
	if value == "" {
		return nil
	}
	return value
}
