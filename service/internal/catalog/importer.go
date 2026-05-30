package catalog

import "context"

type ImportedBook struct {
	Title          string
	Author         string
	Description    string
	Language       string
	ChapterCount   int
	CoverMediaType string
	CoverData      []byte
}

type Manifest struct {
	BookID   string
	Title    string
	Author   string
	Chapters []Chapter
}

type Chapter struct {
	ID    string
	Title string
	Text  string
}

type Importer interface {
	Format() string
	Inspect(ctx context.Context, filePath string) (ImportedBook, error)
	Manifest(ctx context.Context, filePath string) (Manifest, error)
	Chapter(ctx context.Context, filePath string, chapterID string) (Chapter, error)
	Cover(ctx context.Context, filePath string) (*Cover, error)
}

type Cover struct {
	MediaType string
	Data      []byte
}
