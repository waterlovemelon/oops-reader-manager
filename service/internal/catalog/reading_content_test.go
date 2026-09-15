package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeEPUBImporter struct{}

func (fakeEPUBImporter) Format() string { return readingFormatEPUB }
func (fakeEPUBImporter) Inspect(context.Context, string) (ImportedBook, error) {
	return ImportedBook{Title: "玄鉴仙族", ChapterCount: 1, WordCount: 3}, nil
}
func (fakeEPUBImporter) Manifest(context.Context, string) (Manifest, error) { return Manifest{}, nil }
func (fakeEPUBImporter) Chapter(context.Context, string, string) (Chapter, error) {
	return Chapter{}, nil
}
func (fakeEPUBImporter) Cover(context.Context, string) (*Cover, error) { return nil, nil }

type recordingPreparer struct {
	err   error
	calls []string
}

func (p *recordingPreparer) Prepare(_ context.Context, bookKey, sourcePath string) error {
	p.calls = append(p.calls, bookKey+"|"+sourcePath)
	return p.err
}

func TestImportUploadedEPUBBuildsReadingContent(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "catalog")
	temp := filepath.Join(dir, "upload.epub")
	if err := os.WriteFile(temp, []byte("epub-bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	store := &fakeCatalogStore{bySHA: map[string]Book{}}
	service := NewService(store, NewLocalStorage(root, filepath.Join(dir, "tmp")), []Importer{fakeEPUBImporter{}})
	preparer := &recordingPreparer{}
	service.SetReadingPreparer(preparer)

	book, err := service.ImportUploadedFile(context.Background(), UploadInput{
		AdminUsername:    "admin",
		OriginalFilename: "玄鉴仙族.epub",
		TempPath:         temp,
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(preparer.calls) != 1 {
		t.Fatalf("preparer calls = %v, want 1", preparer.calls)
	}
	want := book.BookKey + "|" + filepath.Join(root, filepath.FromSlash(book.StoragePath))
	if preparer.calls[0] != want {
		t.Fatalf("preparer called with %q, want %q", preparer.calls[0], want)
	}
}

func TestImportUploadedEPUBAbortsWhenReadingContentFails(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "catalog")
	temp := filepath.Join(dir, "upload.epub")
	if err := os.WriteFile(temp, []byte("epub-bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	store := &fakeCatalogStore{bySHA: map[string]Book{}}
	service := NewService(store, NewLocalStorage(root, filepath.Join(dir, "tmp")), []Importer{fakeEPUBImporter{}})
	service.SetReadingPreparer(&recordingPreparer{err: fmt.Errorf("%w: tool failed", ErrReadingPreprocess)})

	_, err := service.ImportUploadedFile(context.Background(), UploadInput{
		AdminUsername:    "admin",
		OriginalFilename: "玄鉴仙族.epub",
		TempPath:         temp,
	})
	if !errors.Is(err, ErrReadingPreprocess) {
		t.Fatalf("err = %v, want ErrReadingPreprocess", err)
	}
	if len(store.created) != 0 {
		t.Fatalf("created %d books, want 0", len(store.created))
	}
	// Nothing may be left on disk: the book has no artifacts, so it must not
	// be listed or served either.
	var stored []string
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			stored = append(stored, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 0 {
		t.Fatalf("stored files left behind: %v", stored)
	}
}

func TestImportUploadedTXTSkipsReadingContent(t *testing.T) {
	dir := t.TempDir()
	temp := filepath.Join(dir, "upload.txt")
	if err := os.WriteFile(temp, []byte("第一章 开始\n内容"), 0644); err != nil {
		t.Fatal(err)
	}
	service := NewService(
		&fakeCatalogStore{bySHA: map[string]Book{}},
		NewLocalStorage(filepath.Join(dir, "catalog"), filepath.Join(dir, "tmp")),
		[]Importer{TXTImporter{}, fakeEPUBImporter{}},
	)
	preparer := &recordingPreparer{}
	service.SetReadingPreparer(preparer)

	if _, err := service.ImportUploadedFile(context.Background(), UploadInput{
		AdminUsername:    "admin",
		OriginalFilename: "测试.txt",
		TempPath:         temp,
	}); err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(preparer.calls) != 0 {
		t.Fatalf("preparer calls = %v, want none for txt", preparer.calls)
	}
}

func TestDeleteBookFilesRemovesReadingContent(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "catalog")
	service := NewService(&fakeCatalogStore{bySHA: map[string]Book{}}, NewLocalStorage(root, filepath.Join(dir, "tmp")), nil)

	original := filepath.Join(root, "originals", "epub", "ab", "cd", "book-1.epub")
	if err := os.MkdirAll(filepath.Dir(original), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("epub"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, ".reading", "book-1", "v1", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifest), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{"book_id":"book-1"}`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := service.DeleteBookFiles(Book{BookKey: "book-1", StoragePath: "originals/epub/ab/cd/book-1.epub"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	for _, path := range []string{original, filepath.Join(root, ".reading", "book-1")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still present: %v", path, err)
		}
	}
}

func TestPreprocessCommandRunsToolWithBookIdAndOutputRoot(t *testing.T) {
	dir := t.TempDir()
	logged := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "stub.sh")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + logged + "\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	command := NewPreprocessCommand(script, "/catalog")
	if err := command.Prepare(context.Background(), "book-1", "/catalog/originals/book-1.epub"); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	got, err := os.ReadFile(logged)
	if err != nil {
		t.Fatal(err)
	}
	want := "--source\n/catalog/originals/book-1.epub\n--output\n/catalog\n--book-id\nbook-1\n"
	if string(got) != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

func TestPreprocessCommandReportsToolDiagnostics(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "stub.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'immutable version exists but failed validation' >&2\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	command := NewPreprocessCommand(script, "/catalog")
	err := command.Prepare(context.Background(), "book-1", "/catalog/originals/book-1.epub")
	if !errors.Is(err, ErrReadingPreprocess) {
		t.Fatalf("err = %v, want ErrReadingPreprocess", err)
	}
	if !strings.Contains(err.Error(), "failed validation") {
		t.Fatalf("err = %v, want tool stderr", err)
	}
}
