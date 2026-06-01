package audit

import (
	"context"
	"testing"
)

type fakeStore struct {
	entries []Entry
}

func (s *fakeStore) Create(ctx context.Context, entry Entry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func (s *fakeStore) List(ctx context.Context, input ListInput) ([]Entry, int, error) {
	return s.entries, len(s.entries), nil
}

func TestRecordRequiresActionAndResource(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	err := service.Record(context.Background(), Entry{AdminUsername: "admin"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRecordPersistsEntry(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	err := service.Record(context.Background(), Entry{
		AdminUsername: "admin",
		Action:        "catalog.publish",
		ResourceType:  "catalog_book",
		ResourceID:    "book-1",
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if len(store.entries) != 1 {
		t.Fatalf("entries = %d", len(store.entries))
	}
}
