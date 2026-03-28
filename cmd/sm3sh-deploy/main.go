package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

const chunkSize = 512 * 1024 // 512 KB

func main() {
	url := flag.String("url", "", "smesh base URL (e.g. https://smesh.lol)")
	nsec := flag.String("nsec", "", "deploy nsec (or set DEPLOY_NSEC env)")
	dir := flag.String("dir", "app/smesh3", "directory to bundle")
	flag.Parse()

	if *url == "" {
		fatal("--url required")
	}

	nsecStr := *nsec
	if nsecStr == "" {
		nsecStr = os.Getenv("DEPLOY_NSEC")
	}
	if nsecStr == "" {
		fatal("--nsec or DEPLOY_NSEC required")
	}

	skBytes, err := bech32encoding.NsecToBytes(nsecStr)
	if err != nil {
		fatal("invalid nsec: %v", err)
	}

	sign, err := p8k.New()
	if err != nil {
		fatal("signer init: %v", err)
	}
	if err := sign.InitSec(skBytes); err != nil {
		fatal("init secret key: %v", err)
	}

	fmt.Printf("deploy pubkey: %x\n", sign.Pub())

	bundle, err := createBundle(*dir)
	if err != nil {
		fatal("bundle: %v", err)
	}
	fmt.Printf("bundle: %d bytes (xz -9) from %s\n", len(bundle), *dir)

	hash := sha256.Sum256(bundle)
	hashHex := hex.EncodeToString(hash[:])
	sig, err := sign.Sign(hash[:])
	if err != nil {
		fatal("sign: %v", err)
	}

	endpoint := strings.TrimRight(*url, "/") + "/__deploy"

	// Split into chunks.
	nParts := (len(bundle) + chunkSize - 1) / chunkSize
	fmt.Printf("uploading %d parts (%d KB each)...\n", nParts, chunkSize/1024)

	// Begin session.
	resp := doPost(endpoint+"?action=begin&hash="+hashHex+"&parts="+strconv.Itoa(nParts), nil)
	if resp != "ok" {
		fatal("begin failed: %s", resp)
	}

	// Upload parts.
	for i := 0; i < nParts; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(bundle) {
			end = len(bundle)
		}
		chunk := bundle[start:end]

		resp := doPost(
			endpoint+"?action=part&hash="+hashHex+"&index="+strconv.Itoa(i),
			chunk,
		)
		fmt.Printf("  part %d/%d (%d bytes): %s\n", i+1, nParts, len(chunk), resp)
	}

	// Apply with signature.
	req, err := http.NewRequest(http.MethodPost, endpoint+"?action=apply&hash="+hashHex, nil)
	if err != nil {
		fatal("request: %v", err)
	}
	req.Header.Set("X-Sig", hex.EncodeToString(sig))
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal("apply: %v", err)
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)
	if r.StatusCode != 200 {
		fatal("apply failed (%d): %s", r.StatusCode, string(body))
	}
	fmt.Printf("deployed: %s", string(body))
}

func doPost(url string, body []byte) string {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(http.MethodPost, url, reader)
	if err != nil {
		fatal("request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal("upload: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fatal("failed (%d): %s", resp.StatusCode, string(b))
	}
	return string(b)
}

// createBundle creates a tar.xz archive of dir using xz -9.
func createBundle(dir string) ([]byte, error) {
	// Create tar first.
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)

	base := filepath.Clean(dir)
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	// Compress with xz -9.
	var xzBuf bytes.Buffer
	cmd := exec.Command("xz", "-9", "--stdout")
	cmd.Stdin = &tarBuf
	cmd.Stdout = &xzBuf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("xz: %w", err)
	}

	return xzBuf.Bytes(), nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
