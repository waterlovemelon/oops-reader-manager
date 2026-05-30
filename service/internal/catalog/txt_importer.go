package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var txtHeadingPattern = regexp.MustCompile(`(?m)^(第[一二三四五六七八九十百千万0-9]+[章节回卷部].*|Chapter\s+[0-9]+.*)$`)

type TXTImporter struct{}

func (TXTImporter) Format() string {
	return "txt"
}

func (i TXTImporter) Inspect(ctx context.Context, filePath string) (ImportedBook, error) {
	manifest, err := i.Manifest(ctx, filePath)
	if err != nil {
		return ImportedBook{}, err
	}
	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	return ImportedBook{
		Title:        title,
		ChapterCount: len(manifest.Chapters),
	}, nil
}

func (TXTImporter) Manifest(ctx context.Context, filePath string) (Manifest, error) {
	body, err := os.ReadFile(filePath)
	if err != nil {
		return Manifest{}, err
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	matches := txtHeadingPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return Manifest{Chapters: []Chapter{{ID: "ch-1", Title: "正文", Text: text}}}, nil
	}
	chapters := make([]Chapter, 0, len(matches))
	for idx, match := range matches {
		start := match[0]
		end := len(text)
		if idx+1 < len(matches) {
			end = matches[idx+1][0]
		}
		chunk := strings.TrimSpace(text[start:end])
		lines := strings.SplitN(chunk, "\n", 2)
		title := strings.TrimSpace(lines[0])
		chapters = append(chapters, Chapter{
			ID:    fmt.Sprintf("ch-%d", idx+1),
			Title: title,
			Text:  chunk,
		})
	}
	return Manifest{Chapters: chapters}, nil
}

func (i TXTImporter) Chapter(ctx context.Context, filePath string, chapterID string) (Chapter, error) {
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

func (TXTImporter) Cover(ctx context.Context, filePath string) (*Cover, error) {
	return nil, nil
}
