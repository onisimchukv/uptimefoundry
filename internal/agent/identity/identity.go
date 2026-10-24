package identity

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
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Identity holds the agent's cryptographic keys and certified identity.
type Identity struct {
	AgentID         string
	HostFingerprint string
	PrivateKey      *ecdsa.PrivateKey
	Certificate     *x509.Certificate
	RawCertPEM      []byte
	ExpiresAt       time.Time
	KeyPath         string
	CertPath        string
}

// GenerateOrLoadKey loads the private key from disk or generates a new ECDSA P-256 keypair.
func GenerateOrLoadKey(keyPath string) (*ecdsa.PrivateKey, error) {
	if keyPath == "" {
		keyPath = "identity/key.pem"
	}

	if data, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(data)
		if block == nil || block.Type != "EC PRIVATE KEY" {
			return nil, errors.New("invalid pem private key format")
		}
		return x509.ParseECPrivateKey(block.Bytes)
	}

	// Generate new key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ec key: %w", err)
	}

	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal ec key: %w", err)
	}

	_ = os.MkdirAll(filepath.Dir(keyPath), 0700)
	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	})

	if err := os.WriteFile(keyPath, pemData, 0600); err != nil {
		return nil, fmt.Errorf("write key file %s: %w", keyPath, err)
	}

	return key, nil
}

// CreateCSR generates a PKCS#10 Certificate Signing Request for this agent.
func CreateCSR(key *ecdsa.PrivateKey, commonName string) ([]byte, error) {
	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"ForgeOps Agent"},
		},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		return nil, fmt.Errorf("create csr: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	}), nil
}

// ComputeHostFingerprint calculates a stable hash of machine-id, DMI UUID, and sorted MAC addresses.
func ComputeHostFingerprint() string {
	h := sha256.New()

	// 1. Machine-ID
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		h.Write(data)
	} else {
		h.Write([]byte("no-machine-id"))
	}

	// 2. DMI UUID
	if data, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		h.Write(data)
	}

	// 3. Sorted MAC addresses
	if ifaces, err := net.Interfaces(); err == nil {
		var macs []string
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback == 0 && len(iface.HardwareAddr) > 0 {
				macs = append(macs, iface.HardwareAddr.String())
			}
		}
		sort.Strings(macs)
		h.Write([]byte(strings.Join(macs, ",")))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// LoadCertificate reads and validates an existing agent certificate from disk.
func LoadCertificate(certPath string) (*x509.Certificate, []byte, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, nil, errors.New("invalid pem certificate format")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse certificate: %w", err)
	}

	return cert, data, nil
}

// SaveCertificate writes PEM certificate bytes to disk.
func SaveCertificate(certPath string, pemData []byte) error {
	_ = os.MkdirAll(filepath.Dir(certPath), 0700)
	return os.WriteFile(certPath, pemData, 0600)
}
