package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// autoPayNWC implements bridge.NWCRequester with automatic invoice payment.
// All invoices are marked as paid immediately on creation.
// State is owned by an actor goroutine; all access goes through typed channels.
type autoPayNWC struct {
	makeInvoiceCh  chan autoPayMakeReq
	lookupInvoiceCh chan autoPayLookupReq
	stop           chan struct{}
	done           chan struct{}
}

type nwcInvoice struct {
	bolt11      string
	paymentHash string
	amount      int64
	preimage    string
}

type autoPayMakeReq struct {
	params any
	result any
	resp   chan error // buffered 1
}

type autoPayLookupReq struct {
	params any
	result any
	resp   chan error // buffered 1
}

func newAutoPayNWC() *autoPayNWC {
	a := &autoPayNWC{
		makeInvoiceCh:   make(chan autoPayMakeReq),
		lookupInvoiceCh: make(chan autoPayLookupReq),
		stop:            make(chan struct{}),
		done:            make(chan struct{}),
	}
	go a.actor()
	return a
}

func (a *autoPayNWC) actor() {
	defer close(a.done)
	invoices := make(map[string]*nwcInvoice)
	counter := 0
	for {
		select {
		case req := <-a.makeInvoiceCh:
			counter++
			var hashBytes, preBytes [32]byte
			rand.Read(hashBytes[:])
			rand.Read(preBytes[:])
			paymentHash := hex.EncodeToString(hashBytes[:])
			preimage := hex.EncodeToString(preBytes[:])

			var amount int64
			if m, ok := req.params.(map[string]any); ok {
				if v, ok := m["amount"]; ok {
					switch n := v.(type) {
					case int64:
						amount = n
					case float64:
						amount = int64(n)
					case int:
						amount = int64(n)
					}
				}
			}

			bolt11 := fmt.Sprintf("lnbc%du1autopay%s", amount/1000, paymentHash[:16])
			invoices[paymentHash] = &nwcInvoice{
				bolt11:      bolt11,
				paymentHash: paymentHash,
				amount:      amount,
				preimage:    preimage,
			}

			req.resp <- marshalInto(req.result, map[string]any{
				"invoice":      bolt11,
				"payment_hash": paymentHash,
				"amount":       amount,
			})

		case req := <-a.lookupInvoiceCh:
			var paymentHash string
			if m, ok := req.params.(map[string]any); ok {
				if v, ok := m["payment_hash"].(string); ok {
					paymentHash = v
				}
			}
			inv, ok := invoices[paymentHash]
			if !ok {
				req.resp <- fmt.Errorf("invoice not found: %s", paymentHash)
				continue
			}
			req.resp <- marshalInto(req.result, map[string]any{
				"invoice":      inv.bolt11,
				"payment_hash": inv.paymentHash,
				"amount":       inv.amount,
				"preimage":     inv.preimage,
				"settled_at":   1,
			})

		case <-a.stop:
			return
		}
	}
}

func (a *autoPayNWC) Request(ctx context.Context, method string, params, result any) error {
	switch method {
	case "make_invoice":
		resp := make(chan error, 1)
		a.makeInvoiceCh <- autoPayMakeReq{params: params, result: result, resp: resp}
		return <-resp
	case "lookup_invoice":
		resp := make(chan error, 1)
		a.lookupInvoiceCh <- autoPayLookupReq{params: params, result: result, resp: resp}
		return <-resp
	case "get_balance":
		return marshalInto(result, map[string]any{"balance": int64(1000000000)})
	default:
		return fmt.Errorf("unsupported NWC method: %s", method)
	}
}

func marshalInto(dst any, src map[string]any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
