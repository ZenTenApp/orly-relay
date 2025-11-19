// Package secp provides Go bindings to libsecp256k1 without CGO.
// It uses dynamic library loading via purego to call C functions directly.
package secp

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

//go:embed libsecp256k1.so
var embeddedLibLinux []byte

// Constants for context flags
const (
	ContextNone       = 1
	ContextVerify     = 257  // 1 | (1 << 8)
	ContextSign       = 513  // 1 | (1 << 9)
	ContextDeclassify = 1025 // 1 | (1 << 10)
)

// EC flags
const (
	ECCompressed   = 258 // SECP256K1_EC_COMPRESSED
	ECUncompressed = 2   // SECP256K1_EC_UNCOMPRESSED
)

// Size constants
const (
	PublicKeySize             = 64
	CompressedPublicKeySize   = 33
	UncompressedPublicKeySize = 65
	SignatureSize             = 64
	CompactSignatureSize      = 64
	PrivateKeySize            = 32
	SharedSecretSize          = 32
	SchnorrSignatureSize      = 64
	RecoverableSignatureSize  = 65
)

var (
	libHandle      uintptr
	loadLibOnce    sync.Once
	loadLibErr     error
	extractedPath  string
	extractLibOnce sync.Once
)

// Function pointers
var (
	contextCreate                  func(flags uint32) uintptr
	contextDestroy                 func(ctx uintptr)
	contextRandomize               func(ctx uintptr, seed32 *byte) int32
	ecPubkeyCreate                 func(ctx uintptr, pubkey *byte, seckey *byte) int32
	ecPubkeySerialize              func(ctx uintptr, output *byte, outputlen *uint64, pubkey *byte, flags uint32) int32
	ecPubkeyParse                  func(ctx uintptr, pubkey *byte, input *byte, inputlen uint64) int32
	ecdsaSign                      func(ctx uintptr, sig *byte, msg32 *byte, seckey *byte, noncefp uintptr, ndata uintptr) int32
	ecdsaVerify                    func(ctx uintptr, sig *byte, msg32 *byte, pubkey *byte) int32
	ecdsaSignatureSerializeDer     func(ctx uintptr, output *byte, outputlen *uint64, sig *byte) int32
	ecdsaSignatureParseDer         func(ctx uintptr, sig *byte, input *byte, inputlen uint64) int32
	ecdsaSignatureSerializeCompact func(ctx uintptr, output64 *byte, sig *byte) int32
	ecdsaSignatureParseCompact     func(ctx uintptr, sig *byte, input64 *byte) int32
	ecdsaSignatureNormalize        func(ctx uintptr, sigout *byte, sigin *byte) int32

	// Schnorr functions
	schnorrsigSign32     func(ctx uintptr, sig64 *byte, msg32 *byte, keypair *byte, auxrand32 *byte) int32
	schnorrsigVerify     func(ctx uintptr, sig64 *byte, msg32 *byte, msglen uint64, pubkey *byte) int32
	keypairCreate        func(ctx uintptr, keypair *byte, seckey *byte) int32
	xonlyPubkeyParse     func(ctx uintptr, pubkey *byte, input32 *byte) int32
	xonlyPubkeySerialize func(ctx uintptr, output32 *byte, pubkey *byte) int32
	keypairXonlyPub      func(ctx uintptr, pubkey *byte, pkParity *int32, keypair *byte) int32
	keypairPub           func(ctx uintptr, pubkey *byte, keypair *byte) int32

	// ECDH functions
	ecdh func(ctx uintptr, output *byte, pubkey *byte, seckey *byte, hashfp uintptr, data uintptr) int32

	// Recovery functions
	ecdsaRecoverableSignatureSerializeCompact func(ctx uintptr, output64 *byte, recid *int32, sig *byte) int32
	ecdsaRecoverableSignatureParseCompact     func(ctx uintptr, sig *byte, input64 *byte, recid int32) int32
	ecdsaSignRecoverable                      func(ctx uintptr, sig *byte, msg32 *byte, seckey *byte, noncefp uintptr, ndata uintptr) int32
	ecdsaRecover                              func(ctx uintptr, pubkey *byte, sig *byte, msg32 *byte) int32

	// Extrakeys
	xonlyPubkeyFromPubkey func(ctx uintptr, xonlyPubkey *byte, pkParity *int32, pubkey *byte) int32
)

