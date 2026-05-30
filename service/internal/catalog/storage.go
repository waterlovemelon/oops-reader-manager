package catalog

import (
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

func hashPrefix(sha1 string) (string, string) {
	if len(sha1) < 4 {
		return "00", "00"
	}
	return sha1[:2], sha1[2:4]
}
