package catalog

import (
	"context"
	"fmt"

	"github.com/oops-reader/oops-reader-manager/service/internal/content"
)

type EPUBImporter struct{}

func (EPUBImporter) Format() string {
	return "epub"
}

func (EPUBImporter) Inspect(ctx context.Context, filePath string) (ImportedBook, error) {
	book, err := content.ParseEPUB(filePath)
	if err != nil {
		return ImportedBook{}, err
	}
	return ImportedBook{
		Title:          book.Title,
		Author:         book.Author,
		ChapterCount:   len(book.Chapters),
		CoverMediaType: book.CoverMediaType,
	}, nil
}

func (EPUBImporter) Manifest(ctx context.Context, filePath string) (Manifest, error) {
	book, err := content.ParseEPUB(filePath)
	if err != nil {
		return Manifest{}, err
	}
	chapters := make([]Chapter, 0, len(book.Chapters))
	for _, chapter := range book.Chapters {
		chapters = append(chapters, Chapter{ID: chapter.ID, Title: chapter.Title, Text: chapter.Text})
	}
	return Manifest{Title: book.Title, Author: book.Author, Chapters: chapters}, nil
}

func (i EPUBImporter) Chapter(ctx context.Context, filePath string, chapterID string) (Chapter, error) {
	manifest, err := i.Manifest(ctx, filePath)
	if err != nil {
		return Chapter{}, err
	}
	for _, chapter := range manifest.Chapters {
		if chapter.ID == chapterID {
			return chapter, nil
		}
	}
	return Chapter{}, fmt.Errorf("chapter not found: %s", chapterID)
}

func (EPUBImporter) Cover(ctx context.Context, filePath string) (*Cover, error) {
	cover, err := content.ExtractEPUBCover(filePath)
	if err != nil || cover == nil {
		return nil, err
	}
	return &Cover{MediaType: cover.MediaType, Data: cover.Data}, nil
}
