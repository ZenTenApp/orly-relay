package branding

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
)

// BrandingStyle represents the type of branding kit to generate
type BrandingStyle string

const (
	StyleORLY    BrandingStyle = "orly"    // ORLY-branded assets
	StyleGeneric BrandingStyle = "generic" // Generic/white-label assets
)

// InitBrandingKit creates a branding directory with assets and configuration
func InitBrandingKit(dir string, embeddedFS embed.FS, style BrandingStyle) error {
	// Create directory structure
	dirs := []string{
		dir,
		filepath.Join(dir, "assets"),
		filepath.Join(dir, "css"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Write branding.json based on style
	var config Config
	var cssTemplate, varsTemplate string

	switch style {
	case StyleGeneric:
		config = GenericConfig()
		cssTemplate = GenericCSSTemplate
		varsTemplate = GenericCSSVariablesTemplate
	default:
		config = DefaultConfig()
		cssTemplate = CSSTemplate
		varsTemplate = CSSVariablesTemplate
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	configPath := filepath.Join(dir, "branding.json")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write branding.json: %w", err)
	}

	// Generate or extract assets based on style
	if style == StyleGeneric {
		// Generate generic placeholder images
		if err := generateGenericAssets(dir); err != nil {
			return fmt.Errorf("failed to generate generic assets: %w", err)
		}
	} else {
		// Extract ORLY embedded assets
		assetMappings := map[string]string{
			"web/dist/orly.png":     filepath.Join(dir, "assets", "logo.png"),
			"web/dist/favicon.png":  filepath.Join(dir, "assets", "favicon.png"),
			"web/dist/icon-192.png": filepath.Join(dir, "assets", "icon-192.png"),
			"web/dist/icon-512.png": filepath.Join(dir, "assets", "icon-512.png"),
		}

		for src, dst := range assetMappings {
			data, err := fs.ReadFile(embeddedFS, src)
			if err != nil {
				altSrc := "web/" + filepath.Base(src)
				data, err = fs.ReadFile(embeddedFS, altSrc)
				if err != nil {
					fmt.Printf("Warning: could not extract %s: %v\n", src, err)
					continue
				}
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", dst, err)
			}
		}
	}

	// Write CSS template
	cssPath := filepath.Join(dir, "css", "custom.css")
	if err := os.WriteFile(cssPath, []byte(cssTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write custom.css: %w", err)
	}

	// Write variables-only CSS template
	varsPath := filepath.Join(dir, "css", "variables.css")
	if err := os.WriteFile(varsPath, []byte(varsTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write variables.css: %w", err)
	}

	return nil
}

// generateGenericAssets creates simple geometric placeholder images
func generateGenericAssets(dir string) error {
	// Color scheme: neutral blue-gray
	primaryColor := color.RGBA{R: 64, G: 128, B: 192, A: 255}   // #4080C0 - professional blue
	transparent := color.RGBA{R: 0, G: 0, B: 0, A: 0}           // Transparent background

	// Generate each size
	sizes := map[string]int{
		"logo.png":     256,
		"favicon.png":  64,
		"icon-192.png": 192,
		"icon-512.png": 512,
	}

	for filename, size := range sizes {
		img := generateRoundedSquare(size, primaryColor, transparent)
		path := filepath.Join(dir, "assets", filename)
		if err := savePNG(path, img); err != nil {
			return fmt.Errorf("failed to save %s: %w", filename, err)
		}
	}

	return nil
}

// generateRoundedSquare creates a simple rounded square icon
func generateRoundedSquare(size int, primary, bg color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Fill background
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, bg)
		}
	}

	// Draw a rounded square in the center
	margin := size / 8
	cornerRadius := size / 6
	squareSize := size - (margin * 2)

	for y := margin; y < margin+squareSize; y++ {
		for x := margin; x < margin+squareSize; x++ {
			// Check if point is inside rounded rectangle
			if isInsideRoundedRect(x-margin, y-margin, squareSize, squareSize, cornerRadius) {
				img.Set(x, y, primary)
			}
		}
	}

	// Draw a simple inner circle (relay symbol)
	centerX := size / 2
	centerY := size / 2
	innerRadius := size / 5
	ringWidth := size / 20

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x - centerX)
			dy := float64(y - centerY)
			dist := math.Sqrt(dx*dx + dy*dy)

			// Ring (circle outline)
			if dist >= float64(innerRadius-ringWidth) && dist <= float64(innerRadius) {
				img.Set(x, y, bg)
			}
		}
	}

	return img
}

