package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) FindBySHA1(ctx context.Context, sha1 string) (*Book, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT book_key, title, author, description, format, filename, storage_path, cover_storage_path,
file_size, content_sha1, language, chapter_count, word_count, status, source, updated_by
FROM catalog_books WHERE content_sha1 = ? AND status <> 'deleted' LIMIT 1`, sha1)
	book, err := scanBook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find catalog book by sha1: %w", err)
	}
	return &book, nil
}

func (s *MySQLStore) FindByKey(ctx context.Context, bookKey string) (*Book, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT book_key, title, author, description, format, filename, storage_path, cover_storage_path,
file_size, content_sha1, language, chapter_count, word_count, status, source, updated_by
FROM catalog_books WHERE book_key = ? AND status <> 'deleted'`, bookKey)
	book, err := scanBook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find catalog book by key: %w", err)
	}
	return &book, nil
}

func (s *MySQLStore) List(ctx context.Context, query string, status BookStatus, limit, offset int) ([]Book, int, error) {
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	where := "WHERE status <> 'deleted'"
	args := []any{}
	if status != "" {
		where += " AND status = ?"
		args = append(args, string(status))
	}
	query = strings.TrimSpace(query)
	if query != "" {
		where += " AND (LOWER(title) LIKE ? OR LOWER(author) LIKE ? OR book_key LIKE ?)"
		like := "%" + strings.ToLower(query) + "%"
		args = append(args, like, like, like)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM catalog_books "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count catalog books: %w", err)
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT book_key, title, author, description, format, filename, storage_path, cover_storage_path,
file_size, content_sha1, language, chapter_count, word_count, status, source, updated_by
FROM catalog_books `+where+`
ORDER BY uploaded_at DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list catalog books: %w", err)
	}
	defer rows.Close()
	books := []Book{}
	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan catalog book: %w", err)
		}
		books = append(books, book)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate catalog books: %w", err)
	}
	return books, total, nil
}

func (s *MySQLStore) Create(ctx context.Context, book Book) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO catalog_books
(book_key, title, author, description, format, filename, storage_path, cover_storage_path,
file_size, content_sha1, language, chapter_count, word_count, status, source, uploaded_at, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), ?)`,
		book.BookKey, book.Title, nullableString(book.Author), nullableString(book.Description),
		book.Format, book.Filename, book.StoragePath, nullableString(book.CoverStoragePath),
		book.FileSize, nullableString(book.ContentSHA1), nullableString(book.Language),
		book.ChapterCount, nullableInt64(book.WordCount), string(book.Status), book.Source,
		nullableString(book.UpdatedBy),
	)
	if err != nil {
		return fmt.Errorf("create catalog book: %w", err)
	}
	return nil
}

func (s *MySQLStore) Update(ctx context.Context, book Book) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE catalog_books
SET title = ?, author = ?, description = ?, language = ?, updated_by = ?
WHERE book_key = ?`,
		book.Title, nullableString(book.Author), nullableString(book.Description),
		nullableString(book.Language), nullableString(book.UpdatedBy), book.BookKey,
	)
	if err != nil {
		return fmt.Errorf("update catalog book: %w", err)
	}
	return nil
}

func (s *MySQLStore) UpdateStatus(ctx context.Context, bookKey string, status BookStatus, admin string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE catalog_books
SET status = ?,
    published_at = CASE WHEN ? = 'active' THEN NOW() ELSE published_at END,
    deleted_at = CASE WHEN ? = 'deleted' THEN NOW() ELSE deleted_at END,
    updated_by = ?
WHERE book_key = ?`,
		string(status), string(status), string(status), nullableString(admin), bookKey,
	)
	if err != nil {
		return fmt.Errorf("update catalog book status: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanBook(row scanner) (Book, error) {
	var book Book
	var author, description, coverPath, sha1, language, updatedBy sql.NullString
	var wordCount sql.NullInt64
	if err := row.Scan(
		&book.BookKey, &book.Title, &author, &description, &book.Format, &book.Filename,
		&book.StoragePath, &coverPath, &book.FileSize, &sha1, &language, &book.ChapterCount,
		&wordCount, &book.Status, &book.Source, &updatedBy,
	); err != nil {
		return Book{}, err
	}
	book.Author = author.String
	book.Description = description.String
	book.CoverStoragePath = coverPath.String
	book.ContentSHA1 = sha1.String
	book.Language = language.String
	book.UpdatedBy = updatedBy.String
	book.WordCount = wordCount.Int64
	return book, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
