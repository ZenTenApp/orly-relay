package blossom

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"golang.org/x/image/draw"
)

// ThumbnailSize defines the maximum dimension for thumbnails
const ThumbnailSize = 96

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

// GenerateThumbnail creates a thumbnail from image data.
// Returns the thumbnail data, MIME type, and any error.
// Thumbnails are always JPEG for smaller file sizes.
func GenerateThumbnail(data []byte, mimeType string, maxSize int) ([]byte, string, error) {
	if maxSize <= 0 {
		maxSize = ThumbnailSize
	}

	// Decode the image based on MIME type
	var img image.Image
	var err error

	reader := bytes.NewReader(data)

	switch {
	case strings.HasPrefix(mimeType, "image/jpeg"):
		img, err = jpeg.Decode(reader)
	case strings.HasPrefix(mimeType, "image/png"):
		img, err = png.Decode(reader)
	case strings.HasPrefix(mimeType, "image/gif"):
		img, err = gif.Decode(reader)
	default:
		// Try generic decode
		img, _, err = image.Decode(reader)
	}

	if err != nil {
		return nil, "", err
	}

	// Calculate new dimensions maintaining aspect ratio
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

	// Create the thumbnail using high-quality CatmullRom interpolation
	thumb := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(thumb, thumb.Bounds(), img, bounds, draw.Over, nil)

	// Encode as JPEG for smaller file size
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 80}); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), "image/jpeg", nil
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
