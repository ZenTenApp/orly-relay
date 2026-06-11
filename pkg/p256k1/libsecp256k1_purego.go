//go:build !js && !wasm && !tinygo && !wasm32

package p256k1

import (
	"errors"

	"github.com/ebitengine/purego"
)

// request types for LibSecp256k1 actor
type libSecpCallReq struct {
	fn   func(*LibSecp256k1) interface{}
	resp chan interface{}
}

// LibSecp256k1 wraps the native libsecp256k1.so library using purego for CGO-free operation.
// This provides a way to benchmark against the C implementation without CGO.
type LibSecp256k1 struct {
	lib    uintptr
	ctx    uintptr
	loaded bool
	callCh chan libSecpCallReq
	stop   chan struct{}
	done   chan struct{}

	// Function pointers
	contextCreate        func(uint) uintptr
	contextDestroy       func(uintptr)
	contextRandomize     func(uintptr, *byte) int
	schnorrsigSign32     func(uintptr, *byte, *byte, *byte, *byte) int
	schnorrsigVerify     func(uintptr, *byte, *byte, uint, *byte) int
	keypairCreate        func(uintptr, *byte, *byte) int
	keypairXonlyPub      func(uintptr, *byte, *int, *byte) int
	xonlyPubkeyParse     func(uintptr, *byte, *byte) int
	ecPubkeyCreate       func(uintptr, *byte, *byte) int
	ecPubkeyParse        func(uintptr, *byte, *byte, uint) int
	ecPubkeySerialize    func(uintptr, *byte, *uint, *byte, uint) int
	xonlyPubkeySerialize func(uintptr, *byte, *byte) int
	ecdh                 func(uintptr, *byte, *byte, *byte, uintptr, uintptr) int

	// ECDSA function pointers
	ecdsaSign                    func(uintptr, *byte, *byte, *byte, uintptr, uintptr) int
	ecdsaVerify                  func(uintptr, *byte, *byte, *byte) int
	ecdsaSignatureParseDER       func(uintptr, *byte, *byte, uint) int
	ecdsaSignatureSerializeDER   func(uintptr, *byte, *uint, *byte) int
	ecdsaSignatureParseCompact   func(uintptr, *byte, *byte) int
	ecdsaSignatureSerializeCompact func(uintptr, *byte, *byte) int
	ecdsaSignatureNormalize      func(uintptr, *byte, *byte) int
	ecdsaRecover                 func(uintptr, *byte, *byte, *byte, int) int
	ecdsaRecoverableSignatureParseCompact func(uintptr, *byte, *byte, int) int
	ecdsaRecoverableSignatureSerializeCompact func(uintptr, *byte, *int, *byte) int
	ecdsaSignRecoverable         func(uintptr, *byte, *byte, *byte, uintptr, uintptr) int
}

// Secp256k1 context flags
// In modern libsecp256k1, SECP256K1_CONTEXT_NONE = 1 is the only valid flag.
// The old SIGN (256) and VERIFY (257) flags are deprecated.
const (
	libContextNone = 1
)

// Global instance - initialized once via init-once gate channel.
var (
	libSecp        *LibSecp256k1
	libSecpInitErr error
	libSecpInitGate = make(chan struct{})
)

func init() {
	go func() {
		defer close(libSecpInitGate)
		libSecp = newLibSecp256k1()
		paths := []string{
			"./libsecp256k1.so",
			"../libsecp256k1.so",
			"/home/mleku/src/p256k1.mleku.dev/libsecp256k1.so",
			"libsecp256k1.so",
		}
		for _, path := range paths {
			err := libSecp.loadInternal(path)
			if err == nil {
				libSecpInitErr = nil
				return
			}
			libSecpInitErr = err
		}
	}()
}

// GetLibSecp256k1 returns the global LibSecp256k1 instance, loading it if necessary.
// Returns nil and an error if the library cannot be loaded.
func GetLibSecp256k1() (*LibSecp256k1, error) {
	<-libSecpInitGate
	if libSecpInitErr != nil {
		return nil, libSecpInitErr
	}
	return libSecp, nil
}

