package p256k1

import (
	"errors"
)

// PublicKey represents a secp256k1 public key
type PublicKey struct {
	data [64]byte // Internal representation
}

// Compression flags for public key serialization
const (
	ECCompressed   = 0x02
	ECUncompressed = 0x04
)

// ECPubkeyParse parses a public key from bytes
func ECPubkeyParse(pubkey *PublicKey, input []byte) error {
	if len(input) == 0 {
		return errors.New("input cannot be empty")
	}
	
	var point GroupElementAffine
	
	switch len(input) {
	case 33:
		// Compressed format
		if input[0] != 0x02 && input[0] != 0x03 {
			return errors.New("invalid compressed public key prefix")
		}
		
		// Extract X coordinate
		var x FieldElement
		if err := x.setB32(input[1:33]); err != nil {
			return err
		}
		
		// Determine Y coordinate from X and parity
		odd := input[0] == 0x03
		if !point.setXOVar(&x, odd) {
			return errors.New("invalid public key")
		}
		
	case 65:
		// Uncompressed format
		if input[0] != 0x04 {
			return errors.New("invalid uncompressed public key prefix")
		}
		
		// Extract X and Y coordinates
		var x, y FieldElement
		if err := x.setB32(input[1:33]); err != nil {
			return err
		}
		if err := y.setB32(input[33:65]); err != nil {
			return err
		}
		
		point.setXY(&x, &y)
		
	default:
		return errors.New("invalid public key length")
	}
	
	// Validate the point is on the curve
	if !point.isValid() {
		return errors.New("public key not on curve")
	}
	
	// Store in internal format
	point.toBytes(pubkey.data[:])
	
	return nil
}

// ECPubkeySerialize serializes a public key to bytes
func ECPubkeySerialize(output []byte, pubkey *PublicKey, flags uint) int {
	// Load the public key
	var point GroupElementAffine
	point.fromBytes(pubkey.data[:])
	
	if point.isInfinity() {
		return 0 // Invalid public key
	}
	
	// Normalize coordinates
	point.x.normalize()
	point.y.normalize()
	
	if flags == ECCompressed {
		if len(output) < 33 {
			return 0 // Buffer too small
		}
		
		// Compressed format: 0x02/0x03 + X coordinate
		if point.y.isOdd() {
			output[0] = 0x03
		} else {
			output[0] = 0x02
		}
		point.x.getB32(output[1:33])
		return 33
		
	} else if flags == ECUncompressed {
		if len(output) < 65 {
			return 0 // Buffer too small
		}
		
		// Uncompressed format: 0x04 + X + Y coordinates
		output[0] = 0x04
		point.x.getB32(output[1:33])
		point.y.getB32(output[33:65])
		return 65
		
	} else {
		return 0 // Invalid flags
	}
}

// ECPubkeyCmp compares two public keys
func ECPubkeyCmp(pubkey1, pubkey2 *PublicKey) int {
	// Load both public keys
	var point1, point2 GroupElementAffine
	point1.fromBytes(pubkey1.data[:])
	point2.fromBytes(pubkey2.data[:])
	
	if point1.equal(&point2) {
		return 0
	}
	
	// For ordering, compare the serialized forms
	var buf1, buf2 [33]byte
	ECPubkeySerialize(buf1[:], pubkey1, ECCompressed)
	ECPubkeySerialize(buf2[:], pubkey2, ECCompressed)
	
	for i := 0; i < 33; i++ {
		if buf1[i] < buf2[i] {
			return -1
		}
		if buf1[i] > buf2[i] {
			return 1
		}
	}
	
	return 0
}

// ECPubkeyCreate creates a public key from a private key
func ECPubkeyCreate(pubkey *PublicKey, seckey []byte) error {
	if len(seckey) != 32 {
		return errors.New("private key must be 32 bytes")
	}
	
	// Parse the private key as a scalar
	var scalar Scalar
	if !scalar.setB32Seckey(seckey) {
		return errors.New("invalid private key")
	}
	
	// Compute pubkey = scalar * G
	var point GroupElementJacobian
	EcmultGen(&point, &scalar)
	
	// Convert to affine and store directly - optimize by avoiding intermediate copy
	var affine GroupElementAffine
	affine.setGEJ(&point)
	
	// Normalize in-place and write directly to pubkey.data to avoid copy allocation
	affine.x.normalize()
	affine.y.normalize()
	affine.x.getB32(pubkey.data[:32])
	affine.y.getB32(pubkey.data[32:64])
	
	// Clear sensitive data
	scalar.clear()
	point.clear()
	affine.clear()
	
	return nil
}

// pubkeyLoad loads a public key from internal format (helper function)
func pubkeyLoad(point *GroupElementAffine, pubkey *PublicKey) {
	point.fromBytes(pubkey.data[:])
}

// pubkeySave saves a public key to internal format (helper function)
func pubkeySave(pubkey *PublicKey, point *GroupElementAffine) {
	point.toBytes(pubkey.data[:])
}