// isInsideRoundedRect checks if a point is inside a rounded rectangle
func isInsideRoundedRect(x, y, w, h, r int) bool {
	// Check corners
	if x < r && y < r {
		// Top-left corner
		return isInsideCircle(x, y, r, r, r)
	}
	if x >= w-r && y < r {
		// Top-right corner
		return isInsideCircle(x, y, w-r-1, r, r)
	}
	if x < r && y >= h-r {
		// Bottom-left corner
		return isInsideCircle(x, y, r, h-r-1, r)
	}
	if x >= w-r && y >= h-r {
		// Bottom-right corner
		return isInsideCircle(x, y, w-r-1, h-r-1, r)
	}

	// Inside main rectangle
	return x >= 0 && x < w && y >= 0 && y < h
}

// isInsideCircle checks if a point is inside a circle
func isInsideCircle(x, y, cx, cy, r int) bool {
	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= r*r
}

// savePNG saves an image as a PNG file
func savePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// GenericConfig returns a generic/white-label configuration
func GenericConfig() Config {
	return Config{
		Version: 1,
		App: AppConfig{
			Name:        "Relay",
			ShortName:   "Relay",
			Title:       "Relay Dashboard",
			Description: "Nostr relay service",
		},
		NIP11: NIP11Config{
			Name:        "Relay",
			Description: "A Nostr relay",
			Icon:        "",
		},
		Manifest: ManifestConfig{
			ThemeColor:      "#4080C0",
			BackgroundColor: "#F0F4F8",
		},
		Assets: AssetsConfig{
			Logo:    "assets/logo.png",
			Favicon: "assets/favicon.png",
			Icon192: "assets/icon-192.png",
			Icon512: "assets/icon-512.png",
		},
		CSS: CSSConfig{
			CustomCSS:    "css/custom.css",
			VariablesCSS: "css/variables.css",
		},
	}
}