// newLibSecp256k1 creates a new LibSecp256k1 with its actor goroutine.
func newLibSecp256k1() *LibSecp256k1 {
	l := &LibSecp256k1{
		callCh: make(chan libSecpCallReq),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go l.run()
	return l
}

func (l *LibSecp256k1) run() {
	defer close(l.done)
	for {
		select {
		case req := <-l.callCh:
			req.resp <- req.fn(l)
		case <-l.stop:
			return
		}
	}
}

// call sends a function to the actor and returns the result.
func (l *LibSecp256k1) call(fn func(*LibSecp256k1) interface{}) interface{} {
	resp := make(chan interface{}, 1)
	l.callCh <- libSecpCallReq{fn: fn, resp: resp}
	return <-resp
}

// Load loads the libsecp256k1.so library from the given path (through actor).
func (l *LibSecp256k1) Load(path string) error {
	result := l.call(func(ll *LibSecp256k1) interface{} {
		return ll.loadInternal(path)
	})
	if result == nil {
		return nil
	}
	return result.(error)
}

// loadInternal loads the library (must be called from actor or during init).
func (l *LibSecp256k1) loadInternal(path string) error {
	if l.loaded {
		return nil
	}

	lib, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return err
	}
	l.lib = lib

	// Register function pointers
	purego.RegisterLibFunc(&l.contextCreate, lib, "secp256k1_context_create")
	purego.RegisterLibFunc(&l.contextDestroy, lib, "secp256k1_context_destroy")
	purego.RegisterLibFunc(&l.contextRandomize, lib, "secp256k1_context_randomize")
	purego.RegisterLibFunc(&l.schnorrsigSign32, lib, "secp256k1_schnorrsig_sign32")
	purego.RegisterLibFunc(&l.schnorrsigVerify, lib, "secp256k1_schnorrsig_verify")
	purego.RegisterLibFunc(&l.keypairCreate, lib, "secp256k1_keypair_create")
	purego.RegisterLibFunc(&l.keypairXonlyPub, lib, "secp256k1_keypair_xonly_pub")
	purego.RegisterLibFunc(&l.xonlyPubkeyParse, lib, "secp256k1_xonly_pubkey_parse")
	purego.RegisterLibFunc(&l.ecPubkeyCreate, lib, "secp256k1_ec_pubkey_create")
	purego.RegisterLibFunc(&l.ecPubkeyParse, lib, "secp256k1_ec_pubkey_parse")
	purego.RegisterLibFunc(&l.ecPubkeySerialize, lib, "secp256k1_ec_pubkey_serialize")
	purego.RegisterLibFunc(&l.xonlyPubkeySerialize, lib, "secp256k1_xonly_pubkey_serialize")
	purego.RegisterLibFunc(&l.ecdh, lib, "secp256k1_ecdh")

	// Register ECDSA function pointers
	purego.RegisterLibFunc(&l.ecdsaSign, lib, "secp256k1_ecdsa_sign")
	purego.RegisterLibFunc(&l.ecdsaVerify, lib, "secp256k1_ecdsa_verify")
	purego.RegisterLibFunc(&l.ecdsaSignatureParseDER, lib, "secp256k1_ecdsa_signature_parse_der")
	purego.RegisterLibFunc(&l.ecdsaSignatureSerializeDER, lib, "secp256k1_ecdsa_signature_serialize_der")
	purego.RegisterLibFunc(&l.ecdsaSignatureParseCompact, lib, "secp256k1_ecdsa_signature_parse_compact")
	purego.RegisterLibFunc(&l.ecdsaSignatureSerializeCompact, lib, "secp256k1_ecdsa_signature_serialize_compact")
	purego.RegisterLibFunc(&l.ecdsaSignatureNormalize, lib, "secp256k1_ecdsa_signature_normalize")

	// Register ECDSA recovery functions (may not be available in all builds)
	// These are optional - the library may not have the recovery module enabled
	// We use a helper that ignores errors for optional functions
	registerOptional := func(fn interface{}, lib uintptr, name string) {
		defer func() { recover() }() // Ignore panic from missing symbol
		purego.RegisterLibFunc(fn, lib, name)
	}
	registerOptional(&l.ecdsaRecover, lib, "secp256k1_ecdsa_recover")
	registerOptional(&l.ecdsaRecoverableSignatureParseCompact, lib, "secp256k1_ecdsa_recoverable_signature_parse_compact")
	registerOptional(&l.ecdsaRecoverableSignatureSerializeCompact, lib, "secp256k1_ecdsa_recoverable_signature_serialize_compact")
	registerOptional(&l.ecdsaSignRecoverable, lib, "secp256k1_ecdsa_sign_recoverable")

	// Create context (modern libsecp256k1 uses SECP256K1_CONTEXT_NONE = 1)
	l.ctx = l.contextCreate(libContextNone)
	if l.ctx == 0 {
		return errors.New("failed to create secp256k1 context")
	}

	// Randomize context for better security
	var seed [32]byte
	// Use zero seed for deterministic benchmarks
	l.contextRandomize(l.ctx, &seed[0])

	l.loaded = true
	return nil
}

