package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LocalStorage struct {
	root     string
	tempRoot string
}

func NewLocalStorage(root, tempRoot string) *LocalStorage {
	return &LocalStorage{root: filepath.Clean(root), tempRoot: filepath.Clean(tempRoot)}
}

func (s *LocalStorage) Root() string {
	return s.root
}

func (s *LocalStorage) TempRoot() string {
	return s.tempRoot
}

func (s *LocalStorage) OriginalPath(format, sha1, bookKey, ext string) string {
	return filepath.Join(s.root, s.RelativeOriginalPath(format, sha1, bookKey, ext))
}

func (s *LocalStorage) RelativeOriginalPath(format, sha1, bookKey, ext string) string {
	a, b := hashPrefix(sha1)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return filepath.ToSlash(filepath.Join("originals", format, a, b, bookKey+ext))
}

func (s *LocalStorage) RelativeCoverPath(format, sha1, bookKey, mediaType string) string {
	a, b := hashPrefix(sha1)
	ext := extForMediaType(mediaType)
	return filepath.ToSlash(filepath.Join("covers", format, a, b, bookKey+ext))
}

func (s *LocalStorage) RelativeCoverDirectory(format, sha1, bookKey string) string {
	a, b := hashPrefix(sha1)
	return filepath.ToSlash(filepath.Join("covers", format, a, b, bookKey))
}

// DeleteFiles removes the original book file and optional cover file from disk.
// Missing files are silently ignored.
func (s *LocalStorage) DeleteFiles(storagePath, coverStoragePath string) error {
	if storagePath != "" {
		full := filepath.Join(s.root, filepath.FromSlash(storagePath))
		if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if coverStoragePath != "" {
		full := filepath.Join(s.root, filepath.FromSlash(coverStoragePath))
		if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		variantDir := filepath.Dir(full)
		// Legacy covers share their hash-prefix directory, so only remove a
		// directory proven to be a new per-book variant directory.
		if _, err := os.Stat(filepath.Join(variantDir, "variants.json")); err == nil {
			if err := os.RemoveAll(variantDir); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteReadingContent removes the preprocessed reading artifacts of one book.
// The directory is owned by the import pipeline, so a missing one is not an
// error. Book keys never contain a path separator, but reject them anyway so a
// malformed record cannot escape the reading root.
func (s *LocalStorage) DeleteReadingContent(bookKey string) error {
	if bookKey == "" || strings.ContainsAny(bookKey, `/\`) || bookKey == "." || bookKey == ".." {
		return fmt.Errorf("invalid book key for reading content: %q", bookKey)
	}
	dir := filepath.Join(s.root, ".reading", bookKey)
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func extForMediaType(mediaType string) string {
	switch strings.ToLower(mediaType) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

func hashPrefix(sha1 string) (string, string) {
	if len(sha1) < 4 {
		return "00", "00"
	}
	return sha1[:2], sha1[2:4]
}
