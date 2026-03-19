//go:build interop

package marmot

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/emersion/go-mls"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

var interopBinaryPath string

func TestMain(m *testing.M) {
	// Locate testdata/interop relative to this source file.
	_, thisFile, _, _ := runtime.Caller(0)
	crateDir := filepath.Join(filepath.Dir(thisFile), "testdata", "interop")

	interopBinaryPath = filepath.Join(crateDir, "target", "release", "mls-interop")

	// Build if missing or stale (cargo handles staleness).
	if _, err := exec.LookPath("cargo"); err == nil {
		fmt.Println("building mls-interop (cargo build --release)...")
		cmd := exec.Command("cargo", "build", "--release")
		cmd.Dir = crateDir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "cargo build failed: %v\n", err)
			os.Exit(1)
		}
	}

	os.Exit(m.Run())
}

type rustProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
}

type rustResponse struct {
	OK              bool   `json:"ok"`
	Error           string `json:"error,omitempty"`
	EventJSON       string `json:"event_json,omitempty"`
	RumorJSON       string `json:"rumor_json,omitempty"`
	Pubkey          string `json:"pubkey,omitempty"`
	Content         string `json:"content,omitempty"`
	Kind            uint16 `json:"kind,omitempty"`
	MLSGroupIDHex   string `json:"mls_group_id_hex,omitempty"`
	NostrGroupIDHex string `json:"nostr_group_id_hex,omitempty"`
}

type rustInit struct {
	Ready  bool   `json:"ready"`
	Pubkey string `json:"pubkey"`
}

func startRust(t *testing.T) (*rustProcess, string) {
	t.Helper()
	if _, err := os.Stat(interopBinaryPath); os.IsNotExist(err) {
		t.Skipf("interop binary not found at %s; build it from testdata/interop", interopBinaryPath)
	}

	cmd := exec.Command(interopBinaryPath)
	cmd.Env = append(os.Environ(), "RUST_BACKTRACE=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrFile, _ := os.Create("/tmp/interop-stderr.log")
	cmd.Stderr = stderrFile
	t.Cleanup(func() { stderrFile.Close() })
	if err := cmd.Start(); err != nil {
		t.Fatalf("start rust binary: %v", err)
	}
	t.Cleanup(func() {
		stdin.Close()
		cmd.Wait()
	})

	reader := bufio.NewReader(stdout)

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read init: %v", err)
	}
	var init rustInit
	if err := json.Unmarshal([]byte(line), &init); err != nil {
		t.Fatalf("parse init: %v (line: %s)", err, line)
	}
	if !init.Ready {
		t.Fatal("rust binary not ready")
	}

	return &rustProcess{cmd: cmd, stdin: stdin, reader: reader}, init.Pubkey
}

func (r *rustProcess) send(t *testing.T, cmd interface{}) rustResponse {
	t.Helper()
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	if _, err := fmt.Fprintf(r.stdin, "%s\n", data); err != nil {
		t.Fatalf("write command: %v", err)
	}
	line, err := r.reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	var resp rustResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("parse response: %v (line: %s)", err, line)
	}
	return resp
}

// TestInterop_KeyPackageFormat validates Go kind 443 events are accepted
// by Rust MDK, and Rust kind 443 events are parsed by Go.
func TestInterop_KeyPackageFormat(t *testing.T) {
	rust, _ := startRust(t)

	goSign, err := p8k.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := goSign.Generate(); err != nil {
		t.Fatal(err)
	}

	// Go generates key package event
	kpp, err := GenerateKeyPackage(goSign)
	if err != nil {
		t.Fatalf("go GenerateKeyPackage: %v", err)
	}
	ev, err := KeyPackageToEvent(kpp, goSign, []string{"wss://relay.example.com"})
	if err != nil {
		t.Fatalf("go KeyPackageToEvent: %v", err)
	}
	evJSON, err := ev.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	// First test: parse raw KP bytes directly through OpenMLS
	kpRawB64 := base64.StdEncoding.EncodeToString(kpp.Public.RawBytes())
	resp := rust.send(t, map[string]string{
		"cmd":       "parse_kp_raw",
		"kp_base64": kpRawB64,
	})
	if !resp.OK {
		t.Fatalf("rust OpenMLS rejected Go raw KP bytes: %s", resp.Error)
	}
	t.Logf("Rust OpenMLS parsed raw Go KP: %s", resp.Content)

	// Rust validates full event
	resp = rust.send(t, map[string]string{
		"cmd":        "process_key_package",
		"event_json": string(evJSON),
	})
	if !resp.OK {
		t.Fatalf("rust rejected Go key package: %s", resp.Error)
	}
	t.Logf("Rust accepted Go key package (pubkey: %s)", resp.Pubkey)

	// Rust generates key package event
	resp = rust.send(t, map[string]string{
		"cmd":   "generate_key_package",
		"relay": "wss://relay.example.com",
	})
	if !resp.OK {
		t.Fatalf("rust generate_key_package failed: %s", resp.Error)
	}

	// Go parses it
	var rustEv event.E
	if err := rustEv.UnmarshalJSON([]byte(resp.EventJSON)); err != nil {
		t.Fatalf("parse rust event: %v", err)
	}
	if rustEv.Kind != KindKeyPackage {
		t.Fatalf("wrong kind: want %d, got %d", KindKeyPackage, rustEv.Kind)
	}
	rustKP, err := EventToKeyPackage(&rustEv)
	if err != nil {
		t.Fatalf("go EventToKeyPackage from rust: %v", err)
	}
	if rustKP == nil {
		t.Fatal("nil key package from rust")
	}
	t.Log("Go parsed Rust key package")
}

