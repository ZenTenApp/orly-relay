package directory_client

import (
	"sync"

	"next.orly.dev/pkg/lol/errorf"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/protocol/directory"
)

// IdentityResolver manages identity resolution and key delegation tracking.
//
// It maintains mappings between delegate keys and their primary identities,
// enabling clients to resolve the actual identity behind any signing key.
type IdentityResolver struct {
	mu sync.RWMutex

	// delegateToIdentity maps delegate public keys to their primary identity
	delegateToIdentity map[string]string

	// identityToDelegates maps primary identities to their delegate keys
	identityToDelegates map[string]map[string]bool

	// identityTagCache stores full identity tags by delegate key
	identityTagCache map[string]*directory.IdentityTag

	// publicKeyAds stores public key advertisements by key ID
	publicKeyAds map[string]*directory.PublicKeyAdvertisement
}

// NewIdentityResolver creates a new identity resolver instance.
func NewIdentityResolver() *IdentityResolver {
	return &IdentityResolver{
		delegateToIdentity:  make(map[string]string),
		identityToDelegates: make(map[string]map[string]bool),
		identityTagCache:    make(map[string]*directory.IdentityTag),
		publicKeyAds:        make(map[string]*directory.PublicKeyAdvertisement),
	}
}

// ProcessEvent processes an event to extract and cache identity information.
//
// This should be called for all directory events to keep the resolver's
// internal state up to date.
func (r *IdentityResolver) ProcessEvent(ev *event.E) {
	if ev == nil {
		return
	}

	// Try to parse identity tag (I tag)
	identityTag := extractIdentityTag(ev)
	if identityTag != nil {
		r.cacheIdentityTag(identityTag)
	}

	// Handle public key advertisements specially
	if uint16(ev.Kind) == 39103 {
		if keyAd, err := directory.ParsePublicKeyAdvertisement(ev); err == nil {
			r.mu.Lock()
			r.publicKeyAds[keyAd.KeyID] = keyAd
			r.mu.Unlock()
		}
	}
}

// extractIdentityTag extracts an identity tag from an event if present.
func extractIdentityTag(ev *event.E) *directory.IdentityTag {
	if ev == nil || ev.Tags == nil {
		return nil
	}

	// Find the I tag
	for _, t := range *ev.Tags {
		if t != nil && len(t.T) > 0 && string(t.T[0]) == "I" {
			if identityTag, err := directory.ParseIdentityTag(t); err == nil {
				return identityTag
			}
		}
	}
	return nil
}

// cacheIdentityTag caches an identity tag mapping.
func (r *IdentityResolver) cacheIdentityTag(tag *directory.IdentityTag) {
	if tag == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	identity := tag.NPubIdentity
	// For now, we use the identity as the delegate too since the structure is different
	// This should be updated when the IdentityTag structure is clarified
	delegate := identity

	// Store delegate -> identity mapping
	r.delegateToIdentity[delegate] = identity

	// Store identity -> delegates mapping
	if r.identityToDelegates[identity] == nil {
		r.identityToDelegates[identity] = make(map[string]bool)
	}
	r.identityToDelegates[identity][delegate] = true

	// Cache the full tag
	r.identityTagCache[delegate] = tag
}

// ResolveIdentity resolves the actual identity behind a public key.
//
// If the public key is a delegate, it returns the primary identity.
// If the public key is already an identity, it returns the input unchanged.
func (r *IdentityResolver) ResolveIdentity(pubkey string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if identity, ok := r.delegateToIdentity[pubkey]; ok {
		return identity
	}
	return pubkey
}

// ResolveEventIdentity resolves the actual identity behind an event's pubkey.
func (r *IdentityResolver) ResolveEventIdentity(ev *event.E) string {
	if ev == nil {
		return ""
	}
	return r.ResolveIdentity(string(ev.Pubkey))
}

// IsDelegateKey checks if a public key is a known delegate.
func (r *IdentityResolver) IsDelegateKey(pubkey string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.delegateToIdentity[pubkey]
	return ok
}

// IsIdentityKey checks if a public key is a known identity (has delegates).
func (r *IdentityResolver) IsIdentityKey(pubkey string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	delegates, ok := r.identityToDelegates[pubkey]
	return ok && len(delegates) > 0
}

// GetDelegatesForIdentity returns all delegate keys for a given identity.
func (r *IdentityResolver) GetDelegatesForIdentity(identity string) (delegates []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	delegateMap, ok := r.identityToDelegates[identity]
	if !ok {
		return []string{}
	}

	delegates = make([]string, 0, len(delegateMap))
	for delegate := range delegateMap {
		delegates = append(delegates, delegate)
	}
	return
}

// GetIdentityTag returns the identity tag for a delegate key.
func (r *IdentityResolver) GetIdentityTag(delegate string) (*directory.IdentityTag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tag, ok := r.identityTagCache[delegate]
	if !ok {
		return nil, errorf.E("identity tag not found for delegate: %s", delegate)
	}
	return tag, nil
}

// GetPublicKeyAdvertisements returns all public key advertisements for an identity.
func (r *IdentityResolver) GetPublicKeyAdvertisements(identity string) (ads []*directory.PublicKeyAdvertisement) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	delegates := r.identityToDelegates[identity]
	ads = make([]*directory.PublicKeyAdvertisement, 0)

	for _, keyAd := range r.publicKeyAds {
		adIdentity := r.delegateToIdentity[string(keyAd.Event.Pubkey)]
		if adIdentity == "" {
			adIdentity = string(keyAd.Event.Pubkey)
		}

		if adIdentity == identity {
			ads = append(ads, keyAd)
			continue
		}

		// Check if the advertised key is a delegate
		if delegates != nil && delegates[keyAd.PublicKey] {
			ads = append(ads, keyAd)
		}
	}

	return
}

// GetPublicKeyAdvertisementByID returns a public key advertisement by key ID.
func (r *IdentityResolver) GetPublicKeyAdvertisementByID(keyID string) (*directory.PublicKeyAdvertisement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keyAd, ok := r.publicKeyAds[keyID]
	if !ok {
		return nil, errorf.E("public key advertisement not found: %s", keyID)
	}
	return keyAd, nil
}

// FilterEventsByIdentity filters events to only those signed by a specific identity or its delegates.
func (r *IdentityResolver) FilterEventsByIdentity(events []*event.E, identity string) (filtered []*event.E) {
	r.mu.RLock()
	delegates := r.identityToDelegates[identity]
	r.mu.RUnlock()

	filtered = make([]*event.E, 0)
	for _, ev := range events {
		pubkey := string(ev.Pubkey)
		if pubkey == identity {
			filtered = append(filtered, ev)
			continue
		}
		if delegates != nil && delegates[pubkey] {
			filtered = append(filtered, ev)
		}
	}

	return
}

// ClearCache clears all cached identity mappings.
func (r *IdentityResolver) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.delegateToIdentity = make(map[string]string)
	r.identityToDelegates = make(map[string]map[string]bool)
	r.identityTagCache = make(map[string]*directory.IdentityTag)
	r.publicKeyAds = make(map[string]*directory.PublicKeyAdvertisement)
}

// Stats returns statistics about tracked identities and delegates.
type Stats struct {
	Identities   int // Number of primary identities
	Delegates    int // Number of delegate keys
	PublicKeyAds int // Number of public key advertisements
}

// GetStats returns statistics about the resolver's state.
func (r *IdentityResolver) GetStats() Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return Stats{
		Identities:   len(r.identityToDelegates),
		Delegates:    len(r.delegateToIdentity),
		PublicKeyAds: len(r.publicKeyAds),
	}
}
