package nwc

import (
	"errors"
	"net/url"

	"lol.mleku.dev/chk"
	"git.mleku.dev/mleku/nostr/crypto/encryption"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/interfaces/signer"
)

type ConnectionParams struct {
	clientSecretKey signer.I
	walletPublicKey []byte
	conversationKey []byte
	relay           string
}

// GetWalletPublicKey returns the wallet public key from the ConnectionParams.
func (c *ConnectionParams) GetWalletPublicKey() []byte {
	return c.walletPublicKey
}

// GetConversationKey returns the conversation key from the ConnectionParams.
func (c *ConnectionParams) GetConversationKey() []byte {
	return c.conversationKey
}

func ParseConnectionURI(nwcUri string) (parts *ConnectionParams, err error) {
	var p *url.URL
	if p, err = url.Parse(nwcUri); chk.E(err) {
		return
	}
	if p == nil {
		err = errors.New("invalid uri")
		return
	}
	parts = &ConnectionParams{}
	if p.Scheme != "nostr+walletconnect" {
		err = errors.New("incorrect scheme")
		return
	}
	if parts.walletPublicKey, err = hex.Dec(p.Host); chk.E(err) {
		err = errors.New("invalid public key")
		return
	}
	query := p.Query()
	var ok bool
	var relay []string
	if relay, ok = query["relay"]; !ok {
		err = errors.New("missing relay parameter")
		return
	}
	if len(relay) == 0 {
		return nil, errors.New("no relays")
	}
	parts.relay = relay[0]
	var secret string
	if secret = query.Get("secret"); secret == "" {
		err = errors.New("missing secret parameter")
		return
	}
	var secretBytes []byte
	if secretBytes, err = hex.Dec(secret); chk.E(err) {
		err = errors.New("invalid secret")
		return
	}
	var clientKey *p8k.Signer
	if clientKey, err = p8k.New(); chk.E(err) {
		return
	}
	if err = clientKey.InitSec(secretBytes); chk.E(err) {
		return
	}
	parts.clientSecretKey = clientKey
	if parts.conversationKey, err = encryption.GenerateConversationKey(
		clientKey.Sec(),
		parts.walletPublicKey,
	); chk.E(err) {
		return
	}
	return
}
