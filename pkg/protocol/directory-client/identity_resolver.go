package directory_client

import (
	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/protocol/directory"
)

// --- actor request/response types ---

type irProcessEventReq struct {
	ev *event.E
}

type irResolveIdentityReq struct {
	pubkey string
	resp   chan string
}

type irResolveEventIdentityReq struct {
	ev   *event.E
	resp chan string
}

type irIsDelegateKeyReq struct {
	pubkey string
	resp   chan bool
}

type irIsIdentityKeyReq struct {
	pubkey string
	resp   chan bool
}

type irGetDelegatesReq struct {
	identity string
	resp     chan []string
}

type irGetIdentityTagReq struct {
	delegate string
	resp     chan irGetIdentityTagResp
}

type irGetIdentityTagResp struct {
	tag *directory.IdentityTag
	err error
}

type irGetPubKeyAdsReq struct {
	identity string
	resp     chan []*directory.PublicKeyAdvertisement
}

type irGetPubKeyAdByIDReq struct {
	keyID string
	resp  chan irGetPubKeyAdByIDResp
}

type irGetPubKeyAdByIDResp struct {
	ad  *directory.PublicKeyAdvertisement
	err error
}

type irFilterEventsByIdentityReq struct {
	events   []*event.E
	identity string
	resp     chan []*event.E
}

type irClearCacheReq struct {
	resp chan struct{}
}

type irGetStatsReq struct {
	resp chan Stats
}

// IdentityResolver manages identity resolution and key delegation tracking.
//
// It maintains mappings between delegate keys and their primary identities,
// enabling clients to resolve the actual identity behind any signing key.
type IdentityResolver struct {
	// state - owned exclusively by actor goroutine
	delegateToIdentity  map[string]string
	identityToDelegates map[string]map[string]bool
	identityTagCache    map[string]*directory.IdentityTag
	publicKeyAds        map[string]*directory.PublicKeyAdvertisement

	// actor channels
	processEventCh          chan irProcessEventReq
	resolveIdentityCh       chan irResolveIdentityReq
	resolveEventIdentityCh  chan irResolveEventIdentityReq
	isDelegateKeyCh         chan irIsDelegateKeyReq
	isIdentityKeyCh         chan irIsIdentityKeyReq
	getDelegatesCh          chan irGetDelegatesReq
	getIdentityTagCh        chan irGetIdentityTagReq
	getPubKeyAdsCh          chan irGetPubKeyAdsReq
	getPubKeyAdByIDCh       chan irGetPubKeyAdByIDReq
	filterEventsByIdentityCh chan irFilterEventsByIdentityReq
	clearCacheCh            chan irClearCacheReq
	getStatsCh              chan irGetStatsReq

	stop chan struct{}
	done chan struct{}
}

