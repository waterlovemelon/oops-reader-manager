package catalog

import (
	"errors"
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
