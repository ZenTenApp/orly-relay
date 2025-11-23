package app

import (
	"encoding/json"
	"io"
	"net/http"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/httpauth"
)

// NIP86Request represents a NIP-86 JSON-RPC request
type NIP86Request struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

// NIP86Response represents a NIP-86 JSON-RPC response
type NIP86Response struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// handleNIP86Management handles NIP-86 management API requests
func (s *Server) handleNIP86Management(w http.ResponseWriter, r *http.Request) {
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

	// Check permissions - require owner level only
	accessLevel := acl.Registry.GetAccessLevel(pubkey, r.RemoteAddr)
	if accessLevel != "owner" {
		http.Error(w, "Owner permission required", http.StatusForbidden)
		return
	}

	// Check if managed ACL is active
	if acl.Registry.Type() != "managed" {
		http.Error(w, "Managed ACL mode is not active", http.StatusBadRequest)
		return
	}

	// Get the managed ACL instance
	var managedACL *database.ManagedACL
	for _, aclInstance := range acl.Registry.ACL {
		if aclInstance.Type() == "managed" {
			if managed, ok := aclInstance.(*acl.Managed); ok {
				managedACL = managed.GetManagedACL()
				break
			}
		}
	}

	if managedACL == nil {
		http.Error(w, "Managed ACL not available", http.StatusInternalServerError)
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
	response := s.handleNIP86Method(request, managedACL)

	// Send response
	jsonData, err := json.Marshal(response)
	if chk.E(err) {
		http.Error(w, "Error generating response", http.StatusInternalServerError)
		return
	}

	w.Write(jsonData)
}

// handleNIP86Method handles individual NIP-86 methods
func (s *Server) handleNIP86Method(request NIP86Request, managedACL *database.ManagedACL) NIP86Response {
	switch request.Method {
	case "supportedmethods":
		return s.handleSupportedMethods()
	case "banpubkey":
		return s.handleBanPubkey(request.Params, managedACL)
	case "listbannedpubkeys":
		return s.handleListBannedPubkeys(managedACL)
	case "allowpubkey":
		return s.handleAllowPubkey(request.Params, managedACL)
	case "listallowedpubkeys":
		return s.handleListAllowedPubkeys(managedACL)
	case "listeventsneedingmoderation":
		return s.handleListEventsNeedingModeration(managedACL)
	case "allowevent":
		return s.handleAllowEvent(request.Params, managedACL)
	case "banevent":
		return s.handleBanEvent(request.Params, managedACL)
	case "listbannedevents":
		return s.handleListBannedEvents(managedACL)
	case "changerelayname":
		return s.handleChangeRelayName(request.Params, managedACL)
	case "changerelaydescription":
		return s.handleChangeRelayDescription(request.Params, managedACL)
	case "changerelayicon":
		return s.handleChangeRelayIcon(request.Params, managedACL)
	case "allowkind":
		return s.handleAllowKind(request.Params, managedACL)
	case "disallowkind":
		return s.handleDisallowKind(request.Params, managedACL)
	case "listallowedkinds":
		return s.handleListAllowedKinds(managedACL)
	case "blockip":
		return s.handleBlockIP(request.Params, managedACL)
	case "unblockip":
		return s.handleUnblockIP(request.Params, managedACL)
	case "listblockedips":
		return s.handleListBlockedIPs(managedACL)
	default:
		return NIP86Response{Error: "Unknown method: " + request.Method}
	}
}

// handleSupportedMethods returns the list of supported methods
func (s *Server) handleSupportedMethods() NIP86Response {
	methods := []string{
		"supportedmethods",
		"banpubkey",
		"listbannedpubkeys",
		"allowpubkey",
		"listallowedpubkeys",
		"listeventsneedingmoderation",
		"allowevent",
		"banevent",
		"listbannedevents",
		"changerelayname",
		"changerelaydescription",
		"changerelayicon",
		"allowkind",
		"disallowkind",
		"listallowedkinds",
		"blockip",
		"unblockip",
		"listblockedips",
	}
	return NIP86Response{Result: methods}
}

// handleBanPubkey bans a public key
func (s *Server) handleBanPubkey(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: pubkey"}
	}

	pubkey, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid pubkey parameter"}
	}

	// Validate pubkey format
	if len(pubkey) != 64 {
		return NIP86Response{Error: "Invalid pubkey format"}
	}

	reason := ""
	if len(params) > 1 {
		if r, ok := params[1].(string); ok {
			reason = r
		}
	}

	if err := managedACL.SaveBannedPubkey(pubkey, reason); chk.E(err) {
		return NIP86Response{Error: "Failed to ban pubkey: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleListBannedPubkeys returns the list of banned pubkeys
func (s *Server) handleListBannedPubkeys(managedACL *database.ManagedACL) NIP86Response {
	banned, err := managedACL.ListBannedPubkeys()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list banned pubkeys: " + err.Error()}
	}

	// Convert to the expected format
	result := make([]map[string]interface{}, len(banned))
	for i, b := range banned {
		result[i] = map[string]interface{}{
			"pubkey": b.Pubkey,
			"reason": b.Reason,
		}
	}

	return NIP86Response{Result: result}
}

// handleAllowPubkey allows a public key
func (s *Server) handleAllowPubkey(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: pubkey"}
	}

	pubkey, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid pubkey parameter"}
	}

	// Validate pubkey format
	if len(pubkey) != 64 {
		return NIP86Response{Error: "Invalid pubkey format"}
	}

	reason := ""
	if len(params) > 1 {
		if r, ok := params[1].(string); ok {
			reason = r
		}
	}

	if err := managedACL.SaveAllowedPubkey(pubkey, reason); chk.E(err) {
		return NIP86Response{Error: "Failed to allow pubkey: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleListAllowedPubkeys returns the list of allowed pubkeys
func (s *Server) handleListAllowedPubkeys(managedACL *database.ManagedACL) NIP86Response {
	allowed, err := managedACL.ListAllowedPubkeys()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list allowed pubkeys: " + err.Error()}
	}

	// Convert to the expected format
	result := make([]map[string]interface{}, len(allowed))
	for i, a := range allowed {
		result[i] = map[string]interface{}{
			"pubkey": a.Pubkey,
			"reason": a.Reason,
		}
	}

	return NIP86Response{Result: result}
}

// handleListEventsNeedingModeration returns events needing moderation
func (s *Server) handleListEventsNeedingModeration(managedACL *database.ManagedACL) NIP86Response {
	events, err := managedACL.ListEventsNeedingModeration()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list events needing moderation: " + err.Error()}
	}

	// Convert to the expected format
	result := make([]map[string]interface{}, len(events))
	for i, e := range events {
		result[i] = map[string]interface{}{
			"id":     e.ID,
			"reason": e.Reason,
		}
	}

	return NIP86Response{Result: result}
}

