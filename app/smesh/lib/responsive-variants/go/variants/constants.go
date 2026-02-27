// Package variants provides responsive image variant definitions per NIP-XX.
//
// Variant sizes are designed for modern device widths with the "next-larger"
// selection rule: clients should pick the smallest variant >= target width
// for minimal client-side downscaling.
package variants

// VariantConfig holds the target width and quality for a variant.
type VariantConfig struct {
	Width   int
	Quality int // JPEG quality 0-100
}

// VariantName is the identifier for a variant size category.
type VariantName string

const (
	Thumb     VariantName = "thumb"
	MobileSm  VariantName = "mobile-sm"
	MobileLg  VariantName = "mobile-lg"
	DesktopSm VariantName = "desktop-sm"
	DesktopMd VariantName = "desktop-md"
	DesktopLg VariantName = "desktop-lg"
	Original  VariantName = "original"
)

// VariantSizes maps variant names to their configuration.
// These are ordered from smallest to largest.
var VariantSizes = map[VariantName]VariantConfig{
	Thumb:     {Width: 128, Quality: 70},
	MobileSm:  {Width: 512, Quality: 75},
	MobileLg:  {Width: 1024, Quality: 80},
	DesktopSm: {Width: 1536, Quality: 85},
	DesktopMd: {Width: 2048, Quality: 88},
	DesktopLg: {Width: 2560, Quality: 90},
}

// OriginalQuality is the JPEG quality for the EXIF-stripped original.
const OriginalQuality = 92

// VariantOrder defines the order from smallest to largest.
var VariantOrder = []VariantName{
	Thumb,
	MobileSm,
	MobileLg,
	DesktopSm,
	DesktopMd,
	DesktopLg,
	Original,
}

// FileMetadataKind is the event kind for NIP-94 file metadata events.
const FileMetadataKind = 1063

// GetVariantsToGenerate returns the list of variants to generate
// based on the original image width (no upscaling).
func GetVariantsToGenerate(originalWidth int) []VariantName {
	variants := []VariantName{Original}

	for _, name := range VariantOrder[:len(VariantOrder)-1] { // Exclude Original
		config := VariantSizes[name]
		if config.Width < originalWidth {
			variants = append(variants, name)
		}
	}

	// Sort by variant order (already in order due to iteration)
	result := make([]VariantName, 0, len(variants))
	for _, name := range VariantOrder {
		for _, v := range variants {
			if v == name {
				result = append(result, v)
				break
			}
		}
	}

	return result
}
