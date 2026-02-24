//go:build !(js && wasm)

package database

// SavePaidSubscription saves or updates a paid subscription.
// Delegates to the PaidACL storage layer.
func (d *D) SavePaidSubscription(sub *PaidSubscription) error {
	p := NewPaidACL(d)
	return p.SaveSubscription(sub)
}

// GetPaidSubscription returns the paid subscription for a pubkey.
func (d *D) GetPaidSubscription(pubkeyHex string) (*PaidSubscription, error) {
	p := NewPaidACL(d)
	return p.GetSubscription(pubkeyHex)
}

// DeletePaidSubscription removes a paid subscription.
func (d *D) DeletePaidSubscription(pubkeyHex string) error {
	p := NewPaidACL(d)
	return p.DeleteSubscription(pubkeyHex)
}

// ListPaidSubscriptions returns all paid subscriptions.
func (d *D) ListPaidSubscriptions() ([]*PaidSubscription, error) {
	p := NewPaidACL(d)
	return p.ListSubscriptions()
}

// ClaimAlias atomically claims an alias for a pubkey.
func (d *D) ClaimAlias(alias, pubkeyHex string) error {
	p := NewPaidACL(d)
	return p.ClaimAlias(alias, pubkeyHex)
}

// GetAliasByPubkey returns the alias for a pubkey, or "" if none.
func (d *D) GetAliasByPubkey(pubkeyHex string) (string, error) {
	p := NewPaidACL(d)
	return p.GetAliasByPubkey(pubkeyHex)
}

// GetPubkeyByAlias returns the pubkey for an alias, or "" if not found.
func (d *D) GetPubkeyByAlias(alias string) (string, error) {
	p := NewPaidACL(d)
	return p.GetPubkeyByAlias(alias)
}

// IsAliasTaken returns true if the alias is claimed by any pubkey.
func (d *D) IsAliasTaken(alias string) (bool, error) {
	p := NewPaidACL(d)
	return p.IsAliasTaken(alias)
}
