package main

import (
	"fmt"
	"os"

	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

func main() {
	for _, arg := range os.Args[1:] {
		sk, err := hex.Dec(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad key %s: %v\n", arg, err)
			continue
		}
		signer, err := keys.SecretBytesToSigner(sk)
		if err != nil {
			fmt.Fprintf(os.Stderr, "signer %s: %v\n", arg, err)
			continue
		}
		fmt.Printf("%s\n", hex.Enc(signer.Pub()))
	}
}
