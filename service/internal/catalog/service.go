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

	"github.com/oops-reader/oops-reader-manager/service/internal/imageutil"
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

// DeleteBookFiles removes the original and cover files for a book from disk.
func (s *Service) DeleteBookFiles(book Book) error {
	return s.storage.DeleteFiles(book.StoragePath, book.CoverStoragePath)
}

// GetCover returns the default small cover rendition. Serving never extracts
// data from the original EPUB or performs an image conversion.
func (s *Service) GetCover(ctx context.Context, bookKey string) (*Cover, string, error) {
	return s.GetCoverForWidth(ctx, bookKey, imageutil.SmallWidth)
}

// GetCoverForWidth selects an already generated rendition nearest to the
// physical width requested by a client. Ties choose the larger rendition.
func (s *Service) GetCoverForWidth(ctx context.Context, bookKey string, widthPx int) (*Cover, string, error) {
	book, err := s.store.FindByKey(ctx, bookKey)
	if err != nil {
		return nil, "", err
	}
	manifest, dirRel, err := s.storage.ReadCoverVariantManifest(book.CoverStoragePath)
	if err != nil {
		return nil, "", err
	}
	variant, err := nearestCoverVariant(manifest.Variants, widthPx)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(filepath.Join(s.storage.Root(), filepath.FromSlash(dirRel), variant.Path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", fmt.Errorf("%w: selected cover variant is missing", ErrNotFound)
		}
		return nil, "", fmt.Errorf("read selected cover variant: %w", err)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("%w: selected cover variant is empty", ErrNotFound)
	}
	return &Cover{MediaType: variant.MediaType, Data: data}, book.Title, nil
}

// BackfillCoverVariants is intentionally an offline operation. It extracts a
// cover from the stored original and creates all renditions without changing
// cover_storage_path, so it is safe to run before the reader API is upgraded.
func (s *Service) BackfillCoverVariants(ctx context.Context, bookKey string) (bool, error) {
	book, err := s.store.FindByKey(ctx, bookKey)
	if err != nil {
		return false, err
	}
	if err := s.storage.CoverVariantsReady(book.CoverStoragePath); err == nil {
		return false, nil
	}
	importer, ok := s.importers[book.Format]
	if !ok {
		return false, fmt.Errorf("%w: %s", ErrUnsupportedFormat, book.Format)
	}
	cover, err := importer.Cover(ctx, filepath.Join(s.storage.Root(), filepath.FromSlash(book.StoragePath)))
	if err != nil {
		return false, fmt.Errorf("extract cover for backfill: %w", err)
	}
	if cover == nil || len(cover.Data) == 0 {
		return false, fmt.Errorf("%w: source book has no cover", ErrNotFound)
	}
	variants, err := imageutil.BuildCoverVariants(cover.Data, cover.MediaType)
	if err != nil {
		return false, err
	}
	_, _, err = s.storage.WriteCoverVariants(book.Format, book.ContentSHA1, book.BookKey, variants)
	if err != nil {
		return false, err
	}
	return true, nil
}

func mediaTypeForPath(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
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
	// Generate all cover renditions during import. Request handling is strictly
	// read-only and never unpacks an EPUB or re-encodes an image.
	coverPath := ""
	cover, coverErr := importer.Cover(ctx, finalPath)
	if coverErr != nil {
		_ = os.Remove(finalPath)
		return Book{}, fmt.Errorf("extract import-time cover: %w", coverErr)
	}
	if cover != nil && len(cover.Data) > 0 {
		variants, err := imageutil.BuildCoverVariants(cover.Data, cover.MediaType)
		if err != nil {
			_ = os.Remove(finalPath)
			return Book{}, err
		}
		_, coverPath, err = s.storage.WriteCoverVariants(format, sha, bookKey, variants)
		if err != nil {
			_ = os.Remove(finalPath)
			return Book{}, err
		}
	}

	now := time.Now()
	book := Book{
		BookKey:          bookKey,
		Title:            fallbackTitle(inspected.Title, input.OriginalFilename),
		Author:           inspected.Author,
		Description:      inspected.Description,
		Format:           format,
		Filename:         input.OriginalFilename,
		StoragePath:      relativePath,
		CoverStoragePath: coverPath,
		FileSize:         size,
		ContentSHA1:      sha,
		Language:         inspected.Language,
		ChapterCount:     inspected.ChapterCount,
		WordCount:        inspected.WordCount,
		Status:           StatusDraft,
		Source:           "admin_upload",
		UploadedAt:       &now,
		UpdatedBy:        input.AdminUsername,
	}
	if err := s.store.Create(ctx, book); err != nil {
		_ = os.Remove(finalPath)
		_ = s.storage.DeleteFiles("", coverPath)
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
