package imageutil

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestResizeCover_LargeJPEG(t *testing.T) {
	// Create a 3000x4500 JPEG (similar to the problematic book cover).
	img := image.NewRGBA(image.Rect(0, 0, 3000, 4500))
	for y := 0; y < 4500; y++ {
		for x := 0; x < 3000; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
	original := buf.Bytes()

	compressed, mediaType, err := ResizeCover(original, "image/jpeg")
	if err != nil {
		t.Fatalf("ResizeCover failed: %v", err)
	}
	if mediaType != "image/jpeg" {
		t.Errorf("expected media type image/jpeg, got %s", mediaType)
	}
	if len(compressed) >= len(original) {
		t.Errorf("compressed (%d) should be smaller than original (%d)", len(compressed), len(original))
	}

	// Verify the result is a valid JPEG with correct dimensions.
	result, err := jpeg.Decode(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	bounds := result.Bounds()
	if bounds.Dx() != LargeWidth {
		t.Errorf("expected width %d, got %d", LargeWidth, bounds.Dx())
	}
	expectedH := 4500 * LargeWidth / 3000
	if bounds.Dy() != expectedH {
		t.Errorf("expected height %d, got %d", expectedH, bounds.Dy())
	}
	t.Logf("Original: %d bytes, Compressed: %d bytes (%.1f%% reduction)",
		len(original), len(compressed), 100*(1-float64(len(compressed))/float64(len(original))))
}

func TestBuildCoverVariantsReusesSmallSource(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	variants, err := BuildCoverVariants(buf.Bytes(), "image/png")
	if err != nil {
		t.Fatalf("BuildCoverVariants: %v", err)
	}
	if len(variants) != 3 {
		t.Fatalf("variants = %d, want 3", len(variants))
	}
	for _, variant := range variants {
		if variant.Width != 200 || variant.Height != 100 {
			t.Errorf("%s = %dx%d, want 200x100", variant.Key, variant.Width, variant.Height)
		}
		if variant.SHA256 != variants[0].SHA256 {
			t.Errorf("%s did not reuse physical rendition", variant.Key)
		}
	}
}

func TestBuildCoverVariantsUsesThreeWidths(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1600, 800))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	variants, err := BuildCoverVariants(buf.Bytes(), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	for i, width := range []int{SmallWidth, MediumWidth, LargeWidth} {
		if variants[i].Width != width || variants[i].Height != width/2 {
			t.Errorf("%s = %dx%d, want %dx%d", variants[i].Key, variants[i].Width, variants[i].Height, width, width/2)
		}
	}
}

func TestResizeCover_SmallImage(t *testing.T) {
	// Create a 400x300 image — already within limits.
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	original := buf.Bytes()

	compressed, mediaType, err := ResizeCover(original, "image/png")
	if err != nil {
		t.Fatalf("ResizeCover failed: %v", err)
	}
	if mediaType != "image/jpeg" {
		t.Errorf("expected media type image/jpeg, got %s", mediaType)
	}

	result, err := jpeg.Decode(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	bounds := result.Bounds()
	if bounds.Dx() != 400 || bounds.Dy() != 300 {
		t.Errorf("dimensions changed: got %dx%d, want 400x300", bounds.Dx(), bounds.Dy())
	}
}

func TestResizeCover_InvalidData(t *testing.T) {
	data := []byte("not an image")
	original, mediaType, err := ResizeCover(data, "image/jpeg")
	if err == nil {
		t.Error("expected error for invalid data")
	}
	// Should return original data unchanged.
	if !bytes.Equal(original, data) {
		t.Error("expected original data returned on error")
	}
	if mediaType != "image/jpeg" {
		t.Errorf("expected original media type returned on error, got %s", mediaType)
	}
}