// Close releases the library resources.
func (l *LibSecp256k1) Close() {
	l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return nil
		}
		if ll.ctx != 0 {
			ll.contextDestroy(ll.ctx)
			ll.ctx = 0
		}
		if ll.lib != 0 {
			purego.Dlclose(ll.lib)
			ll.lib = 0
		}
		ll.loaded = false
		return nil
	})
}

// IsLoaded returns true if the library is loaded.
func (l *LibSecp256k1) IsLoaded() bool {
	result := l.call(func(ll *LibSecp256k1) interface{} {
		return ll.loaded
	})
	return result.(bool)
}

// SchnorrSign signs a 32-byte message using a 32-byte secret key.
// Returns a 64-byte signature.
func (l *LibSecp256k1) SchnorrSign(msg32, seckey32 []byte) ([]byte, error) {
	type result struct {
		sig []byte
		err error
	}
	r := l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return result{nil, errors.New("library not loaded")}
		}
		if len(msg32) != 32 {
			return result{nil, errors.New("message must be 32 bytes")}
		}
		if len(seckey32) != 32 {
			return result{nil, errors.New("secret key must be 32 bytes")}
		}
		keypair := make([]byte, 96)
		if ll.keypairCreate(ll.ctx, &keypair[0], &seckey32[0]) != 1 {
			return result{nil, errors.New("failed to create keypair")}
		}
		sig := make([]byte, 64)
		if ll.schnorrsigSign32(ll.ctx, &sig[0], &msg32[0], &keypair[0], nil) != 1 {
			return result{nil, errors.New("signing failed")}
		}
		return result{sig, nil}
	}).(result)
	return r.sig, r.err
}

// SchnorrVerify verifies a Schnorr signature.
func (l *LibSecp256k1) SchnorrVerify(sig64, msg32, pubkey32 []byte) bool {
	return l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return false
		}
		if len(sig64) != 64 || len(msg32) != 32 || len(pubkey32) != 32 {
			return false
		}
		xonlyPubkey := make([]byte, 64)
		if ll.xonlyPubkeyParse(ll.ctx, &xonlyPubkey[0], &pubkey32[0]) != 1 {
			return false
		}
		return ll.schnorrsigVerify(ll.ctx, &sig64[0], &msg32[0], 32, &xonlyPubkey[0]) == 1
	}).(bool)
}

