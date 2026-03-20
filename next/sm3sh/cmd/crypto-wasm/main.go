// WASM module exporting secp256k1 Schnorr operations.
// Compiled with: GOOS=js GOARCH=wasm go build -o crypto.wasm ./cmd/crypto-wasm/
package main

import (
	"smesh3/crypto/secp256k1"
	"syscall/js"
)

func main() {
	js.Global().Set("_secp256k1_pubkey", js.FuncOf(pubKeyFromSecKey))
	js.Global().Set("_secp256k1_sign", js.FuncOf(signSchnorr))
	js.Global().Set("_secp256k1_verify", js.FuncOf(verifySchnorr))
	// Block forever — keep the Go runtime alive.
	<-make(chan struct{})
}

func pubKeyFromSecKey(_ js.Value, args []js.Value) any {
	seckey := jsBytes32(args[0])
	pub, ok := secp256k1.PubKeyFromSecKey(seckey)
	if !ok {
		return js.Null()
	}
	return toJS(pub[:])
}

func signSchnorr(_ js.Value, args []js.Value) any {
	seckey := jsBytes32(args[0])
	msg := jsBytes32(args[1])
	aux := jsBytes32(args[2])
	sig, ok := secp256k1.SignSchnorr(seckey, msg, aux)
	if !ok {
		return js.Null()
	}
	return toJS(sig[:])
}

func verifySchnorr(_ js.Value, args []js.Value) any {
	pubkey := jsBytes32(args[0])
	msg := jsBytes32(args[1])
	sig := jsBytes64(args[2])
	return secp256k1.VerifySchnorr(pubkey, msg, sig)
}

func jsBytes32(v js.Value) [32]byte {
	var b [32]byte
	buf := make([]byte, 32)
	js.CopyBytesToGo(buf, v)
	copy(b[:], buf)
	return b
}

func jsBytes64(v js.Value) [64]byte {
	var b [64]byte
	buf := make([]byte, 64)
	js.CopyBytesToGo(buf, v)
	copy(b[:], buf)
	return b
}

func toJS(b []byte) any {
	if b == nil {
		return js.Null()
	}
	r := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(r, b)
	return r
}
