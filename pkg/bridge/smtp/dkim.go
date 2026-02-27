package smtp

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/emersion/go-msgauth/dkim"
	"next.orly.dev/pkg/lol/log"
)

// DKIMSigner signs outbound emails with a DKIM signature.
type DKIMSigner struct {
	domain   string
	selector string
	key      crypto.Signer
}

// NewDKIMSigner creates a DKIM signer from a PEM-encoded RSA private key file.
func NewDKIMSigner(domain, selector, keyPath string) (*DKIMSigner, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read DKIM key: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", keyPath)
	}

	var key crypto.Signer
	switch block.Type {
	case "RSA PRIVATE KEY":
		rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse RSA key: %w", err)
		}
		key = rsaKey
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 key: %w", err)
		}
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not RSA")
		}
		key = rsaKey
	default:
		return nil, fmt.Errorf("unsupported PEM type: %s", block.Type)
	}

	return &DKIMSigner{
		domain:   domain,
		selector: selector,
		key:      key,
	}, nil
}

// NewDKIMSignerFromPEM creates a DKIM signer from PEM-encoded key bytes.
func NewDKIMSignerFromPEM(domain, selector string, pemData []byte) (*DKIMSigner, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8
		parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not RSA")
		}
		return &DKIMSigner{domain: domain, selector: selector, key: key}, nil
	}
	return &DKIMSigner{domain: domain, selector: selector, key: rsaKey}, nil
}

// NewDKIMSignerFromKey creates a DKIM signer from an in-memory key.
// Useful for testing.
func NewDKIMSignerFromKey(domain, selector string, key crypto.Signer) *DKIMSigner {
	return &DKIMSigner{
		domain:   domain,
		selector: selector,
		key:      key,
	}
}

// Sign adds a DKIM-Signature header to the message.
// Returns the signed message bytes.
func (ds *DKIMSigner) Sign(msg []byte) ([]byte, error) {
	opts := &dkim.SignOptions{
		Domain:                 ds.domain,
		Selector:               ds.selector,
		Signer:                 ds.key,
		Hash:                   crypto.SHA256,
		HeaderCanonicalization: dkim.CanonicalizationRelaxed,
		BodyCanonicalization:   dkim.CanonicalizationRelaxed,
		HeaderKeys: []string{
			"from", "to", "cc", "subject", "date", "message-id",
			"mime-version", "content-type",
		},
	}

	var signed bytes.Buffer
	if err := dkim.Sign(&signed, bytes.NewReader(msg), opts); err != nil {
		return nil, fmt.Errorf("DKIM sign: %w", err)
	}

	log.D.F("DKIM signed message for domain=%s selector=%s", ds.domain, ds.selector)
	return signed.Bytes(), nil
}

// GenerateDKIMKeyPair generates a new RSA-2048 key pair for DKIM signing.
// Returns the private key PEM and the public key DNS TXT record value.
func GenerateDKIMKeyPair() (privateKeyPEM []byte, dnsRecord string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", fmt.Errorf("generate RSA key: %w", err)
	}

	// Encode private key
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	// Encode public key for DNS TXT record
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, "", fmt.Errorf("marshal public key: %w", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})

	// Format as DNS TXT record value: strip PEM headers, join lines
	lines := bytes.Split(pubPEM, []byte("\n"))
	var pubB64 []byte
	for _, line := range lines {
		if len(line) == 0 || line[0] == '-' {
			continue
		}
		pubB64 = append(pubB64, line...)
	}

	dns := fmt.Sprintf("v=DKIM1; k=rsa; p=%s", string(pubB64))

	return privPEM, dns, nil
}