// handleAllowEvent allows an event
func (s *Server) handleAllowEvent(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: event_id"}
	}

	eventID, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid event_id parameter"}
	}

	// Validate event ID format
	if len(eventID) != 64 {
		return NIP86Response{Error: "Invalid event_id format"}
	}

	reason := ""
	if len(params) > 1 {
		if r, ok := params[1].(string); ok {
			reason = r
		}
	}

	if err := managedACL.SaveAllowedEvent(eventID, reason); chk.E(err) {
		return NIP86Response{Error: "Failed to allow event: " + err.Error()}
	}

	// Remove from moderation queue if it was there
	managedACL.RemoveEventNeedingModeration(eventID)

	return NIP86Response{Result: true}
}

// handleBanEvent bans an event
func (s *Server) handleBanEvent(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: event_id"}
	}

	eventID, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid event_id parameter"}
	}

	// Validate event ID format
	if len(eventID) != 64 {
		return NIP86Response{Error: "Invalid event_id format"}
	}

	reason := ""
	if len(params) > 1 {
		if r, ok := params[1].(string); ok {
			reason = r
		}
	}

	if err := managedACL.SaveBannedEvent(eventID, reason); chk.E(err) {
		return NIP86Response{Error: "Failed to ban event: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleListBannedEvents returns the list of banned events
func (s *Server) handleListBannedEvents(managedACL *database.ManagedACL) NIP86Response {
	banned, err := managedACL.ListBannedEvents()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list banned events: " + err.Error()}
	}

	// Convert to the expected format
	result := make([]map[string]interface{}, len(banned))
	for i, b := range banned {
		result[i] = map[string]interface{}{
			"id":     b.ID,
			"reason": b.Reason,
		}
	}

	return NIP86Response{Result: result}
}

// handleChangeRelayName changes the relay name
func (s *Server) handleChangeRelayName(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: name"}
	}

	name, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid name parameter"}
	}

	config, err := managedACL.GetRelayConfig()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to get relay config: " + err.Error()}
	}

	config.RelayName = name
	if err := managedACL.SaveRelayConfig(config); chk.E(err) {
		return NIP86Response{Error: "Failed to save relay config: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleChangeRelayDescription changes the relay description
func (s *Server) handleChangeRelayDescription(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: description"}
	}

	description, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid description parameter"}
	}

	config, err := managedACL.GetRelayConfig()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to get relay config: " + err.Error()}
	}

	config.RelayDescription = description
	if err := managedACL.SaveRelayConfig(config); chk.E(err) {
		return NIP86Response{Error: "Failed to save relay config: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleChangeRelayIcon changes the relay icon
func (s *Server) handleChangeRelayIcon(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: icon_url"}
	}

	iconURL, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid icon_url parameter"}
	}

	config, err := managedACL.GetRelayConfig()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to get relay config: " + err.Error()}
	}

	config.RelayIcon = iconURL
	if err := managedACL.SaveRelayConfig(config); chk.E(err) {
		return NIP86Response{Error: "Failed to save relay config: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleAllowKind allows an event kind
func (s *Server) handleAllowKind(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: kind"}
	}

	kindFloat, ok := params[0].(float64)
	if !ok {
		return NIP86Response{Error: "Invalid kind parameter"}
	}

	kind := int(kindFloat)
	if err := managedACL.SaveAllowedKind(kind); chk.E(err) {
		return NIP86Response{Error: "Failed to allow kind: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleDisallowKind disallows an event kind
func (s *Server) handleDisallowKind(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: kind"}
	}

	kindFloat, ok := params[0].(float64)
	if !ok {
		return NIP86Response{Error: "Invalid kind parameter"}
	}

	kind := int(kindFloat)
	if err := managedACL.RemoveAllowedKind(kind); chk.E(err) {
		return NIP86Response{Error: "Failed to disallow kind: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleListAllowedKinds returns the list of allowed kinds
func (s *Server) handleListAllowedKinds(managedACL *database.ManagedACL) NIP86Response {
	kinds, err := managedACL.ListAllowedKinds()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list allowed kinds: " + err.Error()}
	}

	return NIP86Response{Result: kinds}
}

// handleBlockIP blocks an IP address
func (s *Server) handleBlockIP(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: ip"}
	}

	ip, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid ip parameter"}
	}

	reason := ""
	if len(params) > 1 {
		if r, ok := params[1].(string); ok {
			reason = r
		}
	}

	if err := managedACL.SaveBlockedIP(ip, reason); chk.E(err) {
		return NIP86Response{Error: "Failed to block IP: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleUnblockIP unblocks an IP address
func (s *Server) handleUnblockIP(params []interface{}, managedACL *database.ManagedACL) NIP86Response {
	if len(params) < 1 {
		return NIP86Response{Error: "Missing required parameter: ip"}
	}

	ip, ok := params[0].(string)
	if !ok {
		return NIP86Response{Error: "Invalid ip parameter"}
	}

	if err := managedACL.RemoveBlockedIP(ip); chk.E(err) {
		return NIP86Response{Error: "Failed to unblock IP: " + err.Error()}
	}

	return NIP86Response{Result: true}
}

// handleListBlockedIPs returns the list of blocked IPs
func (s *Server) handleListBlockedIPs(managedACL *database.ManagedACL) NIP86Response {
	blocked, err := managedACL.ListBlockedIPs()
	if chk.E(err) {
		return NIP86Response{Error: "Failed to list blocked IPs: " + err.Error()}
	}

	// Convert to the expected format
	result := make([]map[string]interface{}, len(blocked))
	for i, b := range blocked {
		result[i] = map[string]interface{}{
			"ip":     b.IP,
			"reason": b.Reason,
		}
	}

	return NIP86Response{Result: result}
}
