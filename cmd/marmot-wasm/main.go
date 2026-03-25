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

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/protocol/marmot"
)

var (
	client *marmot.Client
	crypto *marmot.ProxyCrypto
	store  *jsGroupStore
	relay  *jsRelay
	mu     sync.Mutex
)

// jsRelay bridges RelayConnection to JS callbacks.
type jsRelay struct {
	publishFn   js.Value // (eventJSON: string) => void
	subscribeFn js.Value // (filterJSON: string) => int (subscription handle)
	eventChs    map[int]chan *event.E
	nextSub     int
	mu          sync.Mutex
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
	ch := make(chan *event.E, 16)
	r.eventChs[id] = ch
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
func deliverEvent(subID int, evJSON string) {
	relay.mu.Lock()
	ch, ok := relay.eventChs[subID]
	relay.mu.Unlock()
	if !ok {
		return
	}
	ev := event.New()
	if err := ev.UnmarshalJSON([]byte(evJSON)); err != nil {
		return
	}
	select {
	case ch <- ev:
	default:
	}
}

func main() {
	store = newJSGroupStore()
	relay = &jsRelay{eventChs: make(map[int]chan *event.E)}

	// Register JS API
	js.Global().Set("_marmot", js.ValueOf(map[string]any{
		"init":          js.FuncOf(jsInit),
		"sendDM":        js.FuncOf(jsSendDM),
		"subscribe":     js.FuncOf(jsSubscribe),
		"publishKP":     js.FuncOf(jsPublishKP),
		"listGroups":    js.FuncOf(jsListGroups),
		"handleEvent":   js.FuncOf(jsHandleEvent),
		"deliverEvent":  js.FuncOf(jsDeliverEvent),
		"cryptoResult":    js.FuncOf(jsCryptoResult),
		"storeResult":     js.FuncOf(jsStoreResult),
		"keyPackageEvent": js.FuncOf(jsKeyPackageEvent),
	}))

	// Keep WASM alive
	select {}
}

// jsInit(pubkeyHex, publishFn, subscribeFn, cryptoSendFn, onDMFn, onStatusFn, relayURLs[])
func jsInit(this js.Value, args []js.Value) any {
	if len(args) < 6 {
		return "error: need pubkeyHex, publishFn, subscribeFn, cryptoSendFn, onDMFn, onStatusFn"
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

	var relays []string
	if len(args) > 6 {
		for i := 6; i < len(args); i++ {
			relays = append(relays, args[i].String())
		}
	}

	crypto = marmot.NewProxyCrypto(pubBytes, func(op, peerHex, data string, id int) {
		cryptoSendFn.Invoke(op, peerHex, data, id)
	})

	mu.Lock()
	defer mu.Unlock()

	client, err = marmot.NewClient(crypto, store, relay, relays...)
	if err != nil {
		return "error: " + err.Error()
	}

	client.OnDM(func(senderPub []byte, plaintext []byte) {
		onDMFn.Invoke(hex.Enc(senderPub), string(plaintext))
	})

	_ = onStatusFn
	return "ok"
}

func jsSendDM(this js.Value, args []js.Value) any {
	if len(args) < 2 || client == nil {
		return nil
	}
	recipientHex := args[0].String()
	content := args[1].String()

	go func() {
		recipientPub, err := hex.Dec(recipientHex)
		if err != nil {
			return
		}
		if err := client.SendDM(context.Background(), recipientPub, []byte(content)); err != nil {
			fmt.Println("marmot-wasm: sendDM error:", err)
		}
	}()
	return nil
}

func jsSubscribe(this js.Value, args []js.Value) any {
	if client == nil {
		return nil
	}
	go func() {
		ctx := context.Background()
		ff := client.SubscriptionFilters()
		stream, err := relay.Subscribe(ctx, ff)
		if err != nil {
			fmt.Println("marmot-wasm: subscribe error:", err)
			return
		}
		defer stream.Close()

		for {
			select {
			case ev := <-stream.Events():
				if ev == nil {
					return
				}
				if err := client.HandleEvent(ctx, ev); err != nil {
					fmt.Println("marmot-wasm: handleEvent error:", err)
				}
			case <-client.GroupsChanged():
				// Resubscribe with updated filters
				stream.Close()
				ff = client.SubscriptionFilters()
				stream, err = relay.Subscribe(ctx, ff)
				if err != nil {
					fmt.Println("marmot-wasm: resubscribe error:", err)
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
		if err := client.HandleEvent(context.Background(), ev); err != nil {
			fmt.Println("marmot-wasm: handleEvent error:", err)
		}
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