// CreatePubkey derives a public key from a secret key.
// Returns the 32-byte x-only public key.
func (l *LibSecp256k1) CreatePubkey(seckey32 []byte) ([]byte, error) {
	type result struct {
		pub []byte
		err error
	}
	r := l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return result{nil, errors.New("library not loaded")}
		}
		if len(seckey32) != 32 {
			return result{nil, errors.New("secret key must be 32 bytes")}
		}
		keypair := make([]byte, 96)
		if ll.keypairCreate(ll.ctx, &keypair[0], &seckey32[0]) != 1 {
			return result{nil, errors.New("failed to create keypair")}
		}
		xonlyPubkey := make([]byte, 64)
		var parity int
		if ll.keypairXonlyPub(ll.ctx, &xonlyPubkey[0], &parity, &keypair[0]) != 1 {
			return result{nil, errors.New("failed to extract x-only pubkey")}
		}
		pubkey32 := make([]byte, 32)
		if ll.xonlyPubkeySerialize(ll.ctx, &pubkey32[0], &xonlyPubkey[0]) != 1 {
			return result{nil, errors.New("failed to serialize x-only pubkey")}
		}
		return result{pubkey32, nil}
	}).(result)
	return r.pub, r.err
}

// CreatePubkeyCompressed derives a compressed public key (33 bytes) from a secret key.
// Returns (compressed_pubkey, parity) where parity is 0 for even Y, 1 for odd Y.
func (l *LibSecp256k1) CreatePubkeyCompressed(seckey32 []byte) ([]byte, int, error) {
	type result struct {
		pub    []byte
		parity int
		err    error
	}
	r := l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return result{nil, 0, errors.New("library not loaded")}
		}
		if len(seckey32) != 32 {
			return result{nil, 0, errors.New("secret key must be 32 bytes")}
		}
		pubkey := make([]byte, 64)
		if ll.ecPubkeyCreate(ll.ctx, &pubkey[0], &seckey32[0]) != 1 {
			return result{nil, 0, errors.New("failed to create pubkey")}
		}
		compressed := make([]byte, 33)
		var outputLen uint = 33
		const SECP256K1_EC_COMPRESSED = 258
		if ll.ecPubkeySerialize(ll.ctx, &compressed[0], &outputLen, &pubkey[0], SECP256K1_EC_COMPRESSED) != 1 {
			return result{nil, 0, errors.New("failed to serialize pubkey")}
		}
		parity := 0
		if compressed[0] == 0x03 {
			parity = 1
		}
		return result{compressed, parity, nil}
	}).(result)
	return r.pub, r.parity, r.err
}

// CreatePubkeyUncompressed derives an uncompressed public key (65 bytes) from a secret key.
func (l *LibSecp256k1) CreatePubkeyUncompressed(seckey32 []byte) ([]byte, error) {
	type result struct {
		pub []byte
		err error
	}
	r := l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return result{nil, errors.New("library not loaded")}
		}
		if len(seckey32) != 32 {
			return result{nil, errors.New("secret key must be 32 bytes")}
		}
		pubkey := make([]byte, 64)
		if ll.ecPubkeyCreate(ll.ctx, &pubkey[0], &seckey32[0]) != 1 {
			return result{nil, errors.New("failed to create pubkey")}
		}
		uncompressed := make([]byte, 65)
		var outputLen uint = 65
		const SECP256K1_EC_UNCOMPRESSED = 2
		if ll.ecPubkeySerialize(ll.ctx, &uncompressed[0], &outputLen, &pubkey[0], SECP256K1_EC_UNCOMPRESSED) != 1 {
			return result{nil, errors.New("failed to serialize pubkey")}
		}
		return result{uncompressed, nil}
	}).(result)
	return r.pub, r.err
}

// ECDH computes the shared secret using ECDH.
func (l *LibSecp256k1) ECDH(seckey32, pubkey33 []byte) ([]byte, error) {
	type result struct {
		secret []byte
		err    error
	}
	r := l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return result{nil, errors.New("library not loaded")}
		}
		if len(seckey32) != 32 {
			return result{nil, errors.New("secret key must be 32 bytes")}
		}
		if len(pubkey33) != 33 && len(pubkey33) != 65 {
			return result{nil, errors.New("public key must be 33 or 65 bytes")}
		}
		pubkey := make([]byte, 64)
		if ll.ecPubkeyParse(ll.ctx, &pubkey[0], &pubkey33[0], uint(len(pubkey33))) != 1 {
			return result{nil, errors.New("failed to parse public key")}
		}
		output := make([]byte, 32)
		if ll.ecdh(ll.ctx, &output[0], &pubkey[0], &seckey32[0], 0, 0) != 1 {
			return result{nil, errors.New("ECDH failed")}
		}
		return result{output, nil}
	}).(result)
	return r.secret, r.err
}

