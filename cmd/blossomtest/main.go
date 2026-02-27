package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"next.orly.dev/pkg/nostr/crypto/ec/schnorr"
	"next.orly.dev/pkg/nostr/crypto/ec/secp256k1"
	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"github.com/minio/sha256-simd"
)

const (
	// BlossomAuthKind is the Nostr event kind for Blossom authorization (BUD-01)
	BlossomAuthKind = 24242
)

var (
	relayURL = flag.String("url", "http://localhost:3334", "Relay base URL")
	nsec     = flag.String("nsec", "", "Nostr private key (nsec format). If empty, generates a new key")
	blobSize = flag.Int("size", 1024, "Size of test blob in bytes")
	verbose  = flag.Bool("v", false, "Verbose output")
	noAuth   = flag.Bool("no-auth", false, "Skip authentication (test anonymous uploads)")
)

// BlossomDescriptor represents a blob descriptor returned by the server
type BlossomDescriptor struct {
	URL       string     `json:"url"`
	SHA256    string     `json:"sha256"`
	Size      int64      `json:"size"`
	Type      string     `json:"type,omitempty"`
	Uploaded  int64      `json:"uploaded"`
	PublicKey string     `json:"public_key,omitempty"`
	Tags      [][]string `json:"tags,omitempty"`
}

func main() {
	flag.Parse()

	fmt.Println("🌸 Blossom Test Tool")
	fmt.Println("===================")

	// Get or generate keypair (only if auth is enabled)
	var sec, pub []byte
	var err error

	if !*noAuth {
		sec, pub, err = getKeypair()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting keypair: %v\n", err)
			os.Exit(1)
		}

		pubkey, _ := schnorr.ParsePubKey(pub)
		npubBytes, _ := bech32encoding.PublicKeyToNpub(pubkey)
		fmt.Printf("Using identity: %s\n", string(npubBytes))
	} else {
		fmt.Printf("Testing anonymous uploads (no authentication)\n")
	}
	fmt.Printf("Relay URL: %s\n\n", *relayURL)

	// Generate random test data
	testData := make([]byte, *blobSize)
	if _, err := rand.Read(testData); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating test data: %v\n", err)
		os.Exit(1)
	}

	// Calculate SHA256
	hash := sha256.Sum256(testData)
	hashHex := hex.EncodeToString(hash[:])

	fmt.Printf("📦 Generated %d bytes of random data\n", *blobSize)
	fmt.Printf("   SHA256: %s\n\n", hashHex)

	// Step 1: Upload blob
	fmt.Println("📤 Step 1: Uploading blob...")
	descriptor, err := uploadBlob(sec, pub, testData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Upload failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Upload successful!\n")
	fmt.Printf("   URL: %s\n", descriptor.URL)
	fmt.Printf("   SHA256: %s\n", descriptor.SHA256)
	fmt.Printf("   Size: %d bytes\n\n", descriptor.Size)

	// Step 2: Fetch blob
	fmt.Println("📥 Step 2: Fetching blob...")
	fetchedData, err := fetchBlob(hashHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Fetch failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Fetch successful! Retrieved %d bytes\n", len(fetchedData))

	// Verify data matches
	if !bytes.Equal(testData, fetchedData) {
		fmt.Fprintf(os.Stderr, "❌ Data mismatch! Retrieved data doesn't match uploaded data\n")
		os.Exit(1)
	}
	fmt.Printf("✅ Data verification passed - hashes match!\n\n")

	// Step 3: Delete blob
	fmt.Println("🗑️  Step 3: Deleting blob...")
	if err := deleteBlob(sec, pub, hashHex); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Delete failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Delete successful!\n\n")

	// Step 4: Verify deletion
	fmt.Println("🔍 Step 4: Verifying deletion...")
	if err := verifyDeleted(hashHex); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Verification failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Blob successfully deleted - returns 404 as expected\n\n")

	fmt.Println("🎉 All tests passed! Blossom service is working correctly.")
}

func getKeypair() (sec, pub []byte, err error) {
	if *nsec != "" {
		// Decode provided nsec
		var secKey *secp256k1.SecretKey
		secKey, err = bech32encoding.NsecToSecretKey(*nsec)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid nsec: %w", err)
		}
		sec = secKey.Serialize()
	} else {
		// Generate new keypair
		sec = make([]byte, 32)
		if _, err := rand.Read(sec); err != nil {
			return nil, nil, fmt.Errorf("failed to generate key: %w", err)
		}
		fmt.Println("ℹ️  No key provided, generated new keypair")
	}

	// Derive public key using p8k signer
	var signer *p8k.Signer
	if signer, err = p8k.New(); err != nil {
		return nil, nil, fmt.Errorf("failed to create signer: %w", err)
	}
	if err = signer.InitSec(sec); err != nil {
		return nil, nil, fmt.Errorf("failed to initialize signer: %w", err)
	}
	pub = signer.Pub()

	return sec, pub, nil
}

