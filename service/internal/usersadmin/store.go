package usersadmin

import "context"

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type Store interface {
	List(ctx context.Context, query string, limit, offset int) ([]User, int, error)
	UpdateStatus(ctx context.Context, id string, status string) error
}
