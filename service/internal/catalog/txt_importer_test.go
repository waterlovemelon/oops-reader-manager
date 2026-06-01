package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTXTImporterSplitsHeadings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	body := "第一章 开始\n这里是第一章。\n第二章 继续\n这里是第二章。"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	importer := TXTImporter{}
	manifest, err := importer.Manifest(context.Background(), path)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if len(manifest.Chapters) != 2 {
		t.Fatalf("chapters = %d", len(manifest.Chapters))
	}
	if manifest.Chapters[0].Title != "第一章 开始" {
		t.Fatalf("title = %q", manifest.Chapters[0].Title)
	}
}

func TestTXTImporterFallsBackToSingleChapter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(path, []byte("没有章节标题的内容"), 0644); err != nil {
		t.Fatal(err)
	}
	importer := TXTImporter{}
	manifest, err := importer.Manifest(context.Background(), path)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if len(manifest.Chapters) != 1 {
		t.Fatalf("chapters = %d", len(manifest.Chapters))
	}
}

func TestTXTImporterCountsWords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "words.txt")
	body := "第一章 开始\n一二三\n\n第二章 继续\n四五"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	importer := TXTImporter{}
	inspected, err := importer.Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspected.ChapterCount != 2 {
		t.Fatalf("chapter_count = %d, want 2", inspected.ChapterCount)
	}
	if inspected.WordCount <= 0 {
		t.Fatalf("word_count = %d, want > 0", inspected.WordCount)
	}
}
