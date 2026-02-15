package blossom

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"github.com/minio/sha256-simd"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/utils"
)

// handleGetBlob handles GET /<sha256> requests (BUD-01)
// Uses http.ServeFile for efficient streaming with zero-copy sendfile(2)
// Supports ?thumb=1 or ?w=N query params for thumbnails
func (s *Server) handleGetBlob(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Extract SHA256 and extension
	sha256Hex, ext, err := ExtractSHA256FromPath(path)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Convert hex to bytes
	sha256Hash, err := hex.Dec(sha256Hex)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid SHA256 format")
		return
	}

	// Get blob metadata (also confirms existence)
	metadata, err := s.storage.GetBlobMetadata(sha256Hash)
	if err != nil {
		s.setErrorResponse(w, http.StatusNotFound, "blob not found")
		return
	}

	// Optional authorization check (BUD-01)
	if s.requireAuth {
		authEv, err := ValidateAuthEventForGet(r, s.getBaseURL(r), sha256Hash)
		if err != nil {
			s.setErrorResponse(w, http.StatusUnauthorized, "authorization required")
			return
		}
		if authEv == nil {
			s.setErrorResponse(w, http.StatusUnauthorized, "authorization required")
			return
		}
	}

	// Check for thumbnail request: ?thumb=1 or ?w=N
	thumbSize := 0
	if r.URL.Query().Get("thumb") == "1" {
		thumbSize = ThumbnailSize
	} else if wStr := r.URL.Query().Get("w"); wStr != "" {
		if w, err := strconv.Atoi(wStr); err == nil && w > 0 && w <= 512 {
			thumbSize = w
		}
	}

	// Serve thumbnail if requested and it's an image
	if thumbSize > 0 && IsImageMimeType(metadata.MimeType) {
		s.serveThumbnail(w, r, sha256Hash, sha256Hex, metadata, thumbSize)
		return
	}

	// Get blob file path
	blobPath := s.storage.GetBlobPath(sha256Hex, metadata.Extension)

	// Set caching headers - content-addressed blobs are immutable
	// Cache for 1 year (max recommended), immutable since SHA256 is content hash
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+sha256Hex+`"`)

	// Set Content-Type before ServeFile (it won't override if already set)
	mimeType := DetectMimeType(metadata.MimeType, ext)
	w.Header().Set("Content-Type", mimeType)

	// Use http.ServeFile for efficient streaming with:
	// - Automatic range request handling (RFC 7233)
	// - Zero-copy sendfile(2) on supported platforms
	// - Proper Last-Modified headers
	// - No full blob load into memory
	http.ServeFile(w, r, blobPath)
}

// serveThumbnail generates or serves a cached thumbnail for an image blob
func (s *Server) serveThumbnail(w http.ResponseWriter, r *http.Request, sha256Hash []byte, sha256Hex string, metadata *BlobMetadata, size int) {
	// Try to get cached thumbnail first
	thumbKey := fmt.Sprintf("%s_thumb_%d", sha256Hex, size)
	thumbData, err := s.storage.GetThumbnail(thumbKey)
	if err == nil && len(thumbData) > 0 {
		// Serve cached thumbnail
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("ETag", `"`+thumbKey+`"`)
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", strconv.Itoa(len(thumbData)))
		w.Write(thumbData)
		return
	}

	// Generate thumbnail from original blob
	blobData, _, err := s.storage.GetBlob(sha256Hash)
	if err != nil {
		s.setErrorResponse(w, http.StatusNotFound, "blob not found")
		return
	}

	thumbData, thumbMime, err := GenerateThumbnail(blobData, metadata.MimeType, size)
	if err != nil {
		log.W.F("failed to generate thumbnail for %s: %v", sha256Hex, err)
		// Fall back to serving original
		blobPath := s.storage.GetBlobPath(sha256Hex, metadata.Extension)
		http.ServeFile(w, r, blobPath)
		return
	}

	// Cache the thumbnail for future requests
	if err := s.storage.SaveThumbnail(thumbKey, thumbData); err != nil {
		log.W.F("failed to cache thumbnail %s: %v", thumbKey, err)
	}

	// Serve the thumbnail
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+thumbKey+`"`)
	w.Header().Set("Content-Type", thumbMime)
	w.Header().Set("Content-Length", strconv.Itoa(len(thumbData)))
	w.Write(thumbData)
}

