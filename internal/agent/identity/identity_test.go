package identity

import (
	"path/filepath"
	"testing"
)

func TestIdentityGeneration(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "key.pem")

	// 1. Generate key
	key, err := GenerateOrLoadKey(keyPath)
	if err != nil {
		t.Fatalf("unexpected error generating key: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil private key")
	}

	// 2. Re-load key from disk
	key2, err := GenerateOrLoadKey(keyPath)
	if err != nil {
		t.Fatalf("unexpected error reloading key: %v", err)
	}
	if key.X.Cmp(key2.X) != 0 || key.Y.Cmp(key2.Y) != 0 {
		t.Fatal("reloaded key does not match generated key")
	}

	// 3. Create CSR
	csrPEM, err := CreateCSR(key, "agent-test-123")
	if err != nil {
		t.Fatalf("failed to create csr: %v", err)
	}
	if len(csrPEM) == 0 {
		t.Fatal("expected non-empty CSR PEM")
	}

	// 4. Compute Host Fingerprint
	fp := ComputeHostFingerprint()
	if len(fp) != 64 { // SHA-256 hex string is 64 characters
		t.Fatalf("expected 64 char hex fingerprint, got %d (%s)", len(fp), fp)
	}
}
