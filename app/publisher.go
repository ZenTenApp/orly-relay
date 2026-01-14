package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"next.orly.dev/pkg/interfaces/publisher"
	"next.orly.dev/pkg/interfaces/typer"
	"next.orly.dev/pkg/policy"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/utils"
)

const Type = "socketapi"

// WriteChanMap maps websocket connections to their write channels
type WriteChanMap map[*websocket.Conn]chan publish.WriteRequest

type Subscription struct {
	remote       string
	AuthedPubkey []byte
	Receiver     event.C // Channel for delivering events to this subscription
	AuthRequired bool    // Whether ACL requires authentication for privileged events
	*filter.S
}

// Map is a map of filters associated with a collection of ws.Listener
// connections.
type Map map[*websocket.Conn]map[string]Subscription

type W struct {
	*websocket.Conn

	remote string

	// If Cancel is true, this is a close command.
	Cancel bool

	// Id is the subscription Id. If Cancel is true, cancel the named
	// subscription, otherwise, cancel the publisher for the socket.
	Id string

	// The Receiver holds the event channel for receiving notifications or data
	// relevant to this WebSocket connection.
	Receiver event.C

	// Filters holds a collection of filters used to match or process events
	// associated with this WebSocket connection. It is used to determine which
	// notifications or data should be received by the subscriber.
	Filters *filter.S

	// AuthedPubkey is the authenticated pubkey associated with the listener (if any).
	AuthedPubkey []byte

	// AuthRequired indicates whether the ACL in operation requires auth. If
	// this is set to true, the publisher will not publish privileged or other
	// restricted events to non-authed listeners, otherwise, it will.
	AuthRequired bool
}

func (w *W) Type() (typeName string) { return Type }

// P is a structure that manages subscriptions and associated filters for
// websocket listeners. It uses a mutex to synchronize access to a map storing
// subscriber connections and their filter configurations.
type P struct {
	c context.Context
	// Mx is the mutex for the Map.
	Mx sync.RWMutex
	// Map is the map of subscribers and subscriptions from the websocket api.
	Map
	// WriteChans maps websocket connections to their write channels
	WriteChans WriteChanMap
}

var _ publisher.I = &P{}

func NewPublisher(c context.Context) (publisher *P) {
	return &P{
		c:          c,
		Map:        make(Map),
		WriteChans: make(WriteChanMap, 100),
	}
}

func (p *P) Type() (typeName string) { return Type }

// Receive handles incoming messages to manage websocket listener subscriptions
// and associated filters.
//
// # Parameters
//
// - msg (publisher.Message): The incoming message to process; expected to be of
// type *W to trigger subscription management actions.
//
// # Expected behaviour
//
// - Checks if the message is of type *W.
//
// - If Cancel is true, removes a subscriber by ID or the entire listener.
//
// - Otherwise, adds the subscription to the map under a mutex lock.
//
// - Logs actions related to subscription creation or removal.
func (p *P) Receive(msg typer.T) {
	if m, ok := msg.(*W); ok {
		if m.Cancel {
			if m.Id == "" {
				p.removeSubscriber(m.Conn)
			} else {
				p.removeSubscriberId(m.Conn, m.Id)
			}
			return
		}
		p.Mx.Lock()
		defer p.Mx.Unlock()
		if subs, ok := p.Map[m.Conn]; !ok {
			subs = make(map[string]Subscription)
			subs[m.Id] = Subscription{
				S: m.Filters, remote: m.remote, AuthedPubkey: m.AuthedPubkey,
				Receiver: m.Receiver, AuthRequired: m.AuthRequired,
			}
			p.Map[m.Conn] = subs
		} else {
			subs[m.Id] = Subscription{
				S: m.Filters, remote: m.remote, AuthedPubkey: m.AuthedPubkey,
				Receiver: m.Receiver, AuthRequired: m.AuthRequired,
			}
		}
	}
}

