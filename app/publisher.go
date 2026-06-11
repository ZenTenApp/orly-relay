package app

import (
	"context"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/interfaces/publisher"
	"git.smesh.lol/orly/pkg/interfaces/typer"
	"git.smesh.lol/orly/pkg/policy"
	"git.smesh.lol/orly/pkg/protocol/publish"
	"git.smesh.lol/orly/pkg/utils"
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
	// associated with this WebSocket connection.
	Filters *filter.S

	// AuthedPubkey is the authenticated pubkey associated with the listener (if any).
	AuthedPubkey []byte

	// AuthRequired indicates whether the ACL in operation requires auth.
	AuthRequired bool
}

func (w *W) Type() (typeName string) { return Type }

// -- actor request types for publisher --

type pubReceiveReq struct {
	msg *W
}

type pubDeliverReq struct {
	ev *event.E
}

type pubSetWriteChanReq struct {
	conn      *websocket.Conn
	writeChan chan publish.WriteRequest // nil to remove
}

type pubGetWriteChanReq struct {
	conn *websocket.Conn
	resp chan pubGetWriteChanResp // buffered 1
}
type pubGetWriteChanResp struct {
	ch chan publish.WriteRequest
	ok bool
}

type pubHasNIP46Req struct {
	signerPubkey []byte
	resp         chan bool // buffered 1
}

// P is a structure that manages subscriptions and associated filters for
// websocket listeners. It uses an actor goroutine to synchronize access
// to a map storing subscriber connections and their filter configurations.
type P struct {
	c context.Context

	receiveCh    chan pubReceiveReq
	deliverCh    chan pubDeliverReq
	setWriteCh   chan pubSetWriteChanReq
	getWriteCh   chan pubGetWriteChanReq
	hasNIP46Ch   chan pubHasNIP46Req
	stop         chan struct{}
	done         chan struct{}

	// ChannelMembership is used for NIRC channel access control (kinds 40-44)
	ChannelMembership *ChannelMembership
	// PrivilegedOpen disables all privileged-kind auth checks in delivery
	PrivilegedOpen bool
}

var _ publisher.I = &P{}

func NewPublisher(c context.Context) (pub *P) {
	pub = &P{
		c:          c,
		receiveCh:  make(chan pubReceiveReq, 128),
		deliverCh:  make(chan pubDeliverReq, 128),
		setWriteCh: make(chan pubSetWriteChanReq, 16),
		getWriteCh: make(chan pubGetWriteChanReq),
		hasNIP46Ch: make(chan pubHasNIP46Req),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	go pub.actor()
	return pub
}

func (p *P) actor() {
	defer close(p.done)
	m := make(Map)
	writeChans := make(WriteChanMap, 100)

	for {
		select {
		case req := <-p.receiveCh:
			p.doReceive(req.msg, m, writeChans)
		case req := <-p.deliverCh:
			p.doDeliver(req.ev, m, writeChans)
		case req := <-p.setWriteCh:
			if req.writeChan == nil {
				delete(writeChans, req.conn)
			} else {
				writeChans[req.conn] = req.writeChan
			}
		case req := <-p.getWriteCh:
			ch, ok := writeChans[req.conn]
			req.resp <- pubGetWriteChanResp{ch: ch, ok: ok}
		case req := <-p.hasNIP46Ch:
			req.resp <- p.doHasNIP46(req.signerPubkey, m)
		case <-p.stop:
			return
		case <-p.c.Done():
			return
		}
	}
}

func (p *P) Type() (typeName string) { return Type }

// Receive handles incoming messages to manage websocket listener subscriptions.
func (p *P) Receive(msg typer.T) {
	if m, ok := msg.(*W); ok {
		p.receiveCh <- pubReceiveReq{msg: m}
	}
}

func (p *P) doReceive(m *W, subMap Map, writeChans WriteChanMap) {
	if m.Cancel {
		if m.Id == "" {
			// removeSubscriber
			clear(subMap[m.Conn])
			delete(subMap, m.Conn)
			delete(writeChans, m.Conn)
		} else {
			// removeSubscriberId
			if subs, ok := subMap[m.Conn]; ok {
				delete(subs, m.Id)
				if len(subMap[m.Conn]) == 0 {
					delete(subMap, m.Conn)
				}
			}
		}
		return
	}
	if subs, ok := subMap[m.Conn]; !ok {
		subs = make(map[string]Subscription)
		subs[m.Id] = Subscription{
			S: m.Filters, remote: m.remote, AuthedPubkey: m.AuthedPubkey,
			Receiver: m.Receiver, AuthRequired: m.AuthRequired,
		}
		subMap[m.Conn] = subs
	} else {
		subs[m.Id] = Subscription{
			S: m.Filters, remote: m.remote, AuthedPubkey: m.AuthedPubkey,
			Receiver: m.Receiver, AuthRequired: m.AuthRequired,
		}
	}
}

// Deliver processes and distributes an event to all matching subscribers.
func (p *P) Deliver(ev *event.E) {
	p.deliverCh <- pubDeliverReq{ev: ev}
}

func (p *P) doDeliver(ev *event.E, subMap Map, writeChans WriteChanMap) {
	// Build deliveries list
	type delivery struct {
		w   *websocket.Conn
		id  string
		sub Subscription
	}
	var deliveries []delivery
	for w, subs := range subMap {
		for id, subscriber := range subs {
			if subscriber.Match(ev) {
				deliveries = append(
					deliveries, delivery{w: w, id: id, sub: subscriber},
				)
			}
		}
	}
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
	// Track subscriptions that timeout so we can remove them afterward
	type stuckSub struct {
		w  *websocket.Conn
		id string
	}
	var stuckSubs []stuckSub

	for _, d := range deliveries {
		isChannel := kind.IsChannelKind(ev.Kind)
		if !p.PrivilegedOpen && kind.IsPrivileged(ev.Kind) && (d.sub.AuthRequired || isChannel) {
			pk := d.sub.AuthedPubkey

			var allowed bool
			if kind.IsChannelKind(ev.Kind) && p.ChannelMembership != nil {
				channelID := ExtractChannelIDFromEvent(ev)
				allowed = p.ChannelMembership.IsChannelMemberByID(channelID, ev.Kind, pk, p.c)
			} else {
				allowed = policy.IsPartyInvolved(ev, pk)
			}

			if !allowed {
				log.D.F(
					"subscription delivery DENIED for privileged event %s to %s (not authenticated or not a party involved)",
					hex.Enc(ev.ID), d.sub.remote,
				)
				continue
			}
		}

		if !kind.IsChannelKind(ev.Kind) && !kind.IsPrivileged(ev.Kind) && p.ChannelMembership != nil {
			if channelIDHex, isChannel := p.ChannelMembership.ReferencesChannelEvent(ev, p.c); isChannel {
				pk := d.sub.AuthedPubkey
				if !p.ChannelMembership.IsChannelMemberByID(channelIDHex, ev.Kind, pk, p.c) {
					log.D.F(
						"subscription delivery DENIED for channel-referencing event %s kind %d to %s (not a member of channel %s)",
						hex.Enc(ev.ID), ev.Kind, d.sub.remote, channelIDHex,
					)
					continue
				}
			}
		}

		// Check for private tags
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

		log.D.F(
			"attempting delivery of event %s (kind=%d) to subscription %s @ %s",
			hex.Enc(ev.ID), ev.Kind, d.id, d.sub.remote,
		)

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
			log.W.F(
				"subscription delivery TIMEOUT: event=%s to=%s sub=%s - removing stuck subscription",
				hex.Enc(ev.ID), d.sub.remote, d.id,
			)
			stuckSubs = append(stuckSubs, stuckSub{w: d.w, id: d.id})
		}
	}

	// Remove stuck subscriptions
	for _, s := range stuckSubs {
		if subs, ok := subMap[s.w]; ok {
			delete(subs, s.id)
			if len(subs) == 0 {
				delete(subMap, s.w)
			}
		}
	}
}

