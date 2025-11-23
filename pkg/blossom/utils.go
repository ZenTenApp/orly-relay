package blossom

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"lol.mleku.dev/errorf"
	"github.com/minio/sha256-simd"
	"git.mleku.dev/mleku/nostr/encoders/hex"
)

const (
	sha256HexLength = 64
	maxRangeSize    = 10 * 1024 * 1024 // 10MB max range request
)

var sha256Regex = regexp.MustCompile(`[a-fA-F0-9]{64}`)

// CalculateSHA256 calculates the SHA256 hash of data
func CalculateSHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// CalculateSHA256Hex calculates the SHA256 hash and returns it as hex string
func CalculateSHA256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.Enc(hash[:])
}

// ExtractSHA256FromPath extracts SHA256 hash from URL path
// Supports both /<sha256> and /<sha256>.<ext> formats
func ExtractSHA256FromPath(path string) (sha256Hex string, ext string, err error) {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")
	
	// Split by dot to separate hash and extension
	parts := strings.SplitN(path, ".", 2)
	sha256Hex = parts[0]
	
	if len(parts) > 1 {
		ext = "." + parts[1]
	}
	
	// Validate SHA256 hex format
	if len(sha256Hex) != sha256HexLength {
		err = errorf.E(
			"invalid SHA256 length: expected %d, got %d",
			sha256HexLength, len(sha256Hex),
		)
		return
	}
	
	if !sha256Regex.MatchString(sha256Hex) {
		err = errorf.E("invalid SHA256 format: %s", sha256Hex)
		return
	}
	
	return
}

// ExtractSHA256FromURL extracts SHA256 hash from a URL string
// Uses the last occurrence of a 64 char hex string (as per BUD-03)
func ExtractSHA256FromURL(urlStr string) (sha256Hex string, err error) {
	// Find all 64-char hex strings
	matches := sha256Regex.FindAllString(urlStr, -1)
	if len(matches) == 0 {
		err = errorf.E("no SHA256 hash found in URL: %s", urlStr)
		return
	}
	
	// Return the last occurrence
	sha256Hex = matches[len(matches)-1]
	return
}

// GetMimeTypeFromExtension returns MIME type based on file extension
func GetMimeTypeFromExtension(ext string) string {
	ext = strings.ToLower(ext)
	mimeTypes := map[string]string{
		".pdf":  "application/pdf",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".txt":  "text/plain",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".zip":  "application/zip",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
	}
	
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// DetectMimeType detects MIME type from Content-Type header or file extension
func DetectMimeType(contentType string, ext string) string {
	// First try Content-Type header
	if contentType != "" {
		// Remove any parameters (e.g., "text/plain; charset=utf-8")
		parts := strings.Split(contentType, ";")
		mime := strings.TrimSpace(parts[0])
		if mime != "" && mime != "application/octet-stream" {
			return mime
		}
	}
	
	// Fall back to extension
	if ext != "" {
		return GetMimeTypeFromExtension(ext)
	}
	
	return "application/octet-stream"
}

// ParseRangeHeader parses HTTP Range header (RFC 7233)
// Returns start, end, and total length
func ParseRangeHeader(rangeHeader string, contentLength int64) (
	start, end int64, valid bool, err error,
) {
	if rangeHeader == "" {
		return 0, 0, false, nil
	}
	
	// Only support "bytes" unit
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, false, errorf.E("unsupported range unit")
	}
	
	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeSpec, "-")
	
	if len(parts) != 2 {
		return 0, 0, false, errorf.E("invalid range format")
	}
	
	var startStr, endStr string
	startStr = strings.TrimSpace(parts[0])
	endStr = strings.TrimSpace(parts[1])
	
	if startStr == "" && endStr == "" {
		return 0, 0, false, errorf.E("invalid range: both start and end empty")
	}
	
	// Parse start
	if startStr != "" {
		if start, err = strconv.ParseInt(startStr, 10, 64); err != nil {
			return 0, 0, false, errorf.E("invalid range start: %w", err)
		}
		if start < 0 {
			return 0, 0, false, errorf.E("range start cannot be negative")
		}
		if start >= contentLength {
			return 0, 0, false, errorf.E("range start exceeds content length")
		}
	} else {
		// Suffix range: last N bytes
		if end, err = strconv.ParseInt(endStr, 10, 64); err != nil {
			return 0, 0, false, errorf.E("invalid range end: %w", err)
		}
		if end <= 0 {
			return 0, 0, false, errorf.E("suffix range must be positive")
		}
		start = contentLength - end
		if start < 0 {
			start = 0
		}
		end = contentLength - 1
		return start, end, true, nil
	}
	
	// Parse end
	if endStr != "" {
		if end, err = strconv.ParseInt(endStr, 10, 64); err != nil {
			return 0, 0, false, errorf.E("invalid range end: %w", err)
		}
		if end < start {
			return 0, 0, false, errorf.E("range end before start")
		}
		if end >= contentLength {
			end = contentLength - 1
		}
	} else {
		// Open-ended range: from start to end
		end = contentLength - 1
	}
	
	// Validate range size
	if end-start+1 > maxRangeSize {
		return 0, 0, false, errorf.E("range too large: max %d bytes", maxRangeSize)
	}
	
	return start, end, true, nil
}

// WriteRangeResponse writes a partial content response (206)
func WriteRangeResponse(
	w http.ResponseWriter, data []byte, start, end, totalLength int64,
) {
	w.Header().Set("Content-Range", 
		"bytes "+strconv.FormatInt(start, 10)+"-"+
			strconv.FormatInt(end, 10)+"/"+
			strconv.FormatInt(totalLength, 10))
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(data[start : end+1])
}

// BuildBlobURL builds a blob URL with optional extension
func BuildBlobURL(baseURL, sha256Hex, ext string) string {
	url := baseURL + sha256Hex
	if ext != "" {
		url += ext
	}
	return url
}

// ValidateSHA256Hex validates that a string is a valid SHA256 hex string
func ValidateSHA256Hex(s string) bool {
	if len(s) != sha256HexLength {
		return false
	}
	_, err := hex.Dec(s)
	return err == nil
}

// GetFileExtensionFromPath extracts file extension from a path
func GetFileExtensionFromPath(path string) string {
	ext := filepath.Ext(path)
	return ext
}

// GetExtensionFromMimeType returns file extension based on MIME type
func GetExtensionFromMimeType(mimeType string) string {
	// Reverse lookup of GetMimeTypeFromExtension
	mimeToExt := map[string]string{
		"application/pdf":        ".pdf",
		"image/png":              ".png",
		"image/jpeg":             ".jpg",
		"image/gif":              ".gif",
		"image/webp":             ".webp",
		"image/svg+xml":          ".svg",
		"video/mp4":              ".mp4",
		"video/webm":             ".webm",
		"audio/mpeg":             ".mp3",
		"audio/wav":              ".wav",
		"audio/ogg":              ".ogg",
		"text/plain":             ".txt",
		"text/html":              ".html",
		"text/css":               ".css",
		"application/javascript": ".js",
		"application/json":       ".json",
		"application/xml":         ".xml",
		"application/zip":        ".zip",
		"application/x-tar":       ".tar",
		"application/gzip":       ".gz",
	}

	if ext, ok := mimeToExt[mimeType]; ok {
		return ext
	}
	return "" // No extension for unknown MIME types
}

