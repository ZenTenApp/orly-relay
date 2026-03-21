package subtle

// AES-CBC via SubtleCrypto and crypto.getRandomValues.
// Available in both window and service worker scopes.

// RandomBytes fills dst with cryptographically random bytes.
func RandomBytes(dst []byte) { panic("jsbridge") }

// AESCBCEncrypt encrypts plaintext with AES-256-CBC.
// key: 32 bytes, iv: 16 bytes.
// Calls fn with the ciphertext when done.
func AESCBCEncrypt(key, iv, plaintext []byte, fn func([]byte)) { panic("jsbridge") }

// AESCBCDecrypt decrypts ciphertext with AES-256-CBC.
// key: 32 bytes, iv: 16 bytes.
// Calls fn with the plaintext when done.
func AESCBCDecrypt(key, iv, ciphertext []byte, fn func([]byte)) { panic("jsbridge") }
