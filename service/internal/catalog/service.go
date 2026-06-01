package catalog

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

var ErrDuplicateBook = errors.New("duplicate catalog book")
var ErrUnsupportedFormat = errors.New("unsupported book format")

type Service struct {
	store     Store
	storage   *LocalStorage
	importers map[string]Importer
}

type UploadInput struct {
	AdminUsername    string
	OriginalFilename string
	TempPath         string
}

func NewService(store Store, storage *LocalStorage, importers []Importer) *Service {
	byFormat := map[string]Importer{}
	for _, importer := range importers {
		byFormat[importer.Format()] = importer
	}
	return &Service{store: store, storage: storage, importers: byFormat}
}

func (s *Service) Store() Store {
	return s.store
}

// GetCover extracts the cover image from the book file on-the-fly.
// Returns ErrUnsupportedFormat for non-EPUB books.
func (s *Service) GetCover(ctx context.Context, bookKey string) (*Cover, string, error) {
	book, err := s.store.FindByKey(ctx, bookKey)
	if err != nil {
		return nil, "", err
	}
	if book.Format != "epub" {
		return nil, "", fmt.Errorf("%w: cover not available for format %s", ErrUnsupportedFormat, book.Format)
	}
	fullPath := filepath.Join(s.storage.Root(), filepath.FromSlash(book.StoragePath))
	importer, ok := s.importers[book.Format]
	if !ok {
		return nil, "", ErrUnsupportedFormat
	}
	cover, err := importer.Cover(ctx, fullPath)
	if err != nil {
		return nil, "", err
	}
	if cover == nil {
		return nil, "", ErrNotFound
	}
	return cover, book.Title, nil
}

func (s *Service) ImportUploadedFile(ctx context.Context, input UploadInput) (Book, error) {
	ext := strings.ToLower(filepath.Ext(input.OriginalFilename))
	format := strings.TrimPrefix(ext, ".")
	importer, ok := s.importers[format]
	if !ok {
		return Book{}, ErrUnsupportedFormat
	}
	sha, size, err := fileSHA1(input.TempPath)
	if err != nil {
		return Book{}, err
	}
	if existing, err := s.store.FindBySHA1(ctx, sha); err == nil && existing != nil {
		return Book{}, ErrDuplicateBook
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return Book{}, err
	}
	inspected, err := importer.Inspect(ctx, input.TempPath)
	if err != nil {
		return Book{}, err
	}
	bookKey := stableBookKey(inspected.Title, input.OriginalFilename, sha)
	relativePath := s.storage.RelativeOriginalPath(format, sha, bookKey, ext)
	finalPath := s.storage.OriginalPath(format, sha, bookKey, ext)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return Book{}, err
	}
	if err := os.Rename(input.TempPath, finalPath); err != nil {
		return Book{}, err
	}
	now := time.Now()
	book := Book{
		BookKey:      bookKey,
		Title:        fallbackTitle(inspected.Title, input.OriginalFilename),
		Author:       inspected.Author,
		Description:  inspected.Description,
		Format:       format,
		Filename:     input.OriginalFilename,
		StoragePath:  relativePath,
		FileSize:     size,
		ContentSHA1:  sha,
		Language:     inspected.Language,
		ChapterCount: inspected.ChapterCount,
		WordCount:    inspected.WordCount,
		Status:       StatusDraft,
		Source:       "admin_upload",
		UploadedAt:   &now,
		UpdatedBy:    input.AdminUsername,
	}
	if err := s.store.Create(ctx, book); err != nil {
		_ = os.Remove(finalPath)
		return Book{}, err
	}
	return book, nil
}

func fileSHA1(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha1.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func stableBookKey(title, filename, sha string) string {
	base := title
	if strings.TrimSpace(base) == "" {
		base = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	id := slugify(base)
	if id == "" {
		id = "book"
	}
	if len(sha) >= 10 {
		return id + "-" + sha[:10]
	}
	return id
}

func fallbackTitle(title, filename string) string {
	if strings.TrimSpace(title) != "" {
		return strings.TrimSpace(title)
	}
	return strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
}

func slugify(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
