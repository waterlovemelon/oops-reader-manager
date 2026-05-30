package communityadmin

import "context"

type Thread struct {
	ID           string `json:"id"`
	BoardID      string `json:"board_id"`
	Title        string `json:"title"`
	AuthorID     string `json:"author_id"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	CommentCount int    `json:"comment_count"`
}

type Comment struct {
	ID        string `json:"id"`
	ThreadID  string `json:"thread_id"`
	AuthorID  string `json:"author_id"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type Store interface {
	ListThreads(ctx context.Context, query string, limit, offset int) ([]Thread, int, error)
	UpdateThreadStatus(ctx context.Context, id string, status string) error
	ListComments(ctx context.Context, threadID string, limit, offset int) ([]Comment, int, error)
	UpdateCommentStatus(ctx context.Context, id string, status string) error
}
