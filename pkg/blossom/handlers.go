package blossom

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"lol.mleku.dev/log"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"next.orly.dev/pkg/utils"
)

// handleGetBlob handles GET /<sha256> requests (BUD-01)
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

	// Check if blob exists
	exists, err := s.storage.HasBlob(sha256Hash)
	if err != nil {
		log.E.F("error checking blob existence: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if !exists {
		s.setErrorResponse(w, http.StatusNotFound, "blob not found")
		return
	}

	// Get blob metadata
	metadata, err := s.storage.GetBlobMetadata(sha256Hash)
	if err != nil {
		log.E.F("error getting blob metadata: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
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

	// Get blob data
	blobData, _, err := s.storage.GetBlob(sha256Hash)
	if err != nil {
		log.E.F("error getting blob: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Set headers
	mimeType := DetectMimeType(metadata.MimeType, ext)
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(blobData)), 10))
	w.Header().Set("Accept-Ranges", "bytes")

	// Handle range requests (RFC 7233)
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		start, end, valid, err := ParseRangeHeader(rangeHeader, int64(len(blobData)))
		if err != nil {
			s.setErrorResponse(w, http.StatusRequestedRangeNotSatisfiable, err.Error())
			return
		}
		if valid {
			WriteRangeResponse(w, blobData, start, end, int64(len(blobData)))
			return
		}
	}

	// Send full blob
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(blobData)
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

	// Check if blob exists
	exists, err := s.storage.HasBlob(sha256Hash)
	if err != nil {
		log.E.F("error checking blob existence: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if !exists {
		s.setErrorResponse(w, http.StatusNotFound, "blob not found")
		return
	}

	// Get blob metadata
	metadata, err := s.storage.GetBlobMetadata(sha256Hash)
	if err != nil {
		log.E.F("error getting blob metadata: %v", err)
		s.setErrorResponse(w, http.StatusInternalServerError, "internal server error")
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

	// Detect MIME type
	mimeType := DetectMimeType(
		r.Header.Get("Content-Type"),
		GetFileExtensionFromPath(r.URL.Path),
	)

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

	// Set URLs for descriptors
	for _, desc := range descriptors {
		desc.URL = BuildBlobURL(s.getBaseURL(r), desc.SHA256, "")
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
	authEv, err := ValidateAuthEvent(r, "delete", sha256Hash)
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

	// Note: pubkey may be nil for anonymous uploads if ACL allows it

	// Detect MIME type from remote response
	mimeType := DetectMimeType(
		resp.Header.Get("Content-Type"),
		GetFileExtensionFromPath(mirrorURL.Path),
	)

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

	// Note: pubkey may be nil for anonymous uploads if ACL allows it

	// Optimize media (placeholder - actual optimization would be implemented here)
	originalMimeType := DetectMimeType(
		r.Header.Get("Content-Type"),
		GetFileExtensionFromPath(r.URL.Path),
	)
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
