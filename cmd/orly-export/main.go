// Package main is a CLI tool to export all events from an ORLY relay via the
// /api/export HTTP endpoint using NIP-98 authentication.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"

	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/httpauth"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"

	"git.smesh.lol/orly/pkg/version"
)

var userAgent = fmt.Sprintf("orly-export/%s", strings.TrimSpace(version.V))

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}

// progressWriter wraps an io.Writer and tracks bytes written and line count.
type progressWriter struct {
	w          io.Writer
	bytes      int64
	lines      int64
	lastReport time.Time
	start      time.Time
}

func newProgressWriter(w io.Writer) *progressWriter {
	now := time.Now()
	return &progressWriter{w: w, start: now, lastReport: now}
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.w.Write(p)
	pw.bytes += int64(n)
	for _, b := range p[:n] {
		if b == '\n' {
			pw.lines++
		}
	}
	if time.Since(pw.lastReport) >= 2*time.Second {
		pw.report()
		pw.lastReport = time.Now()
	}
	return
}

func (pw *progressWriter) report() {
	elapsed := time.Since(pw.start)
	mb := float64(pw.bytes) / 1024 / 1024
	fmt.Fprintf(os.Stderr, "\r  %d events, %.2f MB downloaded (%.1fs)",
		pw.lines, mb, elapsed.Seconds())
}

func (pw *progressWriter) final() {
	elapsed := time.Since(pw.start)
	mb := float64(pw.bytes) / 1024 / 1024
	fmt.Fprintf(os.Stderr, "\r  %d events, %.2f MB downloaded in %.1fs\n",
		pw.lines, mb, elapsed.Seconds())
}

func main() {
	var (
		relayURL string
		nsec     string
		output   string
	)

	flag.StringVar(&relayURL, "relay", "", "relay URL (e.g. https://plebeian.market)")
	flag.StringVar(&nsec, "nsec", "", "nsec (bech32) for NIP-98 auth (or set NOSTR_SECRET_KEY)")
	flag.StringVar(&output, "output", "", "output file path (default: auto-generated)")
	flag.Parse()

	if relayURL == "" {
		fail("--relay is required (e.g. --relay https://plebeian.market)")
	}

	// Normalize the relay URL
	relayURL = strings.TrimRight(relayURL, "/")
	if !strings.HasPrefix(relayURL, "http") {
		relayURL = "https://" + relayURL
	}

	// Get nsec from flag or env
	if nsec == "" {
		nsec = os.Getenv("NOSTR_SECRET_KEY")
	}

	var sign signer.I
	if nsec != "" {
		var err error
		sign, err = makeSigner(nsec)
		if err != nil {
			fail("failed to initialize signer: %s", err)
		}
		fmt.Fprintf(os.Stderr, "authenticated with NIP-98\n")
	} else {
		fmt.Fprintf(os.Stderr, "warning: no nsec provided, attempting unauthenticated export\n")
	}

	// Build export URL
	exportURL := relayURL + "/api/export"
	ur, err := url.Parse(exportURL)
	if err != nil {
		fail("invalid URL: %s", err)
	}

	// Determine output file
	if output == "" {
		host := ur.Hostname()
		host = strings.ReplaceAll(host, ".", "-")
		output = fmt.Sprintf("export-%s-%s.jsonl",
			host,
			time.Now().UTC().Format("20060102-150405Z"))
	}

	fmt.Fprintf(os.Stderr, "exporting from %s -> %s\n", exportURL, output)

	// Create HTTP request
	req, err := http.NewRequest("GET", exportURL, nil)
	if err != nil {
		fail("failed to create request: %s", err)
	}
	req.Header.Set("User-Agent", userAgent)

	if sign != nil {
		if err = httpauth.AddNIP98Header(req, ur, "GET", "", sign, 0); chk.E(err) {
			fail("failed to add NIP-98 header: %s", err)
		}
	}

	// Execute request with no timeout (export can be large)
	client := &http.Client{
		Timeout: 0,
	}
	resp, err := client.Do(req)
	if err != nil {
		fail("request failed: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		fail("server returned %d: %s", resp.StatusCode, string(body))
	}

	// Open output file
	f, err := os.Create(output)
	if err != nil {
		fail("failed to create output file: %s", err)
	}
	defer f.Close()

	// Stream response to file with progress tracking
	pw := newProgressWriter(f)
	if _, err = io.Copy(pw, resp.Body); err != nil {
		fmt.Fprintln(os.Stderr)
		fail("download error: %s", err)
	}
	pw.final()

	fmt.Fprintf(os.Stderr, "export saved to %s\n", output)
}

func makeSigner(nsec string) (signer.I, error) {
	sk, err := bech32encoding.NsecToBytes([]byte(nsec))
	if err != nil {
		return nil, fmt.Errorf("failed to decode nsec: %w", err)
	}
	s, err := p8k.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}
	if err = s.InitSec(sk); err != nil {
		return nil, fmt.Errorf("failed to init signer: %w", err)
	}
	return s, nil
}
