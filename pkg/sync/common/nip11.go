// Package common provides shared utilities for sync services
package common

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/nostr/relayinfo"
)

// --- Actor request/response types ---

type nip11GetReq struct {
	ctx      context.Context
	relayURL string
	resp     chan nip11GetResp
}

type nip11GetResp struct {
	info *relayinfo.T
	err  error
}

// NIP11Cache caches relay information documents with TTL.
// All mutable state is owned by an actor goroutine.
type NIP11Cache struct {
	getCh chan nip11GetReq
	stop  chan struct{}
	done  chan struct{}
	ttl   time.Duration
}

// cachedRelayInfo holds cached relay info with expiration
type cachedRelayInfo struct {
	info      *relayinfo.T
	expiresAt time.Time
}

// NewNIP11Cache creates a new NIP-11 cache with the specified TTL
func NewNIP11Cache(ttl time.Duration) *NIP11Cache {
	c := &NIP11Cache{
		getCh: make(chan nip11GetReq),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
		ttl:   ttl,
	}
	go c.actorLoop()
	return c
}

func (c *NIP11Cache) actorLoop() {
	defer close(c.done)

	cache := make(map[string]*cachedRelayInfo)

	for {
		select {
		case <-c.stop:
			return
		case req := <-c.getCh:
			// Normalize URL
			normalizedURL := strings.TrimPrefix(req.relayURL, "https://")
			normalizedURL = strings.TrimPrefix(normalizedURL, "http://")
			normalizedURL = strings.TrimSuffix(normalizedURL, "/")

			// Check cache
			if cached, exists := cache[normalizedURL]; exists && time.Now().Before(cached.expiresAt) {
				req.resp <- nip11GetResp{info: cached.info}
				continue
			}

			// Fetch fresh data (blocking but with context timeout)
			info, err := fetchNIP11(req.ctx, req.relayURL)
			if err != nil {
				req.resp <- nip11GetResp{err: err}
				continue
			}

			// Cache the result
			cache[normalizedURL] = &cachedRelayInfo{
				info:      info,
				expiresAt: time.Now().Add(c.ttl),
			}
			req.resp <- nip11GetResp{info: info}
		}
	}
}

// Shutdown stops the actor goroutine.
func (c *NIP11Cache) Shutdown() {
	close(c.stop)
	<-c.done
}

// Get fetches relay information for a given URL, using cache if available
func (c *NIP11Cache) Get(ctx context.Context, relayURL string) (*relayinfo.T, error) {
	resp := make(chan nip11GetResp, 1)
	c.getCh <- nip11GetReq{ctx: ctx, relayURL: relayURL, resp: resp}
	r := <-resp
	return r.info, r.err
}

// fetchNIP11 fetches relay information document from a given URL
func fetchNIP11(ctx context.Context, relayURL string) (*relayinfo.T, error) {
	// Convert WebSocket URL to HTTP URL for NIP-11 fetch
	nip11URL := relayURL
	nip11URL = strings.Replace(nip11URL, "wss://", "https://", 1)
	nip11URL = strings.Replace(nip11URL, "ws://", "http://", 1)
	if !strings.HasSuffix(nip11URL, "/") {
		nip11URL += "/"
	}
	nip11URL += ".well-known/nostr.json"

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", nip11URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/nostr+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NIP-11 document from %s: %w", nip11URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NIP-11 request failed with status %d", resp.StatusCode)
	}

	var info relayinfo.T
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode NIP-11 document: %w", err)
	}

	return &info, nil
}

// GetPubkey fetches the relay's identity pubkey from its NIP-11 document
func (c *NIP11Cache) GetPubkey(ctx context.Context, relayURL string) (string, error) {
	info, err := c.Get(ctx, relayURL)
	if err != nil {
		return "", err
	}

	if info.PubKey == "" {
		return "", fmt.Errorf("relay %s does not provide pubkey in NIP-11 document", relayURL)
	}

	return info.PubKey, nil
}
