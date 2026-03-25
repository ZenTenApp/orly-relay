package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
)

// autoPayNWC implements bridge.NWCRequester with automatic invoice payment.
// All invoices are marked as paid immediately on creation.
type autoPayNWC struct {
	mu       sync.Mutex
	invoices map[string]*nwcInvoice
	counter  int
}

type nwcInvoice struct {
	bolt11      string
	paymentHash string
	amount      int64
	preimage    string
}

func newAutoPayNWC() *autoPayNWC {
	return &autoPayNWC{invoices: make(map[string]*nwcInvoice)}
}

func (a *autoPayNWC) Request(ctx context.Context, method string, params, result any) error {
	switch method {
	case "make_invoice":
		return a.makeInvoice(params, result)
	case "lookup_invoice":
		return a.lookupInvoice(params, result)
	case "get_balance":
		return marshalInto(result, map[string]any{"balance": int64(1000000000)})
	default:
		return fmt.Errorf("unsupported NWC method: %s", method)
	}
}

func (a *autoPayNWC) makeInvoice(params, result any) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.counter++
	var hashBytes, preBytes [32]byte
	rand.Read(hashBytes[:])
	rand.Read(preBytes[:])
	paymentHash := hex.EncodeToString(hashBytes[:])
	preimage := hex.EncodeToString(preBytes[:])

	var amount int64
	if m, ok := params.(map[string]any); ok {
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
	a.invoices[paymentHash] = &nwcInvoice{
		bolt11:      bolt11,
		paymentHash: paymentHash,
		amount:      amount,
		preimage:    preimage,
	}

	return marshalInto(result, map[string]any{
		"invoice":      bolt11,
		"payment_hash": paymentHash,
		"amount":       amount,
	})
}

func (a *autoPayNWC) lookupInvoice(params, result any) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var paymentHash string
	if m, ok := params.(map[string]any); ok {
		if v, ok := m["payment_hash"].(string); ok {
			paymentHash = v
		}
	}
	inv, ok := a.invoices[paymentHash]
	if !ok {
		return fmt.Errorf("invoice not found: %s", paymentHash)
	}
	return marshalInto(result, map[string]any{
		"invoice":      inv.bolt11,
		"payment_hash": inv.paymentHash,
		"amount":       inv.amount,
		"preimage":     inv.preimage,
		"settled_at":   1,
	})
}

func marshalInto(dst any, src map[string]any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
