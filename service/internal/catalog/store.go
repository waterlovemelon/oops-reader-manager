package catalog

import "context"

type Store interface {
	FindBySHA1(ctx context.Context, sha1 string) (*Book, error)
	FindByKey(ctx context.Context, bookKey string) (*Book, error)
	List(ctx context.Context, query string, status BookStatus, limit, offset int) ([]Book, int, error)
	Create(ctx context.Context, book Book) error
	Update(ctx context.Context, book Book) error
	UpdateStatus(ctx context.Context, bookKey string, status BookStatus, admin string) error
}
