package catalog

import "context"

type Store interface {
	FindBySHA1(ctx context.Context, sha1 string) (*Book, error)
	Create(ctx context.Context, book Book) error
	Update(ctx context.Context, book Book) error
	UpdateStatus(ctx context.Context, bookKey string, status BookStatus, admin string) error
}
