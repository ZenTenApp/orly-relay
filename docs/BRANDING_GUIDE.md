# White-Label Branding Guide

ORLY supports full white-label branding, allowing relay operators to customize the UI appearance without rebuilding the application. All branding is loaded at runtime from a configuration directory.

## Quick Start

Generate a branding kit:

```bash
# Generic/white-label branding (recommended for customization)
./orly init-branding --style generic

# ORLY-branded template
./orly init-branding --style orly

# Custom output directory
./orly init-branding --style generic /path/to/branding
```

The branding kit is created at `~/.config/ORLY/branding/` by default.

## Directory Structure

```
~/.config/ORLY/branding/
  branding.json          # Main configuration
  assets/
    logo.png             # Header logo (replaces default)
    favicon.png          # Browser favicon
    icon-192.png         # PWA icon 192x192
    icon-512.png         # PWA icon 512x512
  css/
    custom.css           # Full CSS override
    variables.css        # CSS variable overrides only
```

## Configuration (branding.json)

```json
{
  "version": 1,
  "app": {
    "name": "My Relay",
    "shortName": "Relay",
    "title": "My Relay Dashboard",
    "description": "A high-performance Nostr relay"
  },
  "nip11": {
    "name": "My Relay",
    "description": "Custom relay description for NIP-11",
    "icon": "https://example.com/icon.png"
  },
  "manifest": {
    "themeColor": "#4080C0",
    "backgroundColor": "#F0F4F8"
  },
  "assets": {
    "logo": "assets/logo.png",
    "favicon": "assets/favicon.png",
    "icon192": "assets/icon-192.png",
    "icon512": "assets/icon-512.png"
  },
  "css": {
    "customCSS": "css/custom.css",
    "variablesCSS": "css/variables.css"
  }
}
```

### Configuration Sections

| Section | Description |
|---------|-------------|
| `app` | Application name and titles displayed in the UI |
| `nip11` | NIP-11 relay information document fields |
| `manifest` | PWA manifest colors |
| `assets` | Paths to custom images (relative to branding dir) |
| `css` | Paths to custom CSS files |

## Custom Assets

Replace the generated placeholder images with your own:

| Asset | Size | Purpose |
|-------|------|---------|
| `logo.png` | 256x256 recommended | Header logo |
| `favicon.png` | 64x64 | Browser tab icon |
| `icon-192.png` | 192x192 | PWA icon (Android) |
| `icon-512.png` | 512x512 | PWA splash screen |

**Tip**: Use PNG format with transparency for best results.

## CSS Customization

### Quick Theme Changes (variables.css)

Edit `css/variables.css` to change colors without touching component styles:

```css
/* Light theme */
html, body {
    --bg-color: #F0F4F8;
    --header-bg: #FFFFFF;
    --primary: #4080C0;
    --text-color: #334155;
    /* ... see generated file for all variables */
}

/* Dark theme */
body.dark-theme {
    --bg-color: #0F172A;
    --header-bg: #1E293B;
    --primary: #60A5FA;
    --text-color: #F8FAFC;
}
```

### Full CSS Override (custom.css)

Edit `css/custom.css` for complete control over styling:

```css
/* Custom header */
.header {
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
}

/* Custom buttons */
button {
    border-radius: 8px;
    font-weight: 500;
}

/* Custom cards */
.card {
    border-radius: 12px;
    box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}
```

### Available CSS Variables

#### Background Colors
- `--bg-color` - Main page background
- `--header-bg` - Header background
- `--sidebar-bg` - Sidebar background
- `--card-bg` - Card/container background
- `--panel-bg` - Panel background

#### Text Colors
- `--text-color` - Primary text
- `--text-muted` - Secondary/muted text

#### Theme Colors
- `--primary` - Primary accent color
- `--primary-bg` - Primary background tint
- `--secondary` - Secondary color
- `--accent-color` - Link color
- `--accent-hover-color` - Link hover color

#### Status Colors
- `--success`, `--success-bg`, `--success-text`
- `--warning`, `--warning-bg`
- `--danger`, `--danger-bg`, `--danger-text`
- `--info`

#### Form/Input Colors
- `--input-bg` - Input background
- `--input-border` - Input border
- `--input-text-color` - Input text

#### Button Colors
- `--button-bg` - Default button background
- `--button-hover-bg` - Button hover background
- `--button-text` - Button text color
- `--button-hover-border` - Button hover border

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_BRANDING_DIR` | `~/.config/ORLY/branding` | Branding directory path |
| `ORLY_BRANDING_ENABLED` | `true` | Enable/disable custom branding |

## Applying Changes

Restart the relay to apply branding changes:

```bash
# Stop and start the relay
pkill orly
./orly
```

Changes to CSS and assets require a restart. The relay logs will show:

```
custom branding loaded from /home/user/.config/ORLY/branding
```

## Branding Endpoints

The relay serves branding assets at these endpoints:

| Endpoint | Description |
|----------|-------------|
| `/branding/logo.png` | Custom logo |
| `/branding/favicon.png` | Custom favicon |
| `/branding/icon-192.png` | PWA icon 192x192 |
| `/branding/icon-512.png` | PWA icon 512x512 |
| `/branding/custom.css` | Combined CSS (variables + custom) |
| `/branding/manifest.json` | Customized PWA manifest |

## Disabling Branding

To use the default ORLY branding:

```bash
# Option 1: Remove branding directory
rm -rf ~/.config/ORLY/branding

# Option 2: Disable via environment
ORLY_BRANDING_ENABLED=false ./orly
```

## Troubleshooting

### Branding not loading
- Check that `~/.config/ORLY/branding/branding.json` exists
- Verify file permissions (readable by relay process)
- Check relay logs for branding load messages

### CSS changes not appearing
- Hard refresh the browser (Ctrl+Shift+R)
- Clear browser cache
- Verify CSS syntax is valid

### Logo not showing
- Ensure image path in `branding.json` is correct
- Check image file exists and is readable
- Use PNG format with appropriate dimensions

### Colors look wrong in light/dark mode
- Light theme uses `html, body` selector
- Dark theme uses `body.dark-theme` selector
- Ensure both themes are defined if customizing
