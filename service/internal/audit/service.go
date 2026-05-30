package audit

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidEntry = errors.New("invalid audit entry")

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Record(ctx context.Context, entry Entry) error {
	if strings.TrimSpace(entry.AdminUsername) == "" ||
		strings.TrimSpace(entry.Action) == "" ||
		strings.TrimSpace(entry.ResourceType) == "" ||
		strings.TrimSpace(entry.ResourceID) == "" {
		return ErrInvalidEntry
	}
	return s.store.Create(ctx, entry)
}
