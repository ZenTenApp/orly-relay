package bridge

import (
	"context"
	"sync"
	"testing"
	"time"

	"git.mleku.dev/mleku/nostr/crypto/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ensure Bridge wg field is accessible for testing.
var _ = (*sync.WaitGroup)(nil)

func TestBridge_StartStop(t *testing.T) {
	sk, err := keys.GenerateSecretKey()
	require.NoError(t, err)

	cfg := &Config{
		NSEC:    keys.GenerateSecretKeyHex(),
		DataDir: t.TempDir(),
	}
	_ = sk // we use GenerateSecretKeyHex for a quick hex key

	b := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, b.Start(ctx))
	assert.NotNil(t, b.Signer())
	assert.Len(t, b.Signer().Pub(), 32)
	assert.Equal(t, IdentityFromConfig, b.IdentitySource())

	b.Stop()
}

func TestBridge_StartWithDBGetter(t *testing.T) {
	sk, err := keys.GenerateSecretKey()
	require.NoError(t, err)

	cfg := &Config{
		DataDir: t.TempDir(),
	}

	dbGetter := func() ([]byte, error) {
		return sk, nil
	}

	b := New(cfg, dbGetter)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, b.Start(ctx))
	assert.Equal(t, IdentityFromDB, b.IdentitySource())
	assert.NotNil(t, b.Signer())

	b.Stop()
}

func TestBridge_StartFileGeneration(t *testing.T) {
	cfg := &Config{
		DataDir: t.TempDir(),
	}

	b := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, b.Start(ctx))
	assert.Equal(t, IdentityFromFile, b.IdentitySource())
	assert.NotNil(t, b.Signer())
	assert.Len(t, b.Signer().Pub(), 32)

	b.Stop()
}

func TestBridge_StartWithDomain(t *testing.T) {
	cfg := &Config{
		NSEC:    keys.GenerateSecretKeyHex(),
		DataDir: t.TempDir(),
		Domain:  "bridge.example.com",
	}

	b := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, b.Start(ctx))
	b.Stop()
}

func TestBridge_StopWithoutStart(t *testing.T) {
	cfg := &Config{
		DataDir: t.TempDir(),
	}
	b := New(cfg, nil)
	// Should not panic
	b.Stop()
}

func TestBridge_RelayWatchLoop(t *testing.T) {
	// relayWatchLoop just blocks on ctx.Done — verify it exits cleanly
	cfg := &Config{
		NSEC:    keys.GenerateSecretKeyHex(),
		DataDir: t.TempDir(),
	}
	b := New(cfg, nil)
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.wg.Add(1)
	go b.relayWatchLoop()

	// Cancel should cause relayWatchLoop to exit
	b.cancel()
	b.wg.Wait() // will hang if relayWatchLoop doesn't exit
}

func TestIdentitySourceString(t *testing.T) {
	tests := []struct {
		source IdentitySource
		want   string
	}{
		{IdentityFromConfig, "config"},
		{IdentityFromDB, "database"},
		{IdentityFromFile, "file"},
		{IdentitySource(99), "unknown"},
	}
	for _, tt := range tests {
		got := identitySourceString(tt.source)
		if got != tt.want {
			t.Errorf("identitySourceString(%d) = %q, want %q", tt.source, got, tt.want)
		}
	}
}

func TestBridge_StartWithRelayURL(t *testing.T) {
	// Starting with a RelayURL that can't connect should fail
	cfg := &Config{
		NSEC:     keys.GenerateSecretKeyHex(),
		DataDir:  t.TempDir(),
		RelayURL: "wss://nonexistent.example.com",
	}

	b := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := b.Start(ctx)
	if err == nil {
		b.Stop()
		t.Fatal("expected error connecting to nonexistent relay")
	}
}

func TestBridge_StartBadDataDir(t *testing.T) {
	cfg := &Config{
		NSEC:    keys.GenerateSecretKeyHex(),
		DataDir: "/dev/null/impossible/path",
	}

	b := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := b.Start(ctx)
	if err == nil {
		b.Stop()
		t.Fatal("expected error for impossible data dir")
	}
}

func TestBridge_StartInvalidNSEC(t *testing.T) {
	cfg := &Config{
		NSEC:    "nsecINVALID",
		DataDir: t.TempDir(),
	}

	b := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := b.Start(ctx)
	if err == nil {
		b.Stop()
		t.Fatal("expected error from invalid NSEC")
	}
}

func TestBridge_StopWithRelay(t *testing.T) {
	// Test Stop when relay is set but not connected (just Close on RelayConn)
	cfg := &Config{
		NSEC:    keys.GenerateSecretKeyHex(),
		DataDir: t.TempDir(),
	}

	b := New(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, b.Start(ctx))

	// Manually set relay to test Stop path
	b.relay = NewRelayConn("wss://example.com")

	b.Stop()
}