// Deliver processes and distributes an event to all matching subscribers based on their filter configurations.
//
// # Parameters
//
// - ev (*event.E): The event to be delivered to subscribed clients.
//
// # Expected behaviour
//
// Delivers the event to all subscribers whose filters match the event. It
// applies authentication checks if required by the server and skips delivery
// for unauthenticated users when events are privileged.
func (p *P) Deliver(ev *event.E) {
	// Snapshot the deliveries under read lock to avoid holding locks during I/O
	p.Mx.RLock()
	type delivery struct {
		w   *websocket.Conn
		id  string
		sub Subscription
	}
	var deliveries []delivery
	// Debug: log ephemeral event delivery attempts
	isEphemeral := ev.Kind >= 20000 && ev.Kind < 30000
	if isEphemeral {
		var tagInfo string
		if ev.Tags != nil {
			tagInfo = string(ev.Tags.Marshal(nil))
		}
		log.I.F("ephemeral event kind %d, id %0x, checking %d connections for matches, tags: %s",
			ev.Kind, ev.ID[:8], len(p.Map), tagInfo)
	}
	for w, subs := range p.Map {
		for id, subscriber := range subs {
			if subscriber.Match(ev) {
				deliveries = append(
					deliveries, delivery{w: w, id: id, sub: subscriber},
				)
			} else if isEphemeral {
				// Debug: log why ephemeral events don't match
				log.I.F("ephemeral event kind %d did NOT match subscription %s (filters: %s)",
					ev.Kind, id, string(subscriber.S.Marshal(nil)))
			}
		}
	}
	p.Mx.RUnlock()
	if len(deliveries) > 0 {
		log.D.C(
			func() string {
				return fmt.Sprintf(
					"delivering event %0x to websocket subscribers %d", ev.ID,
					len(deliveries),
				)
			},
		)
	}
	for _, d := range deliveries {
		// If the event is privileged, enforce that the subscriber's authed pubkey matches
		// either the event pubkey or appears in any 'p' tag of the event.
		// Only check authentication if AuthRequired is true (ACL is active)
		if kind.IsPrivileged(ev.Kind) && d.sub.AuthRequired {
			pk := d.sub.AuthedPubkey

			// Use centralized IsPartyInvolved function for consistent privilege checking
			if !policy.IsPartyInvolved(ev, pk) {
				log.D.F(
					"subscription delivery DENIED for privileged event %s to %s (not authenticated or not a party involved)",
					hex.Enc(ev.ID), d.sub.remote,
				)
				// Skip delivery for this subscriber
				continue
			}
		}

		// Check for private tags - only deliver to authorized users
		if ev.Tags != nil && ev.Tags.Len() > 0 {
			hasPrivateTag := false
			var privatePubkey []byte

			for _, t := range *ev.Tags {
				if t.Len() >= 2 {
					keyBytes := t.Key()
					if len(keyBytes) == 7 && string(keyBytes) == "private" {
						hasPrivateTag = true
						privatePubkey = t.Value()
						break
					}
				}
			}

			if hasPrivateTag {
				canSeePrivate := p.canSeePrivateEvent(
					d.sub.AuthedPubkey, privatePubkey, d.sub.remote,
				)
				if !canSeePrivate {
					log.D.F(
						"subscription delivery DENIED for private event %s to %s (unauthorized)",
						hex.Enc(ev.ID), d.sub.remote,
					)
					continue
				}
				log.D.F(
					"subscription delivery ALLOWED for private event %s to %s (authorized)",
					hex.Enc(ev.ID), d.sub.remote,
				)
			}
		}

		// Send event to the subscription's receiver channel
		// The consumer goroutine (in handle-req.go) will read from this channel
		// and forward it to the client via the write channel
		log.D.F(
			"attempting delivery of event %s (kind=%d) to subscription %s @ %s",
			hex.Enc(ev.ID), ev.Kind, d.id, d.sub.remote,
		)

		// Check if receiver channel exists
		if d.sub.Receiver == nil {
			log.E.F(
				"subscription %s has nil receiver channel for %s", d.id,
				d.sub.remote,
			)
			continue
		}

		// Send to receiver channel - non-blocking with timeout
		select {
		case <-p.c.Done():
			continue
		case d.sub.Receiver <- ev:
			log.D.F(
				"subscription delivery QUEUED: event=%s to=%s sub=%s",
				hex.Enc(ev.ID), d.sub.remote, d.id,
			)
		case <-time.After(DefaultWriteTimeout):
			log.E.F(
				"subscription delivery TIMEOUT: event=%s to=%s sub=%s",
				hex.Enc(ev.ID), d.sub.remote, d.id,
			)
			// Receiver channel is full - subscription consumer is stuck or slow
			// The subscription should be removed by the cleanup logic
		}
	}
}

