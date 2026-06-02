package recommendation

import (
	"context"
	"time"
)

type Store interface {
	List(ctx context.Context, input ListInput) ([]Recommendation, int, error)
	FindByID(ctx context.Context, id int64) (*Recommendation, error)
	FindActiveBook(ctx context.Context, bookKey string) (*BookSnapshot, error)
	Create(ctx context.Context, input CreateInput) (int64, error)
	Update(ctx context.Context, input UpdateInput) error
	SoftDelete(ctx context.Context, id int64, deletedBy string) error
}

type ListInput struct {
	Query      string
	Status     Status
	Limit      int
	Offset     int
}

type CreateInput struct {
	BookKey            string
	Comment            string
	ScheduledPublishAt time.Time
	CreatedBy          string
}

type UpdateInput struct {
	ID                 int64
	Comment            *string
	ScheduledPublishAt *time.Time
	UpdatedBy          string
}
