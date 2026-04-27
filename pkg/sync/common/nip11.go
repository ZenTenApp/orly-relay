// Package common provides shared utilities for sync services
package common

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"git.smesh.lol/orly/pkg/nostr/relayinfo"
)

// NIP11Cache caches relay information documents with TTL
type NIP11Cache struct {
	cache map[string]*cachedRelayInfo
	mutex sync.RWMutex
	ttl   time.Duration
}

// cachedRelayInfo holds cached relay info with expiration
type cachedRelayInfo struct {
	info      *relayinfo.T
	expiresAt time.Time
}

// NewNIP11Cache creates a new NIP-11 cache with the specified TTL
func NewNIP11Cache(ttl time.Duration) *NIP11Cache {
	return &NIP11Cache{
		cache: make(map[string]*cachedRelayInfo),
		ttl:   ttl,
	}
}

// Get fetches relay information for a given URL, using cache if available
func (c *NIP11Cache) Get(ctx context.Context, relayURL string) (*relayinfo.T, error) {
	// Normalize URL - remove protocol and trailing slash
	normalizedURL := strings.TrimPrefix(relayURL, "https://")
	normalizedURL = strings.TrimPrefix(normalizedURL, "http://")
	normalizedURL = strings.TrimSuffix(normalizedURL, "/")

	// Check cache first
	c.mutex.RLock()
	if cached, exists := c.cache[normalizedURL]; exists && time.Now().Before(cached.expiresAt) {
		c.mutex.RUnlock()
		return cached.info, nil
	}
	c.mutex.RUnlock()

	// Fetch fresh data
	info, err := c.fetchNIP11(ctx, relayURL)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.mutex.Lock()
	c.cache[normalizedURL] = &cachedRelayInfo{
		info:      info,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mutex.Unlock()

	return info, nil
}

// fetchNIP11 fetches relay information document from a given URL
func (c *NIP11Cache) fetchNIP11(ctx context.Context, relayURL string) (*relayinfo.T, error) {
	// Convert WebSocket URL to HTTP URL for NIP-11 fetch
	// wss:// -> https://, ws:// -> http://
	nip11URL := relayURL
	nip11URL = strings.Replace(nip11URL, "wss://", "https://", 1)
	nip11URL = strings.Replace(nip11URL, "ws://", "http://", 1)
	if !strings.HasSuffix(nip11URL, "/") {
		nip11URL += "/"
	}
	nip11URL += ".well-known/nostr.json"

	// Create HTTP client with timeout
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
