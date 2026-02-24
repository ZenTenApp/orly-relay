package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"next.orly.dev/pkg/protocol/nwc"
)

func main() {
	uri := os.Getenv("ORLY_NWC_URI")
	if uri == "" && len(os.Args) > 1 {
		uri = os.Args[1]
	}
	if uri == "" {
		fmt.Fprintf(os.Stderr, "usage: test-nwc <nwc-uri> or set ORLY_NWC_URI\n")
		os.Exit(1)
	}

	client, err := nwc.NewClient(uri)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse NWC URI: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("testing get_info...")
	var info map[string]any
	if err := client.Request(ctx, "get_info", nil, &info); err != nil {
		fmt.Fprintf(os.Stderr, "get_info failed: %v\n", err)
	} else {
		fmt.Printf("get_info result: %v\n", info)
	}

	fmt.Println("\ntesting make_invoice (100 sats)...")
	params := map[string]any{
		"amount":      int64(100000), // 100 sats in msats
		"description": "test invoice from test-nwc",
	}
	var invoice map[string]any
	if err := client.Request(ctx, "make_invoice", params, &invoice); err != nil {
		fmt.Fprintf(os.Stderr, "make_invoice failed: %v\n", err)
	} else {
		fmt.Printf("make_invoice result: %v\n", invoice)
	}
}
