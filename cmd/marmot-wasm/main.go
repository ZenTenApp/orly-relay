// marmot-wasm — WASM module exposing the marmot MLS DM protocol to JS.
// Compiled with: GOOS=js GOARCH=wasm go build -o marmot.wasm ./cmd/marmot-wasm
// Loaded by the marmot service worker.
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall/js"
	"time"

	"next.orly.dev/pkg/lol"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/protocol/marmot"
	"next.orly.dev/pkg/version"
)

var (
	client     *marmot.Client
	crypto     *marmot.ProxyCrypto
	store      *jsGroupStore
	relay      *jsRelay
	mu         sync.Mutex
	statusFn   js.Value // onStatusFn callback — pushes status messages to page
)

// jsRelay bridges RelayConnection to JS callbacks.
type jsRelay struct {
	publishFn   js.Value // (eventJSON: string) => void
	subscribeFn js.Value // (filterJSON: string) => int (subscription handle)
	eventChs    map[int]chan *event.E
	nextSub     int
	mu          sync.Mutex
	// Fix 2a: buffer events arriving before any subscription is registered.
	preSubBuf    []*preSubEvent
	preSubActive bool
}

type preSubEvent struct {
	subID int
	ev    *event.E
}

func (r *jsRelay) Publish(ctx context.Context, ev *event.E) error {
	b, err := ev.MarshalJSON()
	if err != nil {
		return err
	}
	r.publishFn.Invoke(string(b))
	return nil
}

func (r *jsRelay) Subscribe(ctx context.Context, ff *filter.S) (marmot.EventStream, error) {
	b := ff.Marshal(nil)
	r.mu.Lock()
	id := r.nextSub
	r.nextSub++
	ch := make(chan *event.E, 64)
	r.eventChs[id] = ch
	// Fix 2a: drain pre-subscription buffer into the new channel.
	if r.preSubActive {
		r.preSubActive = false
		for _, pe := range r.preSubBuf {
			select {
			case ch <- pe.ev:
			default:
			}
		}
		r.preSubBuf = nil
	}
	r.mu.Unlock()

	r.subscribeFn.Invoke(id, string(b))
	return &jsEventStream{id: id, ch: ch, relay: r}, nil
}

type jsEventStream struct {
	id    int
	ch    chan *event.E
	relay *jsRelay
}

func (s *jsEventStream) Events() <-chan *event.E { return s.ch }
func (s *jsEventStream) Close() {
	s.relay.mu.Lock()
	delete(s.relay.eventChs, s.id)
	s.relay.mu.Unlock()
}

// deliverEvent routes an incoming event JSON to the right subscription channel.
// Fix 2a: if no subscription channel exists yet, buffer the event for later delivery.
func deliverEvent(subID int, evJSON string) {
	ev := event.New()
	if err := ev.UnmarshalJSON([]byte(evJSON)); err != nil {
		return
	}
	relay.mu.Lock()
	ch, ok := relay.eventChs[subID]
	if !ok && relay.preSubActive {
		if len(relay.preSubBuf) < 64 {
			relay.preSubBuf = append(relay.preSubBuf, &preSubEvent{subID: subID, ev: ev})
		}
		relay.mu.Unlock()
		return
	}
	relay.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- ev:
	default:
	}
}

// safeFunc wraps a js.Func callback with recover so panics don't kill the WASM instance.
func safeFunc(name string, fn func(this js.Value, args []js.Value) any) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) (ret any) {
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprintf("[marmot-wasm] PANIC in %s: %v", name, r)
				js.Global().Get("console").Call("error", msg)
				ret = "error: panic: " + fmt.Sprint(r)
			}
		}()
		return fn(this, args)
	})
}

func main() {
	// Suppress error-level logging from hex decoder etc — these fire on every
	// corrupt event and burn CPU with formatted output. Errors still propagate
	// via return values; they just don't print.
	lol.Level.Store(lol.Off)

	store = newJSGroupStore()
	relay = &jsRelay{eventChs: make(map[int]chan *event.E), preSubActive: true}

	// Register JS API — all wrapped with panic recovery
	js.Global().Set("_marmot", js.ValueOf(map[string]any{
		"init":            safeFunc("init", jsInit),
		"sendDM":          safeFunc("sendDM", jsSendDM),
		"subscribe":       safeFunc("subscribe", jsSubscribe),
		"publishKP":       safeFunc("publishKP", jsPublishKP),
		"listGroups":      safeFunc("listGroups", jsListGroups),
		"handleEvent":     safeFunc("handleEvent", jsHandleEvent),
		"deliverEvent":    safeFunc("deliverEvent", jsDeliverEvent),
		"cryptoResult":    safeFunc("cryptoResult", jsCryptoResult),
		"storeResult":     safeFunc("storeResult", jsStoreResult),
		"keyPackageEvent": safeFunc("keyPackageEvent", jsKeyPackageEvent),
		"lastEventTS":     safeFunc("lastEventTS", jsLastEventTS),
		"version":         version.V,
	}))


	// Keep WASM alive
	select {}
}