// SetWriteChan stores the write channel for a websocket connection
func (p *P) SetWriteChan(
	conn *websocket.Conn, writeChan chan publish.WriteRequest,
) {
	p.setWriteCh <- pubSetWriteChanReq{conn: conn, writeChan: writeChan}
}

// GetWriteChan returns the write channel for a websocket connection
func (p *P) GetWriteChan(conn *websocket.Conn) (
	chan publish.WriteRequest, bool,
) {
	resp := make(chan pubGetWriteChanResp, 1)
	p.getWriteCh <- pubGetWriteChanReq{conn: conn, resp: resp}
	r := <-resp
	return r.ch, r.ok
}

// HasActiveNIP46Signer checks if there's an active subscription for kind 24133
func (p *P) HasActiveNIP46Signer(signerPubkey []byte) bool {
	resp := make(chan bool, 1)
	p.hasNIP46Ch <- pubHasNIP46Req{signerPubkey: signerPubkey, resp: resp}
	return <-resp
}

func (p *P) doHasNIP46(signerPubkey []byte, subMap Map) bool {
	const kindNIP46 = 24133

	for _, subs := range subMap {
		for _, sub := range subs {
			if sub.S == nil {
				continue
			}
			for _, f := range *sub.S {
				if f == nil || f.Kinds == nil {
					continue
				}
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
				if f.Tags != nil {
					pTag := f.Tags.GetFirst([]byte("p"))
					if pTag != nil && pTag.Len() >= 2 {
						for i := 1; i < pTag.Len(); i++ {
							tagValue := pTag.T[i]
							if len(tagValue) == 32 && len(signerPubkey) == 32 {
								if utils.FastEqual(tagValue, signerPubkey) {
									return true
								}
							} else if len(tagValue) == 64 && len(signerPubkey) == 32 {
								if string(tagValue) == hex.Enc(signerPubkey) {
									return true
								}
							} else if len(tagValue) == 32 && len(signerPubkey) == 64 {
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
	if len(authedPubkey) == 0 {
		return false
	}
	if len(privatePubkey) > 0 && utils.FastEqual(authedPubkey, privatePubkey) {
		return true
	}
	accessLevel := acl.Registry.GetAccessLevel(authedPubkey, remote)
	if accessLevel == "admin" || accessLevel == "owner" {
		return true
	}
	return false
}
