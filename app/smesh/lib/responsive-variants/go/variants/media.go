package variants

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // Register PNG decoder
	"io"

	"github.com/disintegration/imaging"
)

// GeneratedVariant holds the result of generating a single variant.
type GeneratedVariant struct {
	Name     VariantName
	Data     []byte
	Width    int
	Height   int
	MimeType string
}

// GenerateResponsiveVariants creates all applicable variants for an image.
// It returns the variants in order from smallest to largest.
// EXIF metadata is automatically stripped by re-encoding.
func GenerateResponsiveVariants(r io.Reader) ([]GeneratedVariant, error) {
	// Decode the image (this strips EXIF by only reading pixel data)
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	variantsToGenerate := GetVariantsToGenerate(originalWidth)
	results := make([]GeneratedVariant, 0, len(variantsToGenerate))

	for _, name := range variantsToGenerate {
		var data []byte
		var width, height int

		if name == Original {
			// Re-encode at original size to strip EXIF
			data, err = encodeJPEG(img, OriginalQuality)
			if err != nil {
				return nil, fmt.Errorf("failed to encode original: %w", err)
			}
			width = originalWidth
			height = originalHeight
		} else {
			config := VariantSizes[name]

			// Resize using Lanczos (high quality)
			resized := imaging.Resize(img, config.Width, 0, imaging.Lanczos)
			resizedBounds := resized.Bounds()
			width = resizedBounds.Dx()
			height = resizedBounds.Dy()

			data, err = encodeJPEG(resized, config.Quality)
			if err != nil {
				return nil, fmt.Errorf("failed to encode %s: %w", name, err)
			}
		}

		results = append(results, GeneratedVariant{
			Name:     name,
			Data:     data,
			Width:    width,
			Height:   height,
			MimeType: "image/jpeg",
		})

		_ = format // Keep for potential format-preserving in future
	}

	return results, nil
}

// encodeJPEG encodes an image as JPEG with the specified quality.
func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GenerateThumbnail creates just a thumbnail variant for an image.
// Useful for on-demand thumbnail generation.
func GenerateThumbnail(r io.Reader, width int) ([]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	if width <= 0 {
		width = VariantSizes[Thumb].Width
	}

	resized := imaging.Resize(img, width, 0, imaging.Lanczos)

	quality := VariantSizes[Thumb].Quality
	return encodeJPEG(resized, quality)
}
