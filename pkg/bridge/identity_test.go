package bridge

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"git.mleku.dev/mleku/nostr/crypto/keys"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
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
