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
	r.Header.Set("Content-Type", "application/json")
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

	// Override with managed ACL config if in managed mode
	if s.Config.ACLMode == "managed" {
		// Get managed ACL instance
		for _, aclInstance := range acl.Registry.ACL {
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

	info = &relayinfo.T{
		Name:        name,
		Description: description,
		PubKey:      relayPubkey,
		Nips:        supportedNIPs,
		Software:    version.URL,
		Version:     strings.TrimPrefix(version.V, "v"),
		Limitation: relayinfo.Limits{
			AuthRequired:     s.Config.AuthRequired || s.Config.ACLMode != "none",
			RestrictedWrites: s.Config.ACLMode != "managed" && s.Config.ACLMode != "none",
			PaymentRequired:  s.Config.MonthlyPriceSats > 0,
		},
		Icon: icon,
	}
	if err := json.NewEncoder(w).Encode(info); chk.E(err) {
	}
}