// handleHeadBlob handles HEAD /<sha256> requests (BUD-01)
func (s *Server) handleHeadBlob(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Extract SHA256 and extension
	sha256Hex, ext, err := ExtractSHA256FromPath(path)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Convert hex to bytes
	sha256Hash, err := hex.Dec(sha256Hex)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid SHA256 format")
		return
	}

	// Get blob metadata (also confirms existence)
	metadata, err := s.storage.GetBlobMetadata(sha256Hash)
	if err != nil {
		s.setErrorResponse(w, http.StatusNotFound, "blob not found")
		return
	}

	// Optional authorization check
	if s.requireAuth {
		authEv, err := ValidateAuthEventForGet(r, s.getBaseURL(r), sha256Hash)
		if err != nil {
			s.setErrorResponse(w, http.StatusUnauthorized, "authorization required")
			return
		}
		if authEv == nil {
			s.setErrorResponse(w, http.StatusUnauthorized, "authorization required")
			return
		}
	}

	// Set caching headers - content-addressed blobs are immutable
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+sha256Hex+`"`)

	// Set headers (same as GET but no body)
	mimeType := DetectMimeType(metadata.MimeType, ext)
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(metadata.Size, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusOK)
}

// streamResult holds the result of streaming a blob to a temp file.
type streamResult struct {
	tempPath    string // Path to the temp file (caller must clean up on error)
	sha256Hash  []byte // Computed SHA256 hash
	size        int64  // Total bytes written
	sniffedMime string // MIME type detected from first 512 bytes of content
}