// jsInit(pubkeyHex, publishFn, subscribeFn, cryptoSendFn, onDMFn, onStatusFn, onReadyFn, lastEventTS, relayURLs[])
// NewClient calls store.ListGroups/LoadGroup which block on async IDB callbacks.
// Running it in a goroutine lets the JS event loop process those callbacks.
func jsInit(this js.Value, args []js.Value) any {
	if len(args) < 8 {
		return "error: need pubkeyHex, publishFn, subscribeFn, cryptoSendFn, onDMFn, onStatusFn, onReadyFn, lastEventTS"
	}
	pubHex := args[0].String()
	pubBytes, err := hex.Dec(pubHex)
	if err != nil {
		return "error: invalid pubkey: " + err.Error()
	}

	relay.publishFn = args[1]
	relay.subscribeFn = args[2]
	cryptoSendFn := args[3]
	onDMFn := args[4]
	onStatusFn := args[5]
	onReadyFn := args[6]
	lastEventTS := int64(args[7].Int())

	var relays []string
	if len(args) > 8 {
		for i := 8; i < len(args); i++ {
			relays = append(relays, args[i].String())
		}
	}

	crypto = marmot.NewProxyCrypto(pubBytes, func(op, peerHex, data string, id int) {
		cryptoSendFn.Invoke(op, peerHex, data, id)
	})

	// Run NewClient in a goroutine — it calls store.ListGroups() which blocks
	// on a channel waiting for async IDB callbacks. Running synchronously would
	// deadlock because Go WASM's single thread can't yield to the JS event loop.
	go func() {
		mu.Lock()
		defer mu.Unlock()

		c, err := marmot.NewClient(crypto, store, relay, relays...)
		if err != nil {
			onReadyFn.Invoke("error: " + err.Error())
			return
		}

		if lastEventTS > 0 {
			c.SetLastEventTS(lastEventTS)
		}

		c.OnDM(func(senderPub []byte, plaintext []byte) {
			onDMFn.Invoke(hex.Enc(senderPub), string(plaintext))
		})

		statusFn = onStatusFn
		client = c
		onReadyFn.Invoke("ok")
	}()

	return nil
}

func sendStatus(msg string) {
	if statusFn.IsUndefined() || statusFn.IsNull() {
		return
	}
	statusFn.Invoke(msg)
}

func jsSendDM(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return "error: missing args"
	}
	if client == nil {
		return "error: not initialized"
	}
	recipientHex := args[0].String()
	content := args[1].String()
	sendStatus("sendDM: starting for " + recipientHex[:8] + "...")

	go func() {
		recipientPub, err := hex.Dec(recipientHex)
		if err != nil {
			sendStatus("sendDM error: invalid recipient: " + err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := client.SendDM(ctx, recipientPub, []byte(content)); err != nil {
			sendStatus("sendDM error: " + err.Error())
			return
		}
		sendStatus("sendDM ok: sent to " + recipientHex[:8] + "...")
	}()
	return nil
}

// Fix 2c: track active subscription so duplicate calls cancel the previous one.
var (
	activeSubCancel context.CancelFunc
	activeSubMu     sync.Mutex
)

func jsSubscribe(this js.Value, args []js.Value) any {
	if client == nil {
		return nil
	}
	activeSubMu.Lock()
	if activeSubCancel != nil {
		activeSubCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	activeSubCancel = cancel
	activeSubMu.Unlock()

	go func() {
		defer cancel()
		ff := client.SubscriptionFilters()
		stream, err := relay.Subscribe(ctx, ff)
		if err != nil {
			return
		}
		defer stream.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-stream.Events():
				if ev == nil {
					return
				}
				_ = client.HandleEvent(ctx, ev)
			case <-client.GroupsChanged():
				stream.Close()
				ff = client.SubscriptionFilters()
				stream, err = relay.Subscribe(ctx, ff)
				if err != nil {
					return
				}
			}
		}
	}()
	return nil
}

