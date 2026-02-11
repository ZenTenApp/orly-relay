package blossom

import (
	"bytes"
	"image"
	"image/jpeg"
	"strings"

	"github.com/disintegration/imaging"
)

// ThumbnailSize defines the maximum dimension for thumbnails (used for list display)
const ThumbnailSize = 128

// ImageVariant represents a responsive image size category per NIP-XX
// Selection rule: pick smallest variant >= target width (next-larger)
type ImageVariant string

const (
	VariantThumb      ImageVariant = "thumb"      // 128px - list display, previews
	VariantMobileSm   ImageVariant = "mobile-sm"  // 512px - small mobile
	VariantMobileLg   ImageVariant = "mobile-lg"  // 1024px - large mobile, small tablets
	VariantDesktopSm  ImageVariant = "desktop-sm" // 1536px - laptops
	VariantDesktopMd  ImageVariant = "desktop-md" // 2048px - standard desktops
	VariantDesktopLg  ImageVariant = "desktop-lg" // 2560px - large/HiDPI displays
	VariantOriginal   ImageVariant = "original"   // unchanged size, EXIF stripped
)

// VariantSpec defines the target width and JPEG quality for a variant
type VariantSpec struct {
	Width   int
	Quality int
}

// VariantSpecs maps variant types to their specifications
var VariantSpecs = map[ImageVariant]VariantSpec{
	VariantThumb:     {Width: 128, Quality: 70},
	VariantMobileSm:  {Width: 512, Quality: 75},
	VariantMobileLg:  {Width: 1024, Quality: 80},
	VariantDesktopSm: {Width: 1536, Quality: 85},
	VariantDesktopMd: {Width: 2048, Quality: 88},
	VariantDesktopLg: {Width: 2560, Quality: 90},
	VariantOriginal:  {Width: 0, Quality: 92}, // Width 0 means keep original
}

// ScaledImage represents a generated image variant
type ScaledImage struct {
	Variant  ImageVariant
	Data     []byte
	Width    int
	Height   int
	MimeType string
}

// OptimizeMedia optimizes media content (BUD-05)
// This is a placeholder implementation - actual optimization would use
// libraries like image processing, video encoding, etc.
func OptimizeMedia(data []byte, mimeType string) (optimizedData []byte, optimizedMimeType string) {
	// For now, just return the original data unchanged
	// In a real implementation, this would:
	// - Resize images to optimal dimensions
	// - Compress images (JPEG quality, PNG optimization)
	// - Convert formats if beneficial
	// - Optimize video encoding
	// - etc.

	optimizedData = data
	optimizedMimeType = mimeType
	return
}

// GenerateThumbnail creates a thumbnail from image data using Lanczos resampling.
// Returns the thumbnail data, MIME type, and any error.
// Thumbnails are always JPEG for smaller file sizes.
func GenerateThumbnail(data []byte, mimeType string, maxSize int) ([]byte, string, error) {
	if maxSize <= 0 {
		maxSize = ThumbnailSize
	}

	// Decode using imaging library (automatically strips EXIF)
	reader := bytes.NewReader(data)
	img, err := imaging.Decode(reader, imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", err
	}

	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	// Don't upscale small images
	if origWidth <= maxSize && origHeight <= maxSize {
		// Return a compressed version of the original
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/jpeg", nil
	}

	// Calculate new dimensions maintaining aspect ratio
	var newWidth, newHeight int
	if origWidth > origHeight {
		newWidth = maxSize
		newHeight = (origHeight * maxSize) / origWidth
	} else {
		newHeight = maxSize
		newWidth = (origWidth * maxSize) / origHeight
	}

	// Ensure minimum dimensions
	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	// Resize using Lanczos (sinc-based) for highest quality
	thumb := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	// Encode as JPEG for smaller file size
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 80}); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), "image/jpeg", nil
}

// GenerateResponsiveVariants creates multiple resolution variants of an image.
// Uses Lanczos resampling for high-quality downscaling.
// Returns variants from smallest to largest: thumb, mobile, desktop, original.
// EXIF metadata is automatically stripped from all variants.
func GenerateResponsiveVariants(data []byte, mimeType string) ([]ScaledImage, error) {
	// Decode using imaging library (automatically strips EXIF and handles orientation)
	reader := bytes.NewReader(data)
	img, err := imaging.Decode(reader, imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	// Order of variants from smallest to largest
	variantOrder := []ImageVariant{
		VariantThumb,
		VariantMobileSm,
		VariantMobileLg,
		VariantDesktopSm,
		VariantDesktopMd,
		VariantDesktopLg,
		VariantOriginal,
	}
	variants := make([]ScaledImage, 0, len(variantOrder))

	for _, variant := range variantOrder {
		spec := VariantSpecs[variant]

		var resized image.Image
		var newWidth, newHeight int

		if spec.Width == 0 || origWidth <= spec.Width {
			// Original variant or image smaller than target - just compress
			resized = img
			newWidth = origWidth
			newHeight = origHeight
		} else {
			// Calculate height maintaining aspect ratio
			newWidth = spec.Width
			newHeight = (origHeight * spec.Width) / origWidth
			if newHeight < 1 {
				newHeight = 1
			}
			// Resize using Lanczos for highest quality
			resized = imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
		}

		// Encode as JPEG
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: spec.Quality}); err != nil {
			return nil, err
		}

		variants = append(variants, ScaledImage{
			Variant:  variant,
			Data:     buf.Bytes(),
			Width:    newWidth,
			Height:   newHeight,
			MimeType: "image/jpeg",
		})
	}

	return variants, nil
}

// IsImageMimeType returns true if the MIME type is a supported image format
func IsImageMimeType(mimeType string) bool {
	switch {
	case strings.HasPrefix(mimeType, "image/jpeg"),
		strings.HasPrefix(mimeType, "image/png"),
		strings.HasPrefix(mimeType, "image/gif"),
		strings.HasPrefix(mimeType, "image/webp"):
		return true
	default:
		return false
	}
}