// CSSTemplate is the full CSS template with all variables and documentation
const CSSTemplate = `/*
 * Custom Branding CSS for ORLY Relay
 * ==================================
 *
 * This file is loaded AFTER the default styles, so any rules here
 * will override the defaults. You can customize:
 *
 * 1. CSS Variables (colors, spacing, etc.)
 * 2. Component styles (buttons, cards, headers, etc.)
 * 3. Add completely custom styles
 *
 * Restart the relay to apply changes.
 *
 * For variable-only overrides, edit variables.css instead.
 */

/* =============================================================================
   LIGHT THEME VARIABLES
   ============================================================================= */

:root {
    /* Background colors */
    --bg-color: #ddd;                    /* Main page background */
    --header-bg: #eee;                   /* Header background */
    --sidebar-bg: #eee;                  /* Sidebar background */
    --card-bg: #f8f9fa;                  /* Card/container background */
    --panel-bg: #f8f9fa;                 /* Panel background */

    /* Border colors */
    --border-color: #dee2e6;             /* Default border color */

    /* Text colors */
    --text-color: #444444;               /* Primary text color */
    --text-muted: #6c757d;               /* Secondary/muted text */

    /* Input/form colors */
    --input-border: #ccc;                /* Input border color */
    --input-bg: #ffffff;                 /* Input background */
    --input-text-color: #495057;         /* Input text color */

    /* Button colors */
    --button-bg: #ddd;                   /* Default button background */
    --button-hover-bg: #eee;             /* Button hover background */
    --button-text: #444444;              /* Button text color */
    --button-hover-border: #adb5bd;      /* Button hover border */

    /* Theme/accent colors */
    --primary: #00bcd4;                  /* Primary accent (cyan) */
    --primary-bg: rgba(0, 188, 212, 0.1); /* Primary background tint */
    --secondary: #6c757d;                /* Secondary color */

    /* Status colors */
    --success: #28a745;                  /* Success/positive */
    --success-bg: #d4edda;               /* Success background */
    --success-text: #155724;             /* Success text */
    --info: #17a2b8;                     /* Info/neutral */
    --warning: #ff3e00;                  /* Warning (Svelte orange) */
    --warning-bg: #fff3cd;               /* Warning background */
    --danger: #dc3545;                   /* Danger/error */
    --danger-bg: #f8d7da;                /* Danger background */
    --danger-text: #721c24;              /* Danger text */
    --error-bg: #f8d7da;                 /* Error background */
    --error-text: #721c24;               /* Error text */

    /* Code block colors */
    --code-bg: #f8f9fa;                  /* Code block background */
    --code-text: #495057;                /* Code text color */

    /* Tab colors */
    --tab-inactive-bg: #bbb;             /* Inactive tab background */

    /* Link/accent colors */
    --accent-color: #007bff;             /* Link color */
    --accent-hover-color: #0056b3;       /* Link hover color */
}

/* =============================================================================
   DARK THEME VARIABLES
   ============================================================================= */

body.dark-theme {
    /* Background colors */
    --bg-color: #263238;                 /* Main page background */
    --header-bg: #1e272c;                /* Header background */
    --sidebar-bg: #1e272c;               /* Sidebar background */
    --card-bg: #37474f;                  /* Card/container background */
    --panel-bg: #37474f;                 /* Panel background */

    /* Border colors */
    --border-color: #404040;             /* Default border color */

    /* Text colors */
    --text-color: #ffffff;               /* Primary text color */
    --text-muted: #adb5bd;               /* Secondary/muted text */

    /* Input/form colors */
    --input-border: #555;                /* Input border color */
    --input-bg: #37474f;                 /* Input background */
    --input-text-color: #ffffff;         /* Input text color */

    /* Button colors */
    --button-bg: #263238;                /* Default button background */
    --button-hover-bg: #1e272c;          /* Button hover background */
    --button-text: #ffffff;              /* Button text color */
    --button-hover-border: #6c757d;      /* Button hover border */

    /* Theme/accent colors */
    --primary: #00bcd4;                  /* Primary accent (cyan) */
    --primary-bg: rgba(0, 188, 212, 0.2); /* Primary background tint */
    --secondary: #6c757d;                /* Secondary color */

    /* Status colors */
    --success: #28a745;                  /* Success/positive */
    --success-bg: #1e4620;               /* Success background (dark) */
    --success-text: #d4edda;             /* Success text (light) */
    --info: #17a2b8;                     /* Info/neutral */
    --warning: #ff3e00;                  /* Warning (Svelte orange) */
    --warning-bg: #4d1f00;               /* Warning background (dark) */
    --danger: #dc3545;                   /* Danger/error */
    --danger-bg: #4d1319;                /* Danger background (dark) */
    --danger-text: #f8d7da;              /* Danger text (light) */
    --error-bg: #4d1319;                 /* Error background */
    --error-text: #f8d7da;               /* Error text */

    /* Code block colors */
    --code-bg: #1e272c;                  /* Code block background */
    --code-text: #ffffff;                /* Code text color */

    /* Tab colors */
    --tab-inactive-bg: #1a1a1a;          /* Inactive tab background */

    /* Link/accent colors */
    --accent-color: #007bff;             /* Link color */
    --accent-hover-color: #0056b3;       /* Link hover color */
}

/* =============================================================================
   CUSTOM STYLES
   Add your custom CSS rules below. These will override any default styles.
   ============================================================================= */

/* Example: Custom header styling
.header {
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
}
*/

/* Example: Custom button styling
.btn {
    border-radius: 8px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
}
*/

/* Example: Custom card styling
.card {
    border-radius: 12px;
    box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}
*/
`

