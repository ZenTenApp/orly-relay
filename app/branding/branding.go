package branding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

// Manager handles loading and serving custom branding assets
type Manager struct {
	dir    string
	config Config

	// Cached assets for performance
	cachedAssets map[string][]byte
	cachedCSS    []byte
}

// New creates a new branding Manager by loading configuration from the specified directory
func New(dir string) (*Manager, error) {
	m := &Manager{
		dir:          dir,
		cachedAssets: make(map[string][]byte),
	}

	// Load branding.json
	configPath := filepath.Join(dir, "branding.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.I.F("branding.json not found in %s, using defaults", dir)
			m.config = DefaultConfig()
		} else {
			return nil, fmt.Errorf("failed to read branding.json: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &m.config); err != nil {
			return nil, fmt.Errorf("failed to parse branding.json: %w", err)
		}
	}

	// Pre-load and cache CSS
	if err := m.loadCSS(); err != nil {
		log.W.F("failed to load custom CSS: %v", err)
	}

	return m, nil
}

// Dir returns the branding directory path
func (m *Manager) Dir() string {
	return m.dir
}

// Config returns the loaded branding configuration
func (m *Manager) Config() Config {
	return m.config
}

// GetAsset returns a custom asset by name with its MIME type
// Returns the asset data, MIME type, and whether it was found
func (m *Manager) GetAsset(name string) ([]byte, string, bool) {
	var assetPath string

	switch name {
	case "logo":
		assetPath = m.config.Assets.Logo
	case "favicon":
		assetPath = m.config.Assets.Favicon
	case "icon-192":
		assetPath = m.config.Assets.Icon192
	case "icon-512":
		assetPath = m.config.Assets.Icon512
	default:
		return nil, "", false
	}

	if assetPath == "" {
		return nil, "", false
	}

	// Check cache first
	if data, ok := m.cachedAssets[name]; ok {
		return data, m.getMimeType(assetPath), true
	}

	// Load from disk
	fullPath := filepath.Join(m.dir, assetPath)
	data, err := os.ReadFile(fullPath)
	if chk.D(err) {
		return nil, "", false
	}

	// Cache for next time
	m.cachedAssets[name] = data
	return data, m.getMimeType(assetPath), true
}

// GetAssetPath returns the full filesystem path for a custom asset
func (m *Manager) GetAssetPath(name string) (string, bool) {
	var assetPath string

	switch name {
	case "logo":
		assetPath = m.config.Assets.Logo
	case "favicon":
		assetPath = m.config.Assets.Favicon
	case "icon-192":
		assetPath = m.config.Assets.Icon192
	case "icon-512":
		assetPath = m.config.Assets.Icon512
	default:
		return "", false
	}

	if assetPath == "" {
		return "", false
	}

	fullPath := filepath.Join(m.dir, assetPath)
	if _, err := os.Stat(fullPath); err != nil {
		return "", false
	}

	return fullPath, true
}

// loadCSS loads and caches the custom CSS files
func (m *Manager) loadCSS() error {
	var combined bytes.Buffer

	// Load variables CSS first (if exists)
	if m.config.CSS.VariablesCSS != "" {
		varsPath := filepath.Join(m.dir, m.config.CSS.VariablesCSS)
		if data, err := os.ReadFile(varsPath); err == nil {
			combined.Write(data)
			combined.WriteString("\n")
		}
	}

	// Load custom CSS (if exists)
	if m.config.CSS.CustomCSS != "" {
		customPath := filepath.Join(m.dir, m.config.CSS.CustomCSS)
		if data, err := os.ReadFile(customPath); err == nil {
			combined.Write(data)
		}
	}

	if combined.Len() > 0 {
		m.cachedCSS = combined.Bytes()
	}

	return nil
}

// GetCustomCSS returns the combined custom CSS content
func (m *Manager) GetCustomCSS() ([]byte, error) {
	if m.cachedCSS == nil {
		return nil, fs.ErrNotExist
	}
	return m.cachedCSS, nil
}

// HasCustomCSS returns true if custom CSS is available
func (m *Manager) HasCustomCSS() bool {
	return len(m.cachedCSS) > 0
}

