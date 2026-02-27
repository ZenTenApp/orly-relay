package encryption

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"math"

	"next.orly.dev/pkg/nostr/crypto/ec/secp256k1"
	"golang.org/x/crypto/chacha20"
	"golang.org/x/crypto/hkdf"
	"next.orly.dev/pkg/lol/errorf"
)

var (
	MinPlaintextSize = 0x0001 // 1b msg => padded to 32b
	MaxPlaintextSize = 0xffff // 65535 (64kb-1) => padded to 64kb
)

type EncryptOptions struct {
	Salt    []byte
	Version int
}

func Encrypt(
	conversationKey []byte, plaintext []byte, options *EncryptOptions,
) (ciphertext string, err error) {
	var (
		version   int = 2
		salt      []byte
		enc       []byte
		nonce     []byte
		auth      []byte
		padded    []byte
		encrypted []byte
		hmac_     []byte
		concat    []byte
	)
	if options != nil && options.Version != 0 {
		version = options.Version
	}
	if options != nil && options.Salt != nil {
		salt = options.Salt
	} else {
		if salt, err = randomBytes(32); err != nil {
			return
		}
	}
	if version != 2 {
		err = errorf.E("unknown version %d", version)
		return
	}
	if len(salt) != 32 {
		err = errorf.E("salt must be 32 bytes")
		return
	}
	if enc, nonce, auth, err = MessageKeys(conversationKey, salt); err != nil {
		return
	}
	if padded, err = pad(plaintext); err != nil {
		return
	}
	if encrypted, err = chacha20_(enc, nonce, padded); err != nil {
		return
	}
	if hmac_, err = sha256Hmac(auth, encrypted, salt); err != nil {
		return
	}
	concat = append(concat, []byte{byte(version)}...)
	concat = append(concat, salt...)
	concat = append(concat, encrypted...)
	concat = append(concat, hmac_...)
	ciphertext = base64.StdEncoding.EncodeToString(concat)
	return
}

func Decrypt(conversationKey []byte, ciphertext string) (
	plaintext string, err error,
) {
	var (
		version     int = 2
		decoded     []byte
		cLen        int
		dLen        int
		salt        []byte
		ciphertext_ []byte
		hmac        []byte
		hmac_       []byte
		enc         []byte
		nonce       []byte
		auth        []byte
		padded      []byte
		unpaddedLen uint16
		unpadded    []byte
	)
	cLen = len(ciphertext)
	if cLen < 132 || cLen > 87472 {
		err = errorf.E("invalid payload length: %d", cLen)
		return
	}
	if ciphertext[0:1] == "#" {
		err = errorf.E("unknown version")
		return
	}
	if decoded, err = base64.StdEncoding.DecodeString(ciphertext); err != nil {
		err = errorf.E("invalid base64")
		return
	}
	if version = int(decoded[0]); version != 2 {
		err = errorf.E("unknown version %d", version)
		return
	}
	dLen = len(decoded)
	if dLen < 99 || dLen > 65603 {
		err = errorf.E("invalid data length: %d", dLen)
		return
	}
	salt, ciphertext_, hmac_ = decoded[1:33], decoded[33:dLen-32], decoded[dLen-32:]
	if enc, nonce, auth, err = MessageKeys(conversationKey, salt); err != nil {
		return
	}
	if hmac, err = sha256Hmac(auth, ciphertext_, salt); err != nil {
		return
	}
	if !bytes.Equal(hmac_, hmac) {
		err = errorf.E("invalid hmac")
		return
	}
	if padded, err = chacha20_(enc, nonce, ciphertext_); err != nil {
		return
	}
	unpaddedLen = binary.BigEndian.Uint16(padded[0:2])
	if unpaddedLen < uint16(MinPlaintextSize) || unpaddedLen > uint16(MaxPlaintextSize) || len(padded) != 2+calcPadding(int(unpaddedLen)) {
		err = errorf.E("invalid padding")
		return
	}
	unpadded = padded[2 : unpaddedLen+2]
	if len(unpadded) == 0 || len(unpadded) != int(unpaddedLen) {
		err = errorf.E("invalid padding")
		return
	}
	plaintext = string(unpadded)
	return
}