// CSSVariablesTemplate contains only CSS variable definitions
const CSSVariablesTemplate = `/*
 * CSS Variables Override for ORLY Relay
 * ======================================
 *
 * This file contains only CSS variable definitions.
 * Edit values here to customize colors without touching component styles.
 *
 * For full CSS customization (including component styles),
 * edit custom.css instead.
 */

/* Light theme variables */
:root {
    --bg-color: #ddd;
    --header-bg: #eee;
    --sidebar-bg: #eee;
    --card-bg: #f8f9fa;
    --panel-bg: #f8f9fa;
    --border-color: #dee2e6;
    --text-color: #444444;
    --text-muted: #6c757d;
    --input-border: #ccc;
    --input-bg: #ffffff;
    --input-text-color: #495057;
    --button-bg: #ddd;
    --button-hover-bg: #eee;
    --button-text: #444444;
    --button-hover-border: #adb5bd;
    --primary: #00bcd4;
    --primary-bg: rgba(0, 188, 212, 0.1);
    --secondary: #6c757d;
    --success: #28a745;
    --success-bg: #d4edda;
    --success-text: #155724;
    --info: #17a2b8;
    --warning: #ff3e00;
    --warning-bg: #fff3cd;
    --danger: #dc3545;
    --danger-bg: #f8d7da;
    --danger-text: #721c24;
    --error-bg: #f8d7da;
    --error-text: #721c24;
    --code-bg: #f8f9fa;
    --code-text: #495057;
    --tab-inactive-bg: #bbb;
    --accent-color: #007bff;
    --accent-hover-color: #0056b3;
}

/* Dark theme variables */
body.dark-theme {
    --bg-color: #263238;
    --header-bg: #1e272c;
    --sidebar-bg: #1e272c;
    --card-bg: #37474f;
    --panel-bg: #37474f;
    --border-color: #404040;
    --text-color: #ffffff;
    --text-muted: #adb5bd;
    --input-border: #555;
    --input-bg: #37474f;
    --input-text-color: #ffffff;
    --button-bg: #263238;
    --button-hover-bg: #1e272c;
    --button-text: #ffffff;
    --button-hover-border: #6c757d;
    --primary: #00bcd4;
    --primary-bg: rgba(0, 188, 212, 0.2);
    --secondary: #6c757d;
    --success: #28a745;
    --success-bg: #1e4620;
    --success-text: #d4edda;
    --info: #17a2b8;
    --warning: #ff3e00;
    --warning-bg: #4d1f00;
    --danger: #dc3545;
    --danger-bg: #4d1319;
    --danger-text: #f8d7da;
    --error-bg: #4d1319;
    --error-text: #f8d7da;
    --code-bg: #1e272c;
    --code-text: #ffffff;
    --tab-inactive-bg: #1a1a1a;
    --accent-color: #007bff;
    --accent-hover-color: #0056b3;
}
`

