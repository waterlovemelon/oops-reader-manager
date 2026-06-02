package imageutil

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	// MaxCoverWidth is the maximum width in pixels for stored cover images.
	// Covers wider than this are resized down proportionally.
	MaxCoverWidth = 800

	// JPEGQuality is the encoding quality for output JPEG images (1-100).
	JPEGQuality = 80
)

// ResizeCover decodes an image from data, resizes it if wider than MaxCoverWidth,
// and re-encodes it as JPEG. Returns the compressed bytes and "image/jpeg".
// If the image is already within limits, it is still re-encoded for compression.
// If decoding fails, the original data and mediaType are returned unchanged.
func ResizeCover(data []byte, mediaType string) ([]byte, string, error) {
	img, _, err := decodeImage(data, mediaType)
	if err != nil {
		return data, mediaType, err
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if w > MaxCoverWidth {
		ratio := float64(MaxCoverWidth) / float64(w)
		newH := int(float64(h) * ratio)
		img = resizeImage(img, MaxCoverWidth, newH)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		return data, mediaType, err
	}
	return buf.Bytes(), "image/jpeg", nil
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
		// Registered via blank import of golang.org/x/image/webp.
		img, format, err := image.Decode(r)
		return img, format, err
	default:
		// Try generic decode as fallback.
		img, format, err := image.Decode(r)
		return img, format, err
	}
}

func resizeImage(src image.Image, newW, newH int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
