package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalPathUsesFormatAndHashPrefix(t *testing.T) {
	storage := NewLocalStorage("/catalog", "/tmp/catalog")
	got := storage.OriginalPath("epub", "abcdef1234567890", "book-1", ".epub")
	want := "/catalog/originals/epub/ab/cd/book-1.epub"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestRelativeOriginalPath(t *testing.T) {
	storage := NewLocalStorage("/catalog", "/tmp/catalog")
	got := storage.RelativeOriginalPath("txt", "abcdef1234567890", "book-1", ".txt")
	want := "originals/txt/ab/cd/book-1.txt"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestDeleteFilesRemovesOriginalAndCover(t *testing.T) {
	dir := t.TempDir()
	// Create dummy files.
	origPath := filepath.Join(dir, "originals", "epub", "ab", "cd", "book-1.epub")
	coverPath := filepath.Join(dir, "covers", "epub", "ab", "cd", "book-1.jpg")
	for _, p := range []string{origPath, coverPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	storage := NewLocalStorage(dir, filepath.Join(dir, "tmp"))
	if err := storage.DeleteFiles("originals/epub/ab/cd/book-1.epub", "covers/epub/ab/cd/book-1.jpg"); err != nil {
		t.Fatalf("DeleteFiles: %v", err)
	}
	if _, err := os.Stat(origPath); !os.IsNotExist(err) {
		t.Fatalf("original file should be removed")
	}
	if _, err := os.Stat(coverPath); !os.IsNotExist(err) {
		t.Fatalf("cover file should be removed")
	}
}

func TestDeleteFilesIgnoresMissingFiles(t *testing.T) {
	dir := t.TempDir()
	storage := NewLocalStorage(dir, filepath.Join(dir, "tmp"))
	if err := storage.DeleteFiles("originals/epub/ab/cd/nonexistent.epub", "covers/epub/ab/cd/nonexistent.jpg"); err != nil {
		t.Fatalf("DeleteFiles should not error on missing files: %v", err)
	}
}

func TestDeleteFilesSkipsEmptyCoverPath(t *testing.T) {
	dir := t.TempDir()
	origPath := filepath.Join(dir, "originals", "txt", "ab", "cd", "book-1.txt")
	if err := os.MkdirAll(filepath.Dir(origPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(origPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	storage := NewLocalStorage(dir, filepath.Join(dir, "tmp"))
	if err := storage.DeleteFiles("originals/txt/ab/cd/book-1.txt", ""); err != nil {
		t.Fatalf("DeleteFiles: %v", err)
	}
	if _, err := os.Stat(origPath); !os.IsNotExist(err) {
		t.Fatalf("original file should be removed")
	}
}