// extractEmbeddedLibrary extracts the embedded library to a temporary location
func extractEmbeddedLibrary() (path string, err error) {
	extractLibOnce.Do(func() {
		var libData []byte
		var filename string

		// Select the appropriate embedded library for this platform
		switch runtime.GOOS {
		case "linux":
			if len(embeddedLibLinux) == 0 {
				err = fmt.Errorf("no embedded library for linux")
				return
			}
			libData = embeddedLibLinux
			filename = "libsecp256k1.so"
		default:
			err = fmt.Errorf("no embedded library for %s", runtime.GOOS)
			return
		}

		// Create a temporary directory for the library
		// Use a deterministic name so we don't create duplicates
		tmpDir := filepath.Join(os.TempDir(), "orly-libsecp256k1")
		if err = os.MkdirAll(tmpDir, 0755); err != nil {
			err = fmt.Errorf("failed to create temp directory: %w", err)
			return
		}

		// Write the library to the temp directory
		extractedPath = filepath.Join(tmpDir, filename)

		// Check if file already exists and is valid
		if info, e := os.Stat(extractedPath); e == nil && info.Size() == int64(len(libData)) {
			// File exists and has correct size, assume it's valid
			return
		}

		if err = os.WriteFile(extractedPath, libData, 0755); err != nil {
			err = fmt.Errorf("failed to write library to %s: %w", extractedPath, err)
			return
		}

		log.Printf("INFO: Extracted embedded libsecp256k1 to %s", extractedPath)
	})

	return extractedPath, err
}

