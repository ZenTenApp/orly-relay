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
	GroupID  []byte
	PeerPub  []byte // 32-byte x-only Nostr pubkey of the peer
	group    *mls.Group
	mlsBytes []byte // serialized MLS state for persistence
}

// DMGroupID derives a deterministic group ID for a 1:1 DM between two pubkeys.
// The ID is SHA256(sort(pubA, pubB)) so both sides compute the same value.
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

// CreateDMGroup creates a new 2-member MLS group and generates a Welcome
// message for the peer. Returns the group state, the welcome (to send to peer),
// and the serialized commit message.
func CreateDMGroup(selfKPP *mls.KeyPairPackage, peerKP *mls.KeyPackage, selfPub, peerPub []byte) (*GroupState, *mls.Welcome, []byte, error) {
	groupID := DMGroupID(selfPub, peerPub)

	group, err := mls.CreateGroup(groupID, selfKPP)
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

	gs := &GroupState{
		GroupID: groupID,
		PeerPub: peerPub,
		group:   group,
	}
	return gs, welcome, welcome.Bytes(), nil
}

// JoinDMGroup joins a group from a received Welcome message.
func JoinDMGroup(welcome *mls.Welcome, selfKPP *mls.KeyPairPackage, peerPub []byte) (*GroupState, error) {
	group, err := mls.GroupFromWelcome(welcome, selfKPP)
	if err != nil {
		return nil, fmt.Errorf("mls join from welcome: %w", err)
	}

	// Derive the group ID from the MLS group
	gs := &GroupState{
		PeerPub: peerPub,
		group:   group,
	}
	return gs, nil
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
