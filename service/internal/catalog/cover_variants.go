package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oops-reader/oops-reader-manager/service/internal/imageutil"
)

const coverVariantManifestVersion = 1

type CoverVariant struct {
	Key       string `json:"key"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
}

type CoverVariantManifest struct {
	Version  int            `json:"version"`
	Variants []CoverVariant `json:"variants"`
}

// WriteCoverVariants writes immutable, content-addressed renditions and their
// manifest. Equal renditions share one file and one manifest path.
func (s *LocalStorage) WriteCoverVariants(format, sourceSHA1, bookKey string, variants []imageutil.Variant) (CoverVariantManifest, string, error) {
	if len(variants) == 0 {
		return CoverVariantManifest{}, "", fmt.Errorf("no cover variants")
	}
	dirRel := s.RelativeCoverDirectory(format, sourceSHA1, bookKey)
	dir := filepath.Join(s.root, filepath.FromSlash(dirRel))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return CoverVariantManifest{}, "", fmt.Errorf("create cover variant directory: %w", err)
	}

	manifest := CoverVariantManifest{Version: coverVariantManifestVersion, Variants: make([]CoverVariant, 0, len(variants))}
	fileByHash := make(map[string]string, len(variants))
	for _, variant := range variants {
		if variant.Key == "" || variant.Width < 1 || variant.Height < 1 || len(variant.Data) == 0 || variant.SHA256 == "" {
			return CoverVariantManifest{}, "", fmt.Errorf("invalid %q cover variant", variant.Key)
		}
		filename, exists := fileByHash[variant.SHA256]
		if !exists {
			filename = variant.Key + "-" + variant.SHA256 + ".jpg"
			if err := writeFileAtomically(filepath.Join(dir, filename), variant.Data, 0644); err != nil {
				return CoverVariantManifest{}, "", fmt.Errorf("write %s cover variant: %w", variant.Key, err)
			}
			fileByHash[variant.SHA256] = filename
		}
		manifest.Variants = append(manifest.Variants, CoverVariant{
			Key: variant.Key, Width: variant.Width, Height: variant.Height, Path: filename,
			MediaType: variant.MediaType, SHA256: variant.SHA256,
		})
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return CoverVariantManifest{}, "", fmt.Errorf("encode cover variant manifest: %w", err)
	}
	if err := writeFileAtomically(filepath.Join(dir, "variants.json"), manifestData, 0644); err != nil {
		return CoverVariantManifest{}, "", fmt.Errorf("write cover variant manifest: %w", err)
	}
	for _, variant := range manifest.Variants {
		if variant.Key == "large" {
			return manifest, filepath.ToSlash(filepath.Join(dirRel, variant.Path)), nil
		}
	}
	return CoverVariantManifest{}, "", fmt.Errorf("large cover variant missing")
}

func (s *LocalStorage) ReadCoverVariantManifest(coverStoragePath string) (CoverVariantManifest, string, error) {
	if strings.TrimSpace(coverStoragePath) == "" {
		return CoverVariantManifest{}, "", ErrNotFound
	}
	dirRel := filepath.ToSlash(filepath.Dir(coverStoragePath))
	data, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(dirRel), "variants.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return CoverVariantManifest{}, "", fmt.Errorf("%w: cover variants have not been backfilled", ErrNotFound)
		}
		return CoverVariantManifest{}, "", fmt.Errorf("read cover variant manifest: %w", err)
	}
	var manifest CoverVariantManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return CoverVariantManifest{}, "", fmt.Errorf("decode cover variant manifest: %w", err)
	}
	if manifest.Version != coverVariantManifestVersion || len(manifest.Variants) == 0 {
		return CoverVariantManifest{}, "", fmt.Errorf("%w: invalid cover variant manifest", ErrNotFound)
	}
	return manifest, dirRel, nil
}

func (s *LocalStorage) CoverVariantsReady(coverStoragePath string) error {
	manifest, dirRel, err := s.ReadCoverVariantManifest(coverStoragePath)
	if err != nil {
		return err
	}
	for _, variant := range manifest.Variants {
		if filepath.Base(variant.Path) != variant.Path {
			return fmt.Errorf("%w: invalid cover variant path", ErrNotFound)
		}
		info, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(dirRel), variant.Path))
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: %s is missing", ErrNotFound, variant.Key)
			}
			return fmt.Errorf("stat %s cover variant: %w", variant.Key, err)
		}
		if info.Size() == 0 {
			return fmt.Errorf("%w: %s is empty", ErrNotFound, variant.Key)
		}
	}
	return nil
}

func nearestCoverVariant(variants []CoverVariant, requestedWidth int) (CoverVariant, error) {
	if requestedWidth < 1 {
		requestedWidth = imageutil.SmallWidth
	}
	valid := make([]CoverVariant, 0, len(variants))
	for _, variant := range variants {
		if variant.Width > 0 && variant.Height > 0 && variant.Path != "" && filepath.Base(variant.Path) == variant.Path {
			valid = append(valid, variant)
		}
	}
	if len(valid) == 0 {
		return CoverVariant{}, fmt.Errorf("%w: no valid cover variants", ErrNotFound)
	}
	sort.SliceStable(valid, func(i, j int) bool {
		left := abs(valid[i].Width - requestedWidth)
		right := abs(valid[j].Width - requestedWidth)
		if left == right {
			return valid[i].Width > valid[j].Width
		}
		return left < right
	})
	return valid[0], nil
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func writeFileAtomically(path string, data []byte, perm os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-cover-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(perm); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}
