package entitlementadmin

import (
	"context"
	"time"
)

type Entitlement struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	EntitlementKey string     `json:"entitlement_key"`
	Status         string     `json:"status"`
	Source         string     `json:"source"`
	StartsAt       time.Time  `json:"starts_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Store interface {
	ListByUser(ctx context.Context, userID int64) ([]Entitlement, error)
	Create(ctx context.Context, e Entitlement) (int64, error)
	Revoke(ctx context.Context, id int64) error
	Extend(ctx context.Context, id int64, newExpiry time.Time) error
}
