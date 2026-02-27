package directory_client

import (
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/protocol/directory"
)

// EventCollector provides utility methods for collecting specific types of
// directory events from a slice.
type EventCollector struct {
	events []*event.E
}

// NewEventCollector creates a new event collector for the given events.
func NewEventCollector(events []*event.E) *EventCollector {
	return &EventCollector{events: events}
}

// RelayIdentities returns all relay identity declarations.
func (ec *EventCollector) RelayIdentities() (identities []*directory.RelayIdentityAnnouncement) {
	identities = make([]*directory.RelayIdentityAnnouncement, 0)
	for _, ev := range ec.events {
		if uint16(ev.Kind) == 39100 {
			if identity, err := directory.ParseRelayIdentityAnnouncement(ev); err == nil {
				identities = append(identities, identity)
			}
		}
	}
	return
}

// TrustActs returns all trust acts.
func (ec *EventCollector) TrustActs() (acts []*directory.TrustAct) {
	acts = make([]*directory.TrustAct, 0)
	for _, ev := range ec.events {
		if uint16(ev.Kind) == 39101 {
			if act, err := directory.ParseTrustAct(ev); err == nil {
				acts = append(acts, act)
			}
		}
	}
	return
}

// GroupTagActs returns all group tag acts.
func (ec *EventCollector) GroupTagActs() (acts []*directory.GroupTagAct) {
	acts = make([]*directory.GroupTagAct, 0)
	for _, ev := range ec.events {
		if uint16(ev.Kind) == 39102 {
			if act, err := directory.ParseGroupTagAct(ev); err == nil {
				acts = append(acts, act)
			}
		}
	}
	return
}

// PublicKeyAdvertisements returns all public key advertisements.
func (ec *EventCollector) PublicKeyAdvertisements() (ads []*directory.PublicKeyAdvertisement) {
	ads = make([]*directory.PublicKeyAdvertisement, 0)
	for _, ev := range ec.events {
		if uint16(ev.Kind) == 39103 {
			if ad, err := directory.ParsePublicKeyAdvertisement(ev); err == nil {
				ads = append(ads, ad)
			}
		}
	}
	return
}

// ReplicationRequests returns all replication requests.
func (ec *EventCollector) ReplicationRequests() (requests []*directory.DirectoryEventReplicationRequest) {
	requests = make([]*directory.DirectoryEventReplicationRequest, 0)
	for _, ev := range ec.events {
		if uint16(ev.Kind) == 39104 {
			if req, err := directory.ParseDirectoryEventReplicationRequest(ev); err == nil {
				requests = append(requests, req)
			}
		}
	}
	return
}

// ReplicationResponses returns all replication responses.
func (ec *EventCollector) ReplicationResponses() (responses []*directory.DirectoryEventReplicationResponse) {
	responses = make([]*directory.DirectoryEventReplicationResponse, 0)
	for _, ev := range ec.events {
		if uint16(ev.Kind) == 39105 {
			if resp, err := directory.ParseDirectoryEventReplicationResponse(ev); err == nil {
				responses = append(responses, resp)
			}
		}
	}
	return
}

// FindRelayIdentity finds a relay identity by relay URL.
func FindRelayIdentity(events []*event.E, relayURL string) (*directory.RelayIdentityAnnouncement, bool) {
	normalizedURL := NormalizeRelayURL(relayURL)
	for _, ev := range events {
		if uint16(ev.Kind) == 39100 {
			if identity, err := directory.ParseRelayIdentityAnnouncement(ev); err == nil {
				if NormalizeRelayURL(identity.RelayURL) == normalizedURL {
					return identity, true
				}
			}
		}
	}
	return nil, false
}

// FindTrustActsForRelay finds all trust acts targeting a specific relay.
func FindTrustActsForRelay(events []*event.E, targetPubkey string) (acts []*directory.TrustAct) {
	acts = make([]*directory.TrustAct, 0)
	for _, ev := range events {
		if uint16(ev.Kind) == 39101 {
			if act, err := directory.ParseTrustAct(ev); err == nil {
				if act.TargetPubkey == targetPubkey {
					acts = append(acts, act)
				}
			}
		}
	}
	return
}

// FindGroupTagActsForRelay finds all group tag acts targeting a specific relay.
// Note: This function needs to be updated based on the actual GroupTagAct structure
// which doesn't have a Target field. The filtering logic should be clarified.
func FindGroupTagActsForRelay(events []*event.E, targetPubkey string) (acts []*directory.GroupTagAct) {
	acts = make([]*directory.GroupTagAct, 0)
	for _, ev := range events {
		if uint16(ev.Kind) == 39102 {
			if act, err := directory.ParseGroupTagAct(ev); err == nil {
				// Filter by actor since GroupTagAct doesn't have a Target field
				if act.Actor == targetPubkey {
					acts = append(acts, act)
				}
			}
		}
	}
	return
}

// FindGroupTagActsByGroup finds all group tag acts for a specific group.
func FindGroupTagActsByGroup(events []*event.E, groupID string) (acts []*directory.GroupTagAct) {
	acts = make([]*directory.GroupTagAct, 0)
	for _, ev := range events {
		if uint16(ev.Kind) == 39102 {
			if act, err := directory.ParseGroupTagAct(ev); err == nil {
				if act.GroupID == groupID {
					acts = append(acts, act)
				}
			}
		}
	}
	return
}

// TrustGraph represents a directed graph of trust relationships.
type TrustGraph struct {
	// edges maps source pubkey -> list of trust acts
	edges map[string][]*directory.TrustAct
}

// NewTrustGraph creates a new trust graph instance.
func NewTrustGraph() *TrustGraph {
	return &TrustGraph{
		edges: make(map[string][]*directory.TrustAct),
	}
}

// AddTrustAct adds a trust act to the graph.
func (tg *TrustGraph) AddTrustAct(act *directory.TrustAct) {
	if act == nil {
		return
	}
	source := string(act.Event.Pubkey)
	tg.edges[source] = append(tg.edges[source], act)
}

// GetTrustActs returns all trust acts from a source pubkey.
func (tg *TrustGraph) GetTrustActs(source string) []*directory.TrustAct {
	return tg.edges[source]
}

// GetTrustedBy returns all pubkeys that trust the given target.
func (tg *TrustGraph) GetTrustedBy(target string) []string {
	trustedBy := make([]string, 0)
	for source, acts := range tg.edges {
		for _, act := range acts {
			if act.TargetPubkey == target {
				trustedBy = append(trustedBy, source)
				break
			}
		}
	}
	return trustedBy
}

// GetTrustTargets returns all pubkeys trusted by the given source.
func (tg *TrustGraph) GetTrustTargets(source string) []string {
	acts := tg.edges[source]
	targets := make(map[string]bool)
	for _, act := range acts {
		targets[act.TargetPubkey] = true
	}

	result := make([]string, 0, len(targets))
	for target := range targets {
		result = append(result, target)
	}
	return result
}

// BuildTrustGraph builds a trust graph from a collection of events.
func BuildTrustGraph(events []*event.E) *TrustGraph {
	graph := NewTrustGraph()
	for _, ev := range events {
		if uint16(ev.Kind) == 39101 {
			if act, err := directory.ParseTrustAct(ev); err == nil {
				graph.AddTrustAct(act)
			}
		}
	}
	return graph
}
