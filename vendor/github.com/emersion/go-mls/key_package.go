package mls

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/cryptobyte"
)

// A KeyPackage provides some public information about a user, such as
// a supported protocol version and cipher suite, public keys, and credentials.
//
// Key packages should not be used more than once.
type KeyPackage struct {
	version     protocolVersion
	cipherSuite CipherSuite
	initKey     hpkePublicKey
	leafNode    leafNode
	extensions  []extension
	signature   []byte
}

// UnmarshalKeyPackage reads a key package encoded as an MLS message.
func UnmarshalKeyPackage(raw []byte) (*KeyPackage, error) {
	var msg mlsMessage
	if err := unmarshal(raw, &msg); err != nil {
		return nil, err
	} else if msg.wireFormat != wireFormatMLSKeyPackage {
		return nil, fmt.Errorf("mls: expected a key package message, got wire format %v", msg.wireFormat)
	}
	return msg.keyPackage, nil
}

// Bytes encodes the key package wrapped in an MLSMessage envelope
// (version + wireFormat + keyPackage). For the bare TLS serialization
// without the envelope, use RawBytes().
func (pkg *KeyPackage) Bytes() []byte {
	raw, err := marshal(&mlsMessage{
		version:    protocolVersionMLS10,
		wireFormat: wireFormatMLSKeyPackage,
		keyPackage: pkg,
	})
	if err != nil {
		// should never happen
		panic(fmt.Errorf("mls: failed to marshal key package message: %v", err))
	}
	return raw
}

// RawBytes encodes the key package as bare TLS bytes without the MLSMessage
// envelope. This is the format used by NIP-EE (kind 443 events).
func (pkg *KeyPackage) RawBytes() []byte {
	var b cryptobyte.Builder
	pkg.marshal(&b)
	raw, err := b.Bytes()
	if err != nil {
		panic(fmt.Errorf("mls: failed to marshal raw key package: %v", err))
	}
	return raw
}

// UnmarshalRawKeyPackage reads a key package from bare TLS bytes (no MLSMessage
// envelope). This is the format used by NIP-EE (kind 443 events).
func UnmarshalRawKeyPackage(raw []byte) (*KeyPackage, error) {
	s := cryptobyte.String(raw)
	pkg := new(KeyPackage)
	if err := pkg.unmarshal(&s); err != nil {
		return nil, err
	}
	if !s.Empty() {
		return nil, fmt.Errorf("mls: raw key package contains %d excess bytes", len(s))
	}
	return pkg, nil
}

func (pkg *KeyPackage) unmarshal(s *cryptobyte.String) error {
	*pkg = KeyPackage{}

	ok := s.ReadUint16((*uint16)(&pkg.version)) &&
		s.ReadUint16((*uint16)(&pkg.cipherSuite)) &&
		readOpaqueVec(s, (*[]byte)(&pkg.initKey))
	if !ok {
		return io.ErrUnexpectedEOF
	}

	if pkg.version != protocolVersionMLS10 {
		return fmt.Errorf("mls: invalid protocol version %d", pkg.version)
	}

	if err := pkg.leafNode.unmarshal(s); err != nil {
		return err
	}

	exts, err := unmarshalExtensionVec(s)
	if err != nil {
		return err
	}
	pkg.extensions = exts

	if !readOpaqueVec(s, &pkg.signature) {
		return err
	}

	return nil
}

func (pkg *KeyPackage) marshalTBS(b *cryptobyte.Builder) {
	b.AddUint16(uint16(pkg.version))
	b.AddUint16(uint16(pkg.cipherSuite))
	writeOpaqueVec(b, []byte(pkg.initKey))
	pkg.leafNode.marshal(b)
	marshalExtensionVec(b, pkg.extensions)
}

func (pkg *KeyPackage) marshal(b *cryptobyte.Builder) {
	pkg.marshalTBS(b)
	writeOpaqueVec(b, pkg.signature)
}

func (pkg *KeyPackage) sign(signerPriv signaturePrivateKey) error {
	var b cryptobyte.Builder
	pkg.marshalTBS(&b)
	rawTBS, err := b.Bytes()
	if err != nil {
		return err
	}

	sig, err := pkg.cipherSuite.signWithLabel(signerPriv, []byte("KeyPackageTBS"), rawTBS)
	if err != nil {
		return err
	}

	pkg.signature = sig
	return nil
}

func (pkg *KeyPackage) verifySignature() bool {
	var b cryptobyte.Builder
	pkg.marshalTBS(&b)
	rawTBS, err := b.Bytes()
	if err != nil {
		return false
	}

	return pkg.cipherSuite.verifyWithLabel(pkg.leafNode.signatureKey, []byte("KeyPackageTBS"), rawTBS, pkg.signature)
}

