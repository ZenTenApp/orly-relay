package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/minio/sha256-simd"
	"next.orly.dev/pkg/nostr/crypto/ec/secp256k1"
	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

var (
	nsec    = flag.String("nsec", "", "nostr private key (nsec or hex)")
	servers = flag.String("servers", "", "comma-separated server URLs (overrides defaults)")
	verbose = flag.Bool("v", false, "verbose output")
)

var defaultServers = []string{
	"https://blossom.primal.net",
	"https://nostr.download",
	"https://blossom.oxtr.dev",
	"https://blossom.nostr.build",
	"https://blossom.band",
	"https://blossom.nogood.studio",
	"https://blossom.hzrd149.com",
	"https://cdn.hzrd149.com",
	"https://blossom.f7z.io",
	"https://blossom.azzamo.net",
	"https://files.sovbit.host",
	"https://nosto.re",
	"https://nostrmedia.com",
	"https://cdn.nostrcheck.me",
	"https://media-server.slidestr.net",
	"https://cdn.satellite.earth",
	"https://files.v0l.io",
}

func main() {
	flag.Parse()
	if *nsec == "" {
		fmt.Fprintln(os.Stderr, "usage: blossom-upload -nsec <key> file1 [file2 ...]")
		os.Exit(1)
	}
	files := flag.Args()
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no files specified")
		os.Exit(1)
	}

	sec, pub, err := parseKey(*nsec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "key error: %v\n", err)
		os.Exit(1)
	}

	srvs := defaultServers
	if *servers != "" {
		srvs = strings.Split(*servers, ",")
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
			continue
		}
		hash := sha256.Sum256(data)
		hashHex := hex.EncodeToString(hash[:])
		ct := mimeType(path)
		fmt.Printf("%s (%d bytes, %s, sha256:%s)\n", filepath.Base(path), len(data), ct, hashHex[:16]+"...")

		var wg sync.WaitGroup
		for _, srv := range srvs {
			wg.Add(1)
			go func(srv string) {
				defer wg.Done()
				url, err := upload(sec, pub, srv, data, ct)
				if err != nil {
					fmt.Printf("  %s: FAIL %v\n", srv, err)
				} else {
					fmt.Printf("  %s: OK %s\n", srv, url)
				}
			}(strings.TrimSpace(srv))
		}
		wg.Wait()
		fmt.Println()
	}
}

func parseKey(key string) (sec, pub []byte, err error) {
	if strings.HasPrefix(key, "nsec1") {
		var sk *secp256k1.SecretKey
		sk, err = bech32encoding.NsecToSecretKey(key)
		if err != nil {
			return
		}
		sec = sk.Serialize()
	} else {
		sec, err = hex.DecodeString(key)
		if err != nil {
			return
		}
	}
	var s *p8k.Signer
	s, err = p8k.New()
	if err != nil {
		return
	}
	if err = s.InitSec(sec); err != nil {
		return
	}
	pub = s.Pub()
	return
}

func authHeader(sec, pub []byte, action, hashHex string) (string, error) {
	now := time.Now().Unix()
	tags := [][]string{
		{"t", action},
		{"expiration", fmt.Sprintf("%d", now+300)},
	}
	if hashHex != "" {
		tags = append(tags, []string{"x", hashHex})
	}
	pubHex := hex.EncodeToString(pub)

	// event ID = sha256 of serialized [0, pubkey, created_at, kind, tags, content]
	ser, err := json.Marshal([]interface{}{0, pubHex, now, 24242, tags, ""})
	if err != nil {
		return "", err
	}
	idHash := sha256.Sum256(ser)

	signer, err := p8k.New()
	if err != nil {
		return "", err
	}
	if err = signer.InitSec(sec); err != nil {
		return "", err
	}
	sig, err := signer.Sign(idHash[:])
	if err != nil {
		return "", err
	}

	ev := map[string]interface{}{
		"id":         hex.EncodeToString(idHash[:]),
		"pubkey":     pubHex,
		"created_at": now,
		"kind":       24242,
		"tags":       tags,
		"content":    "",
		"sig":        hex.EncodeToString(sig),
	}
	j, err := json.Marshal(ev)
	if err != nil {
		return "", err
	}
	if *verbose {
		fmt.Printf("    auth: %s\n", j)
	}
	return "Nostr " + base64.StdEncoding.EncodeToString(j), nil
}

func upload(sec, pub []byte, server string, data []byte, contentType string) (string, error) {
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	auth, err := authHeader(sec, pub, "upload", hashHex)
	if err != nil {
		return "", err
	}

	url := strings.TrimSuffix(server, "/") + "/upload"
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", auth)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		reason := resp.Header.Get("X-Reason")
		if reason == "" {
			reason = string(body)
			if len(reason) > 200 {
				reason = reason[:200]
			}
		}
		return "", fmt.Errorf("%d: %s", resp.StatusCode, reason)
	}

	var desc struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &desc); err != nil {
		return string(body), nil
	}
	return desc.URL, nil
}

func mimeType(path string) string {
	ext := filepath.Ext(path)
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		ct = "application/octet-stream"
	}
	return ct
}
