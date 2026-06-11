package ws

import (
	"context"
	"errors"
	"fmt"

	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/closeenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/reqenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

type ReplaceableKey struct {
	PubKey string
	D      string
}

// --- Subscription actor message types ---

type subDispatchEventReq struct {
	evt *event.E
}

type subDispatchEoseReq struct{}

type subHandleClosedReq struct {
	reason string
}

type subUnsubReq struct {
	err error
}

type subFireReq struct {
	resp chan error
}

// Subscription represents a subscription to a relay.
type Subscription struct {
	counter int64
	id      []byte

	Client  *Client
	Filters *filter.S

	// the Events channel emits all EVENTs that come in a Subscription
	// will be closed when the subscription ends
	Events chan *event.E

	// the EndOfStoredEvents channel gets closed when an EOSE comes for that subscription
	EndOfStoredEvents chan struct{}

	// the ClosedReason channel emits the reason when a CLOSED message is received
	ClosedReason chan string

	// Context will be .Done() when the subscription ends
	Context context.Context

	// if it is not nil, checkDuplicate will be called for every event received
	// if it returns true that event will not be processed further.
	checkDuplicate func(id string, relay string) bool

	// if it is not nil, checkDuplicateReplaceable will be called for every event received
	// if it returns true that event will not be processed further.
	checkDuplicateReplaceable func(rk ReplaceableKey, ts *timestamp.T) bool

	match  func(*event.E) bool // this will be either Filters.Match or Filters.MatchIgnoringTimestampConstraints
	cancel context.CancelCauseFunc

	// Actor channels
	dispatchEventCh  chan subDispatchEventReq
	dispatchEoseCh   chan subDispatchEoseReq
	handleClosedCh   chan subHandleClosedReq
	unsubCh          chan subUnsubReq
	fireCh           chan subFireReq
	done             chan struct{}
}

// SubscriptionOption is the type of the argument passed when instantiating relay connections.
// Some examples are WithLabel.
type SubscriptionOption interface {
	IsSubscriptionOption()
}

// WithLabel puts a label on the subscription (it is prepended to the automatic id) that is sent to relays.
type WithLabel string

func (_ WithLabel) IsSubscriptionOption() {}

// WithCheckDuplicate sets checkDuplicate on the subscription
type WithCheckDuplicate func(id, relay string) bool

func (_ WithCheckDuplicate) IsSubscriptionOption() {}

// WithCheckDuplicateReplaceable sets checkDuplicateReplaceable on the subscription
type WithCheckDuplicateReplaceable func(rk ReplaceableKey, ts *timestamp.T) bool

func (_ WithCheckDuplicateReplaceable) IsSubscriptionOption() {}

var (
	_ SubscriptionOption = (WithLabel)("")
	_ SubscriptionOption = (WithCheckDuplicate)(nil)
	_ SubscriptionOption = (WithCheckDuplicateReplaceable)(nil)
)

func (sub *Subscription) start() {
	defer close(sub.done)

	live := true
	eosed := false
	storedPending := 0
	// storedDone is used to count down in-flight stored event dispatches
	storedDone := make(chan struct{}, 128)
	// eoseWaiting: when EOSE arrives before all stored events are dispatched,
	// we track that we need to signal EndOfStoredEvents once storedPending hits 0.
	eoseWaiting := false

	log.T.F("WS.Subscription.start: started id=%s", sub.GetID())

	for {
		select {
		case <-sub.Context.Done():
			log.T.F("WS.Subscription.start: context done for id=%s", sub.GetID())
			if live {
				live = false
				sub.sendClose()
			}
			sub.Client.deleteSub(string(sub.id))
			close(sub.Events)
			return

		case req := <-sub.dispatchEventCh:
			if !live {
				continue
			}
			isStored := !eosed
			if isStored {
				storedPending++
			}
			// Send to Events channel in a goroutine to avoid blocking
			// the actor on a slow consumer.
			go func() {
				select {
				case sub.Events <- req.evt:
				case <-sub.Context.Done():
				}
				if isStored {
					storedDone <- struct{}{}
				}
			}()

		case <-storedDone:
			storedPending--
			if eoseWaiting && storedPending == 0 {
				eoseWaiting = false
				sub.EndOfStoredEvents <- struct{}{}
			}

		case <-sub.dispatchEoseCh:
			if eosed {
				continue
			}
			eosed = true
			sub.match = sub.Filters.MatchIgnoringTimestampConstraints
			if storedPending == 0 {
				sub.EndOfStoredEvents <- struct{}{}
			} else {
				eoseWaiting = true
			}

		case req := <-sub.handleClosedCh:
			live = false // relay already closed it - don't send CLOSE back
			sub.cancel(fmt.Errorf("CLOSED received: %s", req.reason))
			// Non-blocking send so callers who read ClosedReason still get notified,
			// but we don't block on a channel nobody may be reading.
			select {
			case sub.ClosedReason <- req.reason:
			default:
			}

		case req := <-sub.unsubCh:
			sub.cancel(req.err)
			if live {
				live = false
				sub.sendClose()
			}
			sub.Client.deleteSub(string(sub.id))

		case req := <-sub.fireCh:
			var reqb []byte
			reqb = reqenvelope.NewFrom(sub.id, sub.Filters).Marshal(nil)
			live = true
			log.T.F(
				"WS.Subscription.Fire: sending REQ id=%s filters=%d bytes=%d",
				sub.GetID(), len(*sub.Filters), len(reqb),
			)
			log.T.F(
				"WS.Subscription.Fire: outbound REQ to %s: %s", sub.Client.URL,
				string(reqb),
			)
			if err := <-sub.Client.Write(reqb); err != nil {
				err = fmt.Errorf("failed to write: %w", err)
				log.T.F(
					"WS.Subscription.Fire: write failed id=%s: %v", sub.GetID(), err,
				)
				sub.cancel(err)
				req.resp <- err
			} else {
				log.T.F("WS.Subscription.Fire: write ok id=%s", sub.GetID())
				req.resp <- nil
			}
		}
	}
}

// sendClose sends a CLOSE message to the relay.
func (sub *Subscription) sendClose() {
	if sub.Client.IsConnected() {
		closeMsg := closeenvelope.NewFrom(sub.id)
		closeb := closeMsg.Marshal(nil)
		log.T.F(
			"WS.Subscription.Close: outbound CLOSE to %s: %s", sub.Client.URL,
			string(closeb),
		)
		<-sub.Client.Write(closeb)
	}
}

// GetID returns the subscription ID.
func (sub *Subscription) GetID() string { return string(sub.id) }

func (sub *Subscription) dispatchEvent(evt *event.E) {
	sub.dispatchEventCh <- subDispatchEventReq{evt: evt}
}

func (sub *Subscription) dispatchEose() {
	sub.dispatchEoseCh <- subDispatchEoseReq{}
}

// handleClosed handles the CLOSED message from a relay.
func (sub *Subscription) handleClosed(reason string) {
	sub.handleClosedCh <- subHandleClosedReq{reason: reason}
}

// Unsub closes the subscription, sending "CLOSE" to relay as in NIP-01.
// Unsub() also closes the channel sub.Events and makes a new one.
func (sub *Subscription) Unsub() {
	sub.unsub(errors.New("Unsub() called"))
}

// unsub is the internal implementation of Unsub.
func (sub *Subscription) unsub(err error) {
	sub.unsubCh <- subUnsubReq{err: err}
}

// Close just sends a CLOSE message. You probably want Unsub() instead.
func (sub *Subscription) Close() {
	sub.sendClose()
}

// Sub sets sub.Filters and then calls sub.Fire(ctx).
// The subscription will be closed if the context expires.
func (sub *Subscription) Sub(_ context.Context, ff *filter.S) {
	sub.Filters = ff
	chk.E(sub.Fire())
}

// Fire sends the "REQ" command to the relay.
func (sub *Subscription) Fire() (err error) {
	req := subFireReq{resp: make(chan error, 1)}
	sub.fireCh <- req
	return <-req.resp
}
