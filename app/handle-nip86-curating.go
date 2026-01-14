package app

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/httpauth"
)

// handleCuratingNIP86Request handles curating NIP-86 requests with pre-authenticated pubkey.
// This is called from the main NIP-86 handler after authentication.
func (s *Server) handleCuratingNIP86Request(w http.ResponseWriter, r *http.Request, pubkey []byte) {
	_ = pubkey // Pubkey already validated by caller

	// Get the curating ACL instance
	var curatingACL *acl.Curating
	for _, aclInstance := range acl.Registry.ACL {
		if aclInstance.Type() == "curating" {
			if curating, ok := aclInstance.(*acl.Curating); ok {
				curatingACL = curating
				break
			}
		}
	}

	if curatingACL == nil {
		http.Error(w, "Curating ACL not available", http.StatusInternalServerError)
		return
	}

	// Read and parse the request
	body, err := io.ReadAll(r.Body)
	if chk.E(err) {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var request NIP86Request
	if err := json.Unmarshal(body, &request); chk.E(err) {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Handle the request based on method
	response := s.handleCuratingNIP86Method(request, curatingACL)

	// Send response
	jsonData, err := json.Marshal(response)
	if chk.E(err) {
		http.Error(w, "Error generating response", http.StatusInternalServerError)
		return
	}

	w.Write(jsonData)
}

// handleCuratingNIP86Management handles NIP-86 management API requests for curating mode (standalone)
func (s *Server) handleCuratingNIP86Management(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/nostr+json+rpc" {
		http.Error(w, "Content-Type must be application/nostr+json+rpc", http.StatusBadRequest)
		return
	}

	// Validate NIP-98 authentication
	valid, pubkey, err := httpauth.CheckAuth(r)
	if chk.E(err) || !valid {
		errorMsg := "NIP-98 authentication validation failed"
		if err != nil {
			errorMsg = err.Error()
		}
		http.Error(w, errorMsg, http.StatusUnauthorized)
		return
	}

	// Check permissions - require owner or admin level
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "owner" && accessLevel != "admin" {
		http.Error(w, "Owner or admin permission required", http.StatusForbidden)
		return
	}

	// Check if curating ACL is active
	if acl.Registry.Type() != "curating" {
		http.Error(w, "Curating ACL mode is not active", http.StatusBadRequest)
		return
	}

	// Delegate to shared request handler
	s.handleCuratingNIP86Request(w, r, pubkey)
}

// handleCuratingNIP86Method handles individual NIP-86 methods for curating mode
func (s *Server) handleCuratingNIP86Method(request NIP86Request, curatingACL *acl.Curating) NIP86Response {
	dbACL := curatingACL.GetCuratingACL()

	switch request.Method {
	case "supportedmethods":
		return s.handleCuratingSupportedMethods()
	case "trustpubkey":
		return s.handleTrustPubkey(request.Params, curatingACL)
	case "untrustpubkey":
		return s.handleUntrustPubkey(request.Params, curatingACL)
	case "listtrustedpubkeys":
		return s.handleListTrustedPubkeys(dbACL)
	case "blacklistpubkey":
		return s.handleBlacklistPubkey(request.Params, curatingACL)
	case "unblacklistpubkey":
		return s.handleUnblacklistPubkey(request.Params, curatingACL)
	case "listblacklistedpubkeys":
		return s.handleListBlacklistedPubkeys(dbACL)
	case "listunclassifiedusers":
		return s.handleListUnclassifiedUsers(request.Params, dbACL)
	case "markspam":
		return s.handleMarkSpam(request.Params, dbACL)
	case "unmarkspam":
		return s.handleUnmarkSpam(request.Params, dbACL)
	case "listspamevents":
		return s.handleListSpamEvents(dbACL)
	case "deleteevent":
		return s.handleDeleteEvent(request.Params)
	case "getcuratingconfig":
		return s.handleGetCuratingConfig(dbACL)
	case "listblockedips":
		return s.handleListCuratingBlockedIPs(dbACL)
	case "unblockip":
		return s.handleUnblockCuratingIP(request.Params, dbACL)
	case "isconfigured":
		return s.handleIsConfigured(dbACL)
	case "scanpubkeys":
		return s.handleScanPubkeys(dbACL)
	default:
		return NIP86Response{Error: "Unknown method: " + request.Method}
	}
}

// handleCuratingSupportedMethods returns the list of supported methods for curating mode
func (s *Server) handleCuratingSupportedMethods() NIP86Response {
	methods := []string{
		"supportedmethods",
		"trustpubkey",
		"untrustpubkey",
		"listtrustedpubkeys",
		"blacklistpubkey",
		"unblacklistpubkey",
		"listblacklistedpubkeys",
		"listunclassifiedusers",
		"markspam",
		"unmarkspam",
		"listspamevents",
		"deleteevent",
		"getcuratingconfig",
		"listblockedips",
		"unblockip",
		"isconfigured",
		"scanpubkeys",
	}
	return NIP86Response{Result: methods}
}

// handleTrustPubkey adds a pubkey to the trusted list
func (s *Server) handleTrustPubkey(params []interface{}, curatingACL *acl.Curating) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: pubkey"}
	}

	pubkey, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid pubkey parameter"}
	}

	if len(pubkey) != 64 {
		return NIP86Response{Error: "Invalid pubkey format (must be 64 hex characters)"}
	}

	note := ""
	if len(params) > 1 {
		if n, ok := params[1].(string); ok {
			note = n
		}
	}

	if err := curatingACL.TrustPubkey(pubkey, note); chk.E(err) {
		return NIP86Response{Error: "Failed to trust pubkey: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleUntrustPubkey removes a pubkey from the trusted list
func (s *Server) handleUntrustPubkey(params []interface{}, curatingACL *acl.Curating) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: pubkey"}
	}

	pubkey, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid pubkey parameter"}
	}

	if err := curatingACL.UntrustPubkey(pubkey); chk.E(err) {
		return NIP86Response{Error: "Failed to untrust pubkey: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleListTrustedPubkeys returns the list of trusted pubkeys
func (s *Server) handleListTrustedPubkeys(dbACL *database.CuratingACL) NIP86Response {
	trusted, err := dbACL.ListTrustedPubkeys()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list trusted pubkeys: " + err.Error()}
	}

	result := make([]map[string]interface{}, len(trusted))
	for i, t := range trusted {
		result[i] = map[string]interface{}{
			"pubkey": t.Pubkey,
			"note":   t.Note,
			"added":  t.Added.Unix(),
		}
	}

	return NIP86Response{Result: result}
}

// handleBlacklistPubkey adds a pubkey to the blacklist
func (s *Server) handleBlacklistPubkey(params []interface{}, curatingACL *acl.Curating) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: pubkey"}
	}

	pubkey, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid pubkey parameter"}
	}

	if len(pubkey) != 64 {
		return NIP86Response{Error: "Invalid pubkey format (must be 64 hex characters)"}
	}

	reason := ""
	if len(params) > 1 {
		if r, ok := params[1].(string); ok {
			reason = r
		}
	}

	if err := curatingACL.BlacklistPubkey(pubkey, reason); chk.E(err) {
		return NIP86Response{Error: "Failed to blacklist pubkey: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleUnblacklistPubkey removes a pubkey from the blacklist
func (s *Server) handleUnblacklistPubkey(params []interface{}, curatingACL *acl.Curating) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: pubkey"}
	}

	pubkey, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid pubkey parameter"}
	}

	if err := curatingACL.UnblacklistPubkey(pubkey); chk.E(err) {
		return NIP86Response{Error: "Failed to unblacklist pubkey: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleListBlacklistedPubkeys returns the list of blacklisted pubkeys
func (s *Server) handleListBlacklistedPubkeys(dbACL *database.CuratingACL) NIP86Response {
	blacklisted, err := dbACL.ListBlacklistedPubkeys()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list blacklisted pubkeys: " + err.Error()}
	}

	result := make([]map[string]interface{}, len(blacklisted))
	for i, b := range blacklisted {
		result[i] = map[string]interface{}{
			"pubkey": b.Pubkey,
			"reason": b.Reason,
			"added":  b.Added.Unix(),
		}
	}

	return NIP86Response{Result: result}
}

// handleListUnclassifiedUsers returns unclassified users sorted by event count
func (s *Server) handleListUnclassifiedUsers(params []interface{}, dbACL *database.CuratingACL) NIP86Response {
	limit := 100 // Default limit
	if len(params) > 0 {
		if l, ok := params[0].(float64); ok {
			limit = int(l)
		}
	}

	users, err := dbACL.ListUnclassifiedUsers(limit)
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list unclassified users: " + err.Error()}
	}

	result := make([]map[string]interface{}, len(users))
	for i, u := range users {
		result[i] = map[string]interface{}{
			"pubkey":      u.Pubkey,
			"event_count": u.EventCount,
			"last_event":  u.LastEvent.Unix(),
		}
	}

	return NIP86Response{Result: result}
}