// LoadLibrary loads the libsecp256k1 shared library
func LoadLibrary() (err error) {
	loadLibOnce.Do(func() {
		var libPath string

		// First, try to extract and use the embedded library
		usedEmbedded := false
		if embeddedPath, extractErr := extractEmbeddedLibrary(); extractErr == nil {
			libHandle, err = purego.Dlopen(embeddedPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err == nil {
				libPath = embeddedPath
				usedEmbedded = true
			} else {
				log.Printf("WARN: Failed to load embedded library from %s: %v, falling back to system paths", embeddedPath, err)
			}
		} else {
			log.Printf("WARN: Failed to extract embedded library: %v, falling back to system paths", extractErr)
		}

		// If embedded library failed, fall back to system paths
		if err != nil {
			switch runtime.GOOS {
			case "linux":
				// Try common library paths
				paths := []string{
					"./libsecp256k1.so", // Bundled in repo for linux amd64
					"libsecp256k1.so.5",
					"libsecp256k1.so.2",
					"libsecp256k1.so.1",
					"libsecp256k1.so.0",
					"libsecp256k1.so",
					"/usr/lib/libsecp256k1.so",
					"/usr/local/lib/libsecp256k1.so",
					"/usr/lib/x86_64-linux-gnu/libsecp256k1.so",
				}
				for _, p := range paths {
					libHandle, err = purego.Dlopen(p, purego.RTLD_NOW|purego.RTLD_GLOBAL)
					if err == nil {
						libPath = p
						break
					}
				}
			case "darwin":
				paths := []string{
					"libsecp256k1.2.dylib",
					"libsecp256k1.1.dylib",
					"libsecp256k1.0.dylib",
					"libsecp256k1.dylib",
					"/usr/local/lib/libsecp256k1.dylib",
					"/opt/homebrew/lib/libsecp256k1.dylib",
				}
				for _, p := range paths {
					libHandle, err = purego.Dlopen(p, purego.RTLD_NOW|purego.RTLD_GLOBAL)
					if err == nil {
						libPath = p
						break
					}
				}
			case "windows":
				paths := []string{
					"libsecp256k1-2.dll",
					"libsecp256k1-1.dll",
					"libsecp256k1-0.dll",
					"libsecp256k1.dll",
					"secp256k1.dll",
				}
				for _, p := range paths {
					libHandle, err = purego.Dlopen(p, purego.RTLD_NOW|purego.RTLD_GLOBAL)
					if err == nil {
						libPath = p
						break
					}
				}
			default:
				err = fmt.Errorf("unsupported platform: %s", runtime.GOOS)
				loadLibErr = err
				return
			}
		}

		if err != nil {
			loadLibErr = fmt.Errorf("failed to load libsecp256k1: %w", err)
			return
		}

		// Register symbols
		if err = registerSymbols(); err != nil {
			loadLibErr = fmt.Errorf("failed to register symbols from %s: %w", libPath, err)
			return
		}

		if usedEmbedded {
			log.Printf("INFO: Successfully loaded embedded libsecp256k1 v5.0.0 from %s", libPath)
		} else {
			log.Printf("INFO: Successfully loaded libsecp256k1 v5.0.0 from system path: %s", libPath)
		}
		loadLibErr = nil
	})

	return loadLibErr
}

// registerSymbols registers all C function symbols
func registerSymbols() (err error) {
	// Core context functions
	purego.RegisterLibFunc(&contextCreate, libHandle, "secp256k1_context_create")
	purego.RegisterLibFunc(&contextDestroy, libHandle, "secp256k1_context_destroy")
	purego.RegisterLibFunc(&contextRandomize, libHandle, "secp256k1_context_randomize")

	// Public key functions
	purego.RegisterLibFunc(&ecPubkeyCreate, libHandle, "secp256k1_ec_pubkey_create")
	purego.RegisterLibFunc(&ecPubkeySerialize, libHandle, "secp256k1_ec_pubkey_serialize")
	purego.RegisterLibFunc(&ecPubkeyParse, libHandle, "secp256k1_ec_pubkey_parse")

	// ECDSA functions
	purego.RegisterLibFunc(&ecdsaSign, libHandle, "secp256k1_ecdsa_sign")
	purego.RegisterLibFunc(&ecdsaVerify, libHandle, "secp256k1_ecdsa_verify")
	purego.RegisterLibFunc(&ecdsaSignatureSerializeDer, libHandle, "secp256k1_ecdsa_signature_serialize_der")
	purego.RegisterLibFunc(&ecdsaSignatureParseDer, libHandle, "secp256k1_ecdsa_signature_parse_der")
	purego.RegisterLibFunc(&ecdsaSignatureSerializeCompact, libHandle, "secp256k1_ecdsa_signature_serialize_compact")
	purego.RegisterLibFunc(&ecdsaSignatureParseCompact, libHandle, "secp256k1_ecdsa_signature_parse_compact")
	purego.RegisterLibFunc(&ecdsaSignatureNormalize, libHandle, "secp256k1_ecdsa_signature_normalize")

	// Try to load optional modules - don't fail if they're not available

	// Schnorr module
	tryRegister(&schnorrsigSign32, "secp256k1_schnorrsig_sign32")
	tryRegister(&schnorrsigVerify, "secp256k1_schnorrsig_verify")
	tryRegister(&keypairCreate, "secp256k1_keypair_create")
	tryRegister(&xonlyPubkeyParse, "secp256k1_xonly_pubkey_parse")
	tryRegister(&xonlyPubkeySerialize, "secp256k1_xonly_pubkey_serialize")
	tryRegister(&keypairXonlyPub, "secp256k1_keypair_xonly_pub")
	tryRegister(&keypairPub, "secp256k1_keypair_pub")
	tryRegister(&xonlyPubkeyFromPubkey, "secp256k1_xonly_pubkey_from_pubkey")

	// ECDH module
	tryRegister(&ecdh, "secp256k1_ecdh")

	// Recovery module
	tryRegister(&ecdsaRecoverableSignatureSerializeCompact, "secp256k1_ecdsa_recoverable_signature_serialize_compact")
	tryRegister(&ecdsaRecoverableSignatureParseCompact, "secp256k1_ecdsa_recoverable_signature_parse_compact")
	tryRegister(&ecdsaSignRecoverable, "secp256k1_ecdsa_sign_recoverable")
	tryRegister(&ecdsaRecover, "secp256k1_ecdsa_recover")

	return nil
}

// tryRegister attempts to register a symbol without failing if it doesn't exist
func tryRegister(fptr interface{}, symbol string) {
	defer func() {
		if r := recover(); r != nil {
			// Symbol not found, ignore
		}
	}()
	purego.RegisterLibFunc(fptr, libHandle, symbol)
}

// Context represents a secp256k1 context
type Context struct {
	ctx uintptr
}

// NewContext creates a new secp256k1 context
func NewContext(flags uint32) (c *Context, err error) {
	if err = LoadLibrary(); err != nil {
		return
	}

	ctx := contextCreate(flags)
	if ctx == 0 {
		err = fmt.Errorf("failed to create context")
		return
	}

	c = &Context{ctx: ctx}
	runtime.SetFinalizer(c, (*Context).Destroy)

	return
}

// Destroy destroys the context
func (c *Context) Destroy() {
	if c.ctx != 0 {
		contextDestroy(c.ctx)
		c.ctx = 0
	}
}

// Randomize randomizes the context with entropy
func (c *Context) Randomize(seed32 []byte) (err error) {
	if len(seed32) != 32 {
		err = fmt.Errorf("seed must be 32 bytes")
		return
	}

	ret := contextRandomize(c.ctx, &seed32[0])
	if ret != 1 {
		err = fmt.Errorf("failed to randomize context")
		return
	}

	return
}

// CreatePublicKey creates a public key from a private key
func (c *Context) CreatePublicKey(seckey []byte) (pubkey []byte, err error) {
	if len(seckey) != PrivateKeySize {
		err = fmt.Errorf("private key must be %d bytes", PrivateKeySize)
		return
	}

	pubkey = make([]byte, PublicKeySize)
	ret := ecPubkeyCreate(c.ctx, &pubkey[0], &seckey[0])
	if ret != 1 {
		err = fmt.Errorf("failed to create public key")
		return
	}

	return
}

// SerializePublicKey serializes a public key
func (c *Context) SerializePublicKey(pubkey []byte, compressed bool) (output []byte, err error) {
	if len(pubkey) != PublicKeySize {
		err = fmt.Errorf("public key must be %d bytes", PublicKeySize)
		return
	}

	var flags uint32
	if compressed {
		output = make([]byte, CompressedPublicKeySize)
		flags = ECCompressed
	} else {
		output = make([]byte, UncompressedPublicKeySize)
		flags = ECUncompressed
	}

	outputLen := uint64(len(output))
	ret := ecPubkeySerialize(c.ctx, &output[0], &outputLen, &pubkey[0], flags)
	if ret != 1 {
		err = fmt.Errorf("failed to serialize public key")
		return
	}

	output = output[:outputLen]
	return
}

// SerializePublicKeyCompressed serializes a public key in compressed format (33 bytes)
func (c *Context) SerializePublicKeyCompressed(pubkey []byte) (output []byte, err error) {
	return c.SerializePublicKey(pubkey, true)
}

// ParsePublicKey parses a serialized public key
func (c *Context) ParsePublicKey(input []byte) (pubkey []byte, err error) {
	pubkey = make([]byte, PublicKeySize)
	ret := ecPubkeyParse(c.ctx, &pubkey[0], &input[0], uint64(len(input)))
	if ret != 1 {
		err = fmt.Errorf("failed to parse public key")
		return
	}

	return
}

// Sign creates an ECDSA signature
func (c *Context) Sign(msg32 []byte, seckey []byte) (sig []byte, err error) {
	if len(msg32) != 32 {
		err = fmt.Errorf("message must be 32 bytes")
		return
	}
	if len(seckey) != PrivateKeySize {
		err = fmt.Errorf("private key must be %d bytes", PrivateKeySize)
		return
	}

	sig = make([]byte, SignatureSize)
	ret := ecdsaSign(c.ctx, &sig[0], &msg32[0], &seckey[0], 0, 0)
	if ret != 1 {
		err = fmt.Errorf("failed to sign message")
		return
	}

	return
}

// Verify verifies an ECDSA signature
func (c *Context) Verify(msg32 []byte, sig []byte, pubkey []byte) (valid bool, err error) {
	if len(msg32) != 32 {
		err = fmt.Errorf("message must be 32 bytes")
		return
	}
	if len(sig) != SignatureSize {
		err = fmt.Errorf("signature must be %d bytes", SignatureSize)
		return
	}
	if len(pubkey) != PublicKeySize {
		err = fmt.Errorf("public key must be %d bytes", PublicKeySize)
		return
	}

	ret := ecdsaVerify(c.ctx, &sig[0], &msg32[0], &pubkey[0])
	valid = ret == 1

	return
}

// SerializeSignatureDER serializes a signature in DER format
func (c *Context) SerializeSignatureDER(sig []byte) (output []byte, err error) {
	if len(sig) != SignatureSize {
		err = fmt.Errorf("signature must be %d bytes", SignatureSize)
		return
	}

	output = make([]byte, 72) // Max DER signature size
	outputLen := uint64(len(output))

	ret := ecdsaSignatureSerializeDer(c.ctx, &output[0], &outputLen, &sig[0])
	if ret != 1 {
		err = fmt.Errorf("failed to serialize signature")
		return
	}

	output = output[:outputLen]
	return
}

// ParseSignatureDER parses a DER-encoded signature
func (c *Context) ParseSignatureDER(input []byte) (sig []byte, err error) {
	sig = make([]byte, SignatureSize)
	ret := ecdsaSignatureParseDer(c.ctx, &sig[0], &input[0], uint64(len(input)))
	if ret != 1 {
		err = fmt.Errorf("failed to parse DER signature")
		return
	}

	return
}

// SerializeSignatureCompact serializes a signature in compact format (64 bytes)
func (c *Context) SerializeSignatureCompact(sig []byte) (output []byte, err error) {
	if len(sig) != SignatureSize {
		err = fmt.Errorf("signature must be %d bytes", SignatureSize)
		return
	}

	output = make([]byte, CompactSignatureSize)
	ret := ecdsaSignatureSerializeCompact(c.ctx, &output[0], &sig[0])
	if ret != 1 {
		err = fmt.Errorf("failed to serialize signature")
		return
	}

	return
}

// ParseSignatureCompact parses a compact (64-byte) signature
func (c *Context) ParseSignatureCompact(input64 []byte) (sig []byte, err error) {
	if len(input64) != CompactSignatureSize {
		err = fmt.Errorf("compact signature must be %d bytes", CompactSignatureSize)
		return
	}

	sig = make([]byte, SignatureSize)
	ret := ecdsaSignatureParseCompact(c.ctx, &sig[0], &input64[0])
	if ret != 1 {
		err = fmt.Errorf("failed to parse compact signature")
		return
	}

	return
}

// NormalizeSignature normalizes a signature to lower-S form
func (c *Context) NormalizeSignature(sig []byte) (normalized []byte, wasNormalized bool, err error) {
	if len(sig) != SignatureSize {
		err = fmt.Errorf("signature must be %d bytes", SignatureSize)
		return
	}

	normalized = make([]byte, SignatureSize)
	ret := ecdsaSignatureNormalize(c.ctx, &normalized[0], &sig[0])
	wasNormalized = ret == 1

	return
}

// Utility function to convert *byte to unsafe.Pointer
func bytesToPtr(b []byte) unsafe.Pointer {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Pointer(&b[0])
}
