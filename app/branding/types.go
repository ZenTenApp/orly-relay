// Package branding provides white-label customization for the ORLY relay web UI.
// It allows relay operators to customize the appearance, branding, and theme
// without rebuilding the application.
package branding

// Config is the main configuration structure loaded from branding.json
type Config struct {
	Version  int            `json:"version"`
	App      AppConfig      `json:"app"`
	NIP11    NIP11Config    `json:"nip11"`
	Manifest ManifestConfig `json:"manifest"`
	Assets   AssetsConfig   `json:"assets"`
	CSS      CSSConfig      `json:"css"`
}

// AppConfig contains application-level branding settings
type AppConfig struct {
	Name        string `json:"name"`        // Display name (e.g., "My Relay")
	ShortName   string `json:"shortName"`   // Short name for PWA (e.g., "Relay")
	Title       string `json:"title"`       // Browser tab title (e.g., "My Relay Dashboard")
	Description string `json:"description"` // Brief description
}

// NIP11Config contains settings for the NIP-11 relay information document
type NIP11Config struct {
	Name        string `json:"name"`        // Relay name in NIP-11 response
	Description string `json:"description"` // Relay description in NIP-11 response
	Icon        string `json:"icon"`        // Icon URL for NIP-11 response
}

// ManifestConfig contains PWA manifest customization
type ManifestConfig struct {
	ThemeColor      string `json:"themeColor"`      // Theme color (e.g., "#1a1a2e")
	BackgroundColor string `json:"backgroundColor"` // Background color (e.g., "#16213e")
}

// AssetsConfig contains paths to custom asset files (relative to branding directory)
type AssetsConfig struct {
	Logo    string `json:"logo"`    // Header logo image (replaces orly.png)
	Favicon string `json:"favicon"` // Browser favicon
	Icon192 string `json:"icon192"` // PWA icon 192x192
	Icon512 string `json:"icon512"` // PWA icon 512x512
}

// CSSConfig contains paths to custom CSS files (relative to branding directory)
type CSSConfig struct {
	CustomCSS    string `json:"customCSS"`    // Full CSS override file
	VariablesCSS string `json:"variablesCSS"` // CSS variables override file (optional)
}

// DefaultConfig returns a default configuration with example values
func DefaultConfig() Config {
	return Config{
		Version: 1,
		App: AppConfig{
			Name:        "My Relay",
			ShortName:   "Relay",
			Title:       "My Relay Dashboard",
			Description: "A high-performance Nostr relay",
		},
		NIP11: NIP11Config{
			Name:        "My Relay",
			Description: "Custom relay description",
			Icon:        "",
		},
		Manifest: ManifestConfig{
			ThemeColor:      "#000000",
			BackgroundColor: "#000000",
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