// NewIdentityResolver creates a new identity resolver instance.
func NewIdentityResolver() *IdentityResolver {
	r := &IdentityResolver{
		delegateToIdentity:  make(map[string]string),
		identityToDelegates: make(map[string]map[string]bool),
		identityTagCache:    make(map[string]*directory.IdentityTag),
		publicKeyAds:        make(map[string]*directory.PublicKeyAdvertisement),

		processEventCh:          make(chan irProcessEventReq, 16),
		resolveIdentityCh:       make(chan irResolveIdentityReq),
		resolveEventIdentityCh:  make(chan irResolveEventIdentityReq),
		isDelegateKeyCh:         make(chan irIsDelegateKeyReq),
		isIdentityKeyCh:         make(chan irIsIdentityKeyReq),
		getDelegatesCh:          make(chan irGetDelegatesReq),
		getIdentityTagCh:        make(chan irGetIdentityTagReq),
		getPubKeyAdsCh:          make(chan irGetPubKeyAdsReq),
		getPubKeyAdByIDCh:       make(chan irGetPubKeyAdByIDReq),
		filterEventsByIdentityCh: make(chan irFilterEventsByIdentityReq),
		clearCacheCh:            make(chan irClearCacheReq),
		getStatsCh:              make(chan irGetStatsReq),

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go r.run()
	return r
}

// Stop shuts down the actor goroutine and waits for it to exit.
func (r *IdentityResolver) Stop() {
	close(r.stop)
	<-r.done
}

func (r *IdentityResolver) run() {
	defer close(r.done)
	for {
		select {
		case <-r.stop:
			return
		case req := <-r.processEventCh:
			r.handleProcessEvent(req.ev)
		case req := <-r.resolveIdentityCh:
			v := r.delegateToIdentity[req.pubkey]
			if v == "" {
				v = req.pubkey
			}
			req.resp <- v
		case req := <-r.resolveEventIdentityCh:
			result := ""
			if req.ev != nil {
				pubkey := string(req.ev.Pubkey)
				if identity, ok := r.delegateToIdentity[pubkey]; ok {
					result = identity
				} else {
					result = pubkey
				}
			}
			req.resp <- result
		case req := <-r.isDelegateKeyCh:
			_, ok := r.delegateToIdentity[req.pubkey]
			req.resp <- ok
		case req := <-r.isIdentityKeyCh:
			delegates, ok := r.identityToDelegates[req.pubkey]
			req.resp <- (ok && len(delegates) > 0)
		case req := <-r.getDelegatesCh:
			delegateMap, ok := r.identityToDelegates[req.identity]
			if !ok {
				req.resp <- []string{}
			} else {
				delegates := make([]string, 0, len(delegateMap))
				for d := range delegateMap {
					delegates = append(delegates, d)
				}
				req.resp <- delegates
			}
		case req := <-r.getIdentityTagCh:
			tag, ok := r.identityTagCache[req.delegate]
			if !ok {
				req.resp <- irGetIdentityTagResp{nil, errorf.E("identity tag not found for delegate: %s", req.delegate)}
			} else {
				req.resp <- irGetIdentityTagResp{tag, nil}
			}
		case req := <-r.getPubKeyAdsCh:
			req.resp <- r.handleGetPubKeyAds(req.identity)
		case req := <-r.getPubKeyAdByIDCh:
			keyAd, ok := r.publicKeyAds[req.keyID]
			if !ok {
				req.resp <- irGetPubKeyAdByIDResp{nil, errorf.E("public key advertisement not found: %s", req.keyID)}
			} else {
				req.resp <- irGetPubKeyAdByIDResp{keyAd, nil}
			}
		case req := <-r.filterEventsByIdentityCh:
			req.resp <- r.handleFilterEventsByIdentity(req.events, req.identity)
		case req := <-r.clearCacheCh:
			r.delegateToIdentity = make(map[string]string)
			r.identityToDelegates = make(map[string]map[string]bool)
			r.identityTagCache = make(map[string]*directory.IdentityTag)
			r.publicKeyAds = make(map[string]*directory.PublicKeyAdvertisement)
			req.resp <- struct{}{}
		case req := <-r.getStatsCh:
			req.resp <- Stats{
				Identities:   len(r.identityToDelegates),
				Delegates:    len(r.delegateToIdentity),
				PublicKeyAds: len(r.publicKeyAds),
			}
		}
	}
}

func (r *IdentityResolver) handleProcessEvent(ev *event.E) {
	if ev == nil {
		return
	}
	identityTag := extractIdentityTag(ev)
	if identityTag != nil {
		r.cacheIdentityTag(identityTag)
	}
	if uint16(ev.Kind) == 39103 {
		if keyAd, err := directory.ParsePublicKeyAdvertisement(ev); err == nil {
			r.publicKeyAds[keyAd.KeyID] = keyAd
		}
	}
}

func (r *IdentityResolver) cacheIdentityTag(tag *directory.IdentityTag) {
	if tag == nil {
		return
	}
	identity := tag.NPubIdentity
	delegate := identity
	r.delegateToIdentity[delegate] = identity
	if r.identityToDelegates[identity] == nil {
		r.identityToDelegates[identity] = make(map[string]bool)
	}
	r.identityToDelegates[identity][delegate] = true
	r.identityTagCache[delegate] = tag
}

func (r *IdentityResolver) handleGetPubKeyAds(identity string) []*directory.PublicKeyAdvertisement {
	delegates := r.identityToDelegates[identity]
	ads := make([]*directory.PublicKeyAdvertisement, 0)
	for _, keyAd := range r.publicKeyAds {
		adIdentity := r.delegateToIdentity[string(keyAd.Event.Pubkey)]
		if adIdentity == "" {
			adIdentity = string(keyAd.Event.Pubkey)
		}
		if adIdentity == identity {
			ads = append(ads, keyAd)
			continue
		}
		if delegates != nil && delegates[keyAd.PublicKey] {
			ads = append(ads, keyAd)
		}
	}
	return ads
}

func (r *IdentityResolver) handleFilterEventsByIdentity(events []*event.E, identity string) []*event.E {
	delegates := r.identityToDelegates[identity]
	filtered := make([]*event.E, 0)
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
	return filtered
}

// extractIdentityTag extracts an identity tag from an event if present.
func extractIdentityTag(ev *event.E) *directory.IdentityTag {
	if ev == nil || ev.Tags == nil {
		return nil
	}
	for _, t := range *ev.Tags {
		if t != nil && len(t.T) > 0 && string(t.T[0]) == "I" {
			if identityTag, err := directory.ParseIdentityTag(t); err == nil {
				return identityTag
			}
		}
	}
	return nil
}

// ProcessEvent processes an event to extract and cache identity information.
func (r *IdentityResolver) ProcessEvent(ev *event.E) {
	if ev == nil {
		return
	}
	select {
	case r.processEventCh <- irProcessEventReq{ev: ev}:
	case <-r.stop:
	}
}

// ResolveIdentity resolves the actual identity behind a public key.
func (r *IdentityResolver) ResolveIdentity(pubkey string) string {
	resp := make(chan string, 1)
	select {
	case r.resolveIdentityCh <- irResolveIdentityReq{pubkey: pubkey, resp: resp}:
		return <-resp
	case <-r.stop:
		return pubkey
	}
}

// ResolveEventIdentity resolves the actual identity behind an event's pubkey.
func (r *IdentityResolver) ResolveEventIdentity(ev *event.E) string {
	if ev == nil {
		return ""
	}
	resp := make(chan string, 1)
	select {
	case r.resolveEventIdentityCh <- irResolveEventIdentityReq{ev: ev, resp: resp}:
		return <-resp
	case <-r.stop:
		return ""
	}
}

// IsDelegateKey checks if a public key is a known delegate.
func (r *IdentityResolver) IsDelegateKey(pubkey string) bool {
	resp := make(chan bool, 1)
	select {
	case r.isDelegateKeyCh <- irIsDelegateKeyReq{pubkey: pubkey, resp: resp}:
		return <-resp
	case <-r.stop:
		return false
	}
}

// IsIdentityKey checks if a public key is a known identity (has delegates).
func (r *IdentityResolver) IsIdentityKey(pubkey string) bool {
	resp := make(chan bool, 1)
	select {
	case r.isIdentityKeyCh <- irIsIdentityKeyReq{pubkey: pubkey, resp: resp}:
		return <-resp
	case <-r.stop:
		return false
	}
}

// GetDelegatesForIdentity returns all delegate keys for a given identity.
func (r *IdentityResolver) GetDelegatesForIdentity(identity string) []string {
	resp := make(chan []string, 1)
	select {
	case r.getDelegatesCh <- irGetDelegatesReq{identity: identity, resp: resp}:
		return <-resp
	case <-r.stop:
		return []string{}
	}
}

// GetIdentityTag returns the identity tag for a delegate key.
func (r *IdentityResolver) GetIdentityTag(delegate string) (*directory.IdentityTag, error) {
	resp := make(chan irGetIdentityTagResp, 1)
	select {
	case r.getIdentityTagCh <- irGetIdentityTagReq{delegate: delegate, resp: resp}:
		r := <-resp
		return r.tag, r.err
	case <-r.stop:
		return nil, errorf.E("identity resolver stopped")
	}
}

// GetPublicKeyAdvertisements returns all public key advertisements for an identity.
func (r *IdentityResolver) GetPublicKeyAdvertisements(identity string) []*directory.PublicKeyAdvertisement {
	resp := make(chan []*directory.PublicKeyAdvertisement, 1)
	select {
	case r.getPubKeyAdsCh <- irGetPubKeyAdsReq{identity: identity, resp: resp}:
		return <-resp
	case <-r.stop:
		return nil
	}
}

// GetPublicKeyAdvertisementByID returns a public key advertisement by key ID.
func (r *IdentityResolver) GetPublicKeyAdvertisementByID(keyID string) (*directory.PublicKeyAdvertisement, error) {
	resp := make(chan irGetPubKeyAdByIDResp, 1)
	select {
	case r.getPubKeyAdByIDCh <- irGetPubKeyAdByIDReq{keyID: keyID, resp: resp}:
		r := <-resp
		return r.ad, r.err
	case <-r.stop:
		return nil, errorf.E("identity resolver stopped")
	}
}

// FilterEventsByIdentity filters events to only those signed by a specific identity or its delegates.
func (r *IdentityResolver) FilterEventsByIdentity(events []*event.E, identity string) []*event.E {
	resp := make(chan []*event.E, 1)
	select {
	case r.filterEventsByIdentityCh <- irFilterEventsByIdentityReq{events: events, identity: identity, resp: resp}:
		return <-resp
	case <-r.stop:
		return nil
	}
}

// ClearCache clears all cached identity mappings.
func (r *IdentityResolver) ClearCache() {
	resp := make(chan struct{}, 1)
	select {
	case r.clearCacheCh <- irClearCacheReq{resp: resp}:
		<-resp
	case <-r.stop:
	}
}

// Stats returns statistics about tracked identities and delegates.
type Stats struct {
	Identities   int // Number of primary identities
	Delegates    int // Number of delegate keys
	PublicKeyAds int // Number of public key advertisements
}

// GetStats returns statistics about the resolver's state.
func (r *IdentityResolver) GetStats() Stats {
	resp := make(chan Stats, 1)
	select {
	case r.getStatsCh <- irGetStatsReq{resp: resp}:
		return <-resp
	case <-r.stop:
		return Stats{}
	}
}
