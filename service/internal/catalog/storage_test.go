package catalog

import "testing"

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
