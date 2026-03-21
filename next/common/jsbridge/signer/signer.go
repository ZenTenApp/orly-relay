package signer

// HasSigner returns true if window.nostr (NIP-07) is available.
func HasSigner() bool { panic("jsbridge") }

// GetPublicKey requests the public key from the browser extension.
// Calls fn with hex pubkey on success, empty string on failure.
func GetPublicKey(fn func(string)) { panic("jsbridge") }
