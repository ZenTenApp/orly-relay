package marmot

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/emersion/go-mls"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
)

// GenerateKeyPackage creates a new MLS key pair package using the Nostr
// pubkey as the MLS credential identity. The key package advertises support
// for LastResort (0x000a) and NostrGroupData (0xf2ee) extensions per MIP-00.
func GenerateKeyPackage(crypto CryptoProvider) (*mls.KeyPairPackage, error) {
	cred := mls.NewBasicCredential(crypto.Pub())
	kpp, err := mls.GenerateKeyPairPackageWithOptions(cipherSuite, cred, &mls.KeyPackageOptions{
		CapabilityExtensions: []mls.ExtensionType{
			mls.ExtensionTypeLastResort,
			mls.ExtensionTypeNostrGroupData,
		},
		KeyPackageExtensions: []mls.Extension{
			mls.NewExtension(mls.ExtensionTypeLastResort, nil),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("generate key package: %w", err)
	}
	return kpp, nil
}

// KeyPackageToEvent creates a kind 443 Nostr event per MIP-00.
// Content is base64(TLS-serialized KeyPackage).
// Tags: mls_protocol_version, mls_ciphersuite, mls_extensions, encoding, i, relays.
func KeyPackageToEvent(kpp *mls.KeyPairPackage, crypto CryptoProvider, relays []string) (*event.E, error) {
	kpBytes := kpp.Public.RawBytes()
	content := base64.StdEncoding.EncodeToString(kpBytes)

	ref, err := kpp.Public.GenerateRef()
	if err != nil {
		return nil, fmt.Errorf("generate key package ref: %w", err)
	}
	refHex := hex.Enc([]byte(ref))

	tags := []*tag.T{
		tag.NewFromAny("mls_protocol_version", "1.0"),
		tag.NewFromAny("mls_ciphersuite", "0x0001"),
		tag.NewFromAny("mls_extensions", "0x000a", "0xf2ee"),
		tag.NewFromAny("encoding", "base64"),
		tag.NewFromAny("i", refHex),
	}

	if len(relays) > 0 {
		relayArgs := make([]any, 0, len(relays)+1)
		relayArgs = append(relayArgs, "relays")
		for _, r := range relays {
			relayArgs = append(relayArgs, r)
		}
		tags = append(tags, tag.NewFromAny(relayArgs...))
	}

	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = KindKeyPackage
	ev.Content = []byte(content)
	ev.Tags = tag.NewS(tags...)
	if err := crypto.SignEvent(ev); err != nil {
		return nil, fmt.Errorf("sign key package event: %w", err)
	}
	return ev, nil
}

// EventToKeyPackage extracts an MLS key package from a kind 443 event.
// Supports both base64 (MIP-00) and raw binary (legacy) content encoding.
func EventToKeyPackage(ev *event.E) (*mls.KeyPackage, error) {
	if ev.Kind != KindKeyPackage {
		return nil, fmt.Errorf("expected kind %d, got %d", KindKeyPackage, ev.Kind)
	}

	content := ev.Content

	// Check if content is base64 by looking at the encoding tag
	encodingTag := ev.Tags.GetFirst([]byte("encoding"))
	if encodingTag != nil && string(encodingTag.Value()) == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(string(content))
		if err != nil {
			return nil, fmt.Errorf("base64 decode key package: %w", err)
		}
		content = decoded
	}

	kp, err := mls.UnmarshalRawKeyPackage(content)
	if err != nil {
		return nil, fmt.Errorf("unmarshal key package: %w", err)
	}
	return kp, nil
}
