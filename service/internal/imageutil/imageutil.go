package imageutil

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	SmallWidth  = 320
	MediumWidth = 640
	LargeWidth  = 1280
	JPEGQuality = 80
)

// Variant is an import-time image rendition. Images are never resized while
// serving a request.
type Variant struct {
	Key       string
	Width     int
	Height    int
	Data      []byte
	MediaType string
	SHA256    string
}

// BuildCoverVariants decodes once and emits the three supported widths. A
// source smaller than a target is never enlarged. Equal physical dimensions
// reuse the same encoded bytes and hash.
func BuildCoverVariants(data []byte, mediaType string) ([]Variant, error) {
	img, _, err := decodeImage(data, mediaType)
	if err != nil {
		return nil, fmt.Errorf("decode cover: %w", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, fmt.Errorf("cover has invalid dimensions %dx%d", bounds.Dx(), bounds.Dy())
	}

	targets := []struct {
		key   string
		width int
	}{
		{"small", SmallWidth},
		{"medium", MediumWidth},
		{"large", LargeWidth},
	}
	encoded := make(map[int]Variant, len(targets))
	variants := make([]Variant, 0, len(targets))
	for _, target := range targets {
		width := min(target.width, bounds.Dx())
		if reused, ok := encoded[width]; ok {
			reused.Key = target.key
			variants = append(variants, reused)
			continue
		}
		height := int((int64(bounds.Dy())*int64(width) + int64(bounds.Dx())/2) / int64(bounds.Dx()))
		if height < 1 {
			height = 1
		}
		resized := img
		if width != bounds.Dx() {
			resized = resizeImage(img, width, height)
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: JPEGQuality}); err != nil {
			return nil, fmt.Errorf("encode %s cover: %w", target.key, err)
		}
		result := Variant{
			Key:       target.key,
			Width:     width,
			Height:    height,
			Data:      buf.Bytes(),
			MediaType: "image/jpeg",
		}
		hash := sha256.Sum256(result.Data)
		result.SHA256 = hex.EncodeToString(hash[:])
		encoded[width] = result
		variants = append(variants, result)
	}
	return variants, nil
}

// ResizeCover remains for callers that need the largest stored rendition.
func ResizeCover(data []byte, mediaType string) ([]byte, string, error) {
	variants, err := BuildCoverVariants(data, mediaType)
	if err != nil {
		return data, mediaType, err
	}
	return variants[len(variants)-1].Data, "image/jpeg", nil
}

func decodeImage(data []byte, mediaType string) (image.Image, string, error) {
	r := bytes.NewReader(data)
	switch strings.ToLower(mediaType) {
	case "image/jpeg", "image/jpg":
		img, err := jpeg.Decode(r)
		return img, "jpeg", err
	case "image/png":
		img, err := png.Decode(r)
		return img, "png", err
	case "image/gif":
		img, err := gif.Decode(r)
		return img, "gif", err
	case "image/webp":
		img, format, err := image.Decode(r)
		return img, format, err
	default:
		img, format, err := image.Decode(r)
		return img, format, err
	}
}

func resizeImage(src image.Image, newW, newH int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
