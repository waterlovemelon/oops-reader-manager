package audit

import "context"

type Entry struct {
	ID            int64
	AdminUsername string
	Action        string
	ResourceType  string
	ResourceID    string
	BeforeJSON    []byte
	AfterJSON     []byte
	IPAddress     string
	UserAgent     string
	CreatedAt     string
}

type ListInput struct {
	AdminUsername string
	ResourceType  string
	StartTime     string
	EndTime       string
	Limit         int
	Offset        int
}

type Store interface {
	Create(ctx context.Context, entry Entry) error
	List(ctx context.Context, input ListInput) ([]Entry, int, error)
}
