package catalog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeCatalogStore struct {
	bySHA   map[string]Book
	created []Book
}

func (s *fakeCatalogStore) FindBySHA1(ctx context.Context, sha1 string) (*Book, error) {
	if book, ok := s.bySHA[sha1]; ok {
		return &book, nil
	}
	return nil, ErrNotFound
}

func (s *fakeCatalogStore) FindByKey(ctx context.Context, bookKey string) (*Book, error) {
	for _, book := range s.bySHA {
		if book.BookKey == bookKey {
			return &book, nil
		}
	}
	return nil, ErrNotFound
}

func (s *fakeCatalogStore) List(ctx context.Context, query string, status BookStatus, limit, offset int) ([]Book, int, error) {
	var result []Book
	for _, book := range s.bySHA {
		if status == "" || book.Status == status {
			result = append(result, book)
		}
	}
	return result, len(result), nil
}

func (s *fakeCatalogStore) Create(ctx context.Context, book Book) error {
	s.created = append(s.created, book)
	return nil
}

func (s *fakeCatalogStore) Update(ctx context.Context, book Book) error { return nil }
func (s *fakeCatalogStore) UpdateStatus(ctx context.Context, bookKey string, status BookStatus, admin string) error {
	return nil
}

func TestImportUploadedTXTCreatesDraftBook(t *testing.T) {
	dir := t.TempDir()
	temp := filepath.Join(dir, "upload.txt")
	if err := os.WriteFile(temp, []byte("第一章 开始\n内容"), 0644); err != nil {
		t.Fatal(err)
	}
	store := &fakeCatalogStore{bySHA: map[string]Book{}}
	service := NewService(store, NewLocalStorage(filepath.Join(dir, "catalog"), filepath.Join(dir, "tmp")), []Importer{TXTImporter{}})
	book, err := service.ImportUploadedFile(context.Background(), UploadInput{
		AdminUsername:    "admin",
		OriginalFilename: "测试.txt",
		TempPath:         temp,
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if book.Status != StatusDraft {
		t.Fatalf("status = %s", book.Status)
	}
	if len(store.created) != 1 {
		t.Fatalf("created = %d", len(store.created))
	}
	if _, err := os.Stat(filepath.Join(dir, "catalog", book.StoragePath)); err != nil {
		t.Fatalf("final file missing: %v", err)
	}
}

func TestImportUploadedFileRejectsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	temp := filepath.Join(dir, "upload.pdf")
	if err := os.WriteFile(temp, []byte("pdf"), 0644); err != nil {
		t.Fatal(err)
	}
	store := &fakeCatalogStore{bySHA: map[string]Book{}}
	service := NewService(store, NewLocalStorage(filepath.Join(dir, "catalog"), filepath.Join(dir, "tmp")), []Importer{TXTImporter{}})
	_, err := service.ImportUploadedFile(context.Background(), UploadInput{
		AdminUsername:    "admin",
		OriginalFilename: "测试.pdf",
		TempPath:         temp,
	})
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("err = %v", err)
	}
}