// ECDSASign signs a 32-byte message hash with a secret key.
// Returns a 64-byte compact signature (r || s).
func (l *LibSecp256k1) ECDSASign(msghash32, seckey32 []byte) ([]byte, error) {
	type result struct {
		sig []byte
		err error
	}
	r := l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return result{nil, errors.New("library not loaded")}
		}
		if len(msghash32) != 32 {
			return result{nil, errors.New("message hash must be 32 bytes")}
		}
		if len(seckey32) != 32 {
			return result{nil, errors.New("secret key must be 32 bytes")}
		}
		sig := make([]byte, 64)
		if ll.ecdsaSign(ll.ctx, &sig[0], &msghash32[0], &seckey32[0], 0, 0) != 1 {
			return result{nil, errors.New("ECDSA signing failed")}
		}
		ll.ecdsaSignatureNormalize(ll.ctx, &sig[0], &sig[0])
		compact := make([]byte, 64)
		if ll.ecdsaSignatureSerializeCompact(ll.ctx, &compact[0], &sig[0]) != 1 {
			return result{nil, errors.New("failed to serialize signature")}
		}
		return result{compact, nil}
	}).(result)
	return r.sig, r.err
}

// ECDSAVerify verifies an ECDSA signature.
// sig64 is a 64-byte compact signature (r || s).
// msghash32 is a 32-byte message hash.
// pubkey33 is a 33-byte compressed public key (or 65-byte uncompressed).
func (l *LibSecp256k1) ECDSAVerify(sig64, msghash32, pubkey33 []byte) bool {
	return l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return false
		}
		if len(sig64) != 64 || len(msghash32) != 32 {
			return false
		}
		if len(pubkey33) != 33 && len(pubkey33) != 65 {
			return false
		}
		sig := make([]byte, 64)
		if ll.ecdsaSignatureParseCompact(ll.ctx, &sig[0], &sig64[0]) != 1 {
			return false
		}
		pubkey := make([]byte, 64)
		if ll.ecPubkeyParse(ll.ctx, &pubkey[0], &pubkey33[0], uint(len(pubkey33))) != 1 {
			return false
		}
		return ll.ecdsaVerify(ll.ctx, &sig[0], &msghash32[0], &pubkey[0]) == 1
	}).(bool)
}

// ECDSASignDER signs and returns a DER-encoded signature.
func (l *LibSecp256k1) ECDSASignDER(msghash32, seckey32 []byte) ([]byte, error) {
	type result struct {
		der []byte
		err error
	}
	r := l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return result{nil, errors.New("library not loaded")}
		}
		if len(msghash32) != 32 {
			return result{nil, errors.New("message hash must be 32 bytes")}
		}
		if len(seckey32) != 32 {
			return result{nil, errors.New("secret key must be 32 bytes")}
		}
		sig := make([]byte, 64)
		if ll.ecdsaSign(ll.ctx, &sig[0], &msghash32[0], &seckey32[0], 0, 0) != 1 {
			return result{nil, errors.New("ECDSA signing failed")}
		}
		ll.ecdsaSignatureNormalize(ll.ctx, &sig[0], &sig[0])
		der := make([]byte, 72)
		var derLen uint = 72
		if ll.ecdsaSignatureSerializeDER(ll.ctx, &der[0], &derLen, &sig[0]) != 1 {
			return result{nil, errors.New("failed to serialize DER signature")}
		}
		return result{der[:derLen], nil}
	}).(result)
	return r.der, r.err
}

