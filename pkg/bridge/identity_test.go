package bridge

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveIdentity_FromConfig_Hex(t *testing.T) {
	// Generate a secret key in hex
	sk, err := keys.GenerateSecretKey()
	require.NoError(t, err)

	hexKey := hex.EncodeToString(sk)
	sign, source, err := ResolveIdentity(hexKey, nil, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, IdentityFromConfig, source)
	assert.NotNil(t, sign)
	assert.Len(t, sign.Pub(), 32)
}

func TestResolveIdentity_FromConfig_Nsec(t *testing.T) {
	sk, err := keys.GenerateSecretKey()
	require.NoError(t, err)

	nsec, err := bech32encoding.BinToNsec(sk)
	require.NoError(t, err)

	sign, source, err := ResolveIdentity(string(nsec), nil, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, IdentityFromConfig, source)
	assert.NotNil(t, sign)
}

func TestResolveIdentity_FromDB(t *testing.T) {
	sk, err := keys.GenerateSecretKey()
	require.NoError(t, err)

	dbGetter := func() ([]byte, error) {
		return sk, nil
	}

	sign, source, err := ResolveIdentity("", dbGetter, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, IdentityFromDB, source)
	assert.NotNil(t, sign)
}

func TestResolveIdentity_FromFile_Generate(t *testing.T) {
	dir := t.TempDir()

	sign, source, err := ResolveIdentity("", nil, dir)
	require.NoError(t, err)
	assert.Equal(t, IdentityFromFile, source)
	assert.NotNil(t, sign)

	// File should have been created
	nsecPath := filepath.Join(dir, "bridge.nsec")
	_, err = os.Stat(nsecPath)
	assert.NoError(t, err, "bridge.nsec file should exist")

	// Reading the file should give us a valid nsec
	data, err := os.ReadFile(nsecPath)
	require.NoError(t, err)
	assert.True(t, len(data) > 0)
	assert.True(t, string(data[:4]) == "nsec", "file should contain nsec bech32")
}

func TestResolveIdentity_FromFile_Existing(t *testing.T) {
	dir := t.TempDir()

	// Generate and write an nsec file
	sk, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	nsec, err := bech32encoding.BinToNsec(sk)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bridge.nsec"), nsec, 0600))

	// Should read the existing file
	sign, source, err := ResolveIdentity("", nil, dir)
	require.NoError(t, err)
	assert.Equal(t, IdentityFromFile, source)
	assert.NotNil(t, sign)
}

func TestResolveIdentity_Priority(t *testing.T) {
	// Config takes priority over DB and file
	sk1, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	sk2, err := keys.GenerateSecretKey()
	require.NoError(t, err)

	hexKey := hex.EncodeToString(sk1)
	dbGetter := func() ([]byte, error) {
		return sk2, nil
	}

	sign, source, err := ResolveIdentity(hexKey, dbGetter, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, IdentityFromConfig, source)
	assert.NotNil(t, sign)
}

func TestResolveIdentity_DBFailsFallsToFile(t *testing.T) {
	dbGetter := func() ([]byte, error) {
		return nil, os.ErrNotExist
	}

	sign, source, err := ResolveIdentity("", dbGetter, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, IdentityFromFile, source)
	assert.NotNil(t, sign)
}

func TestResolveIdentity_InvalidNsec(t *testing.T) {
	_, _, err := ResolveIdentity("nsecINVALID", nil, t.TempDir())
	assert.Error(t, err)
}

func TestResolveIdentity_InvalidHex(t *testing.T) {
	_, _, err := ResolveIdentity("not-hex-at-all", nil, t.TempDir())
	assert.Error(t, err)
}

func TestResolveIdentity_ShortHex(t *testing.T) {
	// Valid hex but not 32 bytes
	_, _, err := ResolveIdentity("aabbccdd", nil, t.TempDir())
	assert.Error(t, err)
}

func TestResolveIdentity_DBReturnsWrongLength(t *testing.T) {
	// DB returns key that's not 32 bytes — should fall through to file
	dbGetter := func() ([]byte, error) {
		return []byte{1, 2, 3}, nil // too short
	}

	sign, source, err := ResolveIdentity("", dbGetter, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, IdentityFromFile, source)
	assert.NotNil(t, sign)
}

func TestTrimBytes(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{[]byte("hello\n"), "hello"},
		{[]byte("hello\r\n"), "hello"},
		{[]byte("hello  \t\n"), "hello"},
		{[]byte("hello"), "hello"},
		{[]byte(""), ""},
		{[]byte("\n\r\t "), ""},
	}
	for _, tt := range tests {
		got := string(trimBytes(tt.input))
		if got != tt.want {
			t.Errorf("trimBytes(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIdentityFromFile_EmptyNsecFile(t *testing.T) {
	dir := t.TempDir()
	// Write an empty nsec file
	os.WriteFile(filepath.Join(dir, "bridge.nsec"), []byte(""), 0600)

	// Should generate a new key since the file is empty
	sign, err := identityFromFile(dir)
	assert.NoError(t, err)
	assert.NotNil(t, sign)
}

func TestIdentityFromFile_WhitespaceNsecFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bridge.nsec"), []byte("  \n\t  "), 0600)

	sign, err := identityFromFile(dir)
	assert.NoError(t, err)
	assert.NotNil(t, sign)
}

func TestSignerFromNSEC_HexKey(t *testing.T) {
	sk, _ := keys.GenerateSecretKey()
	hexKey := hex.EncodeToString(sk)

	sign, err := signerFromNSEC(hexKey)
	assert.NoError(t, err)
	assert.NotNil(t, sign)
}

func TestSignerFromNSEC_NsecKey(t *testing.T) {
	sk, _ := keys.GenerateSecretKey()
	nsec, _ := bech32encoding.BinToNsec(sk)

	sign, err := signerFromNSEC(string(nsec))
	assert.NoError(t, err)
	assert.NotNil(t, sign)
}

func TestIdentityFromFile_UnwritableDir(t *testing.T) {
	// Use /dev/null as the data dir — MkdirAll will fail
	_, err := identityFromFile("/dev/null/impossible")
	assert.Error(t, err)
}

func TestResolveIdentity_IdentityFileError(t *testing.T) {
	// No config, no DB, and file path is impossible
	_, _, err := ResolveIdentity("", nil, "/dev/null/impossible")
	assert.Error(t, err)
}

func TestSignerFromSecretKey_InvalidKey(t *testing.T) {
	// Invalid key length — InitSec should fail
	_, err := signerFromSecretKey([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for invalid key length")
	}
}

func TestResolveIdentity_DBGoodKeyButSignerFails(t *testing.T) {
	// DB returns exactly 32 bytes but signerFromSecretKey
	// might still fail if the key is degenerate. In practice p8k.New()
	// succeeds and InitSec with 32 bytes succeeds, so this tests the
	// normal DB path fully.
	dbGetter := func() ([]byte, error) {
		sk, _ := keys.GenerateSecretKey()
		return sk, nil
	}
	sign, source, err := ResolveIdentity("", dbGetter, t.TempDir())
	assert.NoError(t, err)
	assert.Equal(t, IdentityFromDB, source)
	assert.NotNil(t, sign)
}
