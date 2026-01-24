package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// CertManager handles certificate acquisition and renewal.
type CertManager struct {
	cfg      *Config
	client   *lego.Client
	user     *User
	certPath string
	keyPath  string
	metaPath string
}

// User implements the lego registration.User interface.
type User struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *User) GetEmail() string {
	return u.Email
}

func (u *User) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *User) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// CertMetadata stores certificate metadata.
type CertMetadata struct {
	Domain    string    `json:"domain"`
	Domains   []string  `json:"domains"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	Issuer    string    `json:"issuer"`
	RenewedAt time.Time `json:"renewed_at"`
}

// NewCertManager creates a new certificate manager.
func NewCertManager(cfg *Config) (*CertManager, error) {
	// Create output directory
	domainDir := filepath.Join(cfg.OutputDir, cfg.BaseDomain())
	if err := os.MkdirAll(domainDir, 0755); chk.E(err) {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate or load account private key
	privateKey, err := loadOrCreateAccountKey(cfg)
	if chk.E(err) {
		return nil, fmt.Errorf("failed to load/create account key: %w", err)
	}

	user := &User{
		Email: cfg.Email,
		key:   privateKey,
	}

	// Create lego config
	legoCfg := lego.NewConfig(user)
	legoCfg.CADirURL = cfg.ACMEServerURL()
	legoCfg.Certificate.KeyType = certcrypto.EC256

	// Create lego client
	client, err := lego.NewClient(legoCfg)
	if chk.E(err) {
		return nil, fmt.Errorf("failed to create ACME client: %w", err)
	}

	// Set up DNS provider
	dnsProvider, err := NewDNSProvider(cfg.DNSProvider)
	if chk.E(err) {
		return nil, err
	}

	if err := client.Challenge.SetDNS01Provider(dnsProvider); chk.E(err) {
		return nil, fmt.Errorf("failed to set DNS provider: %w", err)
	}

	// Register account if needed
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		// Try to recover existing registration
		reg, err = client.Registration.ResolveAccountByKey()
		if chk.E(err) {
			return nil, fmt.Errorf("failed to register account: %w", err)
		}
	}
	user.Registration = reg

	return &CertManager{
		cfg:      cfg,
		client:   client,
		user:     user,
		certPath: filepath.Join(domainDir, "cert.pem"),
		keyPath:  filepath.Join(domainDir, "key.pem"),
		metaPath: filepath.Join(domainDir, "metadata.json"),
	}, nil
}

// EnsureCertificate obtains a certificate if none exists or if it needs renewal.
func (m *CertManager) EnsureCertificate() error {
	// Check if certificate exists and is valid
	if m.certificateExists() {
		needsRenewal, err := m.needsRenewal()
		if chk.E(err) {
			log.W.F("failed to check renewal status, will obtain new cert: %v", err)
		} else if !needsRenewal {
			log.I.F("certificate is valid, no renewal needed")
			return nil
		}
		log.I.F("certificate needs renewal")
	}

	return m.obtainCertificate()
}

// CheckRenewal checks if the certificate needs renewal and renews if needed.
func (m *CertManager) CheckRenewal() error {
	if !m.certificateExists() {
		return m.obtainCertificate()
	}

	needsRenewal, err := m.needsRenewal()
	if chk.E(err) {
		return err
	}

	if needsRenewal {
		log.I.F("certificate expiring soon, renewing...")
		return m.obtainCertificate()
	}

	log.D.F("certificate still valid, no renewal needed")
	return nil
}

func (m *CertManager) certificateExists() bool {
	_, err := os.Stat(m.certPath)
	return err == nil
}

func (m *CertManager) needsRenewal() (bool, error) {
	certPEM, err := os.ReadFile(m.certPath)
	if chk.E(err) {
		return true, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return true, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if chk.E(err) {
		return true, err
	}

	// Check if certificate expires within RenewDays
	renewTime := time.Now().Add(time.Duration(m.cfg.RenewDays) * 24 * time.Hour)
	return cert.NotAfter.Before(renewTime), nil
}

func (m *CertManager) obtainCertificate() error {
	log.I.F("obtaining certificate for %s", m.cfg.Domain)

	request := certificate.ObtainRequest{
		Domains: []string{m.cfg.Domain, m.cfg.BaseDomain()},
		Bundle:  true,
	}

	certificates, err := m.client.Certificate.Obtain(request)
	if chk.E(err) {
		return fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// Write certificate chain
	if err := os.WriteFile(m.certPath, certificates.Certificate, 0644); chk.E(err) {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write private key with restricted permissions
	if err := os.WriteFile(m.keyPath, certificates.PrivateKey, 0600); chk.E(err) {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Write issuer certificate if available
	if len(certificates.IssuerCertificate) > 0 {
		issuerPath := filepath.Join(filepath.Dir(m.certPath), "issuer.pem")
		if err := os.WriteFile(issuerPath, certificates.IssuerCertificate, 0644); chk.E(err) {
			log.W.F("failed to write issuer certificate: %v", err)
		}
	}

	// Write metadata
	if err := m.writeMetadata(certificates.Certificate); chk.E(err) {
		log.W.F("failed to write metadata: %v", err)
	}

	log.I.F("certificate obtained successfully for %s", m.cfg.Domain)
	log.I.F("  cert: %s", m.certPath)
	log.I.F("  key:  %s", m.keyPath)

	return nil
}

func (m *CertManager) writeMetadata(certPEM []byte) error {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode certificate for metadata")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if chk.E(err) {
		return err
	}

	meta := CertMetadata{
		Domain:    m.cfg.Domain,
		Domains:   cert.DNSNames,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		Issuer:    cert.Issuer.CommonName,
		RenewedAt: time.Now(),
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if chk.E(err) {
		return err
	}

	return os.WriteFile(m.metaPath, data, 0644)
}

func loadOrCreateAccountKey(cfg *Config) (crypto.PrivateKey, error) {
	keyPath := cfg.AccountKeyPath
	if keyPath == "" {
		keyPath = filepath.Join(cfg.OutputDir, "account.key")
	}

	// Try to load existing key
	if data, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(data)
		if block != nil {
			key, err := x509.ParseECPrivateKey(block.Bytes)
			if err == nil {
				log.D.F("loaded existing account key from %s", keyPath)
				return key, nil
			}
		}
	}

	// Generate new key
	log.I.F("generating new account key")
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if chk.E(err) {
		return nil, err
	}

	// Save key
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if chk.E(err) {
		return nil, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); chk.E(err) {
		return nil, err
	}

	if err := os.WriteFile(keyPath, keyPEM, 0600); chk.E(err) {
		return nil, err
	}

	log.I.F("saved new account key to %s", keyPath)
	return key, nil
}
