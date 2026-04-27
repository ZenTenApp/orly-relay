# Responsive Image Variants

A library for generating and selecting responsive image variants per the NIP-XX Responsive Image Variants specification.

## Variant Sizes

| Name | Width | Quality | Use Case |
|------|-------|---------|----------|
| `thumb` | 128px | 70% | Previews, galleries |
| `mobile-sm` | 512px | 75% | Small mobile portrait |
| `mobile-lg` | 1024px | 80% | Large mobile, small tablets |
| `desktop-sm` | 1536px | 85% | Laptops |
| `desktop-md` | 2048px | 88% | Standard desktops |
| `desktop-lg` | 2560px | 90% | Large/HiDPI displays |
| `original` | native | 92% | Full resolution (EXIF stripped) |

## Selection Rule

**Next-Larger Selection**: Pick the smallest variant >= target width.

This ensures clients only need to downscale slightly (or not at all), rather than upscaling which would cause blur.

```
targetWidth = containerWidth * devicePixelRatio
selectedVariant = smallest variant where variant.width >= targetWidth
```

## Packages

### TypeScript (`ts/`)

For browser clients (Svelte, React, etc.):

```typescript
import {
  generateImageVariants,
  createResponsiveImageEvent,
  selectVariantForViewport
} from 'responsive-variants'

// Generate variants from a file
const variants = await generateImageVariants(file)

// Upload variants to Blossom...

// Create binding event (kind 1063)
const event = createResponsiveImageEvent(uploadedVariants)

// Select variant for display
const best = selectVariantForViewport(variants, containerWidth, devicePixelRatio)
```

### Go (`go/`)

For relay servers:

```go
import "git.smesh.lol/orly/app/smesh/lib/responsive-variants/go/variants"

// Generate variants
generated, err := variants.GenerateResponsiveVariants(reader)

// Select variant for viewport
best := variants.SelectVariantForViewport(variants, 1920, 1.0)

// Check if string is a blob hash
if variants.IsValidBlobHash(str) {
    // Query for binding event...
}
```

## Binding Event Structure (Kind 1063)

```json
{
  "kind": 1063,
  "content": "Optional caption",
  "tags": [
    ["imeta", "url https://...", "x <sha256>", "m image/jpeg", "dim 128x96", "variant thumb"],
    ["imeta", "url https://...", "x <sha256>", "m image/jpeg", "dim 512x384", "variant mobile-sm"],
    ...
    ["x", "<hash1>"],
    ["x", "<hash2>"],
    ...
  ]
}
```

Separate `x` tags enable NIP-01 tag queries to discover the binding event from any variant hash.

## License

MIT