// GenericCSSTemplate is the CSS template for generic/white-label branding
const GenericCSSTemplate = `/*
 * Custom Branding CSS - White Label Template
 * ==========================================
 *
 * This file is loaded AFTER the default styles, so any rules here
 * will override the defaults. You can customize:
 *
 * 1. CSS Variables (colors, spacing, etc.)
 * 2. Component styles (buttons, cards, headers, etc.)
 * 3. Add completely custom styles
 *
 * Restart the relay to apply changes.
 *
 * For variable-only overrides, edit variables.css instead.
 */

/* =============================================================================
   LIGHT THEME VARIABLES - Professional Blue-Gray
   ============================================================================= */

html, body {
    /* Background colors */
    --bg-color: #F0F4F8;                 /* Light gray-blue background */
    --header-bg: #FFFFFF;                /* Clean white header */
    --sidebar-bg: #FFFFFF;               /* Clean white sidebar */
    --card-bg: #FFFFFF;                  /* White cards */
    --panel-bg: #FFFFFF;                 /* White panels */

    /* Border colors */
    --border-color: #E2E8F0;             /* Subtle gray border */

    /* Text colors */
    --text-color: #334155;               /* Dark slate text */
    --text-muted: #64748B;               /* Muted slate */

    /* Input/form colors */
    --input-border: #CBD5E1;             /* Light slate border */
    --input-bg: #FFFFFF;                 /* White input */
    --input-text-color: #334155;         /* Dark slate text */

    /* Button colors */
    --button-bg: #F1F5F9;                /* Light slate button */
    --button-hover-bg: #E2E8F0;          /* Slightly darker on hover */
    --button-text: #334155;              /* Dark slate text */
    --button-hover-border: #94A3B8;      /* Medium slate border */

    /* Theme/accent colors - Professional Blue */
    --primary: #4080C0;                  /* Professional blue */
    --primary-bg: rgba(64, 128, 192, 0.1); /* Light blue tint */
    --secondary: #64748B;                /* Slate gray */

    /* Status colors */
    --success: #22C55E;                  /* Green */
    --success-bg: #DCFCE7;               /* Light green */
    --success-text: #166534;             /* Dark green */
    --info: #3B82F6;                     /* Blue */
    --warning: #F59E0B;                  /* Amber */
    --warning-bg: #FEF3C7;               /* Light amber */
    --danger: #EF4444;                   /* Red */
    --danger-bg: #FEE2E2;                /* Light red */
    --danger-text: #991B1B;              /* Dark red */
    --error-bg: #FEE2E2;                 /* Light red */
    --error-text: #991B1B;               /* Dark red */

    /* Code block colors */
    --code-bg: #F8FAFC;                  /* Very light slate */
    --code-text: #334155;                /* Dark slate */

    /* Tab colors */
    --tab-inactive-bg: #E2E8F0;          /* Light slate */

    /* Link/accent colors */
    --accent-color: #4080C0;             /* Professional blue */
    --accent-hover-color: #2563EB;       /* Darker blue */
}

/* =============================================================================
   DARK THEME VARIABLES - Professional Dark
   ============================================================================= */

body.dark-theme {
    /* Background colors */
    --bg-color: #0F172A;                 /* Dark navy */
    --header-bg: #1E293B;                /* Slate gray */
    --sidebar-bg: #1E293B;               /* Slate gray */
    --card-bg: #1E293B;                  /* Slate gray */
    --panel-bg: #1E293B;                 /* Slate gray */

    /* Border colors */
    --border-color: #334155;             /* Medium slate */

    /* Text colors */
    --text-color: #F8FAFC;               /* Almost white */
    --text-muted: #94A3B8;               /* Muted slate */

    /* Input/form colors */
    --input-border: #475569;             /* Slate border */
    --input-bg: #1E293B;                 /* Slate background */
    --input-text-color: #F8FAFC;         /* Light text */

    /* Button colors */
    --button-bg: #334155;                /* Slate button */
    --button-hover-bg: #475569;          /* Lighter on hover */
    --button-text: #F8FAFC;              /* Light text */
    --button-hover-border: #64748B;      /* Medium slate */

    /* Theme/accent colors */
    --primary: #60A5FA;                  /* Lighter blue for dark mode */
    --primary-bg: rgba(96, 165, 250, 0.2); /* Blue tint */
    --secondary: #94A3B8;                /* Muted slate */

    /* Status colors */
    --success: #4ADE80;                  /* Bright green */
    --success-bg: #166534;               /* Dark green */
    --success-text: #DCFCE7;             /* Light green */
    --info: #60A5FA;                     /* Light blue */
    --warning: #FBBF24;                  /* Bright amber */
    --warning-bg: #78350F;               /* Dark amber */
    --danger: #F87171;                   /* Light red */
    --danger-bg: #7F1D1D;                /* Dark red */
    --danger-text: #FEE2E2;              /* Light red */
    --error-bg: #7F1D1D;                 /* Dark red */
    --error-text: #FEE2E2;               /* Light red */

    /* Code block colors */
    --code-bg: #0F172A;                  /* Dark navy */
    --code-text: #F8FAFC;                /* Light text */

    /* Tab colors */
    --tab-inactive-bg: #1E293B;          /* Slate */

    /* Link/accent colors */
    --accent-color: #60A5FA;             /* Light blue */
    --accent-hover-color: #93C5FD;       /* Lighter blue */
}

/* =============================================================================
   PRIMARY BUTTON TEXT COLOR FIX
   Ensures buttons with primary background have white text for contrast
   ============================================================================= */

/* Target all common button patterns that use primary background */
button[class*="-btn"],
button[class*="submit"],
button[class*="action"],
button[class*="save"],
button[class*="add"],
button[class*="create"],
button[class*="connect"],
button[class*="refresh"],
button[class*="retry"],
button[class*="send"],
button[class*="apply"],
button[class*="execute"],
button[class*="run"],
.primary-action,
.action-button,
.permission-badge,
[class*="badge"] {
    color: #FFFFFF !important;
}

/* More specific override for any button that visually appears to have primary bg */
/* This uses a broad selector with low impact on non-primary buttons */
html:not(.dark-theme) button:not([disabled]) {
    /* Default to inherit, primary buttons will be caught above */
}

/* =============================================================================
   CUSTOM STYLES
   Add your custom CSS rules below. These will override any default styles.
   ============================================================================= */

/* Example: Custom header styling
.header {
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}
*/

/* Example: Custom button styling
.btn {
    border-radius: 6px;
    font-weight: 500;
}
*/

/* Example: Custom card styling
.card {
    border-radius: 8px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}
*/
`

