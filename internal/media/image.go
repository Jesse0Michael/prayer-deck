package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // registers GIF decoding for uploads
	"image/jpeg"
	_ "image/png" // registers PNG decoding for uploads
	"net/http"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers WebP decoding for uploads
)

const (
	// DisplayMaxEdge bounds the card-face rendition: sharp on a 2x phone screen
	// at a fraction of a full-size upload's weight.
	DisplayMaxEdge = 1600
	// ThumbnailMaxEdge bounds the rendition used in lists and for preloading.
	ThumbnailMaxEdge = 480
	// maxPixels rejects images that would take excessive memory to decode.
	maxPixels = 50_000_000
)

// ErrUnsupportedImage is returned for uploads that are not a decodable image.
var ErrUnsupportedImage = errors.New("unsupported image")

// paper is the warm off-white of the card stock, laid under transparent artwork.
var paper = color.RGBA{R: 251, G: 246, B: 236, A: 255}

// Rendition is one encoded version of an uploaded image.
type Rendition struct {
	Name        string
	Data        []byte
	ContentType string
	Extension   string
	Width       int
	Height      int
}

// Renditions returns the original upload plus resized JPEG display and thumbnail renditions.
func Renditions(data []byte) ([]Rendition, error) {
	contentType := http.DetectContentType(data)
	extension, ok := map[string]string{
		"image/png":  "png",
		"image/jpeg": "jpg",
		"image/webp": "webp",
		"image/gif":  "gif",
	}[contentType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedImage, contentType)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupportedImage, err)
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, fmt.Errorf("%w: %dx%d exceeds %d pixels", ErrUnsupportedImage, cfg.Width, cfg.Height, maxPixels)
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupportedImage, err)
	}

	display, err := resizeJPEG(src, DisplayMaxEdge, 88)
	if err != nil {
		return nil, fmt.Errorf("failed to render display image: %w", err)
	}
	display.Name = "display"
	thumbnail, err := resizeJPEG(src, ThumbnailMaxEdge, 80)
	if err != nil {
		return nil, fmt.Errorf("failed to render thumbnail: %w", err)
	}
	thumbnail.Name = "thumbnail"

	original := Rendition{
		Name:        "original",
		Data:        data,
		ContentType: contentType,
		Extension:   extension,
		Width:       cfg.Width,
		Height:      cfg.Height,
	}
	return []Rendition{original, display, thumbnail}, nil
}

// resizeJPEG scales src to fit within maxEdge, never enlarging it.
func resizeJPEG(src image.Image, maxEdge, quality int) (Rendition, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if longest := max(w, h); longest > maxEdge {
		w = max(1, w*maxEdge/longest)
		h = max(1, h*maxEdge/longest)
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(paper), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
		return Rendition{}, fmt.Errorf("failed to encode jpeg: %w", err)
	}
	return Rendition{
		Data:        buf.Bytes(),
		ContentType: "image/jpeg",
		Extension:   "jpg",
		Width:       w,
		Height:      h,
	}, nil
}
