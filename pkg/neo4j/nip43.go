package neo4j

import (
	"encoding/json"
	"fmt"
	"time"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/nostr/encoders/hex"
)

// NIP-43 Invite-based ACL methods
// Simplified implementation using marker-based storage via Badger
// For production, these could use Neo4j nodes with relationships

// AddNIP43Member adds a member using an invite code
func (n *N) AddNIP43Member(pubkey []byte, inviteCode string) error {
	key := "nip43_" + hex.Enc(pubkey)

	member := database.NIP43Membership{
		InviteCode: inviteCode,
		AddedAt:    time.Now(),
	}
	copy(member.Pubkey[:], pubkey)

	data, err := json.Marshal(member)
	if err != nil {
		return fmt.Errorf("failed to marshal membership: %w", err)
	}

	// Also add to members list
	if err := n.addToMembersList(pubkey); err != nil {
		return err
	}

	return n.SetMarker(key, data)
}

// RemoveNIP43Member removes a member
func (n *N) RemoveNIP43Member(pubkey []byte) error {
	key := "nip43_" + hex.Enc(pubkey)

	// Remove from members list
	if err := n.removeFromMembersList(pubkey); err != nil {
		return err
	}

	return n.DeleteMarker(key)
}

// IsNIP43Member checks if a pubkey is a member
func (n *N) IsNIP43Member(pubkey []byte) (isMember bool, err error) {
	_, err = n.GetNIP43Membership(pubkey)
	return err == nil, nil
}

// GetNIP43Membership retrieves membership information
func (n *N) GetNIP43Membership(pubkey []byte) (*database.NIP43Membership, error) {
	key := "nip43_" + hex.Enc(pubkey)

	data, err := n.GetMarker(key)
	if err != nil {
		return nil, err
	}

	var member database.NIP43Membership
	if err := json.Unmarshal(data, &member); err != nil {
		return nil, fmt.Errorf("failed to unmarshal membership: %w", err)
	}

	return &member, nil
}

// GetAllNIP43Members retrieves all member pubkeys
func (n *N) GetAllNIP43Members() ([][]byte, error) {
	data, err := n.GetMarker("nip43_members_list")
	if err != nil {
		return nil, nil // No members = empty list
	}

	var members []string
	if err := json.Unmarshal(data, &members); err != nil {
		return nil, fmt.Errorf("failed to unmarshal members list: %w", err)
	}

	result := make([][]byte, 0, len(members))
	for _, hexPubkey := range members {
		pubkey, err := hex.Dec(hexPubkey)
		if err != nil {
			continue
		}
		result = append(result, pubkey)
	}

	return result, nil
}

// StoreInviteCode stores an invite code with expiration
func (n *N) StoreInviteCode(code string, expiresAt time.Time) error {
	key := "invite_" + code

	inviteData := map[string]interface{}{
		"code":      code,
		"expiresAt": expiresAt,
	}

	data, err := json.Marshal(inviteData)
	if err != nil {
		return fmt.Errorf("failed to marshal invite: %w", err)
	}

	return n.SetMarker(key, data)
}

// ValidateInviteCode checks if an invite code is valid
func (n *N) ValidateInviteCode(code string) (valid bool, err error) {
	key := "invite_" + code

	data, err := n.GetMarker(key)
	if err != nil {
		return false, nil // Code doesn't exist
	}

	var inviteData map[string]interface{}
	if err := json.Unmarshal(data, &inviteData); err != nil {
		return false, fmt.Errorf("failed to unmarshal invite: %w", err)
	}

	// Check expiration
	if expiresStr, ok := inviteData["expiresAt"].(string); ok {
		expiresAt, err := time.Parse(time.RFC3339, expiresStr)
		if err == nil && time.Now().After(expiresAt) {
			return false, nil // Expired
		}
	}

	return true, nil
}

// DeleteInviteCode removes an invite code
func (n *N) DeleteInviteCode(code string) error {
	key := "invite_" + code
	return n.DeleteMarker(key)
}

// PublishNIP43MembershipEvent publishes a membership event
func (n *N) PublishNIP43MembershipEvent(kind int, pubkey []byte) error {
	// This would require publishing an actual Nostr event
	// For now, just log it
	n.Logger.Infof("would publish NIP-43 event kind %d for %s", kind, hex.Enc(pubkey))
	return nil
}

// addToMembersList adds a pubkey to the members list
func (n *N) addToMembersList(pubkey []byte) error {
	data, err := n.GetMarker("nip43_members_list")

	var members []string
	if err == nil {
		if err := json.Unmarshal(data, &members); err != nil {
			return fmt.Errorf("failed to unmarshal members list: %w", err)
		}
	}

	hexPubkey := hex.Enc(pubkey)

	// Check if already in list
	for _, member := range members {
		if member == hexPubkey {
			return nil // Already in list
		}
	}

	members = append(members, hexPubkey)

	data, err = json.Marshal(members)
	if err != nil {
		return fmt.Errorf("failed to marshal members list: %w", err)
	}

	return n.SetMarker("nip43_members_list", data)
}

// removeFromMembersList removes a pubkey from the members list
func (n *N) removeFromMembersList(pubkey []byte) error {
	data, err := n.GetMarker("nip43_members_list")
	if err != nil {
		return nil // No list = nothing to remove
	}

	var members []string
	if err := json.Unmarshal(data, &members); err != nil {
		return fmt.Errorf("failed to unmarshal members list: %w", err)
	}

	hexPubkey := hex.Enc(pubkey)

	// Filter out the pubkey
	filtered := make([]string, 0, len(members))
	for _, member := range members {
		if member != hexPubkey {
			filtered = append(filtered, member)
		}
	}

	data, err = json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("failed to marshal members list: %w", err)
	}

	return n.SetMarker("nip43_members_list", data)
}
