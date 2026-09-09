package catalog

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/oops-reader/oops-reader-manager/service/internal/imageutil"
)

func TestWriteCoverVariantsSharesDuplicateRenditions(t *testing.T) {
	dir := t.TempDir()
	storage := NewLocalStorage(dir, filepath.Join(dir, "tmp"))
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	var source bytes.Buffer
	if err := jpeg.Encode(&source, img, nil); err != nil {
		t.Fatal(err)
	}
	variants, err := imageutil.BuildCoverVariants(source.Bytes(), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	manifest, coverPath, err := storage.WriteCoverVariants("epub", "abcdef1234567890", "book-1", variants)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Variants[0].Path != manifest.Variants[1].Path || manifest.Variants[1].Path != manifest.Variants[2].Path {
		t.Fatalf("equal renditions must share a file: %#v", manifest.Variants)
	}
	if want := "covers/epub/ab/cd/book-1/" + manifest.Variants[0].Path; coverPath != want {
		t.Fatalf("cover path = %q, want %q", coverPath, want)
	}
	loaded, dirRel, err := storage.ReadCoverVariantManifest(coverPath)
	if err != nil {
		t.Fatal(err)
	}
	if dirRel != "covers/epub/ab/cd/book-1" || len(loaded.Variants) != 3 {
		t.Fatalf("loaded manifest = %#v, dir = %q", loaded, dirRel)
	}
	entries, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(dirRel)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
}

func TestNearestCoverVariantPrefersLargerOnTie(t *testing.T) {
	variant, err := nearestCoverVariant([]CoverVariant{
		{Key: "small", Width: 320, Height: 480, Path: "small-a.jpg"},
		{Key: "medium", Width: 640, Height: 960, Path: "medium-b.jpg"},
		{Key: "large", Width: 1280, Height: 1920, Path: "large-c.jpg"},
	}, 480)
	if err != nil {
		t.Fatal(err)
	}
	if variant.Key != "medium" {
		t.Fatalf("variant = %s, want medium", variant.Key)
	}
}

func TestNearestCoverVariantUsesAvailableSizesOnly(t *testing.T) {
	variant, err := nearestCoverVariant([]CoverVariant{
		{Key: "small", Width: 320, Height: 480, Path: "small-a.jpg"},
		{Key: "large", Width: 1280, Height: 1920, Path: "large-c.jpg"},
	}, 900)
	if err != nil {
		t.Fatal(err)
	}
	if variant.Key != "large" {
		t.Fatalf("variant = %s, want large", variant.Key)
	}
}
