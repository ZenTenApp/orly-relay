package app

import (
	"context"
	"time"

	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	hexenc "git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
)

const (
	// dmStrangerLimit is the max DMs a sender can send to a recipient who hasn't
	// replied yet. After this limit, further DMs are rejected until the recipient
	// sends a DM back (bidirectional = implicit whitelist).
	dmStrangerLimit = 3

	// dmBidirectionalCacheTTL is how long we cache the "has recipient replied" check.
	dmBidirectionalCacheTTL = 5 * time.Minute
)

// dmPairKey identifies a sender->recipient DM direction.
type dmPairKey struct {
	sender    string // hex pubkey
	recipient string // hex pubkey
}

// dmPairState tracks the state of a sender->recipient DM pair.
type dmPairState struct {
	count         int       // messages sent in this direction
	bidirectional bool      // true if recipient has replied (cached)
	checkedAt     time.Time // when bidirectional was last checked
}

// -- actor request types --

type dmCheckReq struct {
	ctx context.Context
	ev  *event.E
	resp chan dmCheckResp // buffered 1
}
type dmCheckResp struct {
	allowed bool
	reason  string
}
type dmIngestedReq struct {
	ev *event.E
}

// DMRateLimiter enforces a per-pair message limit for DMs to strangers.
// Once the recipient replies (bidirectional traffic), the limit is lifted.
// All state is owned by an actor goroutine.
type DMRateLimiter struct {
	db         database.Database
	checkCh    chan dmCheckReq
	ingestedCh chan dmIngestedReq
	stop       chan struct{}
	done       chan struct{}
}

// NewDMRateLimiter creates a new DM rate limiter.
func NewDMRateLimiter(db database.Database) *DMRateLimiter {
	r := &DMRateLimiter{
		db:         db,
		checkCh:    make(chan dmCheckReq),
		ingestedCh: make(chan dmIngestedReq, 16),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	go r.actor()
	return r
}

func (r *DMRateLimiter) actor() {
	defer close(r.done)
	pairs := make(map[dmPairKey]*dmPairState)
	for {
		select {
		case req := <-r.checkCh:
			allowed, reason := r.doCheck(req.ctx, req.ev, pairs)
			req.resp <- dmCheckResp{allowed: allowed, reason: reason}
		case req := <-r.ingestedCh:
			r.doIngested(req.ev, pairs)
		case <-r.stop:
			return
		}
	}
}

func (r *DMRateLimiter) doCheck(ctx context.Context, ev *event.E, pairs map[dmPairKey]*dmPairState) (bool, string) {
	// Only apply to DM kinds
	if ev.Kind != kind.EncryptedDirectMessage.K && ev.Kind != kind.GiftWrap.K {
		return true, ""
	}

	senderHex := hexenc.Enc(ev.Pubkey)

	// Extract recipient from #p tag
	recipientHex := extractRecipient(ev)
	if recipientHex == "" {
		return true, ""
	}

	// Same person - allow
	if senderHex == recipientHex {
		return true, ""
	}

	key := dmPairKey{sender: senderHex, recipient: recipientHex}

	state, exists := pairs[key]
	if !exists {
		state = &dmPairState{}
		pairs[key] = state
	}

	// Check if this pair is bidirectional (recipient has replied)
	if state.bidirectional && time.Since(state.checkedAt) < dmBidirectionalCacheTTL {
		return true, ""
	}

	// Check DB for bidirectional traffic
	if r.checkBidirectional(ctx, recipientHex, senderHex) {
		state.bidirectional = true
		state.checkedAt = time.Now()
		reverseKey := dmPairKey{sender: recipientHex, recipient: senderHex}
		if reverseState, ok := pairs[reverseKey]; ok {
			reverseState.bidirectional = true
			reverseState.checkedAt = time.Now()
		}
		return true, ""
	}

	state.checkedAt = time.Now()

	// Enforce stranger limit
	state.count++
	if state.count > dmStrangerLimit {
		log.D.F("DM rate limit: %s -> %s rejected (count=%d, limit=%d)",
			senderHex[:12], recipientHex[:12], state.count, dmStrangerLimit)
		return false, "restricted: DM limit reached, recipient has not accepted your messages"
	}

	return true, ""
}

func (r *DMRateLimiter) doIngested(ev *event.E, pairs map[dmPairKey]*dmPairState) {
	if ev.Kind != kind.EncryptedDirectMessage.K && ev.Kind != kind.GiftWrap.K {
		return
	}

	senderHex := hexenc.Enc(ev.Pubkey)
	recipientHex := extractRecipient(ev)
	if recipientHex == "" || senderHex == recipientHex {
		return
	}

	// If someone sends a reply, mark the reverse direction as bidirectional
	reverseKey := dmPairKey{sender: recipientHex, recipient: senderHex}
	if reverseState, ok := pairs[reverseKey]; ok {
		reverseState.bidirectional = true
		reverseState.checkedAt = time.Now()
	}

	// Also mark the forward direction
	forwardKey := dmPairKey{sender: senderHex, recipient: recipientHex}
	if forwardState, ok := pairs[forwardKey]; ok {
		forwardState.bidirectional = true
		forwardState.checkedAt = time.Now()
	}
}

// CheckDM checks whether a DM event should be allowed. Returns true if allowed,
// false with a reason message if rejected.
func (r *DMRateLimiter) CheckDM(ctx context.Context, ev *event.E) (allowed bool, reason string) {
	resp := make(chan dmCheckResp, 1)
	r.checkCh <- dmCheckReq{ctx: ctx, ev: ev, resp: resp}
	result := <-resp
	return result.allowed, result.reason
}

// OnDMIngested should be called after a DM is successfully saved.
func (r *DMRateLimiter) OnDMIngested(ev *event.E) {
	r.ingestedCh <- dmIngestedReq{ev: ev}
}

// checkBidirectional queries the DB to see if the recipient has ever sent a DM
// to the sender.
func (r *DMRateLimiter) checkBidirectional(ctx context.Context, recipientHex, senderHex string) bool {
	recipientBytes, err := hexenc.Dec(recipientHex)
	if err != nil {
		return false
	}
	senderBytes, err := hexenc.Dec(senderHex)
	if err != nil {
		return false
	}

	// Query for kind 4 from recipient to sender
	for _, k := range []*kind.K{kind.EncryptedDirectMessage, kind.GiftWrap} {
		f := filter.New()
		f.Kinds = kind.NewS(k)
		f.Authors = tag.NewFromBytesSlice(recipientBytes)

		// Filter by #p tag = sender
		pTag := tag.NewFromBytesSlice([]byte("p"), senderBytes)
		f.Tags = tag.NewSWithCap(1)
		*f.Tags = append(*f.Tags, pTag)

		limit := uint(1)
		f.Limit = &limit

		queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		events, qErr := r.db.QueryEvents(queryCtx, f)
		cancel()

		if chk.E(qErr) {
			continue
		}
		if len(events) > 0 {
			return true
		}
	}

	return false
}

// extractRecipient gets the recipient pubkey (hex) from an event's first #p tag.
func extractRecipient(ev *event.E) string {
	if ev.Tags == nil {
		return ""
	}
	pTags := ev.Tags.GetAll([]byte("p"))
	for _, pt := range pTags {
		if pt.Len() >= 2 {
			val := pt.ValueHex()
			if len(val) > 0 {
				return string(val)
			}
		}
	}
	return ""
}
