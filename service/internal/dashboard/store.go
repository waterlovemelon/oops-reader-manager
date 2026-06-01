package dashboard

import "context"

type Summary struct {
	TotalUsers    int `json:"total_users"`
	TotalBooks    int `json:"total_books"`
	TotalThreads  int `json:"total_threads"`
	PendingReview int `json:"pending_review"`
}

type Store interface {
	Summary(ctx context.Context) (Summary, error)
}
