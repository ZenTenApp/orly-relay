// +build js,wasm

package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"git.smesh.lol/orly/pkg/p256k1"
)

func main() {
	type vec struct {
		sec string
		x   string
		pre byte
	}
	vectors := []vec{
		{"0000000000000000000000000000000000000000000000000000000000000001",
			"79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798", 0x02},
		{"0000000000000000000000000000000000000000000000000000000000000002",
			"c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5", 0x02},
		{"0000000000000000000000000000000000000000000000000000000000000003",
			"f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9", 0x02},
		{"0000000000000000000000000000000000000000000000000000000000000007",
			"5cbdf0646e5db4eaa398f365f2ea7a0e3d419b7e0330e39ce92bddedcac4f9bc", 0x02},
		{"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
			"39a36013301597daef41fbe593a02cc513d0b55527ec2df1050e2e8ff49c85c2", 0x03},
		{"deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef",
			"e3f23b3410abfe4d3e46cf47ca5f9ef47f2b042414a6d2aa527858fce0ad1146", 0x02},
		{"7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0",
			"00000000000000000000003b78ce563f89a0ed9414f5aa28ad0d96d6795f9c63", 0x03},
		{"5363ad4cc05c30e0a5261c028812645a122e22ea20816678df02967c1b23bd72",
			"bcace2e99da01887ab0102b696902325872844067f15e98da7bba04400b88fcb", 0x02},
	}

	fails := 0

	fmt.Println("=== GLV path (ECPubkeyCreate) ===")
	for _, v := range vectors {
		secBytes, _ := hex.DecodeString(v.sec)
		var pub p256k1.PublicKey
		if err := p256k1.ECPubkeyCreate(&pub, secBytes); err != nil {
			fmt.Printf("FAIL [%s] err: %v\n", v.sec[:8], err)
			fails++
			continue
		}
		var ser [33]byte
		p256k1.ECPubkeySerialize(ser[:], &pub, p256k1.ECCompressed)
		gotX := hex.EncodeToString(ser[1:33])
		ok := gotX == v.x && ser[0] == v.pre
		if ok {
			fmt.Printf("PASS [%s]\n", v.sec[:8])
		} else {
			fmt.Printf("FAIL [%s] got 0x%02x:%s\n", v.sec[:8], ser[0], gotX)
			fails++
		}
	}

	fmt.Println("\n=== Simple path (non-GLV) ===")
	for _, v := range vectors {
		secBytes, _ := hex.DecodeString(v.sec)
		ser := p256k1.TestEcmultGenSimple(secBytes)
		gotX := hex.EncodeToString(ser[1:33])
		ok := gotX == v.x && ser[0] == v.pre
		if ok {
			fmt.Printf("PASS [%s]\n", v.sec[:8])
		} else {
			fmt.Printf("FAIL [%s] got 0x%02x:%s\n", v.sec[:8], ser[0], gotX)
			fails++
		}
	}

	if fails > 0 {
		fmt.Printf("\n%d FAILED\n", fails)
		os.Exit(1)
	}
	fmt.Println("\nALL PASS")
}
