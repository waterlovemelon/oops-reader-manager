package audit

import "context"

type Entry struct {
	AdminUsername string
	Action        string
	ResourceType  string
	ResourceID    string
	BeforeJSON    []byte
	AfterJSON     []byte
	IPAddress     string
	UserAgent     string
}

type Store interface {
	Create(ctx context.Context, entry Entry) error
}