// createAuthEvent creates a Blossom authorization event (kind 24242)
func createAuthEvent(sec, pub []byte, action, hash string) (string, error) {
	now := time.Now().Unix()

	// Build tags based on action
	tags := [][]string{
		{"t", action},
	}

	// Add x tag for DELETE and GET actions
	if hash != "" && (action == "delete" || action == "get") {
		tags = append(tags, []string{"x", hash})
	}

	// All Blossom auth events require expiration tag (BUD-01)
	expiry := now + 300 // Event expires in 5 minutes
	tags = append(tags, []string{"expiration", fmt.Sprintf("%d", expiry)})

	pubkeyHex := hex.EncodeToString(pub)

	// Create event ID
	eventJSON, err := json.Marshal([]interface{}{
		0,
		pubkeyHex,
		now,
		BlossomAuthKind,
		tags,
		"",
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal event for ID: %w", err)
	}

	eventHash := sha256.Sum256(eventJSON)
	eventID := hex.EncodeToString(eventHash[:])

	// Sign the event using p8k signer
	signer, err := p8k.New()
	if err != nil {
		return "", fmt.Errorf("failed to create signer: %w", err)
	}
	if err = signer.InitSec(sec); err != nil {
		return "", fmt.Errorf("failed to initialize signer: %w", err)
	}

	sig, err := signer.Sign(eventHash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign event: %w", err)
	}
	sigHex := hex.EncodeToString(sig)

	// Create event JSON (signed)
	event := map[string]interface{}{
		"id":         eventID,
		"pubkey":     pubkeyHex,
		"created_at": now,
		"kind":       BlossomAuthKind,
		"tags":       tags,
		"content":    "",
		"sig":        sigHex,
	}

	// Marshal to JSON for Authorization header
	authJSON, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth event: %w", err)
	}

	if *verbose {
		fmt.Printf("   Auth event: %s\n", string(authJSON))
	}

	return string(authJSON), nil
}

func uploadBlob(sec, pub, data []byte) (*BlossomDescriptor, error) {
	// Create request
	url := strings.TrimSuffix(*relayURL, "/") + "/blossom/upload"
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/octet-stream")

	// Add authorization if not disabled
	if !*noAuth && sec != nil && pub != nil {
		authEvent, err := createAuthEvent(sec, pub, "upload", "")
		if err != nil {
			return nil, err
		}
		// Base64-encode the auth event as per BUD-01
		authEventB64 := base64.StdEncoding.EncodeToString([]byte(authEvent))
		req.Header.Set("Authorization", "Nostr "+authEventB64)
	}

	if *verbose {
		fmt.Printf("   PUT %s\n", url)
		fmt.Printf("   Content-Length: %d\n", len(data))
	}

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		reason := resp.Header.Get("X-Reason")
		if reason == "" {
			reason = string(body)
		}
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, reason)
	}

	// Parse descriptor
	var descriptor BlossomDescriptor
	if err := json.Unmarshal(body, &descriptor); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	return &descriptor, nil
}

func fetchBlob(hash string) ([]byte, error) {
	url := strings.TrimSuffix(*relayURL, "/") + "/blossom/" + hash

	if *verbose {
		fmt.Printf("   GET %s\n", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func deleteBlob(sec, pub []byte, hash string) error {
	// Create request
	url := strings.TrimSuffix(*relayURL, "/") + "/blossom/" + hash
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	// Add authorization if not disabled
	if !*noAuth && sec != nil && pub != nil {
		authEvent, err := createAuthEvent(sec, pub, "delete", hash)
		if err != nil {
			return err
		}
		// Base64-encode the auth event as per BUD-01
		authEventB64 := base64.StdEncoding.EncodeToString([]byte(authEvent))
		req.Header.Set("Authorization", "Nostr "+authEventB64)
	}

	if *verbose {
		fmt.Printf("   DELETE %s\n", url)
	}

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		reason := resp.Header.Get("X-Reason")
		if reason == "" {
			reason = string(body)
		}
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, reason)
	}

	return nil
}

func verifyDeleted(hash string) error {
	url := strings.TrimSuffix(*relayURL, "/") + "/blossom/" + hash

	if *verbose {
		fmt.Printf("   GET %s (expecting 404)\n", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return fmt.Errorf("blob still exists (expected 404, got 200)")
	}

	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unexpected status code: %d (expected 404)", resp.StatusCode)
	}

	return nil
}