// ECDSAVerifyDER verifies a DER-encoded ECDSA signature.
func (l *LibSecp256k1) ECDSAVerifyDER(sigDER, msghash32, pubkey33 []byte) bool {
	return l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return false
		}
		if len(msghash32) != 32 {
			return false
		}
		if len(pubkey33) != 33 && len(pubkey33) != 65 {
			return false
		}
		sig := make([]byte, 64)
		if ll.ecdsaSignatureParseDER(ll.ctx, &sig[0], &sigDER[0], uint(len(sigDER))) != 1 {
			return false
		}
		pubkey := make([]byte, 64)
		if ll.ecPubkeyParse(ll.ctx, &pubkey[0], &pubkey33[0], uint(len(pubkey33))) != 1 {
			return false
		}
		return ll.ecdsaVerify(ll.ctx, &sig[0], &msghash32[0], &pubkey[0]) == 1
	}).(bool)
}

// ECDSASignRecoverable signs and returns a recoverable signature (65 bytes: 64 + recid).
func (l *LibSecp256k1) ECDSASignRecoverable(msghash32, seckey32 []byte) ([]byte, int, error) {
	type result struct {
		sig   []byte
		recid int
		err   error
	}
	r := l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return result{nil, 0, errors.New("library not loaded")}
		}
		if len(msghash32) != 32 {
			return result{nil, 0, errors.New("message hash must be 32 bytes")}
		}
		if len(seckey32) != 32 {
			return result{nil, 0, errors.New("secret key must be 32 bytes")}
		}
		sig := make([]byte, 65)
		if ll.ecdsaSignRecoverable(ll.ctx, &sig[0], &msghash32[0], &seckey32[0], 0, 0) != 1 {
			return result{nil, 0, errors.New("ECDSA recoverable signing failed")}
		}
		compact := make([]byte, 64)
		var recid int
		if ll.ecdsaRecoverableSignatureSerializeCompact(ll.ctx, &compact[0], &recid, &sig[0]) != 1 {
			return result{nil, 0, errors.New("failed to serialize recoverable signature")}
		}
		return result{compact, recid, nil}
	}).(result)
	return r.sig, r.recid, r.err
}

// ECDSARecover recovers a public key from a signature.
// sig64 is a 64-byte compact signature, recid is the recovery id (0-3).
// Returns a 33-byte compressed public key.
func (l *LibSecp256k1) ECDSARecover(sig64, msghash32 []byte, recid int) ([]byte, error) {
	type result struct {
		pub []byte
		err error
	}
	r := l.call(func(ll *LibSecp256k1) interface{} {
		if !ll.loaded {
			return result{nil, errors.New("library not loaded")}
		}
		if len(sig64) != 64 {
			return result{nil, errors.New("signature must be 64 bytes")}
		}
		if len(msghash32) != 32 {
			return result{nil, errors.New("message hash must be 32 bytes")}
		}
		if recid < 0 || recid > 3 {
			return result{nil, errors.New("recovery id must be 0-3")}
		}
		sig := make([]byte, 65)
		if ll.ecdsaRecoverableSignatureParseCompact(ll.ctx, &sig[0], &sig64[0], recid) != 1 {
			return result{nil, errors.New("failed to parse recoverable signature")}
		}
		pubkey := make([]byte, 64)
		if ll.ecdsaRecover(ll.ctx, &pubkey[0], &sig[0], &msghash32[0], recid) != 1 {
			return result{nil, errors.New("public key recovery failed")}
		}
		compressed := make([]byte, 33)
		var outputLen uint = 33
		const SECP256K1_EC_COMPRESSED = 258
		if ll.ecPubkeySerialize(ll.ctx, &compressed[0], &outputLen, &pubkey[0], SECP256K1_EC_COMPRESSED) != 1 {
			return result{nil, errors.New("failed to serialize recovered pubkey")}
		}
		return result{compressed, nil}
	}).(result)
	return r.pub, r.err
}
