package blossom

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
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

// handleUpload handles PUT /upload requests (BUD-02)
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Get initial pubkey from request (may be updated by auth validation)
	pubkey, _ := GetPubkeyFromRequest(r)
	remoteAddr := s.getRemoteAddr(r)

	// Read request body
	body, err := io.ReadAll(io.LimitReader(r.Body, s.maxBlobSize+1))
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "error reading request body")
		return
	}

	if int64(len(body)) > s.maxBlobSize {
		s.setErrorResponse(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("blob too large: max %d bytes", s.maxBlobSize))
		return
	}

	// Optional authorization validation (do this BEFORE ACL check)
	// For upload, we don't pass sha256Hash because upload auth events don't have 'x' tags
	// (the hash isn't known at auth event creation time)
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

	// Check ACL (do this AFTER getting pubkey from auth)
	if !s.checkACL(pubkey, remoteAddr, "write") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	// Check bandwidth rate limit (non-followed users)
	if !s.checkBandwidthLimit(pubkey, remoteAddr, int64(len(body))) {
		s.setErrorResponse(w, http.StatusTooManyRequests, "upload rate limit exceeded, try again later")
		return
	}

	// Calculate SHA256 after auth check
	sha256Hash := CalculateSHA256(body)
	sha256Hex := hex.Enc(sha256Hash)

	// Check if blob already exists
	exists, err := s.storage.HasBlob(sha256Hash)
	if err != nil {
		log.E.F("error checking blob existence: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Note: pubkey may be nil for anonymous uploads if ACL allows it
	// The storage layer will handle anonymous uploads appropriately

	// Detect MIME type from header, extension, or content sniffing
	mimeType := DetectMimeType(
		r.Header.Get("Content-Type"),
		GetFileExtensionFromPath(r.URL.Path),
	)
	if mimeType == "application/octet-stream" && len(body) > 0 {
		if sniffed := http.DetectContentType(body); sniffed != "application/octet-stream" {
			mimeType = sniffed
		}
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

	// Check storage quota if blob doesn't exist (new upload)
	if !exists {
		blobSizeMB := int64(len(body)) / (1024 * 1024)
		if blobSizeMB == 0 && len(body) > 0 {
			blobSizeMB = 1 // At least 1 MB for any non-zero blob
		}

		// Get storage quota from database
		quotaMB, err := s.db.GetBlossomStorageQuota(pubkey)
		if err != nil {
			log.W.F("failed to get storage quota: %v", err)
		} else if quotaMB > 0 {
			// Get current storage used
			usedMB, err := s.storage.GetTotalStorageUsed(pubkey)
			if err != nil {
				log.W.F("failed to calculate storage used: %v", err)
			} else {
				// Check if upload would exceed quota
				if usedMB+blobSizeMB > quotaMB {
					s.setErrorResponse(w, http.StatusPaymentRequired,
						fmt.Sprintf("storage quota exceeded: %d/%d MB used, %d MB needed",
							usedMB, quotaMB, blobSizeMB))
					return
				}
			}
		}
	}

	// Save blob if it doesn't exist
	if !exists {
		if err = s.storage.SaveBlob(sha256Hash, body, pubkey, mimeType, ext); err != nil {
			log.E.F("error saving blob: %v", err)
			s.setErrorResponse(w, http.StatusInternalServerError, "error saving blob")
			return
		}
	} else {
		// Verify ownership
		metadata, err := s.storage.GetBlobMetadata(sha256Hash)
		if err != nil {
			log.E.F("error getting blob metadata: %v", err)
			s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Allow if same pubkey or if ACL allows
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
		int64(len(body)),
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

	// Filter out variant blobs - only show originals
	// Check for variant markers set during migration
	variantHashes := s.getVariantHashes(descriptors)

	// Filter descriptors
	filtered := make([]*BlobDescriptor, 0, len(descriptors))
	for _, desc := range descriptors {
		hashLower := strings.ToLower(desc.SHA256)
		if !variantHashes[hashLower] {
			filtered = append(filtered, desc)
		}
	}
	descriptors = filtered

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

// getVariantHashes returns a set of SHA256 hashes that are variant blobs (not originals).
// It checks the storage for "variantof:" markers set during migration.
func (s *Server) getVariantHashes(blobs []*BlobDescriptor) map[string]bool {
	variantHashes := make(map[string]bool)

	// Check each blob for a variant marker
	for _, blob := range blobs {
		variantKey := "variantof:" + strings.ToLower(blob.SHA256)
		if data, _ := s.storage.GetThumbnail(variantKey); len(data) > 0 {
			// This blob is a variant - mark it for filtering
			variantHashes[strings.ToLower(blob.SHA256)] = true
		}
	}

	return variantHashes
}

// handleDeleteVariants handles DELETE /delete-variants/<sha256> requests
// Deletes all variant blobs associated with an original image
func (s *Server) handleDeleteVariants(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Extract original sha256 from path: delete-variants/<sha256>
	if !strings.HasPrefix(path, "delete-variants/") {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid path")
		return
	}

	originalHashHex := strings.TrimPrefix(path, "delete-variants/")
	// Remove any extension
	if idx := strings.Index(originalHashHex, "."); idx >= 0 {
		originalHashHex = originalHashHex[:idx]
	}
	originalHashHex = strings.ToLower(originalHashHex)

	if len(originalHashHex) != 64 {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid sha256 format")
		return
	}

	// Authorization required
	authEv, err := ValidateAuthEvent(r, "delete", nil)
	if err != nil {
		s.setErrorResponse(w, http.StatusUnauthorized, err.Error())
		return
	}
	if authEv == nil {
		s.setErrorResponse(w, http.StatusUnauthorized, "authorization required")
		return
	}

	// Get requesting user's pubkey
	requestPubkey := authEv.Pubkey

	// Get the original blob to verify ownership
	originalHash, err := hex.Dec(originalHashHex)
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid sha256")
		return
	}

	_, metadata, err := s.storage.GetBlob(originalHash)
	if err != nil {
		s.setErrorResponse(w, http.StatusNotFound, "original blob not found")
		return
	}

	// Check ownership or admin
	if !utils.FastEqual(metadata.Pubkey, requestPubkey) && !s.checkACL(requestPubkey, s.getRemoteAddr(r), "admin") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	// Find all variant blobs by checking markers
	blobs, err := s.storage.ListBlobs(metadata.Pubkey, 0, 0)
	if err != nil {
		s.setErrorResponse(w, http.StatusInternalServerError, "failed to list blobs")
		return
	}

	deleted := 0
	failed := 0

	for _, blob := range blobs {
		blobHashLower := strings.ToLower(blob.SHA256)
		variantKey := "variantof:" + blobHashLower

		// Check if this blob is a variant of our original
		if data, _ := s.storage.GetThumbnail(variantKey); len(data) > 0 {
			storedOriginal := strings.ToLower(string(data))
			if storedOriginal == originalHashHex {
				// This is a variant of our original - delete it
				blobHash, err := hex.Dec(blob.SHA256)
				if err != nil {
					failed++
					continue
				}

				if err := s.storage.DeleteBlob(blobHash, metadata.Pubkey); err != nil {
					log.W.F("failed to delete variant %s: %v", blob.SHA256, err)
					failed++
					continue
				}

				// Delete the variant marker
				s.storage.SaveThumbnail(variantKey, nil)
				deleted++
			}
		}
	}

	// Clear migration marker
	migratedKey := "migrated:" + originalHashHex
	s.storage.SaveThumbnail(migratedKey, nil)

	// Return result
	response := struct {
		Original string `json:"original"`
		Deleted  int    `json:"deleted"`
		Failed   int    `json:"failed"`
	}{
		Original: originalHashHex,
		Deleted:  deleted,
		Failed:   failed,
	}

	log.I.F("deleted variants for %s: %d deleted, %d failed", originalHashHex, deleted, failed)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
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

	// Read request body (JSON with URL)
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

	// Read blob data
	body, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBlobSize+1))
	if err != nil {
		s.setErrorResponse(w, http.StatusBadGateway, "error reading remote blob")
		return
	}

	if int64(len(body)) > s.maxBlobSize {
		s.setErrorResponse(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("blob too large: max %d bytes", s.maxBlobSize))
		return
	}

	// Calculate SHA256
	sha256Hash := CalculateSHA256(body)
	sha256Hex := hex.Enc(sha256Hash)

	// Optional authorization validation (do this BEFORE ACL check)
	// For mirror (which uses upload semantics), don't pass sha256Hash
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

	// Check ACL (do this AFTER getting pubkey from auth)
	if !s.checkACL(pubkey, remoteAddr, "write") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	// Check bandwidth rate limit (non-followed users)
	if !s.checkBandwidthLimit(pubkey, remoteAddr, int64(len(body))) {
		s.setErrorResponse(w, http.StatusTooManyRequests, "upload rate limit exceeded, try again later")
		return
	}

	// Note: pubkey may be nil for anonymous uploads if ACL allows it

	// Detect MIME type from remote response, extension, or content sniffing
	mimeType := DetectMimeType(
		resp.Header.Get("Content-Type"),
		GetFileExtensionFromPath(mirrorURL.Path),
	)
	if mimeType == "application/octet-stream" && len(body) > 0 {
		if sniffed := http.DetectContentType(body); sniffed != "application/octet-stream" {
			mimeType = sniffed
		}
	}

	// Extract extension from path or infer from MIME type
	ext := GetFileExtensionFromPath(mirrorURL.Path)
	if ext == "" {
		ext = GetExtensionFromMimeType(mimeType)
	}

	// Save blob
	if err = s.storage.SaveBlob(sha256Hash, body, pubkey, mimeType, ext); err != nil {
		log.E.F("error saving mirrored blob: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "error saving blob")
		return
	}

	// Build URL
	blobURL := BuildBlobURL(s.getBaseURL(r), sha256Hex, ext)

	// Create descriptor
	descriptor := NewBlobDescriptor(
		blobURL,
		sha256Hex,
		int64(len(body)),
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

// handleMediaUpload handles PUT /media requests (BUD-05)
func (s *Server) handleMediaUpload(w http.ResponseWriter, r *http.Request) {
	// Get initial pubkey from request (may be updated by auth validation)
	pubkey, _ := GetPubkeyFromRequest(r)
	remoteAddr := s.getRemoteAddr(r)

	// Read request body
	body, err := io.ReadAll(io.LimitReader(r.Body, s.maxBlobSize+1))
	if err != nil {
		s.setErrorResponse(w, http.StatusBadRequest, "error reading request body")
		return
	}

	if int64(len(body)) > s.maxBlobSize {
		s.setErrorResponse(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("blob too large: max %d bytes", s.maxBlobSize))
		return
	}

	// Optional authorization validation (do this BEFORE ACL check)
	// For media upload, don't pass sha256Hash (similar to regular upload)
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

	// Check ACL (do this AFTER getting pubkey from auth)
	if !s.checkACL(pubkey, remoteAddr, "write") {
		s.setErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	// Check bandwidth rate limit (non-followed users)
	if !s.checkBandwidthLimit(pubkey, remoteAddr, int64(len(body))) {
		s.setErrorResponse(w, http.StatusTooManyRequests, "upload rate limit exceeded, try again later")
		return
	}

	// Note: pubkey may be nil for anonymous uploads if ACL allows it

	// Detect MIME type from header, extension, or content sniffing
	originalMimeType := DetectMimeType(
		r.Header.Get("Content-Type"),
		GetFileExtensionFromPath(r.URL.Path),
	)
	if originalMimeType == "application/octet-stream" && len(body) > 0 {
		if sniffed := http.DetectContentType(body); sniffed != "application/octet-stream" {
			originalMimeType = sniffed
		}
	}

	// Optimize media (placeholder - actual optimization would be implemented here)
	optimizedData, mimeType := OptimizeMedia(body, originalMimeType)

	// Extract extension from path or infer from MIME type
	ext := GetFileExtensionFromPath(r.URL.Path)
	if ext == "" {
		ext = GetExtensionFromMimeType(mimeType)
	}

	// Calculate optimized blob SHA256
	optimizedHash := CalculateSHA256(optimizedData)
	optimizedHex := hex.Enc(optimizedHash)

	// Check if optimized blob already exists
	exists, err := s.storage.HasBlob(optimizedHash)
	if err != nil {
		log.E.F("error checking blob existence: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Check storage quota if optimized blob doesn't exist (new upload)
	if !exists {
		blobSizeMB := int64(len(optimizedData)) / (1024 * 1024)
		if blobSizeMB == 0 && len(optimizedData) > 0 {
			blobSizeMB = 1 // At least 1 MB for any non-zero blob
		}

		// Get storage quota from database
		quotaMB, err := s.db.GetBlossomStorageQuota(pubkey)
		if err != nil {
			log.W.F("failed to get storage quota: %v", err)
		} else if quotaMB > 0 {
			// Get current storage used
			usedMB, err := s.storage.GetTotalStorageUsed(pubkey)
			if err != nil {
				log.W.F("failed to calculate storage used: %v", err)
			} else {
				// Check if upload would exceed quota
				if usedMB+blobSizeMB > quotaMB {
					s.setErrorResponse(w, http.StatusPaymentRequired,
						fmt.Sprintf("storage quota exceeded: %d/%d MB used, %d MB needed",
							usedMB, quotaMB, blobSizeMB))
					return
				}
			}
		}
	}

	// Save optimized blob
	if err = s.storage.SaveBlob(optimizedHash, optimizedData, pubkey, mimeType, ext); err != nil {
		log.E.F("error saving optimized blob: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "error saving blob")
		return
	}

	// Build URL
	blobURL := BuildBlobURL(s.baseURL, optimizedHex, ext)

	// Create descriptor
	descriptor := NewBlobDescriptor(
		blobURL,
		optimizedHex,
		int64(len(optimizedData)),
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

// handleMigrateResponsive handles POST /migrate-responsive
// Generates responsive image variants for the authenticated user's own blobs.
// Requires write-level access.
func (s *Server) handleMigrateResponsive(w http.ResponseWriter, r *http.Request) {
	// Authorization required
	authEv, err := ValidateAuthEvent(r, "upload", nil)
	if err != nil {
		s.setErrorResponse(w, http.StatusUnauthorized, err.Error())
		return
	}
	if authEv == nil {
		s.setErrorResponse(w, http.StatusUnauthorized, "authorization required")
		return
	}

	// Check write ACL (user migrates their own blobs)
	remoteAddr := s.getRemoteAddr(r)
	if !s.checkACL(authEv.Pubkey, remoteAddr, "write") {
		s.setErrorResponse(w, http.StatusForbidden, "write access required")
		return
	}

	// Use authenticated user's pubkey
	targetPubkey := authEv.Pubkey
	targetPubkeyHex := hex.Enc(targetPubkey)

	s.doMigrateResponsive(w, r, targetPubkey, targetPubkeyHex)
}

// handleAdminMigrateResponsive handles POST /admin/migrate-responsive/{pubkey}
// Admin version that can migrate any user's blobs.
func (s *Server) handleAdminMigrateResponsive(w http.ResponseWriter, r *http.Request) {
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

	// Extract target pubkey from path: admin/migrate-responsive/{pubkey}
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		s.setErrorResponse(w, http.StatusBadRequest, "missing pubkey in path")
		return
	}
	targetPubkeyHex := parts[2]

	targetPubkey, err := hex.Dec(targetPubkeyHex)
	if err != nil || len(targetPubkey) != 32 {
		s.setErrorResponse(w, http.StatusBadRequest, "invalid pubkey format")
		return
	}

	s.doMigrateResponsive(w, r, targetPubkey, targetPubkeyHex)
}

// doMigrateResponsive is the shared implementation for responsive image migration.
func (s *Server) doMigrateResponsive(w http.ResponseWriter, r *http.Request, targetPubkey []byte, targetPubkeyHex string) {

	// Get relay identity secret for signing events
	relaySecret, err := s.db.GetOrCreateRelayIdentitySecret()
	if err != nil {
		log.E.F("failed to get relay identity: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "relay identity unavailable")
		return
	}

	// Initialize signer
	sign := p8k.MustNew()
	if err := sign.InitSec(relaySecret); err != nil {
		log.E.F("failed to init signer: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "signer initialization failed")
		return
	}

	// Get all blobs for this user
	blobs, err := s.storage.ListBlobs(targetPubkey, 0, 0)
	if err != nil {
		log.E.F("failed to list blobs for %s: %v", targetPubkeyHex, err)
		s.setErrorResponse(w, http.StatusInternalServerError, "failed to list blobs")
		return
	}

	// Filter to images only
	var images []*BlobDescriptor
	for _, b := range blobs {
		if IsImageMimeType(b.Type) {
			images = append(images, b)
		}
	}

	// Process each image
	type result struct {
		SHA256  string `json:"sha256"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
		EventID string `json:"event_id,omitempty"`
	}
	results := make([]result, 0, len(images))

	processed := 0
	skipped := 0
	failed := 0

	baseURL := s.getBaseURL(r)

	for _, img := range images {
		sha256Hex := img.SHA256

		// Check if already migrated (check for existing kind 1063 with this hash)
		migratedKey := "migrated:" + sha256Hex
		if thumbData, _ := s.storage.GetThumbnail(migratedKey); len(thumbData) > 0 {
			skipped++
			continue
		}

		// Get the original blob data
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

		// Generate responsive variants using Lanczos
		variants, err := GenerateResponsiveVariants(blobData, metadata.MimeType)
		if err != nil {
			results = append(results, result{SHA256: sha256Hex, Success: false, Error: err.Error()})
			failed++
			continue
		}

		// Save each variant as a new blob and build imeta tags
		type uploadedVariant struct {
			Variant  ImageVariant
			SHA256   string
			URL      string
			Width    int
			Height   int
			MimeType string
			Size     int
		}
		uploadedVariants := make([]uploadedVariant, 0, len(variants))

		variantFailed := false
		for _, v := range variants {
			// Compute SHA256 of variant data
			variantHash := computeSHA256(v.Data)
			variantHex := hex.Enc(variantHash)

			// Check if this variant already exists (deduplication)
			exists, _ := s.storage.HasBlob(variantHash)
			if !exists {
				// Save the variant blob
				if err := s.storage.SaveBlob(variantHash, v.Data, targetPubkey, v.MimeType, ".jpg"); err != nil {
					log.E.F("failed to save variant %s for %s: %v", v.Variant, sha256Hex, err)
					variantFailed = true
					break
				}
			}

			// Mark this blob as a variant of the original (for filtering in listings)
			// Skip marking the original itself
			if string(v.Variant) != "original" {
				variantKey := "variantof:" + variantHex
				s.storage.SaveThumbnail(variantKey, []byte(sha256Hex))
			}

			// Construct URL for this variant
			variantURL := baseURL + "/" + variantHex + ".jpg"

			uploadedVariants = append(uploadedVariants, uploadedVariant{
				Variant:  v.Variant,
				SHA256:   variantHex,
				URL:      variantURL,
				Width:    v.Width,
				Height:   v.Height,
				MimeType: v.MimeType,
				Size:     len(v.Data),
			})
		}

		if variantFailed {
			results = append(results, result{SHA256: sha256Hex, Success: false, Error: "variant save failed"})
			failed++
			continue
		}

		// Create addressable binding event (kind 30063 = 30000 + 1063)
		// Using parameterized replaceable kind allows corrections
		ev := event.New()
		ev.Kind = 30063 // Addressable file metadata
		ev.Pubkey = sign.Pub()
		ev.CreatedAt = timestamp.Now().V
		ev.Content = []byte{}
		ev.Tags = tag.NewS()

		// Add d tag with original hash for addressability
		*ev.Tags = append(*ev.Tags, tag.NewFromAny("d", sha256Hex))

		// Add imeta tag for each variant (ordered from smallest to largest)
		for _, v := range uploadedVariants {
			imetaTag := tag.NewFromAny("imeta",
				"url "+v.URL,
				"x "+v.SHA256,
				"m "+v.MimeType,
				fmt.Sprintf("dim %dx%d", v.Width, v.Height),
				"variant "+string(v.Variant),
				fmt.Sprintf("size %d", v.Size),
			)
			*ev.Tags = append(*ev.Tags, imetaTag)
		}

		// Add separate x tags for each variant hash (enables NIP-01 tag queries)
		for _, v := range uploadedVariants {
			*ev.Tags = append(*ev.Tags, tag.NewFromAny("x", v.SHA256))
		}

		// Sign the event
		if err := ev.Sign(sign); err != nil {
			log.E.F("failed to sign event for %s: %v", sha256Hex, err)
			results = append(results, result{SHA256: sha256Hex, Success: false, Error: "sign failed"})
			failed++
			continue
		}

		// Save event to database
		ctx := r.Context()
		if _, err := s.db.SaveEvent(ctx, ev); err != nil {
			log.E.F("failed to save event for %s: %v", sha256Hex, err)
			results = append(results, result{SHA256: sha256Hex, Success: false, Error: "event save failed"})
			failed++
			continue
		}

		// Mark as migrated (store event ID as marker)
		eventIDHex := hex.Enc(ev.ID)
		s.storage.SaveThumbnail(migratedKey, []byte(eventIDHex))

		results = append(results, result{
			SHA256:  sha256Hex,
			Success: true,
			EventID: eventIDHex,
		})
		processed++

		log.D.F("migrated %s: created %d variants, event %s", sha256Hex, len(variants), eventIDHex)
	}

	log.I.F("responsive migration for %s complete: %d processed, %d skipped, %d failed",
		targetPubkeyHex, processed, skipped, failed)

	// Return summary
	response := struct {
		Total     int      `json:"total"`
		Processed int      `json:"processed"`
		Skipped   int      `json:"skipped"`
		Failed    int      `json:"failed"`
		Results   []result `json:"results,omitempty"`
	}{
		Total:     len(images),
		Processed: processed,
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

// computeSHA256 computes the SHA256 hash of data
func computeSHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}