// streamToTempFile streams from reader to a temp file while computing the SHA256
// hash simultaneously. Memory usage is O(32KB) regardless of blob size.
// On success the caller owns the temp file and must either rename it or remove it.
// On error the temp file is cleaned up automatically.
func (s *Server) streamToTempFile(body io.Reader, maxSize int64) (result streamResult, err error) {
	// Create temp file in the blob directory so os.Rename is atomic (same fs)
	tmpFile, err := os.CreateTemp(s.storage.BlobDir(), "upload-*")
	if err != nil {
		return result, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up on any error
	defer func() {
		tmpFile.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()

	// Read first 512 bytes for MIME sniffing
	sniffBuf := make([]byte, 512)
	n, readErr := io.ReadFull(body, sniffBuf)
	if n == 0 {
		if readErr != nil {
			err = fmt.Errorf("error reading upload body: %w", readErr)
		} else {
			err = fmt.Errorf("empty upload body")
		}
		return
	}
	sniffBuf = sniffBuf[:n]
	result.sniffedMime = http.DetectContentType(sniffBuf)

	// Write sniffed bytes to both hasher and temp file
	hasher.Write(sniffBuf)
	if _, err = tmpFile.Write(sniffBuf); err != nil {
		return result, fmt.Errorf("failed to write to temp file: %w", err)
	}
	result.size = int64(n)

	// Stream the remainder: body → LimitReader → TeeReader(hasher) → tmpFile
	remaining := maxSize + 1 - result.size
	if remaining > 0 {
		limited := io.LimitReader(body, remaining)
		tee := io.TeeReader(limited, hasher)
		written, copyErr := io.Copy(tmpFile, tee)
		result.size += written
		if copyErr != nil {
			err = fmt.Errorf("error streaming upload: %w", copyErr)
			return
		}
	}

	// Check size limit (we read maxSize+1 to detect overflow)
	if result.size > maxSize {
		err = fmt.Errorf("blob too large: max %d bytes", maxSize)
		return
	}

	sum := hasher.Sum(nil)
	result.sha256Hash = sum
	result.tempPath = tmpPath
	return
}

// handleUpload handles PUT /upload requests (BUD-02)
// Streams the upload to disk while hashing — memory usage is O(32KB).
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Get initial pubkey from request (may be updated by auth validation)
	pubkey, _ := GetPubkeyFromRequest(r)
	remoteAddr := s.getRemoteAddr(r)

	// Validate auth BEFORE reading body (only uses headers)
	authHeader := r.Header.Get(AuthorizationHeader)
	if authHeader != "" {
		authEv, err := ValidateAuthEvent(r, "upload", nil)
		if err != nil {
			log.W.F("blossom upload: auth validation failed: %v", err)
			s.setErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		}
		if authEv != nil {
			pubkey = authEv.Pubkey
		}
	}

	// Check ACL BEFORE reading body
	if !s.checkACL(pubkey, remoteAddr, "write") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	// Stream body to temp file while computing SHA256 hash
	sr, err := s.streamToTempFile(r.Body, s.maxBlobSize)
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			s.setErrorResponse(w, http.StatusRequestEntityTooLarge, err.Error())
		} else {
			s.setErrorResponse(w, http.StatusBadRequest, "error reading request body")
		}
		return
	}
	// Clean up temp file on any error from here on
	defer func() {
		if sr.tempPath != "" {
			os.Remove(sr.tempPath)
		}
	}()

	sha256Hex := hex.Enc(sr.sha256Hash)

	// Check bandwidth rate limit (uses actual streamed size)
	if !s.checkBandwidthLimit(pubkey, remoteAddr, sr.size) {
		s.setErrorResponse(w, http.StatusTooManyRequests, "upload rate limit exceeded, try again later")
		return
	}

	// Check if blob already exists
	exists, err := s.storage.HasBlob(sr.sha256Hash)
	if err != nil {
		log.E.F("error checking blob existence: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Detect MIME type: prefer header, fall back to extension, then content sniffing
	mimeType := DetectMimeType(
		r.Header.Get("Content-Type"),
		GetFileExtensionFromPath(r.URL.Path),
	)
	if mimeType == "application/octet-stream" && sr.sniffedMime != "application/octet-stream" {
		mimeType = sr.sniffedMime
	}

	// Extract extension from path or infer from MIME type
	ext := GetFileExtensionFromPath(r.URL.Path)
	if ext == "" {
		ext = GetExtensionFromMimeType(mimeType)
	}

	// Check allowed MIME types
	if len(s.allowedMimeTypes) > 0 && !s.allowedMimeTypes[mimeType] {
		s.setErrorResponse(w, http.StatusUnsupportedMediaType,
			fmt.Sprintf("MIME type %s not allowed", mimeType))
		return
	}

	if !exists {
		// Check storage quota
		blobSizeMB := sr.size / (1024 * 1024)
		if blobSizeMB == 0 && sr.size > 0 {
			blobSizeMB = 1
		}

		quotaMB, err := s.db.GetBlossomStorageQuota(pubkey)
		if err != nil {
			log.W.F("failed to get storage quota: %v", err)
		} else if quotaMB > 0 {
			usedMB, err := s.storage.GetTotalStorageUsed(pubkey)
			if err != nil {
				log.W.F("failed to calculate storage used: %v", err)
			} else {
				if usedMB+blobSizeMB > quotaMB {
					s.setErrorResponse(w, http.StatusPaymentRequired,
						fmt.Sprintf("storage quota exceeded: %d/%d MB used, %d MB needed",
							usedMB, quotaMB, blobSizeMB))
					return
				}
			}
		}

		// Rename temp file to final path and save metadata (no re-hash)
		if err = s.storage.SaveBlobFromFile(sr.sha256Hash, sr.tempPath, sr.size, pubkey, mimeType, ext); err != nil {
			log.E.F("error saving blob: %v", err)
			s.setErrorResponse(w, http.StatusInternalServerError, "error saving blob")
			return
		}
		sr.tempPath = "" // Prevent deferred cleanup — file has been renamed
	} else {
		// Verify ownership
		metadata, err := s.storage.GetBlobMetadata(sr.sha256Hash)
		if err != nil {
			log.E.F("error getting blob metadata: %v", err)
			s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if !utils.FastEqual(metadata.Pubkey, pubkey) && !s.checkACL(pubkey, remoteAddr, "admin") {
			s.setErrorResponse(w, http.StatusConflict, "blob already exists")
			return
		}
	}

	// Build URL with extension
	blobURL := BuildBlobURL(s.getBaseURL(r), sha256Hex, ext)

	// Create descriptor
	descriptor := NewBlobDescriptor(
		blobURL,
		sha256Hex,
		sr.size,
		mimeType,
		time.Now().Unix(),
	)

	// Return descriptor
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(descriptor); err != nil {
		log.E.F("error encoding response: %v", err)
	}
}

// handleUploadRequirements handles HEAD /upload requests (BUD-06)
func (s *Server) handleUploadRequirements(w http.ResponseWriter, r *http.Request) {
	// Get headers
	sha256Hex := r.Header.Get("X-SHA-256")
	contentLengthStr := r.Header.Get("X-Content-Length")
	contentType := r.Header.Get("X-Content-Type")

	// Validate SHA256 header
	if sha256Hex == "" {
		s.setErrorResponse(w, http.StatusBadRequest, "missing X-SHA-256 header")
		return
	}

	if !ValidateSHA256Hex(sha256Hex) {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid X-SHA-256 header format")
		return
	}

	// Validate Content-Length header
	if contentLengthStr == "" {
		s.setErrorResponse(w, http.StatusLengthRequired, "missing X-Content-Length header")
		return
	}

	contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid X-Content-Length header")
		return
	}

	if contentLength > s.maxBlobSize {
		s.setErrorResponse(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("file too large: max %d bytes", s.maxBlobSize))
		return
	}

	// Check MIME type if provided
	if contentType != "" && len(s.allowedMimeTypes) > 0 {
		if !s.allowedMimeTypes[contentType] {
			s.setErrorResponse(w, http.StatusUnsupportedMediaType,
				fmt.Sprintf("unsupported file type: %s", contentType))
			return
		}
	}

	// Check if blob already exists
	sha256Hash, err := hex.Dec(sha256Hex)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid SHA256 format")
		return
	}

	exists, err := s.storage.HasBlob(sha256Hash)
	if err != nil {
		log.E.F("error checking blob existence: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if exists {
		// Return 200 OK - blob already exists, upload can proceed
		w.WriteHeader(http.StatusOK)
		return
	}

	// Optional authorization check
	if r.Header.Get(AuthorizationHeader) != "" {
		authEv, err := ValidateAuthEvent(r, "upload", sha256Hash)
		if err != nil {
			s.setErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		}
		if authEv == nil {
			s.setErrorResponse(w, http.StatusUnauthorized, "authorization required")
			return
		}

		// Check ACL
		remoteAddr := s.getRemoteAddr(r)
		if !s.checkACL(authEv.Pubkey, remoteAddr, "write") {
			s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
			return
		}
	}

	// All checks passed
	w.WriteHeader(http.StatusOK)
}

// handleListBlobs handles GET /list/<pubkey> requests (BUD-02)
func (s *Server) handleListBlobs(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Extract pubkey from path: list/<pubkey>
	if !strings.HasPrefix(path, "list/") {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid path")
		return
	}

	pubkeyHex := strings.TrimPrefix(path, "list/")
	if len(pubkeyHex) != 64 {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid pubkey format")
		return
	}

	pubkey, err := hex.Dec(pubkeyHex)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid pubkey format")
		return
	}

	// Parse query parameters
	var since, until int64
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		since, err = strconv.ParseInt(sinceStr, 10, 64)
		if err != nil {
			s.setErrorResponse(w, http.StatusBadRequest, "invalid since parameter")
			return
		}
	}

	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		until, err = strconv.ParseInt(untilStr, 10, 64)
		if err != nil {
			s.setErrorResponse(w, http.StatusBadRequest, "invalid until parameter")
			return
		}
	}

	// Optional authorization check
	requestPubkey, _ := GetPubkeyFromRequest(r)
	if r.Header.Get(AuthorizationHeader) != "" {
		authEv, err := ValidateAuthEvent(r, "list", nil)
		if err != nil {
			s.setErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		}
		if authEv != nil {
			requestPubkey = authEv.Pubkey
		}
	}

	// Check if requesting own list or has admin access
	if !utils.FastEqual(pubkey, requestPubkey) && !s.checkACL(requestPubkey, s.getRemoteAddr(r), "admin") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	// List blobs
	descriptors, err := s.storage.ListBlobs(pubkey, since, until)
	if err != nil {
		log.E.F("error listing blobs: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Set URLs for descriptors (include file extension for proper MIME handling)
	for _, desc := range descriptors {
		ext := GetExtensionFromMimeType(desc.Type)
		desc.URL = BuildBlobURL(s.getBaseURL(r), desc.SHA256, ext)
	}

	// Return JSON array
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(descriptors); err != nil {
		log.E.F("error encoding response: %v", err)
	}
}

// handleAdminListUsers handles GET /admin/users requests (admin only)
func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	// Authorization required
	authEv, err := ValidateAuthEvent(r, "admin", nil)
	if err != nil {
		s.setErrorResponse(w, http.StatusUnauthorized, err.Error())
		return
	}
	if authEv == nil {
		s.setErrorResponse(w, http.StatusUnauthorized, "authorization required")
		return
	}

	// Check admin ACL
	remoteAddr := s.getRemoteAddr(r)
	if !s.checkACL(authEv.Pubkey, remoteAddr, "admin") {
		s.setErrorResponse(w, http.StatusForbidden, "admin access required")
		return
	}

	// Get all user stats
	stats, err := s.storage.ListAllUserStats()
	if err != nil {
		log.E.F("error listing user stats: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(stats); err != nil {
		log.E.F("error encoding response: %v", err)
	}
}

// handleDeleteBlob handles DELETE /<sha256> requests (BUD-02)
func (s *Server) handleDeleteBlob(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Extract SHA256
	sha256Hex, _, err := ExtractSHA256FromPath(path)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	sha256Hash, err := hex.Dec(sha256Hex)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid SHA256 format")
		return
	}

	// Authorization required for delete
	// Use ValidateAuthEventForDelete which optionally requires server tag for replay protection
	authEv, err := ValidateAuthEventForDelete(
		r, s.getBaseURL(r), sha256Hash, s.deleteRequireServerTag,
	)
	if err != nil {
		s.setErrorResponse(w, http.StatusUnauthorized, err.Error())
		return
	}

	if authEv == nil {
		s.setErrorResponse(w, http.StatusUnauthorized, "authorization required")
		return
	}

	// Check ACL
	remoteAddr := s.getRemoteAddr(r)
	if !s.checkACL(authEv.Pubkey, remoteAddr, "write") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	// Verify ownership
	metadata, err := s.storage.GetBlobMetadata(sha256Hash)
	if err != nil {
		s.setErrorResponse(w, http.StatusNotFound, "blob not found")
		return
	}

	if !utils.FastEqual(metadata.Pubkey, authEv.Pubkey) && !s.checkACL(authEv.Pubkey, remoteAddr, "admin") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions to delete this blob")
		return
	}

	// Delete blob
	if err = s.storage.DeleteBlob(sha256Hash, authEv.Pubkey); err != nil {
		log.E.F("error deleting blob: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "error deleting blob")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleMirror handles PUT /mirror requests (BUD-04)
func (s *Server) handleMirror(w http.ResponseWriter, r *http.Request) {
	// Get initial pubkey from request (may be updated by auth validation)
	pubkey, _ := GetPubkeyFromRequest(r)
	remoteAddr := s.getRemoteAddr(r)

	// Read request body (JSON with URL — small payload, not the blob itself)
	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.URL == "" {
		s.setErrorResponse(w, http.StatusBadRequest, "missing url field")
		return
	}

	// Parse URL
	mirrorURL, err := url.Parse(req.URL)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid URL")
		return
	}

	// Validate auth and ACL BEFORE downloading the remote blob
	if r.Header.Get(AuthorizationHeader) != "" {
		authEv, err := ValidateAuthEvent(r, "upload", nil)
		if err != nil {
			s.setErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		}
		if authEv != nil {
			pubkey = authEv.Pubkey
		}
	}

	if !s.checkACL(pubkey, remoteAddr, "write") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	// Download blob from remote URL
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(mirrorURL.String())
	if err != nil {
		s.setErrorResponse(w, http.StatusBadGateway, "failed to fetch blob from remote URL")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.setErrorResponse(w, http.StatusBadGateway,
			fmt.Sprintf("remote server returned status %d", resp.StatusCode))
		return
	}

	// Stream remote blob to temp file while computing SHA256
	sr, err := s.streamToTempFile(resp.Body, s.maxBlobSize)
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			s.setErrorResponse(w, http.StatusRequestEntityTooLarge, err.Error())
		} else {
			s.setErrorResponse(w, http.StatusBadGateway, "error reading remote blob")
		}
		return
	}
	defer func() {
		if sr.tempPath != "" {
			os.Remove(sr.tempPath)
		}
	}()

	sha256Hex := hex.Enc(sr.sha256Hash)

	// Check bandwidth rate limit
	if !s.checkBandwidthLimit(pubkey, remoteAddr, sr.size) {
		s.setErrorResponse(w, http.StatusTooManyRequests, "upload rate limit exceeded, try again later")
		return
	}

	// Detect MIME type from remote response, extension, or content sniffing
	mimeType := DetectMimeType(
		resp.Header.Get("Content-Type"),
		GetFileExtensionFromPath(mirrorURL.Path),
	)
	if mimeType == "application/octet-stream" && sr.sniffedMime != "application/octet-stream" {
		mimeType = sr.sniffedMime
	}

	ext := GetFileExtensionFromPath(mirrorURL.Path)
	if ext == "" {
		ext = GetExtensionFromMimeType(mimeType)
	}

	// Rename temp file to final path and save metadata
	if err = s.storage.SaveBlobFromFile(sr.sha256Hash, sr.tempPath, sr.size, pubkey, mimeType, ext); err != nil {
		log.E.F("error saving mirrored blob: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "error saving blob")
		return
	}
	sr.tempPath = "" // Prevent deferred cleanup

	// Build URL
	blobURL := BuildBlobURL(s.getBaseURL(r), sha256Hex, ext)

	descriptor := NewBlobDescriptor(
		blobURL,
		sha256Hex,
		sr.size,
		mimeType,
		time.Now().Unix(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(descriptor); err != nil {
		log.E.F("error encoding response: %v", err)
	}
}

// handleMediaUpload handles PUT /media requests (BUD-05)
// Streams the upload to disk while hashing — memory usage is O(32KB).
// NOTE: When OptimizeMedia is implemented beyond a no-op, it will need to read
// from the temp file rather than an in-memory buffer.
func (s *Server) handleMediaUpload(w http.ResponseWriter, r *http.Request) {
	// Get initial pubkey from request (may be updated by auth validation)
	pubkey, _ := GetPubkeyFromRequest(r)
	remoteAddr := s.getRemoteAddr(r)

	// Validate auth BEFORE reading body
	if r.Header.Get(AuthorizationHeader) != "" {
		authEv, err := ValidateAuthEvent(r, "media", nil)
		if err != nil {
			s.setErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		}
		if authEv != nil {
			pubkey = authEv.Pubkey
		}
	}

	if !s.checkACL(pubkey, remoteAddr, "write") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	// Stream body to temp file while computing SHA256
	sr, err := s.streamToTempFile(r.Body, s.maxBlobSize)
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			s.setErrorResponse(w, http.StatusRequestEntityTooLarge, err.Error())
		} else {
			s.setErrorResponse(w, http.StatusBadRequest, "error reading request body")
		}
		return
	}
	defer func() {
		if sr.tempPath != "" {
			os.Remove(sr.tempPath)
		}
	}()

	// Check bandwidth rate limit
	if !s.checkBandwidthLimit(pubkey, remoteAddr, sr.size) {
		s.setErrorResponse(w, http.StatusTooManyRequests, "upload rate limit exceeded, try again later")
		return
	}

	// Detect MIME type
	mimeType := DetectMimeType(
		r.Header.Get("Content-Type"),
		GetFileExtensionFromPath(r.URL.Path),
	)
	if mimeType == "application/octet-stream" && sr.sniffedMime != "application/octet-stream" {
		mimeType = sr.sniffedMime
	}

	ext := GetFileExtensionFromPath(r.URL.Path)
	if ext == "" {
		ext = GetExtensionFromMimeType(mimeType)
	}

	sha256Hex := hex.Enc(sr.sha256Hash)

	// Check if blob already exists
	exists, err := s.storage.HasBlob(sr.sha256Hash)
	if err != nil {
		log.E.F("error checking blob existence: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if !exists {
		// Check storage quota
		blobSizeMB := sr.size / (1024 * 1024)
		if blobSizeMB == 0 && sr.size > 0 {
			blobSizeMB = 1
		}

		quotaMB, err := s.db.GetBlossomStorageQuota(pubkey)
		if err != nil {
			log.W.F("failed to get storage quota: %v", err)
		} else if quotaMB > 0 {
			usedMB, err := s.storage.GetTotalStorageUsed(pubkey)
			if err != nil {
				log.W.F("failed to calculate storage used: %v", err)
			} else {
				if usedMB+blobSizeMB > quotaMB {
					s.setErrorResponse(w, http.StatusPaymentRequired,
						fmt.Sprintf("storage quota exceeded: %d/%d MB used, %d MB needed",
							usedMB, quotaMB, blobSizeMB))
					return
				}
			}
		}

		// Rename temp file to final path and save metadata
		if err = s.storage.SaveBlobFromFile(sr.sha256Hash, sr.tempPath, sr.size, pubkey, mimeType, ext); err != nil {
			log.E.F("error saving media blob: %v", err)
			s.setErrorResponse(w, http.StatusInternalServerError, "error saving blob")
			return
		}
		sr.tempPath = "" // Prevent deferred cleanup
	}

	blobURL := BuildBlobURL(s.getBaseURL(r), sha256Hex, ext)

	descriptor := NewBlobDescriptor(
		blobURL,
		sha256Hex,
		sr.size,
		mimeType,
		time.Now().Unix(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(descriptor); err != nil {
		log.E.F("error encoding response: %v", err)
	}
}

// handleMediaHead handles HEAD /media requests (BUD-05)
func (s *Server) handleMediaHead(w http.ResponseWriter, r *http.Request) {
	// Similar to handleUploadRequirements but for media
	// Return 200 OK if media optimization is available
	w.WriteHeader(http.StatusOK)
}

// handleGenerateThumbnails handles POST /admin/generate-thumbnails (batch thumbnail generation)
func (s *Server) handleGenerateThumbnails(w http.ResponseWriter, r *http.Request) {
	// Authorization required
	authEv, err := ValidateAuthEvent(r, "admin", nil)
	if err != nil {
		s.setErrorResponse(w, http.StatusUnauthorized, err.Error())
		return
	}
	if authEv == nil {
		s.setErrorResponse(w, http.StatusUnauthorized, "authorization required")
		return
	}

	// Check admin ACL
	remoteAddr := s.getRemoteAddr(r)
	if !s.checkACL(authEv.Pubkey, remoteAddr, "admin") {
		s.setErrorResponse(w, http.StatusForbidden, "admin access required")
		return
	}

	// Get all image blobs
	images, err := s.storage.ListImageBlobs()
	if err != nil {
		log.E.F("failed to list image blobs: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "failed to list blobs")
		return
	}

	// Generate thumbnails for each
	type result struct {
		SHA256  string `json:"sha256"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	results := make([]result, 0, len(images))

	generated := 0
	skipped := 0
	failed := 0

	for _, img := range images {
		sha256Hex := img.SHA256
		thumbKey := fmt.Sprintf("%s_thumb_%d", sha256Hex, ThumbnailSize)

		// Check if thumbnail already exists
		if thumbData, _ := s.storage.GetThumbnail(thumbKey); len(thumbData) > 0 {
			skipped++
			continue
		}

		// Get the blob data
		sha256Hash, err := hex.Dec(sha256Hex)
		if err != nil {
			results = append(results, result{SHA256: sha256Hex, Success: false, Error: "invalid hash"})
			failed++
			continue
		}

		blobData, metadata, err := s.storage.GetBlob(sha256Hash)
		if err != nil {
			results = append(results, result{SHA256: sha256Hex, Success: false, Error: "blob not found"})
			failed++
			continue
		}

		// Generate thumbnail
		thumbData, _, err := GenerateThumbnail(blobData, metadata.MimeType, ThumbnailSize)
		if err != nil {
			results = append(results, result{SHA256: sha256Hex, Success: false, Error: err.Error()})
			failed++
			continue
		}

		// Save thumbnail
		if err := s.storage.SaveThumbnail(thumbKey, thumbData); err != nil {
			results = append(results, result{SHA256: sha256Hex, Success: false, Error: "failed to save"})
			failed++
			continue
		}

		results = append(results, result{SHA256: sha256Hex, Success: true})
		generated++
	}

	log.I.F("thumbnail generation complete: %d generated, %d skipped, %d failed", generated, skipped, failed)

	// Return summary
	response := struct {
		Total     int      `json:"total"`
		Generated int      `json:"generated"`
		Skipped   int      `json:"skipped"`
		Failed    int      `json:"failed"`
		Results   []result `json:"results,omitempty"`
	}{
		Total:     len(images),
		Generated: generated,
		Skipped:   skipped,
		Failed:    failed,
		Results:   results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}


// handleReport handles PUT /report requests (BUD-09)
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	// Check ACL
	pubkey, _ := GetPubkeyFromRequest(r)
	remoteAddr := s.getRemoteAddr(r)

	if !s.checkACL(pubkey, remoteAddr, "read") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	// Read request body (NIP-56 report event)
	var reportEv event.E
	if err := json.NewDecoder(r.Body).Decode(&reportEv); err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate report event (kind 1984 per NIP-56)
	if reportEv.Kind != 1984 {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid event kind, expected 1984")
		return
	}

	// Verify signature
	valid, err := reportEv.Verify()
	if err != nil || !valid {
		s.setErrorResponse(w, http.StatusUnauthorized, "invalid event signature")
		return
	}

	// Extract x tags (blob hashes)
	xTags := reportEv.Tags.GetAll([]byte("x"))
	if len(xTags) == 0 {
		s.setErrorResponse(w, http.StatusBadRequest, "report event missing 'x' tags")
		return
	}

	// Serialize report event
	reportData := reportEv.Serialize()

	// Save report for each blob hash
	for _, xTag := range xTags {
		sha256Hex := string(xTag.Value())
		if !ValidateSHA256Hex(sha256Hex) {
			continue
		}

		sha256Hash, err := hex.Dec(sha256Hex)
		if err != nil {
			continue
		}

		if err = s.storage.SaveReport(sha256Hash, reportData); err != nil {
			log.E.F("error saving report: %v", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

