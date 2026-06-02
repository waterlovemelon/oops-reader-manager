package recommendation

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalidInput = errors.New("invalid input")
var ErrNotFound = errors.New("not found")

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Store() Store {
	return s.store
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Recommendation, error) {
	input.BookKey = strings.TrimSpace(input.BookKey)
	input.Comment = strings.TrimSpace(input.Comment)
	if input.BookKey == "" {
		return Recommendation{}, ErrInvalidInput
	}
	if input.Comment == "" || utf8.RuneCountInString(input.Comment) > 2000 {
		return Recommendation{}, ErrInvalidInput
	}
	if input.ScheduledPublishAt.IsZero() {
		input.ScheduledPublishAt = time.Now()
	}
	id, err := s.store.Create(ctx, input)
	if err != nil {
		return Recommendation{}, err
	}
	rec, err := s.store.FindByID(ctx, id)
	if err != nil {
		return Recommendation{}, err
	}
	return *rec, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) error {
	if input.Comment != nil {
		trimmed := strings.TrimSpace(*input.Comment)
		input.Comment = &trimmed
		if *input.Comment == "" || utf8.RuneCountInString(*input.Comment) > 2000 {
			return ErrInvalidInput
		}
	}
	return s.store.Update(ctx, input)
}

func (s *Service) SoftDelete(ctx context.Context, id int64, deletedBy string) error {
	return s.store.SoftDelete(ctx, id, deletedBy)
}
