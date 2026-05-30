package usersadmin

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

func (s *MySQLStore) List(ctx context.Context, query string, limit, offset int) ([]User, int, error) {
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	where := "WHERE 1=1"
	args := []any{}
	query = strings.TrimSpace(query)
	if query != "" {
		where += " AND (CAST(id AS CHAR) = ? OR LOWER(email) LIKE ? OR LOWER(nickname) LIKE ?)"
		like := "%" + strings.ToLower(query) + "%"
		args = append(args, query, like, like)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, COALESCE(email, ''), nickname, account_status, created_at
FROM users `+where+`
ORDER BY created_at DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}
	return users, total, nil
}

func (s *MySQLStore) UpdateStatus(ctx context.Context, id string, status string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE users
SET account_status = ?, status = CASE WHEN ? = 'active' THEN 1 WHEN ? = 'frozen' THEN 2 ELSE 3 END
WHERE id = ?`, status, status, status, id)
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	return nil
}