// verify performs KeyPackage verification as described in RFC 9420 section 10.1.
func (pkg *KeyPackage) verify(ctx *groupContext) error {
	if pkg.version != ctx.version {
		return fmt.Errorf("mls: key package version doesn't match group context")
	}
	if pkg.cipherSuite != ctx.cipherSuite {
		return fmt.Errorf("mls: cipher suite doesn't match group context")
	}
	if pkg.leafNode.leafNodeSource != leafNodeSourceKeyPackage {
		return fmt.Errorf("mls: key package contains a leaf node with an invalid source")
	}
	if !pkg.verifySignature() {
		return fmt.Errorf("mls: invalid key package signature")
	}
	if bytes.Equal(pkg.leafNode.encryptionKey, pkg.initKey) {
		return fmt.Errorf("mls: key package encryption key and init key are identical")
	}
	return nil
}

// GenerateRef generates this key package's reference.
func (pkg *KeyPackage) GenerateRef() (KeyPackageRef, error) {
	var b cryptobyte.Builder
	pkg.marshal(&b)
	raw, err := b.Bytes()
	if err != nil {
		return nil, err
	}

	hash, err := pkg.cipherSuite.refHash([]byte("MLS 1.0 KeyPackage Reference"), raw)
	if err != nil {
		return nil, err
	}

	return KeyPackageRef(hash), nil
}

// KeyPackageRef is a hash uniquely identifying a key package.
type KeyPackageRef []byte

// Equal checks whether two key package references are equal.
func (ref KeyPackageRef) Equal(other KeyPackageRef) bool {
	return bytes.Equal([]byte(ref), []byte(other))
}

// PrivateKeyPackage holds private information about a user.
type PrivateKeyPackage struct {
	InitKey       []byte
	EncryptionKey []byte
	SignatureKey  []byte
}

// KeyPairPackage holds both public and private information about a user.
type KeyPairPackage struct {
	Public  KeyPackage
	Private PrivateKeyPackage
}

// KeyPackageOptions configures key package generation.
type KeyPackageOptions struct {
	// CapabilityExtensions are extension types to advertise in the leaf
	// node capabilities (e.g., ExtensionTypeLastResort, ExtensionTypeNostrGroupData).
	// RatchetTree is always included.
	CapabilityExtensions []extensionType

	// LeafExtensions are extensions to include in the leaf node itself
	// (e.g., a LastResort extension with empty data).
	LeafExtensions []extension

	// KeyPackageExtensions are extensions to include at the key package level.
	KeyPackageExtensions []extension
}

// GenerateKeyPairPackage generates a new key pair package.
func GenerateKeyPairPackage(cs CipherSuite, credential *Credential) (*KeyPairPackage, error) {
	return GenerateKeyPairPackageWithOptions(cs, credential, nil)
}

// GenerateKeyPairPackageWithOptions generates a new key pair package with
// custom capabilities and extensions.
func GenerateKeyPairPackageWithOptions(cs CipherSuite, credential *Credential, opts *KeyPackageOptions) (*KeyPairPackage, error) {
	initPub, initPriv, err := cs.generateEncryptionKeyPair()
	if err != nil {
		return nil, err
	}

	encPub, encPriv, err := cs.generateEncryptionKeyPair()
	if err != nil {
		return nil, err
	}

	sigPub, sigPriv, err := cs.signatureScheme().GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	var capExts []extensionType
	var leafExts []extension
	var kpExts []extension

	if opts != nil {
		capExts = opts.CapabilityExtensions
		leafExts = opts.LeafExtensions
		kpExts = opts.KeyPackageExtensions
	}
	if capExts == nil {
		// Default: only ratchetTree for backwards compat when no opts given
		capExts = []extensionType{extensionTypeRatchetTree}
	}

	keyPkg := KeyPackage{
		version:     protocolVersionMLS10,
		cipherSuite: cs,
		initKey:     initPub,
		leafNode: leafNode{
			encryptionKey:  encPub,
			signatureKey:   sigPub,
			leafNodeSource: leafNodeSourceKeyPackage,
			credential:     *credential,
			capabilities: capabilities{
				versions:     []protocolVersion{protocolVersionMLS10},
				cipherSuites: []CipherSuite{cs},
				extensions:   capExts,
				proposals:    []proposalType{proposalTypeAdd, proposalTypeUpdate, proposalTypeRemove},
				credentials:  []credentialType{credentialTypeBasic},
			},
			lifetime:   newLifetime(time.Now().Add(-1*time.Hour), time.Now().Add(84*24*time.Hour)),
			extensions: leafExts,
		},
		extensions: kpExts,
	}

	if err := keyPkg.leafNode.sign(cs, nil, 0, sigPriv); err != nil {
		return nil, fmt.Errorf("failed to sign leaf node: %v", err)
	}
	if err := keyPkg.sign(sigPriv); err != nil {
		return nil, fmt.Errorf("failed to sign key package: %v", err)
	}

	return &KeyPairPackage{
		Public: keyPkg,
		Private: PrivateKeyPackage{
			InitKey:       initPriv,
			EncryptionKey: encPriv,
			SignatureKey:  sigPriv,
		},
	}, nil
}
