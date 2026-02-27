package marmot

import (
	"fmt"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer"
	"github.com/emersion/go-mls"
)

// GenerateKeyPackage creates a new MLS key pair package using the Nostr pubkey
// as the MLS credential identity.
func GenerateKeyPackage(sign signer.I) (*mls.KeyPairPackage, error) {
	cred := mls.NewBasicCredential(sign.Pub())
	kpp, err := mls.GenerateKeyPairPackage(cipherSuite, cred)
	if err != nil {
		return nil, fmt.Errorf("generate key package: %w", err)
	}
	return kpp, nil
}

// KeyPackageToEvent creates a kind 443 Nostr event containing the serialized
// MLS key package. The event is signed by the provided signer.
func KeyPackageToEvent(kpp *mls.KeyPairPackage, sign signer.I) (*event.E, error) {
	kpBytes := kpp.Public.Bytes()

	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = KindKeyPackage
	ev.Content = kpBytes
	ev.Tags = tag.NewS(
		tag.NewFromAny("mls_protocol_version", "mls10"),
		tag.NewFromAny("mls_ciphersuite", "MLS_128_DHKEMX25519_AES128GCM_SHA256_Ed25519"),
	)
	if err := ev.Sign(sign); err != nil {
		return nil, fmt.Errorf("sign key package event: %w", err)
	}
	return ev, nil
}

// EventToKeyPackage extracts an MLS key package from a kind 443 event.
func EventToKeyPackage(ev *event.E) (*mls.KeyPackage, error) {
	if ev.Kind != KindKeyPackage {
		return nil, fmt.Errorf("expected kind %d, got %d", KindKeyPackage, ev.Kind)
	}
	kp, err := mls.UnmarshalKeyPackage(ev.Content)
	if err != nil {
		return nil, fmt.Errorf("unmarshal key package: %w", err)
	}
	return kp, nil
}
