package communityadmin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) ListThreads(ctx context.Context, query string, limit, offset int) ([]Thread, int, error) {
	if limit < 1 {
		limit = 20
	}
	where := "WHERE status <> 'deleted'"
	args := []any{}
	query = strings.TrimSpace(query)
	if query != "" {
		where += " AND (LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR id = ?)"
		like := "%" + strings.ToLower(query) + "%"
		args = append(args, like, like, query)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM community_threads "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count community threads: %w", err)
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, board_id, title, CAST(user_id AS CHAR), status, created_at, comment_count
FROM community_threads `+where+`
ORDER BY updated_at DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list community threads: %w", err)
	}
	defer rows.Close()
	threads := []Thread{}
	for rows.Next() {
		var thread Thread
		if err := rows.Scan(&thread.ID, &thread.BoardID, &thread.Title, &thread.AuthorID, &thread.Status, &thread.CreatedAt, &thread.CommentCount); err != nil {
			return nil, 0, fmt.Errorf("scan community thread: %w", err)
		}
		threads = append(threads, thread)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate community threads: %w", err)
	}
	return threads, total, nil
}

func (s *MySQLStore) UpdateThreadStatus(ctx context.Context, id string, status string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE community_threads SET status = ?, updated_at = NOW() WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("update community thread status: %w", err)
	}
	return nil
}

func (s *MySQLStore) ListComments(ctx context.Context, threadID string, limit, offset int) ([]Comment, int, error) {
	if limit < 1 {
		limit = 20
	}
	where := "WHERE status <> 'deleted'"
	args := []any{}
	if strings.TrimSpace(threadID) != "" {
		where += " AND thread_id = ?"
		args = append(args, threadID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM community_comments "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count community comments: %w", err)
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, thread_id, CAST(user_id AS CHAR), content, status, created_at
FROM community_comments `+where+`
ORDER BY created_at DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list community comments: %w", err)
	}
	defer rows.Close()
	comments := []Comment{}
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(&comment.ID, &comment.ThreadID, &comment.AuthorID, &comment.Body, &comment.Status, &comment.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan community comment: %w", err)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate community comments: %w", err)
	}
	return comments, total, nil
}

func (s *MySQLStore) UpdateCommentStatus(ctx context.Context, id string, status string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE community_comments SET status = ?, updated_at = NOW() WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("update community comment status: %w", err)
	}
	return nil
}