func GenerateConversationKey(
	sendPrivkey []byte, recvPubkey []byte,
) (conversationKey []byte, err error) {
	// Parse the private key
	var privKey secp256k1.SecretKey
	if overflow := privKey.Key.SetByteSlice(sendPrivkey); overflow {
		err = errorf.E(
			"invalid private key: x coordinate %x is not on the secp256k1 curve",
			sendPrivkey,
		)
		return
	}

	// Check if private key is zero
	if privKey.Key.IsZero() {
		err = errorf.E(
			"invalid private key: x coordinate %x is not on the secp256k1 curve",
			sendPrivkey,
		)
		return
	}

	// Parse the public key
	// If it's 32 bytes, prepend format byte for compressed format (0x02 for even y)
	// If it's already 33 bytes, use as-is
	var pubKeyBytes []byte
	if len(recvPubkey) == 32 {
		// Nostr-style 32-byte public key - prepend compressed format byte
		pubKeyBytes = make([]byte, 33)
		pubKeyBytes[0] = secp256k1.PubKeyFormatCompressedEven
		copy(pubKeyBytes[1:], recvPubkey)
	} else if len(recvPubkey) == 33 {
		// Already in compressed format
		pubKeyBytes = recvPubkey
	} else {
		err = errorf.E(
			"invalid public key length: %d (expected 32 or 33 bytes)",
			len(recvPubkey),
		)
		return
	}

	pubKey, err := secp256k1.ParsePubKey(pubKeyBytes)
	if err != nil {
		return
	}

	// Compute ECDH shared secret (returns only x-coordinate, 32 bytes)
	shared := secp256k1.GenerateSharedSecret(&privKey, pubKey)

	// Apply HKDF-Extract with salt "nip44-v2"
	conversationKey = hkdf.Extract(sha256.New, shared, []byte("nip44-v2"))
	return
}

func chacha20_(key []byte, nonce []byte, message []byte) ([]byte, error) {
	var (
		cipher *chacha20.Cipher
		dst    = make([]byte, len(message))
		err    error
	)
	if cipher, err = chacha20.NewUnauthenticatedCipher(key, nonce); err != nil {
		return nil, err
	}
	cipher.XORKeyStream(dst, message)
	return dst, nil
}

func randomBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func sha256Hmac(key []byte, ciphertext []byte, aad []byte) ([]byte, error) {
	if len(aad) != 32 {
		return nil, errors.New("aad data must be 32 bytes")
	}
	h := hmac.New(sha256.New, key)
	h.Write(aad)
	h.Write(ciphertext)
	return h.Sum(nil), nil
}

func MessageKeys(conversationKey []byte, salt []byte) (
	[]byte, []byte, []byte, error,
) {
	var (
		r     io.Reader
		enc   []byte = make([]byte, 32)
		nonce []byte = make([]byte, 12)
		auth  []byte = make([]byte, 32)
		err   error
	)
	if len(conversationKey) != 32 {
		return nil, nil, nil, errors.New("conversation key must be 32 bytes")
	}
	if len(salt) != 32 {
		return nil, nil, nil, errors.New("salt must be 32 bytes")
	}
	r = hkdf.Expand(sha256.New, conversationKey, salt)
	if _, err = io.ReadFull(r, enc); err != nil {
		return nil, nil, nil, err
	}
	if _, err = io.ReadFull(r, nonce); err != nil {
		return nil, nil, nil, err
	}
	if _, err = io.ReadFull(r, auth); err != nil {
		return nil, nil, nil, err
	}
	return enc, nonce, auth, nil
}

func pad(s []byte) ([]byte, error) {
	var (
		sb      []byte
		sbLen   int
		padding int
		result  []byte
	)
	sb = s
	sbLen = len(sb)
	if sbLen < 1 || sbLen > MaxPlaintextSize {
		return nil, errors.New("plaintext should be between 1b and 64kB")
	}
	padding = calcPadding(sbLen)
	result = make([]byte, 2)
	binary.BigEndian.PutUint16(result, uint16(sbLen))
	result = append(result, sb...)
	result = append(result, make([]byte, padding-sbLen)...)
	return result, nil
}

func calcPadding(sLen int) int {
	var (
		nextPower int
		chunk     int
	)
	if sLen <= 32 {
		return 32
	}
	nextPower = 1 << int(math.Floor(math.Log2(float64(sLen-1)))+1)
	chunk = int(math.Max(32, float64(nextPower/8)))
	return chunk * int(math.Floor(float64((sLen-1)/chunk))+1)
}
