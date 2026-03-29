package marmot

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/emersion/go-mls"
)

// cipherSuite is the MLS cipher suite used by Marmot. Ed25519 signing with
// X25519 DHKEM, AES-128-GCM encryption, and SHA-256 hashing.
var cipherSuite = mls.CipherSuiteMLS_128_DHKEMX25519_AES128GCM_SHA256_Ed25519

// GroupState holds the MLS group state for a 1:1 DM conversation.
type GroupState struct {
	GroupID      []byte // MLS group ID (deterministic, for internal lookups)
	NostrGroupID []byte // 32-byte random ID from NostrGroupData (0xf2ee), used in "h" tags
	PeerPub      []byte // 32-byte x-only Nostr pubkey of the peer
	group        *mls.Group
	mlsBytes     []byte // serialized MLS state for persistence
}

// DMGroupID derives a deterministic group ID for a 1:1 DM between two pubkeys.
// Used as the MLS GroupID. Both sides compute the same value.
func DMGroupID(pubA, pubB []byte) []byte {
	pair := [2][]byte{pubA, pubB}
	sort.Slice(pair[:], func(i, j int) bool {
		for k := 0; k < len(pair[i]) && k < len(pair[j]); k++ {
			if pair[i][k] != pair[j][k] {
				return pair[i][k] < pair[j][k]
			}
		}
		return len(pair[i]) < len(pair[j])
	})
	h := sha256.New()
	h.Write(pair[0])
	h.Write(pair[1])
	return h.Sum(nil)
}

// CreateDMGroup creates a new 2-member MLS group with a NostrGroupData
// extension and generates a Welcome for the peer. The nostrGroupID is a
// random 32-byte value carried in the 0xf2ee extension and used in "h" tags.
func CreateDMGroup(selfKPP *mls.KeyPairPackage, peerKP *mls.KeyPackage, selfPub, peerPub []byte, relays []string) (*GroupState, *mls.Welcome, []byte, error) {
	groupID := DMGroupID(selfPub, peerPub)

	// Create NostrGroupData extension with random nostr_group_id
	ngd, err := NewNostrGroupData("", selfPub, relays)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create nostr group data: %w", err)
	}

	ngdExt, err := ngd.MarshalExtension()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal nostr group data: %w", err)
	}

	group, err := mls.CreateGroupWithOptions(groupID, selfKPP, &mls.GroupOptions{
		Extensions: []mls.Extension{ngdExt},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("mls create group: %w", err)
	}

	welcome, commitMsg, err := group.CreateWelcome([]mls.KeyPackage{*peerKP})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("mls create welcome: %w", err)
	}

	// The creator must process the commit to advance their own epoch
	if _, err := group.UnmarshalAndProcessMessage(commitMsg); err != nil {
		return nil, nil, nil, fmt.Errorf("process own commit: %w", err)
	}

	mlsBytes, err := group.Marshal()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal group state: %w", err)
	}

	gs := &GroupState{
		GroupID:      groupID,
		NostrGroupID: ngd.NostrGroupID[:],
		PeerPub:      peerPub,
		group:        group,
		mlsBytes:     mlsBytes,
	}
	return gs, welcome, welcome.Bytes(), nil
}

// JoinDMGroup joins a group from a received Welcome message. Extracts the
// nostr_group_id from the NostrGroupData extension in the group context.
func JoinDMGroup(welcome *mls.Welcome, selfKPP *mls.KeyPairPackage, peerPub []byte) (*GroupState, error) {
	group, err := mls.GroupFromWelcome(welcome, selfKPP)
	if err != nil {
		return nil, fmt.Errorf("mls join from welcome: %w", err)
	}

	mlsBytes, err := group.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal group state: %w", err)
	}

	gs := &GroupState{
		GroupID:  group.GroupID(),
		PeerPub:  peerPub,
		group:    group,
		mlsBytes: mlsBytes,
	}

	// Extract nostr_group_id from the 0xf2ee extension
	extData := group.FindGroupContextExtension(mls.ExtensionTypeNostrGroupData)
	if extData != nil {
		ngd, err := UnmarshalNostrGroupData(extData)
		if err == nil {
			gs.NostrGroupID = ngd.NostrGroupID[:]
		}
	}

	return gs, nil
}

// DeriveExporterSecret derives the MLS exporter secret for kind 445 events
// using MLS-Exporter("marmot", "group-event", 32) per MIP-03.
func (gs *GroupState) DeriveExporterSecret() ([]byte, error) {
	if gs.group == nil {
		return nil, fmt.Errorf("group not initialized")
	}
	return DeriveExporterSecret(gs.group)
}

// ExporterSecret derives the raw MLS exporter secret from the current epoch.
// Prefer DeriveExporterSecret for kind 445 encryption.
func (gs *GroupState) ExporterSecret() ([]byte, error) {
	if gs.group == nil {
		return nil, fmt.Errorf("group not initialized")
	}
	return gs.group.ExporterSecret()
}

// Encrypt encrypts a plaintext message within the MLS group.
func (gs *GroupState) Encrypt(plaintext []byte) ([]byte, error) {
	if gs.group == nil {
		return nil, fmt.Errorf("group not initialized")
	}
	return gs.group.CreateApplicationMessage(plaintext)
}

// Decrypt decrypts a ciphertext message received from the MLS group.
func (gs *GroupState) Decrypt(ciphertext []byte) ([]byte, error) {
	if gs.group == nil {
		return nil, fmt.Errorf("group not initialized")
	}
	plaintext, err := gs.group.UnmarshalAndProcessMessage(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("mls decrypt: %w", err)
	}
	return plaintext, nil
}
