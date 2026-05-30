package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEPUBImporterRejectsInvalidZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.epub")
	if err := os.WriteFile(path, []byte("not an epub"), 0644); err != nil {
		t.Fatal(err)
	}
	importer := EPUBImporter{}
	if _, err := importer.Inspect(context.Background(), path); err == nil {
		t.Fatal("expected invalid epub error")
	}
}
