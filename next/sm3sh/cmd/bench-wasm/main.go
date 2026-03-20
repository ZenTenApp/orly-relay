package main

import (
	"fmt"
	"smesh3/crypto/secp256k1"
	"smesh3/crypto/sha256"
	"smesh3/helpers"
	"syscall/js"
	"time"
)

func main() {
	seckey := sha256.Sum([]byte("benchmark test secret key material"))
	pubkey, ok := secp256k1.PubKeyFromSecKey(seckey)
	if !ok {
		fmt.Println("FAIL: PubKeyFromSecKey")
		return
	}

	fmt.Println("=== sm3sh crypto benchmark (Go WASM) ===")
	fmt.Printf("pubkey: %s\n", helpers.HexEncode(pubkey[:]))

	const n = 100

	// Pre-hash messages.
	var msgs [n][32]byte
	for i := range msgs {
		msgs[i] = sha256.Sum([]byte(fmt.Sprintf("benchmark message %d", i)))
	}

	// Aux randomness (zeros for deterministic bench).
	var aux [32]byte

	// Sign.
	start := time.Now()
	var sigs [n][64]byte
	for i := range msgs {
		sigs[i], _ = secp256k1.SignSchnorr(seckey, msgs[i], aux)
	}
	signDur := time.Since(start)

	// Verify.
	start = time.Now()
	valid := 0
	for i := range msgs {
		if secp256k1.VerifySchnorr(pubkey, msgs[i], sigs[i]) {
			valid++
		}
	}
	verifyDur := time.Since(start)

	fmt.Printf("\nsign   %d ops: %v  (%.1f ops/sec)\n", n, signDur, float64(n)/signDur.Seconds())
	fmt.Printf("verify %d ops: %v  (%.1f ops/sec)\n", n, verifyDur, float64(n)/verifyDur.Seconds())
	fmt.Printf("valid: %d/%d\n", valid, n)

	// Keep alive briefly so output flushes.
	done := make(chan struct{})
	js.Global().Get("setTimeout").Invoke(js.FuncOf(func(this js.Value, args []js.Value) any {
		close(done)
		return nil
	}), 100)
	<-done
}
