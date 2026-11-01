// SPDX-License-Identifier: MIT

package actuator

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// CanonicalPayload creates the deterministic byte string for cryptographic signing.
func CanonicalPayload(task SignedActionTask) []byte {
	return []byte(fmt.Sprintf("%s|%s|%s|%s|%d|%d|%t",
		task.TaskID,
		task.ActionType,
		task.Target,
		task.ApprovedBy,
		task.IssuedAt.Unix(),
		task.ExpiresAt.Unix(),
		task.DryRun,
	))
}

// SignTaskHMAC signs a remediation task with a shared secret key (HMAC-SHA256).
func SignTaskHMAC(task *SignedActionTask, key []byte) {
	mac := hmac.New(sha256.New, key)
	mac.Write(CanonicalPayload(*task))
	task.Signature = hex.EncodeToString(mac.Sum(nil))
}

// SignTaskEd25519 signs a remediation task using an Ed25519 private key.
func SignTaskEd25519(task *SignedActionTask, privKey ed25519.PrivateKey) {
	payload := CanonicalPayload(*task)
	sig := ed25519.Sign(privKey, payload)
	task.Signature = hex.EncodeToString(sig)
}

// TaskVerifier validates timestamps and cryptographic signatures on incoming action tasks.
type TaskVerifier interface {
	Verify(task SignedActionTask) error
}

// HMACVerifier validates tasks using an HMAC-SHA256 shared secret.
type HMACVerifier struct {
	secretKey []byte
}

// NewHMACVerifier creates a verifier with a pre-shared key.
func NewHMACVerifier(secretKey []byte) *HMACVerifier {
	return &HMACVerifier{secretKey: secretKey}
}

// Verify checks the expiration window and HMAC signature of a task.
func (v *HMACVerifier) Verify(task SignedActionTask) error {
	now := time.Now()

	// 1. Check expiration
	if now.After(task.ExpiresAt) {
		return ErrTaskExpired
	}

	// 2. Check future timestamp tolerance (up to 30s clock skew)
	if task.IssuedAt.After(now.Add(30 * time.Second)) {
		return ErrTaskNotYetValid
	}

	// 3. Verify signature
	expectedMac := hmac.New(sha256.New, v.secretKey)
	expectedMac.Write(CanonicalPayload(task))
	expectedSig := hex.EncodeToString(expectedMac.Sum(nil))

	if !hmac.Equal([]byte(task.Signature), []byte(expectedSig)) {
		return ErrSignatureInvalid
	}

	return nil
}

// Ed25519Verifier validates tasks using an Ed25519 public key.
type Ed25519Verifier struct {
	publicKey ed25519.PublicKey
}

// NewEd25519Verifier creates an asymmetric signature verifier.
func NewEd25519Verifier(pubKey ed25519.PublicKey) *Ed25519Verifier {
	return &Ed25519Verifier{publicKey: pubKey}
}

// Verify checks the expiration window and Ed25519 signature of a task.
func (v *Ed25519Verifier) Verify(task SignedActionTask) error {
	now := time.Now()

	if now.After(task.ExpiresAt) {
		return ErrTaskExpired
	}

	if task.IssuedAt.After(now.Add(30 * time.Second)) {
		return ErrTaskNotYetValid
	}

	sigBytes, err := hex.DecodeString(task.Signature)
	if err != nil {
		return ErrSignatureInvalid
	}

	payload := CanonicalPayload(task)
	if !ed25519.Verify(v.publicKey, payload, sigBytes) {
		return ErrSignatureInvalid
	}

	return nil
}