// handleMarkSpam marks an event as spam
func (s *Server) handleMarkSpam(params []interface{}, dbACL *database.CuratingACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: event_id"}
	}

	eventID, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid event_id parameter"}
	}

	if len(eventID) != 64 {
		return NIP86Response{Error: "Invalid event_id format (must be 64 hex characters)"}
	}

	pubkey := ""
	if len(params) > 1 {
		if p, ok := params[1].(string); ok {
			pubkey = p
		}
	}

	reason := ""
	if len(params) > 2 {
		if r, ok := params[2].(string); ok {
			reason = r
		}
	}

	if err := dbACL.MarkEventAsSpam(eventID, pubkey, reason); chk.E(err) {
		return NIP86Response{Error: "Failed to mark event as spam: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleUnmarkSpam removes the spam flag from an event
func (s *Server) handleUnmarkSpam(params []interface{}, dbACL *database.CuratingACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: event_id"}
	}

	eventID, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid event_id parameter"}
	}

	if err := dbACL.UnmarkEventAsSpam(eventID); chk.E(err) {
		return NIP86Response{Error: "Failed to unmark event as spam: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleListSpamEvents returns the list of spam-flagged events
func (s *Server) handleListSpamEvents(dbACL *database.CuratingACL) NIP86Response {
	spam, err := dbACL.ListSpamEvents()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list spam events: " + err.Error()}
	}

	result := make([]map[string]interface{}, len(spam))
	for i, sp := range spam {
		result[i] = map[string]interface{}{
			"event_id": sp.EventID,
			"pubkey":   sp.Pubkey,
			"reason":   sp.Reason,
			"added":    sp.Added.Unix(),
		}
	}

	return NIP86Response{Result: result}
}

// handleDeleteEvent permanently deletes an event from the database
func (s *Server) handleDeleteEvent(params []interface{}) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: event_id"}
	}

	eventIDHex, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid event_id parameter"}
	}

	if len(eventIDHex) != 64 {
		return NIP86Response{Error: "Invalid event_id format (must be 64 hex characters)"}
	}

	// Convert hex to bytes
	eventID, err := hex.DecodeString(eventIDHex)
	if err != nil {
		return NIP86Response{Error: "Invalid event_id hex: " + err.Error()}
	}

	// Delete from database
	if err := s.DB.DeleteEvent(context.Background(), eventID); chk.E(err) {
		return NIP86Response{Error: "Failed to delete event: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleGetCuratingConfig returns the current curating configuration
func (s *Server) handleGetCuratingConfig(dbACL *database.CuratingACL) NIP86Response {
	config, err := dbACL.GetConfig()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to get config: " + err.Error()}
	}

	result := map[string]interface{}{
		"daily_limit":       config.DailyLimit,
		"first_ban_hours":   config.FirstBanHours,
		"second_ban_hours":  config.SecondBanHours,
		"allowed_kinds":     config.AllowedKinds,
		"custom_kinds":      config.AllowedKinds, // Alias for frontend compatibility
		"allowed_ranges":    config.AllowedRanges,
		"kind_ranges":       config.AllowedRanges, // Alias for frontend compatibility
		"kind_categories":   config.KindCategories,
		"categories":        config.KindCategories, // Alias for frontend compatibility
		"config_event_id":   config.ConfigEventID,
		"config_pubkey":     config.ConfigPubkey,
		"configured_at":     config.ConfiguredAt,
		"is_configured":     config.ConfigEventID != "",
	}

	return NIP86Response{Result: result}
}

// handleListCuratingBlockedIPs returns the list of blocked IPs in curating mode
func (s *Server) handleListCuratingBlockedIPs(dbACL *database.CuratingACL) NIP86Response {
	blocked, err := dbACL.ListBlockedIPs()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list blocked IPs: " + err.Error()}
	}

	result := make([]map[string]interface{}, len(blocked))
	for i, b := range blocked {
		result[i] = map[string]interface{}{
			"ip":         b.IP,
			"reason":     b.Reason,
			"expires_at": b.ExpiresAt.Unix(),
			"added":      b.Added.Unix(),
		}
	}

	return NIP86Response{Result: result}
}

