package app

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/relayinfo"
	"next.orly.dev/pkg/version"
)

// GraphQueryConfig describes graph query capabilities for NIP-11 advertisement.
type GraphQueryConfig struct {
	Enabled    bool     `json:"enabled"`
	MaxDepth   int      `json:"max_depth"`
	MaxResults int      `json:"max_results"`
	Methods    []string `json:"methods"`
}

// ExtendedRelayInfo extends the standard NIP-11 relay info with additional fields.
// The Addresses field contains alternative WebSocket URLs for the relay (e.g., .onion).
type ExtendedRelayInfo struct {
	*relayinfo.T
	Addresses      []string          `json:"addresses,omitempty"`
	GraphQuery     *GraphQueryConfig `json:"graph_query,omitempty"`
	Theme          string            `json:"theme,omitempty"`
	BlossomEnabled bool              `json:"blossom_enabled,omitempty"`
}

// HandleRelayInfo generates and returns a relay information document in JSON
// format based on the server's configuration and supported NIPs.
//
// # Parameters
//
//   - w: HTTP response writer used to send the generated document.
//
//   - r: HTTP request object containing incoming client request data.
//
// # Expected Behaviour
//
// The function constructs a relay information document using either the
// Informer interface implementation or predefined server configuration. It
// returns this document as a JSON response to the client.
func (s *Server) HandleRelayInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Vary", "Accept")
	log.D.Ln("handling relay information document")
	var info *relayinfo.T
	nips := []relayinfo.NIP{
		relayinfo.BasicProtocol,
		relayinfo.Authentication,
		relayinfo.EncryptedDirectMessage,
		relayinfo.EventDeletion,
		relayinfo.RelayInformationDocument,
		relayinfo.GenericTagQueries,
		// relayinfo.NostrMarketplace,
		relayinfo.CountingResults,
		relayinfo.EventTreatment,
		relayinfo.CommandResults,
		relayinfo.ParameterizedReplaceableEvents,
		relayinfo.ExpirationTimestamp,
		relayinfo.ProtectedEvents,
		relayinfo.RelayListMetadata,
		relayinfo.SearchCapability,
	}
	// Add NIP-43 if enabled
	if s.Config.NIP43Enabled {
		nips = append(nips, relayinfo.RelayAccessMetadata)
	}
	// Add NIP-77 (negentropy) if enabled
	if s.Config.NegentropyEnabled {
		nips = append(nips, relayinfo.NIP{Number: 77, Description: "Negentropy-based sync"})
	}
	// Add NIP-86 (Relay Management API) if ACL mode supports it
	if s.Config.ACLMode == "managed" || s.Config.ACLMode == "curating" {
		nips = append(nips, relayinfo.NIP{Number: 86, Description: "Relay Management API"})
	}
	supportedNIPs := relayinfo.GetList(nips...)
	if s.Config.ACLMode != "none" {
		nipsACL := []relayinfo.NIP{
			relayinfo.BasicProtocol,
			relayinfo.Authentication,
			relayinfo.EncryptedDirectMessage,
			relayinfo.EventDeletion,
			relayinfo.RelayInformationDocument,
			relayinfo.GenericTagQueries,
			// relayinfo.NostrMarketplace,
			relayinfo.CountingResults,
			relayinfo.EventTreatment,
			relayinfo.CommandResults,
			relayinfo.ParameterizedReplaceableEvents,
			relayinfo.ExpirationTimestamp,
			relayinfo.ProtectedEvents,
			relayinfo.RelayListMetadata,
			relayinfo.SearchCapability,
		}
		// Add NIP-43 if enabled
		if s.Config.NIP43Enabled {
			nipsACL = append(nipsACL, relayinfo.RelayAccessMetadata)
		}
		// Add NIP-77 (negentropy) if enabled
		if s.Config.NegentropyEnabled {
			nipsACL = append(nipsACL, relayinfo.NIP{Number: 77, Description: "Negentropy-based sync"})
		}
		// Add NIP-86 (Relay Management API) if ACL mode supports it
		if s.Config.ACLMode == "managed" || s.Config.ACLMode == "curating" {
			nipsACL = append(nipsACL, relayinfo.NIP{Number: 86, Description: "Relay Management API"})
		}
		supportedNIPs = relayinfo.GetList(nipsACL...)
	}
	sort.Sort(supportedNIPs)
	log.I.Ln("supported NIPs", supportedNIPs)
	// Get relay identity pubkey as hex
	var relayPubkey string
	if skb, err := s.DB.GetRelayIdentitySecret(); err == nil && len(skb) == 32 {
		var sign *p8k.Signer
		var sigErr error
		if sign, sigErr = p8k.New(); sigErr == nil {
			if err := sign.InitSec(skb); err == nil {
				relayPubkey = hex.Enc(sign.Pub())
			}
		}
	}

	// Default relay info
	name := s.Config.AppName
	description := version.Description + " dashboard: " + s.DashboardURL(r)
	icon := "https://i.nostr.build/6wGXAn7Zaw9mHxFg.png"

	// Override with branding config if available
	if s.brandingMgr != nil {
		nip11 := s.brandingMgr.NIP11Config()
		if nip11.Name != "" {
			name = nip11.Name
		}
		if nip11.Description != "" {
			description = nip11.Description
		}
		if nip11.Icon != "" {
			icon = nip11.Icon
		}
	}

	// Override with managed ACL config if in managed mode
	if s.Config.ACLMode == "managed" {
		// Get managed ACL instance
		for _, aclInstance := range acl.Registry.ACLs() {
			if aclInstance.Type() == "managed" {
				if managed, ok := aclInstance.(*acl.Managed); ok {
					managedACL := managed.GetManagedACL()
					if managedACL != nil {
						if config, err := managedACL.GetRelayConfig(); err == nil {
							if config.RelayName != "" {
								name = config.RelayName
							}
							if config.RelayDescription != "" {
								description = config.RelayDescription
							}
							if config.RelayIcon != "" {
								icon = config.RelayIcon
							}
						}
					}
				}
				break
			}
		}
	}

	// Restricted writes applies when ACL mode is not managed/curating but also not none
	// (e.g., follows mode restricts writes to followed pubkeys)
	restrictedWrites := s.Config.ACLMode != "managed" && s.Config.ACLMode != "curating" && s.Config.ACLMode != "none"

	info = &relayinfo.T{
		Name:        name,
		Description: description,
		PubKey:      relayPubkey,
		Nips:        supportedNIPs,
		Software:    version.URL,
		Version:     strings.TrimPrefix(version.V, "v"),
		Limitation: relayinfo.Limits{
			AuthRequired:     s.Config.AuthRequired || s.Config.ACLMode != "none",
			RestrictedWrites: restrictedWrites,
			PaymentRequired:  s.Config.MonthlyPriceSats > 0,
		},
		Icon: icon,
	}

	// Build addresses list from config and Tor service
	var addresses []string

	// Add configured relay addresses
	if len(s.Config.RelayAddresses) > 0 {
		addresses = append(addresses, s.Config.RelayAddresses...)
	}

	// Add addresses from all transports (Tor .onion, etc.)
	if s.transportMgr != nil {
		addresses = append(addresses, s.transportMgr.Addresses()...)
	}

	// Build graph query config if enabled
	var graphConfig *GraphQueryConfig
	if s.graphExecutor != nil && s.Config.GraphQueriesEnabled {
		graphEnabled, maxDepth, maxResults, _ := s.Config.GetGraphConfigValues()
		if graphEnabled {
			graphConfig = &GraphQueryConfig{
				Enabled:    true,
				MaxDepth:   maxDepth,
				MaxResults: maxResults,
				Methods:    []string{"follows", "followers", "mentions", "thread"},
			}
		}
	}

	// Return extended info if we have addresses, graph query support, custom theme, or blossom
	theme := s.Config.Theme
	if theme != "auto" && theme != "light" && theme != "dark" {
		theme = "auto"
	}
	// Blossom is only available if the server is actually initialized (requires Badger backend)
	blossomEnabled := s.blossomServer != nil
	if len(addresses) > 0 || graphConfig != nil || theme != "auto" || blossomEnabled {
		extInfo := &ExtendedRelayInfo{
			T:              info,
			Addresses:      addresses,
			GraphQuery:     graphConfig,
			Theme:          theme,
			BlossomEnabled: blossomEnabled,
		}
		if err := json.NewEncoder(w).Encode(extInfo); chk.E(err) {
		}
	} else {
		if err := json.NewEncoder(w).Encode(info); chk.E(err) {
		}
	}
}