// removeSubscriberId removes a specific subscription from a subscriber
// websocket.
func (p *P) removeSubscriberId(ws *websocket.Conn, id string) {
	p.Mx.Lock()
	defer p.Mx.Unlock()
	var subs map[string]Subscription
	var ok bool
	if subs, ok = p.Map[ws]; ok {
		delete(subs, id)
		// Check the actual map after deletion, not the original reference
		if len(p.Map[ws]) == 0 {
			delete(p.Map, ws)
			// Don't remove write channel here - it's tied to the connection, not subscriptions
			// The write channel will be removed when the connection closes (in handle-websocket.go defer)
			// This allows new subscriptions to be created on the same connection
		}
	}
}

// SetWriteChan stores the write channel for a websocket connection
// If writeChan is nil, the entry is removed from the map
func (p *P) SetWriteChan(
	conn *websocket.Conn, writeChan chan publish.WriteRequest,
) {
	p.Mx.Lock()
	defer p.Mx.Unlock()
	if writeChan == nil {
		delete(p.WriteChans, conn)
	} else {
		p.WriteChans[conn] = writeChan
	}
}

// GetWriteChan returns the write channel for a websocket connection
func (p *P) GetWriteChan(conn *websocket.Conn) (
	chan publish.WriteRequest, bool,
) {
	p.Mx.RLock()
	defer p.Mx.RUnlock()
	ch, ok := p.WriteChans[conn]
	return ch, ok
}

// removeSubscriber removes a websocket from the P collection.
func (p *P) removeSubscriber(ws *websocket.Conn) {
	p.Mx.Lock()
	defer p.Mx.Unlock()
	clear(p.Map[ws])
	delete(p.Map, ws)
	delete(p.WriteChans, ws)
}

// HasActiveNIP46Signer checks if there's an active subscription for kind 24133
// where the given pubkey is involved (either as author filter or in #p tag filter).
// This is used to authenticate clients by proving a signer is connected for that pubkey.
func (p *P) HasActiveNIP46Signer(signerPubkey []byte) bool {
	const kindNIP46 = 24133
	p.Mx.RLock()
	defer p.Mx.RUnlock()

	for _, subs := range p.Map {
		for _, sub := range subs {
			if sub.S == nil {
				continue
			}
			for _, f := range *sub.S {
				if f == nil || f.Kinds == nil {
					continue
				}
				// Check if filter is for kind 24133
				hasNIP46Kind := false
				for _, k := range f.Kinds.K {
					if k.K == kindNIP46 {
						hasNIP46Kind = true
						break
					}
				}
				if !hasNIP46Kind {
					continue
				}
				// Check if the signer pubkey matches the #p tag filter
				if f.Tags != nil {
					pTag := f.Tags.GetFirst([]byte("p"))
					if pTag != nil && pTag.Len() >= 2 {
						for i := 1; i < pTag.Len(); i++ {
							tagValue := pTag.T[i]
							// Compare - handle both binary and hex formats
							if len(tagValue) == 32 && len(signerPubkey) == 32 {
								if utils.FastEqual(tagValue, signerPubkey) {
									return true
								}
							} else if len(tagValue) == 64 && len(signerPubkey) == 32 {
								// tagValue is hex, signerPubkey is binary
								if string(tagValue) == hex.Enc(signerPubkey) {
									return true
								}
							} else if len(tagValue) == 32 && len(signerPubkey) == 64 {
								// tagValue is binary, signerPubkey is hex
								if hex.Enc(tagValue) == string(signerPubkey) {
									return true
								}
							} else if utils.FastEqual(tagValue, signerPubkey) {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

// canSeePrivateEvent checks if the authenticated user can see an event with a private tag
func (p *P) canSeePrivateEvent(
	authedPubkey, privatePubkey []byte, remote string,
) (canSee bool) {
	// If no authenticated user, deny access
	if len(authedPubkey) == 0 {
		return false
	}

	// If the authenticated user matches the private tag pubkey, allow access
	if len(privatePubkey) > 0 && utils.FastEqual(authedPubkey, privatePubkey) {
		return true
	}

	// Check if user is an admin or owner (they can see all private events)
	accessLevel := acl.Registry.GetAccessLevel(authedPubkey, remote)
	if accessLevel == "admin" || accessLevel == "owner" {
		return true
	}

	// Default deny
	return false
}