// GetManifest generates a customized manifest.json
func (m *Manager) GetManifest(originalManifest []byte) ([]byte, error) {
	var manifest map[string]any

	if err := json.Unmarshal(originalManifest, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse original manifest: %w", err)
	}

	// Apply customizations
	if m.config.App.Name != "" {
		manifest["name"] = m.config.App.Name
	}
	if m.config.App.ShortName != "" {
		manifest["short_name"] = m.config.App.ShortName
	}
	if m.config.App.Description != "" {
		manifest["description"] = m.config.App.Description
	}
	if m.config.Manifest.ThemeColor != "" {
		manifest["theme_color"] = m.config.Manifest.ThemeColor
	}
	if m.config.Manifest.BackgroundColor != "" {
		manifest["background_color"] = m.config.Manifest.BackgroundColor
	}

	// Update icon paths to use branding endpoints
	if icons, ok := manifest["icons"].([]any); ok {
		for i, icon := range icons {
			if iconMap, ok := icon.(map[string]any); ok {
				if src, ok := iconMap["src"].(string); ok {
					// Replace icon paths with branding paths
					if strings.Contains(src, "192") {
						iconMap["src"] = "/branding/icon-192.png"
					} else if strings.Contains(src, "512") {
						iconMap["src"] = "/branding/icon-512.png"
					}
					icons[i] = iconMap
				}
			}
		}
		manifest["icons"] = icons
	}

	return json.MarshalIndent(manifest, "", "  ")
}

// ModifyIndexHTML modifies the index.html to inject custom branding
func (m *Manager) ModifyIndexHTML(original []byte) ([]byte, error) {
	html := string(original)

	// Inject custom CSS link before </head>
	if m.HasCustomCSS() {
		cssLink := `<link rel="stylesheet" href="/branding/custom.css">`
		html = strings.Replace(html, "</head>", cssLink+"\n</head>", 1)
	}

	// Inject JavaScript to change header text at runtime
	if m.config.App.Name != "" {
		// This script runs after DOM is loaded and updates the header text
		brandingScript := fmt.Sprintf(`<script>
(function() {
    var appName = %q;
    function updateBranding() {
        var titles = document.querySelectorAll('.app-title');
        titles.forEach(function(el) {
            var badge = el.querySelector('.permission-badge');
            el.childNodes.forEach(function(node) {
                if (node.nodeType === 3 && node.textContent.trim()) {
                    node.textContent = appName + ' ';
                }
            });
            if (!el.textContent.includes(appName)) {
                if (badge) {
                    el.innerHTML = appName + ' ' + badge.outerHTML;
                } else {
                    el.textContent = appName;
                }
            }
        });
    }
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', updateBranding);
    } else {
        updateBranding();
    }
    // Also run periodically to catch Svelte updates
    setInterval(updateBranding, 500);
    setTimeout(function() { clearInterval(this); }, 10000);
})();
</script>`, m.config.App.Name+" dashboard")
		html = strings.Replace(html, "</head>", brandingScript+"\n</head>", 1)
	}

	// Replace title if custom title is set
	if m.config.App.Title != "" {
		titleRegex := regexp.MustCompile(`<title>[^<]*</title>`)
		html = titleRegex.ReplaceAllString(html, fmt.Sprintf("<title>%s</title>", m.config.App.Title))
	}

	// Replace logo path to use branding endpoint
	if m.config.Assets.Logo != "" {
		// Replace orly.png references with branding logo endpoint
		html = strings.ReplaceAll(html, `"/orly.png"`, `"/branding/logo.png"`)
		html = strings.ReplaceAll(html, `'/orly.png'`, `'/branding/logo.png'`)
		html = strings.ReplaceAll(html, `src="/orly.png"`, `src="/branding/logo.png"`)
	}

	// Replace favicon path
	if m.config.Assets.Favicon != "" {
		html = strings.ReplaceAll(html, `href="/favicon.png"`, `href="/branding/favicon.png"`)
		html = strings.ReplaceAll(html, `href="favicon.png"`, `href="/branding/favicon.png"`)
	}

	// Replace manifest path to use dynamic endpoint
	html = strings.ReplaceAll(html, `href="/manifest.json"`, `href="/branding/manifest.json"`)
	html = strings.ReplaceAll(html, `href="manifest.json"`, `href="/branding/manifest.json"`)

	return []byte(html), nil
}

// NIP11Config returns the NIP-11 branding configuration
func (m *Manager) NIP11Config() NIP11Config {
	return m.config.NIP11
}

// AppName returns the custom app name, or empty string if not set
func (m *Manager) AppName() string {
	return m.config.App.Name
}

// getMimeType determines the MIME type from a file path
func (m *Manager) getMimeType(path string) string {
	ext := filepath.Ext(path)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// Default fallbacks
		switch strings.ToLower(ext) {
		case ".png":
			return "image/png"
		case ".jpg", ".jpeg":
			return "image/jpeg"
		case ".gif":
			return "image/gif"
		case ".svg":
			return "image/svg+xml"
		case ".ico":
			return "image/x-icon"
		case ".css":
			return "text/css"
		case ".js":
			return "application/javascript"
		default:
			return "application/octet-stream"
		}
	}
	return mimeType
}

// ClearCache clears all cached assets (useful for hot-reload during development)
func (m *Manager) ClearCache() {
	m.cachedAssets = make(map[string][]byte)
	m.cachedCSS = nil
	_ = m.loadCSS()
}