// handleUnblockCuratingIP unblocks an IP in curating mode
func (s *Server) handleUnblockCuratingIP(params []interface{}, dbACL *database.CuratingACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: ip"}
	}

	ip, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid ip parameter"}
	}

	if err := dbACL.UnblockIP(ip); chk.E(err) {
		return NIP86Response{Error: "Failed to unblock IP: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleIsConfigured checks if curating mode is configured
func (s *Server) handleIsConfigured(dbACL *database.CuratingACL) NIP86Response {
	configured, err := dbACL.IsConfigured()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to check configuration: " + err.Error()}
	}

	return NIP86Response{Result: configured}
}

// GetKindCategoriesInfo returns information about available kind categories
func GetKindCategoriesInfo() []map[string]interface{} {
	categories := []map[string]interface{}{
		{
			"id":          "social",
			"name":        "Social/Notes",
			"description": "Profiles, text notes, follows, reposts, reactions",
			"kinds":       []int{0, 1, 3, 6, 7, 10002},
		},
		{
			"id":          "dm",
			"name":        "Direct Messages",
			"description": "NIP-04 DMs, NIP-17 private messages, gift wraps",
			"kinds":       []int{4, 14, 1059},
		},
		{
			"id":          "longform",
			"name":        "Long-form Content",
			"description": "Articles and drafts",
			"kinds":       []int{30023, 30024},
		},
		{
			"id":          "media",
			"name":        "Media",
			"description": "File metadata, video, audio",
			"kinds":       []int{1063, 20, 21, 22},
		},
		{
			"id":          "marketplace_nip15",
			"name":        "Marketplace (NIP-15)",
			"description": "Legacy NIP-15 stalls and products",
			"kinds":       []int{30017, 30018, 30019, 30020, 1021, 1022},
		},
		{
			"id":          "marketplace_nip99",
			"name":        "Marketplace (NIP-99/Gamma)",
			"description": "NIP-99 classified listings, collections, shipping, reviews (Plebeian Market)",
			"kinds":       []int{30402, 30403, 30405, 30406, 31555},
		},
		{
			"id":          "order_communication",
			"name":        "Order Communication",
			"description": "Gamma Markets order messages and payment receipts",
			"kinds":       []int{16, 17},
		},
		{
			"id":          "groups_nip29",
			"name":        "Group Messaging (NIP-29)",
			"description": "Simple group messages and metadata",
			"kinds":       []int{9, 10, 11, 12, 9000, 9001, 9002, 39000, 39001, 39002},
		},
		{
			"id":          "groups_nip72",
			"name":        "Communities (NIP-72)",
			"description": "Moderated communities and post approvals",
			"kinds":       []int{34550, 1111, 4550},
		},
		{
			"id":          "lists",
			"name":        "Lists/Bookmarks",
			"description": "Mute lists, pins, categorized lists, bookmarks",
			"kinds":       []int{10000, 10001, 10003, 30000, 30001, 30003},
		},
	}
	return categories
}

// expandKindRange expands a range string like "1000-1999" into individual kinds
func expandKindRange(rangeStr string) []int {
	var kinds []int
	parts := make([]int, 2)
	n, err := parseRange(rangeStr, parts)
	if err != nil || n != 2 {
		return kinds
	}
	for i := parts[0]; i <= parts[1]; i++ {
		kinds = append(kinds, i)
	}
	return kinds
}

func parseRange(s string, parts []int) (int, error) {
	// Simple parsing of "start-end"
	for i, c := range s {
		if c == '-' && i > 0 {
			start, err := strconv.Atoi(s[:i])
			if err != nil {
				return 0, err
			}
			end, err := strconv.Atoi(s[i+1:])
			if err != nil {
				return 0, err
			}
			parts[0] = start
			parts[1] = end
			return 2, nil
		}
	}
	return 0, nil
}

// handleScanPubkeys scans the database for all pubkeys and populates event counts
// This is used to retroactively populate the unclassified users list
func (s *Server) handleScanPubkeys(dbACL *database.CuratingACL) NIP86Response {
	result, err := dbACL.ScanAllPubkeys()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to scan pubkeys: " + err.Error()}
	}

	return NIP86Response{Result: map[string]interface{}{
		"total_pubkeys": result.TotalPubkeys,
		"total_events":  result.TotalEvents,
		"skipped":       result.Skipped,
	}}
}