func jsPublishKP(this js.Value, args []js.Value) any {
	if client == nil {
		return nil
	}
	go func() {
		if err := client.PublishKeyPackage(context.Background()); err != nil {
			fmt.Println("marmot-wasm: publishKP error:", err)
		}
	}()
	return nil
}

func jsListGroups(this js.Value, args []js.Value) any {
	if client == nil {
		return "[]"
	}
	ids := client.ActiveGroupIDs()
	out := "["
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += "\"" + id + "\""
	}
	out += "]"
	return out
}

func jsHandleEvent(this js.Value, args []js.Value) any {
	if len(args) < 1 || client == nil {
		return nil
	}
	evJSON := args[0].String()
	go func() {
		ev := event.New()
		if err := ev.UnmarshalJSON([]byte(evJSON)); err != nil {
			return
		}
		_ = client.HandleEvent(context.Background(), ev)
	}()
	return nil
}

func jsDeliverEvent(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return nil
	}
	subID := args[0].Int()
	evJSON := args[1].String()
	deliverEvent(subID, evJSON)
	return nil
}

func jsCryptoResult(this js.Value, args []js.Value) any {
	if len(args) < 3 || crypto == nil {
		return nil
	}
	id := args[0].Int()
	result := args[1].String()
	errMsg := args[2].String()
	crypto.Resolve(id, result, errMsg)
	return nil
}

func jsLastEventTS(this js.Value, args []js.Value) any {
	if client == nil {
		return 0
	}
	return client.LastEventTS()
}

func jsKeyPackageEvent(this js.Value, args []js.Value) any {
	if client == nil {
		return ""
	}
	ev, err := client.KeyPackageEvent()
	if err != nil {
		return "error: " + err.Error()
	}
	b, err := ev.MarshalJSON()
	if err != nil {
		return "error: " + err.Error()
	}
	return string(b)
}

// --- IDB-backed GroupStore via JS callbacks ---

type storeResult struct {
	data string
	err  string
}

type jsGroupStore struct {
	mu      sync.Mutex
	pending map[int]chan storeResult
	nextID  int
}

func newJSGroupStore() *jsGroupStore {
	return &jsGroupStore{pending: make(map[int]chan storeResult)}
}

func (s *jsGroupStore) newPending() (int, chan storeResult) {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	ch := make(chan storeResult, 1)
	s.pending[id] = ch
	s.mu.Unlock()
	return id, ch
}

func (s *jsGroupStore) resolve(id int, data, errMsg string) {
	s.mu.Lock()
	ch, ok := s.pending[id]
	if ok {
		delete(s.pending, id)
	}
	s.mu.Unlock()
	if ok {
		ch <- storeResult{data: data, err: errMsg}
	}
}

func (s *jsGroupStore) SaveGroup(groupID, state []byte) error {
	id, ch := s.newPending()
	js.Global().Call("_marmot_store_save", id, hex.Enc(groupID), hex.Enc(state))
	r := <-ch
	if r.err != "" {
		return fmt.Errorf("%s", r.err)
	}
	return nil
}

func (s *jsGroupStore) LoadGroup(groupID []byte) ([]byte, error) {
	id, ch := s.newPending()
	js.Global().Call("_marmot_store_load", id, hex.Enc(groupID))
	r := <-ch
	if r.err != "" {
		return nil, fmt.Errorf("%s", r.err)
	}
	if r.data == "" {
		return nil, os.ErrNotExist
	}
	return hex.Dec(r.data)
}

func (s *jsGroupStore) ListGroups() ([][]byte, error) {
	id, ch := s.newPending()
	js.Global().Call("_marmot_store_list", id)
	r := <-ch
	if r.err != "" {
		return nil, fmt.Errorf("%s", r.err)
	}
	if r.data == "" {
		return nil, nil
	}
	var result [][]byte
	start := 0
	for i := 0; i <= len(r.data); i++ {
		if i == len(r.data) || r.data[i] == ',' {
			h := r.data[start:i]
			start = i + 1
			if h == "" {
				continue
			}
			b, err := hex.Dec(h)
			if err != nil {
				continue
			}
			result = append(result, b)
		}
	}
	return result, nil
}

func (s *jsGroupStore) DeleteGroup(groupID []byte) error {
	id, ch := s.newPending()
	js.Global().Call("_marmot_store_delete", id, hex.Enc(groupID))
	r := <-ch
	if r.err != "" {
		return fmt.Errorf("%s", r.err)
	}
	return nil
}

func jsStoreResult(this js.Value, args []js.Value) any {
	if len(args) < 3 || store == nil {
		return nil
	}
	store.resolve(args[0].Int(), args[1].String(), args[2].String())
	return nil
}
