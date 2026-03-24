package signer

// HasSigner returns true if window.nostr (NIP-07) is available.
func HasSigner() bool { panic("jsbridge") }

// GetPublicKey requests the public key from the browser extension.
// Calls fn with hex pubkey on success, empty string on failure.
func GetPublicKey(fn func(string)) { panic("jsbridge") }

// SignEvent signs a serialized nostr event via NIP-07.
// Calls fn with the signed event JSON on success, empty string on failure.
func SignEvent(eventJSON string, fn func(string)) { panic("jsbridge") }

// Nip04Decrypt decrypts NIP-04 ciphertext via the browser extension.
func Nip04Decrypt(peerPubkey, ciphertext string, fn func(string)) { panic("jsbridge") }

// Nip04Encrypt encrypts plaintext via NIP-04 through the browser extension.
func Nip04Encrypt(peerPubkey, plaintext string, fn func(string)) { panic("jsbridge") }

// Nip44Decrypt decrypts NIP-44 ciphertext via the browser extension.
func Nip44Decrypt(peerPubkey, ciphertext string, fn func(string)) { panic("jsbridge") }

// Nip44Encrypt encrypts plaintext via NIP-44 through the browser extension.
func Nip44Encrypt(peerPubkey, plaintext string, fn func(string)) { panic("jsbridge") }