// TestInterop_WelcomeAndMessage validates the full group lifecycle:
// Rust creates group with Go's key package, Go joins via welcome,
// then Rust sends a message that Go decrypts.
func TestInterop_WelcomeAndMessage(t *testing.T) {
	rust, _ := startRust(t)

	goSign, err := p8k.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := goSign.Generate(); err != nil {
		t.Fatal(err)
	}

	// Go generates key package
	goKPP, err := GenerateKeyPackage(goSign)
	if err != nil {
		t.Fatalf("go GenerateKeyPackage: %v", err)
	}
	goKPEv, err := KeyPackageToEvent(goKPP, goSign, []string{"wss://relay.example.com"})
	if err != nil {
		t.Fatalf("go KeyPackageToEvent: %v", err)
	}
	goKPJSON, err := goKPEv.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal kp event: %v", err)
	}

	// Rust creates group with Go's key package
	resp := rust.send(t, map[string]string{
		"cmd":                  "create_group",
		"member_kp_event_json": string(goKPJSON),
		"name":                 "interop-test",
		"relay":                "wss://relay.example.com",
	})
	if !resp.OK {
		t.Fatalf("rust create_group failed: %s", resp.Error)
	}
	t.Logf("Rust created group (mls_group_id: %s, nostr_group_id: %s)",
		resp.MLSGroupIDHex, resp.NostrGroupIDHex)
	rustMLSGroupID := resp.MLSGroupIDHex
	rustNostrGroupID := resp.NostrGroupIDHex

	// Parse the Rust welcome rumor (kind 444 unsigned event)
	rumorJSON := resp.RumorJSON

	var rumorEv event.E
	if err := rumorEv.UnmarshalJSON([]byte(rumorJSON)); err != nil {
		t.Fatalf("parse rust welcome rumor: %v", err)
	}
	if rumorEv.Kind != KindWelcome {
		t.Fatalf("wrong rumor kind: want %d, got %d", KindWelcome, rumorEv.Kind)
	}

	// Check encoding tag
	encodingTag := rumorEv.Tags.GetFirst([]byte("encoding"))
	if encodingTag == nil {
		t.Fatal("missing 'encoding' tag on welcome rumor")
	}
	if string(encodingTag.Value()) != "base64" {
		t.Fatalf("wrong encoding: %s", string(encodingTag.Value()))
	}

	// Decode the welcome content
	welcomeBytes, err := base64.StdEncoding.DecodeString(string(rumorEv.Content))
	if err != nil {
		t.Fatalf("base64 decode welcome: %v", err)
	}

	// Parse MLS Welcome
	welcome, err := mls.UnmarshalWelcome(welcomeBytes)
	if err != nil {
		t.Fatalf("unmarshal welcome: %v", err)
	}

	// Go joins the group
	goGS, err := JoinDMGroup(welcome, goKPP, nil)
	if err != nil {
		t.Fatalf("go JoinDMGroup: %v", err)
	}
	t.Logf("Go joined group (nostr_group_id: %s)", hex.Enc(goGS.NostrGroupID))

	// Verify nostr_group_id matches
	if hex.Enc(goGS.NostrGroupID) != rustNostrGroupID {
		t.Fatalf("nostr_group_id mismatch: go=%s rust=%s",
			hex.Enc(goGS.NostrGroupID), rustNostrGroupID)
	}
	t.Log("NostrGroupID matches between Go and Rust")

	// Rust sends a message
	resp = rust.send(t, map[string]string{
		"cmd":              "create_message",
		"mls_group_id_hex": rustMLSGroupID,
		"content":          "hello from rust",
	})
	if !resp.OK {
		t.Fatalf("rust create_message failed: %s", resp.Error)
	}

	// Go decrypts the Rust message
	var msgEv event.E
	if err := msgEv.UnmarshalJSON([]byte(resp.EventJSON)); err != nil {
		t.Fatalf("parse rust message event: %v", err)
	}
	if msgEv.Kind != KindGroupMessage {
		t.Fatalf("wrong message kind: want %d, got %d", KindGroupMessage, msgEv.Kind)
	}

	// Derive exporter secret
	exporterSecret, err := goGS.DeriveExporterSecret()
	if err != nil {
		t.Fatalf("derive exporter secret: %v", err)
	}

	// Decrypt outer ChaCha20-Poly1305 layer
	_, mlsCiphertext, err := EventToMessage(&msgEv, exporterSecret)
	if err != nil {
		t.Fatalf("go EventToMessage: %v", err)
	}

	// Decrypt inner MLS layer
	plaintext, err := goGS.Decrypt(mlsCiphertext)
	if err != nil {
		t.Fatalf("go MLS decrypt: %v", err)
	}

	t.Logf("Decrypted MLS payload (%d bytes)", len(plaintext))

	// MDK wraps content in a nostr event (kind 9 rumor)
	var innerEv event.E
	if err := innerEv.UnmarshalJSON(plaintext); err != nil {
		t.Logf("Decrypted payload is not JSON event, raw: %s", string(plaintext))
	} else {
		t.Logf("Inner event kind=%d content=%q", innerEv.Kind, string(innerEv.Content))
		if string(innerEv.Content) != "hello from rust" {
			t.Errorf("content mismatch: want 'hello from rust', got %q", string(innerEv.Content))
		}
	}
	t.Log("INTEROP SUCCESS: Go decrypted Rust MLS message")
}