// GenericCSSVariablesTemplate contains CSS variables for generic/white-label branding
const GenericCSSVariablesTemplate = `/*
 * CSS Variables Override - White Label Template
 * ==============================================
 *
 * This file contains only CSS variable definitions.
 * Edit values here to customize colors without touching component styles.
 *
 * For full CSS customization (including component styles),
 * edit custom.css instead.
 */

/* Light theme variables - Professional Blue-Gray */
/* Applied to both html and body for maximum compatibility */
html, body {
    --bg-color: #F0F4F8;
    --header-bg: #FFFFFF;
    --sidebar-bg: #FFFFFF;
    --card-bg: #FFFFFF;
    --panel-bg: #FFFFFF;
    --border-color: #E2E8F0;
    --text-color: #334155;
    --text-muted: #64748B;
    --input-border: #CBD5E1;
    --input-bg: #FFFFFF;
    --input-text-color: #334155;
    --button-bg: #F1F5F9;
    --button-hover-bg: #E2E8F0;
    --button-text: #334155;
    --button-hover-border: #94A3B8;
    --primary: #4080C0;
    --primary-bg: rgba(64, 128, 192, 0.1);
    --secondary: #64748B;
    --success: #22C55E;
    --success-bg: #DCFCE7;
    --success-text: #166534;
    --info: #3B82F6;
    --warning: #F59E0B;
    --warning-bg: #FEF3C7;
    --danger: #EF4444;
    --danger-bg: #FEE2E2;
    --danger-text: #991B1B;
    --error-bg: #FEE2E2;
    --error-text: #991B1B;
    --code-bg: #F8FAFC;
    --code-text: #334155;
    --tab-inactive-bg: #E2E8F0;
    --accent-color: #4080C0;
    --accent-hover-color: #2563EB;
}

/* Dark theme variables - Professional Dark */
body.dark-theme {
    --bg-color: #0F172A;
    --header-bg: #1E293B;
    --sidebar-bg: #1E293B;
    --card-bg: #1E293B;
    --panel-bg: #1E293B;
    --border-color: #334155;
    --text-color: #F8FAFC;
    --text-muted: #94A3B8;
    --input-border: #475569;
    --input-bg: #1E293B;
    --input-text-color: #F8FAFC;
    --button-bg: #334155;
    --button-hover-bg: #475569;
    --button-text: #F8FAFC;
    --button-hover-border: #64748B;
    --primary: #60A5FA;
    --primary-bg: rgba(96, 165, 250, 0.2);
    --secondary: #94A3B8;
    --success: #4ADE80;
    --success-bg: #166534;
    --success-text: #DCFCE7;
    --info: #60A5FA;
    --warning: #FBBF24;
    --warning-bg: #78350F;
    --danger: #F87171;
    --danger-bg: #7F1D1D;
    --danger-text: #FEE2E2;
    --error-bg: #7F1D1D;
    --error-text: #FEE2E2;
    --code-bg: #0F172A;
    --code-text: #F8FAFC;
    --tab-inactive-bg: #1E293B;
    --accent-color: #60A5FA;
    --accent-hover-color: #93C5FD;
}
`
