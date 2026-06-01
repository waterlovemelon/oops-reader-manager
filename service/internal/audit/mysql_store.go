package audit

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

func (s *MySQLStore) Create(ctx context.Context, entry Entry) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO admin_audit_logs
(admin_username, action, resource_type, resource_id, before_json, after_json, ip_address, user_agent)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.AdminUsername,
		entry.Action,
		entry.ResourceType,
		entry.ResourceID,
		nullableBytes(entry.BeforeJSON),
		nullableBytes(entry.AfterJSON),
		nullableString(entry.IPAddress),
		nullableString(entry.UserAgent),
	)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func (s *MySQLStore) List(ctx context.Context, input ListInput) ([]Entry, int, error) {
	if input.Limit < 1 {
		input.Limit = 50
	}
	if input.Offset < 0 {
		input.Offset = 0
	}
	where := "WHERE 1=1"
	args := []any{}
	if strings.TrimSpace(input.AdminUsername) != "" {
		where += " AND admin_username = ?"
		args = append(args, input.AdminUsername)
	}
	if strings.TrimSpace(input.ResourceType) != "" {
		where += " AND resource_type = ?"
		args = append(args, input.ResourceType)
	}
	if strings.TrimSpace(input.StartTime) != "" {
		where += " AND created_at >= ?"
		args = append(args, input.StartTime)
	}
	if strings.TrimSpace(input.EndTime) != "" {
		where += " AND created_at <= ?"
		args = append(args, input.EndTime)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_audit_logs "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}
	listArgs := append(append([]any{}, args...), input.Limit, input.Offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, admin_username, action, resource_type, resource_id, before_json, after_json, ip_address, user_agent, created_at
FROM admin_audit_logs `+where+`
ORDER BY created_at DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()
	entries := []Entry{}
	for rows.Next() {
		var e Entry
		var beforeJSON, afterJSON sql.NullString
		var ipAddress, userAgent sql.NullString
		if err := rows.Scan(&e.ID, &e.AdminUsername, &e.Action, &e.ResourceType, &e.ResourceID,
			&beforeJSON, &afterJSON, &ipAddress, &userAgent, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		if beforeJSON.Valid {
			e.BeforeJSON = []byte(beforeJSON.String)
		}
		if afterJSON.Valid {
			e.AfterJSON = []byte(afterJSON.String)
		}
		if ipAddress.Valid {
			e.IPAddress = ipAddress.String
		}
		if userAgent.Valid {
			e.UserAgent = userAgent.String
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit logs: %w", err)
	}
	return entries, total, nil
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
