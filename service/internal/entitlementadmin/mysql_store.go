package entitlementadmin

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) ListByUser(ctx context.Context, userID int64) ([]Entitlement, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, entitlement_key, status, source, starts_at, expires_at, created_at
FROM account_entitlements
WHERE user_id = ?
ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list entitlements: %w", err)
	}
	defer rows.Close()
	var result []Entitlement
	for rows.Next() {
		var e Entitlement
		var expiresAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.UserID, &e.EntitlementKey, &e.Status, &e.Source, &e.StartsAt, &expiresAt, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan entitlement: %w", err)
		}
		if expiresAt.Valid {
			e.ExpiresAt = &expiresAt.Time
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entitlements: %w", err)
	}
	return result, nil
}

func (s *MySQLStore) Create(ctx context.Context, e Entitlement) (int64, error) {
	var expiresAt any
	if e.ExpiresAt != nil {
		expiresAt = *e.ExpiresAt
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO account_entitlements (user_id, entitlement_key, status, source, starts_at, expires_at)
VALUES (?, ?, 'active', ?, ?, ?)`,
		e.UserID, e.EntitlementKey, e.Source, e.StartsAt, expiresAt)
	if err != nil {
		return 0, fmt.Errorf("create entitlement: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id: %w", err)
	}
	return id, nil
}

func (s *MySQLStore) Revoke(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE account_entitlements
SET status = 'revoked', updated_at = ?
WHERE id = ?`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("revoke entitlement: %w", err)
	}
	return nil
}

func (s *MySQLStore) Extend(ctx context.Context, id int64, newExpiry time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE account_entitlements
SET expires_at = ?, updated_at = ?
WHERE id = ?`, newExpiry, time.Now(), id)
	if err != nil {
		return fmt.Errorf("extend entitlement: %w", err)
	}
	return nil
}
