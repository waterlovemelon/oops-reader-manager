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
	if bounds.Dx() != MaxCoverWidth {
		t.Errorf("expected width %d, got %d", MaxCoverWidth, bounds.Dx())
	}
	expectedH := int(float64(4500) * (float64(MaxCoverWidth) / float64(3000)))
	if bounds.Dy() != expectedH {
		t.Errorf("expected height %d, got %d", expectedH, bounds.Dy())
	}
	t.Logf("Original: %d bytes, Compressed: %d bytes (%.1f%% reduction)",
		len(original), len(compressed), 100*(1-float64(len(compressed))/float64(len(original))))
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
