package recommendation

import (
	"context"
	"testing"
	"time"
)

type fakeStore struct {
	nextID  int64
	entries map[int64]Recommendation
}

func newFakeStore() *fakeStore {
	return &fakeStore{nextID: 1, entries: map[int64]Recommendation{}}
}

func (s *fakeStore) List(_ context.Context, _ ListInput) ([]Recommendation, int, error) {
	var result []Recommendation
	for _, r := range s.entries {
		if r.Status != StatusDeleted {
			result = append(result, r)
		}
	}
	return result, len(result), nil
}

func (s *fakeStore) FindByID(_ context.Context, id int64) (*Recommendation, error) {
	r, ok := s.entries[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &r, nil
}

func (s *fakeStore) FindActiveBook(_ context.Context, _ string) (*BookSnapshot, error) {
	return nil, ErrNotFound
}

func (s *fakeStore) Create(_ context.Context, input CreateInput) (int64, error) {
	id := s.nextID
	s.nextID++
	now := time.Now()
	s.entries[id] = Recommendation{
		ID:                 id,
		BookKey:            input.BookKey,
		Comment:            input.Comment,
		Status:             StatusActive,
		ScheduledPublishAt: input.ScheduledPublishAt,
		CreatedBy:          input.CreatedBy,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	return id, nil
}

func (s *fakeStore) Update(_ context.Context, input UpdateInput) error {
	r, ok := s.entries[input.ID]
	if !ok {
		return ErrNotFound
	}
	if input.Comment != nil {
		r.Comment = *input.Comment
	}
	if input.ScheduledPublishAt != nil {
		r.ScheduledPublishAt = *input.ScheduledPublishAt
	}
	r.UpdatedBy = input.UpdatedBy
	r.UpdatedAt = time.Now()
	s.entries[input.ID] = r
	return nil
}

func (s *fakeStore) SoftDelete(_ context.Context, id int64, _ string) error {
	r, ok := s.entries[id]
	if !ok {
		return ErrNotFound
	}
	r.Status = StatusDeleted
	now := time.Now()
	r.DeletedAt = &now
	s.entries[id] = r
	return nil
}

func TestServiceCreateDefaultsPublishTimeToNow(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)
	before := time.Now()
	rec, err := svc.Create(context.Background(), CreateInput{
		BookKey: "book-1",
		Comment: "great book",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.ScheduledPublishAt.Before(before) {
		t.Fatalf("expected publish time >= %v, got %v", before, rec.ScheduledPublishAt)
	}
}

func TestServiceCreateRejectsMissingBook(t *testing.T) {
	svc := NewService(newFakeStore())
	_, err := svc.Create(context.Background(), CreateInput{
		Comment: "good",
	})
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestServiceCreateRejectsBlankComment(t *testing.T) {
	svc := NewService(newFakeStore())
	_, err := svc.Create(context.Background(), CreateInput{
		BookKey: "book-1",
		Comment: "",
	})
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestServiceUpdateAllowsChangingCommentAndPublishTime(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)
	rec, err := svc.Create(context.Background(), CreateInput{
		BookKey: "book-1",
		Comment: "original",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	newComment := "updated comment"
	newTime := time.Now().Add(24 * time.Hour)
	if err := svc.Update(context.Background(), UpdateInput{
		ID:                 rec.ID,
		Comment:            &newComment,
		ScheduledPublishAt: &newTime,
		UpdatedBy:          "admin",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, err := store.FindByID(context.Background(), rec.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if updated.Comment != "updated comment" {
		t.Fatalf("comment = %q", updated.Comment)
	}
}

func TestRecommendationPublishState(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		rec   Recommendation
		want  PublishState
	}{
		{
			name: "deleted",
			rec:  Recommendation{Status: StatusDeleted},
			want: PublishStateDeleted,
		},
		{
			name: "published when scheduled time is past",
			rec:  Recommendation{Status: StatusActive, ScheduledPublishAt: now.Add(-time.Hour)},
			want: PublishStatePublished,
		},
		{
			name: "queued when scheduled time is future",
			rec:  Recommendation{Status: StatusActive, ScheduledPublishAt: now.Add(time.Hour)},
			want: PublishStateQueued,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rec.PublishState(now)
			if got != tt.want {
				t.Fatalf("PublishState() = %v, want %v", got, tt.want)
			}
		})
	}
}
