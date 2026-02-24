// Package testsubscribe provides a CLI command for end-to-end testing of the
// paid subscription flow: create invoice via NWC, pay it (loopback), activate
// subscription via ACL gRPC.
package testsubscribe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	aclgrpc "next.orly.dev/pkg/acl/grpc"
	"next.orly.dev/pkg/protocol/nwc"
)

func Run(args []string) {
	var (
		pubkey    string
		alias     string
		aclAddr   string
		nwcURI    string
		priceSats int64 = 100
	)

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--pubkey="):
			pubkey = strings.TrimPrefix(arg, "--pubkey=")
		case strings.HasPrefix(arg, "--alias="):
			alias = strings.TrimPrefix(arg, "--alias=")
		case strings.HasPrefix(arg, "--acl="):
			aclAddr = strings.TrimPrefix(arg, "--acl=")
		case strings.HasPrefix(arg, "--nwc="):
			nwcURI = strings.TrimPrefix(arg, "--nwc=")
		case arg == "--help" || arg == "-h":
			printHelp()
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown argument: %s\n", arg)
			printHelp()
			os.Exit(1)
		}
	}

	// Fallback to env vars
	if nwcURI == "" {
		nwcURI = os.Getenv("ORLY_BRIDGE_NWC_URI")
	}
	if nwcURI == "" {
		nwcURI = os.Getenv("ORLY_NWC_URI")
	}
	if aclAddr == "" {
		aclAddr = os.Getenv("ORLY_BRIDGE_ACL_GRPC_SERVER")
	}
	if aclAddr == "" {
		aclAddr = os.Getenv("ORLY_GRPC_ACL")
	}

	if nwcURI == "" {
		fatal("NWC URI required: --nwc=<uri> or ORLY_NWC_URI env var")
	}
	if aclAddr == "" {
		fatal("ACL gRPC address required: --acl=<addr> or ORLY_GRPC_ACL env var")
	}

	// Generate random pubkey if not specified
	if pubkey == "" {
		var buf [32]byte
		if _, err := rand.Read(buf[:]); err != nil {
			fatal("generate random pubkey: %v", err)
		}
		pubkey = hex.EncodeToString(buf[:])
		fmt.Printf("generated test pubkey: %s\n", pubkey)
	}

	if alias != "" {
		priceSats = 200
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Connect to ACL gRPC
	fmt.Printf("connecting to ACL gRPC at %s...\n", aclAddr)
	aclClient, err := aclgrpc.New(ctx, &aclgrpc.ClientConfig{
		ServerAddress:  aclAddr,
		ConnectTimeout: 10 * time.Second,
	})
	if err != nil {
		fatal("ACL gRPC connect: %v", err)
	}
	defer aclClient.Close()
	fmt.Println("ACL connected")

	// 2. Check existing subscription
	subscribed, err := aclClient.IsSubscribedPaid(pubkey)
	if err != nil {
		fmt.Printf("warning: IsSubscribedPaid check failed: %v\n", err)
	} else if subscribed {
		fmt.Println("pubkey already has active subscription")
	}

	// 3. If alias, check availability
	if alias != "" {
		taken, err := aclClient.IsAliasTaken(alias)
		if err != nil {
			fatal("IsAliasTaken: %v", err)
		}
		if taken {
			fatal("alias %q is already taken", alias)
		}
		fmt.Printf("alias %q is available\n", alias)
	}

	// 4. Connect NWC
	fmt.Println("connecting to NWC wallet...")
	nwcClient, err := nwc.NewClient(nwcURI)
	if err != nil {
		fatal("NWC client: %v", err)
	}

	// 5. Create invoice
	fmt.Printf("creating invoice for %d sats...\n", priceSats)
	var invoice struct {
		Bolt11      string `json:"invoice"`
		PaymentHash string `json:"payment_hash"`
		Amount      int64  `json:"amount"`
	}
	err = nwcClient.Request(ctx, "make_invoice", map[string]any{
		"amount":      priceSats * 1000, // msats
		"description": fmt.Sprintf("test-subscribe: %s", pubkey[:16]),
	}, &invoice)
	if err != nil {
		fatal("make_invoice: %v", err)
	}
	fmt.Printf("invoice created: %s\n", invoice.PaymentHash)
	fmt.Printf("bolt11: %s\n", invoice.Bolt11)

	// 6. Pay invoice (loopback)
	fmt.Println("paying invoice (loopback)...")
	var payResult map[string]any
	err = nwcClient.Request(ctx, "pay_invoice", map[string]any{
		"invoice": invoice.Bolt11,
	}, &payResult)
	if err != nil {
		fatal("pay_invoice: %v", err)
	}
	fmt.Println("payment sent")

	// 7. Poll for settlement
	fmt.Println("waiting for settlement...")
	settled := false
	for i := 0; i < 10; i++ {
		var status struct {
			PaymentHash string `json:"payment_hash"`
			Preimage    string `json:"preimage"`
			SettledAt   int64  `json:"settled_at"`
		}
		err = nwcClient.Request(ctx, "lookup_invoice", map[string]any{
			"payment_hash": invoice.PaymentHash,
		}, &status)
		if err != nil {
			fmt.Printf("  lookup attempt %d failed: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if status.SettledAt > 0 || status.Preimage != "" {
			fmt.Printf("invoice settled (preimage: %s)\n", status.Preimage)
			settled = true
			break
		}
		fmt.Printf("  not yet settled, polling (%d/10)...\n", i+1)
		time.Sleep(2 * time.Second)
	}
	if !settled {
		fatal("invoice not settled after 10 attempts")
	}

	// 8. Activate subscription via ACL gRPC
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	fmt.Printf("activating subscription (expires %s)...\n", expiresAt.Format("2006-01-02"))
	err = aclClient.SubscribePubkey(pubkey, expiresAt, invoice.PaymentHash, alias)
	if err != nil {
		fatal("SubscribePubkey: %v", err)
	}
	fmt.Println("subscription activated")

	// 9. Claim alias if requested
	if alias != "" {
		fmt.Printf("claiming alias %q...\n", alias)
		err = aclClient.ClaimAlias(alias, pubkey)
		if err != nil {
			fatal("ClaimAlias: %v", err)
		}
		fmt.Printf("alias %q claimed\n", alias)
	}

	// 10. Verify
	subscribed, err = aclClient.IsSubscribedPaid(pubkey)
	if err != nil {
		fatal("verify IsSubscribedPaid: %v", err)
	}
	if !subscribed {
		fatal("FAIL: subscription not active after activation")
	}

	fmt.Println("\n--- PASS ---")
	fmt.Printf("pubkey:  %s\n", pubkey)
	if alias != "" {
		fmt.Printf("alias:   %s\n", alias)
	}
	fmt.Printf("expires: %s\n", expiresAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("cost:    %d sats\n", priceSats)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}

func printHelp() {
	fmt.Println(`orly test-subscribe - End-to-end test of paid subscription flow

Usage:
  orly test-subscribe [options]

Options:
  --pubkey=HEX    Pubkey to subscribe (random if omitted)
  --alias=NAME    Also claim an alias (doubles price)
  --acl=ADDR      ACL gRPC address (default: ORLY_GRPC_ACL env)
  --nwc=URI       NWC connection string (default: ORLY_NWC_URI env)

Flow:
  1. Connect to ACL gRPC server
  2. Create Lightning invoice via NWC
  3. Pay the invoice from the same wallet (loopback)
  4. Wait for settlement
  5. Activate subscription via ACL gRPC
  6. Verify subscription is active

Environment variables:
  ORLY_NWC_URI                  NWC connection string
  ORLY_BRIDGE_NWC_URI           Bridge-specific NWC (takes priority)
  ORLY_GRPC_ACL                 ACL gRPC server address
  ORLY_BRIDGE_ACL_GRPC_SERVER   Bridge-specific ACL address (takes priority)`)
}
