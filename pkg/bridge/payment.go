package bridge

import (
	"context"
	"fmt"
	"time"

	"git.smesh.lol/orly/pkg/protocol/nwc"
)

// NWCRequester abstracts the NWC client's Request method for testing.
type NWCRequester interface {
	Request(ctx context.Context, method string, params, result any) error
}

// PaymentProcessor wraps a NWC client for bridge subscription payments.
type PaymentProcessor struct {
	client            NWCRequester
	monthlyPriceMSats int64
}

// NewPaymentProcessor creates a payment processor with the given NWC URI
// and monthly price in satoshis.
func NewPaymentProcessor(nwcURI string, monthlyPriceSats int64) (*PaymentProcessor, error) {
	client, err := nwc.NewClient(nwcURI)
	if err != nil {
		return nil, fmt.Errorf("create NWC client: %w", err)
	}

	return &PaymentProcessor{
		client:            client,
		monthlyPriceMSats: monthlyPriceSats * 1000, // convert sats to msats
	}, nil
}

// NewPaymentProcessorWithClient creates a payment processor with a provided
// NWCRequester. This is useful for testing or custom NWC implementations.
func NewPaymentProcessorWithClient(client NWCRequester, monthlyPriceSats int64) *PaymentProcessor {
	return &PaymentProcessor{
		client:            client,
		monthlyPriceMSats: monthlyPriceSats * 1000,
	}
}

// Invoice represents a Lightning invoice returned by the wallet.
type Invoice struct {
	Bolt11      string `json:"invoice"`
	PaymentHash string `json:"payment_hash"`
	Amount      int64  `json:"amount"` // millisatoshis
	Description string `json:"description"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at"`
}

// InvoiceStatus represents the status of a looked-up invoice.
type InvoiceStatus struct {
	Bolt11      string `json:"invoice"`
	PaymentHash string `json:"payment_hash"`
	Amount      int64  `json:"amount"`
	Preimage    string `json:"preimage,omitempty"`
	SettledAt   int64  `json:"settled_at,omitempty"`
	IsPaid      bool   `json:"-"`
}

// CreateSubscriptionInvoice creates a Lightning invoice for a one-month
// bridge subscription at the default price.
func (pp *PaymentProcessor) CreateSubscriptionInvoice(ctx context.Context) (*Invoice, error) {
	return pp.CreateInvoice(ctx, pp.monthlyPriceMSats/1000) // convert msats back to sats
}

// CreateInvoice creates a Lightning invoice for the given amount in satoshis.
func (pp *PaymentProcessor) CreateInvoice(ctx context.Context, amountSats int64) (*Invoice, error) {
	params := map[string]any{
		"amount":      amountSats * 1000, // NWC uses millisatoshis
		"description": "Marmot Email Bridge — 1 month subscription",
	}

	var result Invoice
	if err := pp.client.Request(ctx, "make_invoice", params, &result); err != nil {
		return nil, fmt.Errorf("make_invoice: %w", err)
	}

	return &result, nil
}

// LookupInvoice checks the payment status of an invoice by its payment hash.
func (pp *PaymentProcessor) LookupInvoice(ctx context.Context, paymentHash string) (*InvoiceStatus, error) {
	params := map[string]any{
		"payment_hash": paymentHash,
	}

	var result InvoiceStatus
	if err := pp.client.Request(ctx, "lookup_invoice", params, &result); err != nil {
		return nil, fmt.Errorf("lookup_invoice: %w", err)
	}

	// Determine paid status: settled_at > 0 or preimage present
	result.IsPaid = result.SettledAt > 0 || result.Preimage != ""

	return &result, nil
}

// WaitForPayment polls the invoice until paid or the context is cancelled.
// Returns the settled invoice status on success.
func (pp *PaymentProcessor) WaitForPayment(ctx context.Context, paymentHash string, pollInterval time.Duration) (*InvoiceStatus, error) {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			status, err := pp.LookupInvoice(ctx, paymentHash)
			if err != nil {
				// Log but keep polling — transient NWC errors shouldn't abort
				continue
			}
			if status.IsPaid {
				return status, nil
			}
		}
	}
}
