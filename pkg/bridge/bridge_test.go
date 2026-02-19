package bridge

import (
	"context"
	"testing"
	"time"

	"git.mleku.dev/mleku/nostr/crypto/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
