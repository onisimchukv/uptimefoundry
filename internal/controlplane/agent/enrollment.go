// SPDX-License-Identifier: MIT

package agent

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EnrollmentToken defines a scoped, single-use or rate-limited bootstrap credential.
type EnrollmentToken struct {
	Token         string
	EnvironmentID string
	MaxUses       int
	UsedCount     int
	ExpiresAt     time.Time
}

// CA manages agent certificate issuance and enrollment verification.
type CA struct {
	cert        *x509.Certificate
	key         *ecdsa.PrivateKey
	certPEM     []byte
	tokens      map[string]*EnrollmentToken
	enrolled    map[string]string // agent_id -> host_fingerprint (clone detection)
	activeCerts map[string]string // serial -> agent_id
	mu          sync.Mutex
}

// NewCA creates or initializes a control-plane Certificate Authority.
func NewCA() (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ca key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "UptimeFoundry Control Plane Root CA",
			Organization: []string{"UptimeFoundry"},
		},
		NotBefore:             time.Now().Add(-10 * time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create ca cert: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	})

	return &CA{
		cert:        cert,
		key:         key,
		certPEM:     certPEM,
		tokens:      make(map[string]*EnrollmentToken),
		enrolled:    make(map[string]string),
		activeCerts: make(map[string]string),
	}, nil
}

// CreateBootstrapToken generates a scoped token for host agent enrollment.
func (ca *CA) CreateBootstrapToken(envID string, maxUses int, duration time.Duration) string {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	raw := uuid.NewString()
	token := hex.EncodeToString(sha256.New().Sum([]byte(raw)))[:32]

	ca.tokens[token] = &EnrollmentToken{
		Token:         token,
		EnvironmentID: envID,
		MaxUses:       maxUses,
		UsedCount:     0,
		ExpiresAt:     time.Now().Add(duration),
	}

	return token
}

// Enroll validates an enrollment token and signs a short-lived client certificate for the agent.
func (ca *CA) Enroll(token, hostFingerprint string, csrPEM []byte) (string, []byte, time.Time, error) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	// 1. Validate Token
	tok, ok := ca.tokens[token]
	if !ok {
		return "", nil, time.Time{}, errors.New("invalid enrollment token")
	}
	if time.Now().After(tok.ExpiresAt) {
		return "", nil, time.Time{}, errors.New("enrollment token expired")
	}
	if tok.MaxUses > 0 && tok.UsedCount >= tok.MaxUses {
		return "", nil, time.Time{}, errors.New("enrollment token usage exceeded")
	}

	tok.UsedCount++

	// 2. Generate Agent ID (UUIDv7)
	agentID, err := uuid.NewV7()
	if err != nil {
		agentID = uuid.New()
	}
	agentIDStr := agentID.String()

	// 3. Register Fingerprint (Clone Detection)
	ca.enrolled[agentIDStr] = hostFingerprint

	// 4. Sign CSR
	certPEM, expiresAt, err := ca.signCSR(csrPEM, agentIDStr, 24*time.Hour)
	if err != nil {
		return "", nil, time.Time{}, fmt.Errorf("sign agent csr: %w", err)
	}

	return agentIDStr, certPEM, expiresAt, nil
}

// RenewCertificate signs a renewed certificate over an authenticated mTLS session.
func (ca *CA) RenewCertificate(agentID, hostFingerprint string, csrPEM []byte) ([]byte, time.Time, error) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	// Verify clone detection
	if existingFP, ok := ca.enrolled[agentID]; ok {
		if existingFP != hostFingerprint {
			return nil, time.Time{}, fmt.Errorf("clone detected: fingerprint mismatch for agent %s", agentID)
		}
	}

	return ca.signCSR(csrPEM, agentID, 24*time.Hour)
}

func (ca *CA) signCSR(csrPEM []byte, agentID string, validity time.Duration) ([]byte, time.Time, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, time.Time{}, errors.New("invalid pem csr")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("parse csr: %w", err)
	}

	if err := csr.CheckSignature(); err != nil {
		return nil, time.Time{}, fmt.Errorf("invalid csr signature: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, time.Time{}, err
	}

	now := time.Now()
	expiresAt := now.Add(validity)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   agentID, // Section 4.2: agent_id only in subject
			Organization: []string{"UptimeFoundry Agent"},
		},
		NotBefore:   now.Add(-5 * time.Minute),
		NotAfter:    expiresAt,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, ca.cert, csr.PublicKey, ca.key)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("create client cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	ca.activeCerts[serialNumber.String()] = agentID

	return certPEM, expiresAt, nil
}

// CACertificatePEM returns the Root CA certificate for client validation.
func (ca *CA) CACertificatePEM() []byte {
	return ca.certPEM
}
