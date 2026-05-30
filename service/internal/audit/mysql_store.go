package audit

import (
	"context"
	"database/sql"
	"fmt"
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
