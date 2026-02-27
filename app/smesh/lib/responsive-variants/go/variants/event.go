package variants

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// UploadedVariant represents a variant after upload to Blossom.
type UploadedVariant struct {
	Variant  VariantName
	URL      string
	SHA256   string
	Width    int
	Height   int
	MimeType string
	Size     int64
	Blurhash string
}

// ImetaTag builds an imeta tag array for a variant.
// Format: ["imeta", "url <url>", "x <sha256>", "m <mime>", "dim <WxH>", "variant <name>", ...]
func (v *UploadedVariant) ImetaTag() []string {
	tag := []string{"imeta"}

	tag = append(tag, fmt.Sprintf("url %s", v.URL))
	tag = append(tag, fmt.Sprintf("x %s", v.SHA256))
	tag = append(tag, fmt.Sprintf("m %s", v.MimeType))
	tag = append(tag, fmt.Sprintf("dim %dx%d", v.Width, v.Height))
	tag = append(tag, fmt.Sprintf("variant %s", v.Variant))

	if v.Size > 0 {
		tag = append(tag, fmt.Sprintf("size %d", v.Size))
	}

	if v.Blurhash != "" {
		tag = append(tag, fmt.Sprintf("blurhash %s", v.Blurhash))
	}

	return tag
}

// ParseImetaTag parses an imeta tag array into an UploadedVariant.
func ParseImetaTag(tag []string) (*UploadedVariant, error) {
	if len(tag) < 2 || tag[0] != "imeta" {
		return nil, fmt.Errorf("not an imeta tag")
	}

	fields := make(map[string]string)
	for i := 1; i < len(tag); i++ {
		part := tag[i]
		idx := strings.Index(part, " ")
		if idx > 0 {
			key := part[:idx]
			value := part[idx+1:]
			fields[key] = value
		}
	}

	url := fields["url"]
	sha256 := fields["x"]
	mimeType := fields["m"]
	dim := fields["dim"]

	if url == "" || sha256 == "" || mimeType == "" || dim == "" {
		return nil, fmt.Errorf("missing required fields")
	}

	// Parse dimensions
	dimRe := regexp.MustCompile(`^(\d+)x(\d+)$`)
	match := dimRe.FindStringSubmatch(dim)
	if match == nil {
		return nil, fmt.Errorf("invalid dimension format: %s", dim)
	}

	width, _ := strconv.Atoi(match[1])
	height, _ := strconv.Atoi(match[2])

	variant := VariantName(fields["variant"])
	if variant == "" {
		variant = Original
	}

	v := &UploadedVariant{
		Variant:  variant,
		URL:      url,
		SHA256:   sha256,
		Width:    width,
		Height:   height,
		MimeType: mimeType,
	}

	if sizeStr := fields["size"]; sizeStr != "" {
		size, _ := strconv.ParseInt(sizeStr, 10, 64)
		v.Size = size
	}

	v.Blurhash = fields["blurhash"]

	return v, nil
}

// ParseBindingEvent parses a kind 1063 event's tags into UploadedVariants.
func ParseBindingEvent(tags [][]string) ([]UploadedVariant, error) {
	var variants []UploadedVariant

	for _, tag := range tags {
		if len(tag) < 2 || tag[0] != "imeta" {
			continue
		}

		v, err := ParseImetaTag(tag)
		if err != nil {
			continue // Skip invalid tags
		}

		variants = append(variants, *v)
	}

	return variants, nil
}

// SelectVariantForViewport selects the best variant for a given viewport.
// Selection rule: Pick the smallest variant >= target width.
func SelectVariantForViewport(variants []UploadedVariant, targetWidth int, pixelRatio float64) *UploadedVariant {
	if len(variants) == 0 {
		return nil
	}

	effectiveWidth := int(float64(targetWidth) * pixelRatio)

	// Sort by width ascending (simple bubble sort for small slices)
	sorted := make([]UploadedVariant, len(variants))
	copy(sorted, variants)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Width > sorted[j].Width {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Find smallest variant >= target width
	for i := range sorted {
		if sorted[i].Width >= effectiveWidth {
			return &sorted[i]
		}
	}

	// If none large enough, return largest
	return &sorted[len(sorted)-1]
}

// IsValidBlobHash checks if a string is a valid 64-character hex hash.
func IsValidBlobHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
