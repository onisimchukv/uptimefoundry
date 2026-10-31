package actuator

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

// ---------- Signature Verification ----------

func TestHMACVerifier_ValidSignature(t *testing.T) {
	key := []byte("super-secret-test-key-32-bytes!!")
	task := SignedActionTask{
		TaskID:     "task-001",
		ActionType: "restart_service",
		Target:     "nginx.service",
		ApprovedBy: "operator@example.com",
		IssuedAt:   time.Now().Add(-10 * time.Second),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
		DryRun:     false,
	}
	SignTaskHMAC(&task, key)

	verifier := NewHMACVerifier(key)
	if err := verifier.Verify(task); err != nil {
		t.Fatalf("expected valid HMAC signature to pass, got: %v", err)
	}
}

func TestHMACVerifier_TamperedPayload(t *testing.T) {
	key := []byte("super-secret-test-key-32-bytes!!")
	task := SignedActionTask{
		TaskID:     "task-002",
		ActionType: "restart_service",
		Target:     "nginx.service",
		ApprovedBy: "operator@example.com",
		IssuedAt:   time.Now().Add(-10 * time.Second),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	}
	SignTaskHMAC(&task, key)

	// Tamper with the target after signing
	task.Target = "sshd.service"

	verifier := NewHMACVerifier(key)
	err := verifier.Verify(task)
	if err == nil {
		t.Fatal("expected tampered payload to be rejected")
	}
	if err != ErrSignatureInvalid {
		t.Fatalf("expected ErrSignatureInvalid, got: %v", err)
	}
}

func TestHMACVerifier_WrongKey(t *testing.T) {
	key1 := []byte("legitimate-control-plane-key-!!!")
	key2 := []byte("attacker-forged-key-xxxxxxxxxx!!")

	task := SignedActionTask{
		TaskID:     "task-003",
		ActionType: "restart_service",
		Target:     "nginx.service",
		ApprovedBy: "operator@example.com",
		IssuedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	}
	SignTaskHMAC(&task, key2) // signed with wrong key

	verifier := NewHMACVerifier(key1)
	if err := verifier.Verify(task); err != ErrSignatureInvalid {
		t.Fatalf("expected ErrSignatureInvalid for wrong key, got: %v", err)
	}
}

func TestHMACVerifier_ExpiredTask(t *testing.T) {
	key := []byte("super-secret-test-key-32-bytes!!")
	task := SignedActionTask{
		TaskID:     "task-004",
		ActionType: "restart_service",
		Target:     "nginx.service",
		ApprovedBy: "operator@example.com",
		IssuedAt:   time.Now().Add(-10 * time.Minute),
		ExpiresAt:  time.Now().Add(-1 * time.Minute), // already expired
	}
	SignTaskHMAC(&task, key)

	verifier := NewHMACVerifier(key)
	if err := verifier.Verify(task); err != ErrTaskExpired {
		t.Fatalf("expected ErrTaskExpired, got: %v", err)
	}
}

func TestHMACVerifier_FutureTimestamp(t *testing.T) {
	key := []byte("super-secret-test-key-32-bytes!!")
	task := SignedActionTask{
		TaskID:     "task-005",
		ActionType: "restart_service",
		Target:     "nginx.service",
		ApprovedBy: "operator@example.com",
		IssuedAt:   time.Now().Add(5 * time.Minute), // far future
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}
	SignTaskHMAC(&task, key)

	verifier := NewHMACVerifier(key)
	if err := verifier.Verify(task); err != ErrTaskNotYetValid {
		t.Fatalf("expected ErrTaskNotYetValid, got: %v", err)
	}
}

// ---------- Ed25519 Verification ----------

func TestEd25519Verifier_ValidSignature(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}

	task := SignedActionTask{
		TaskID:     "task-ed25519-001",
		ActionType: "restart_container",
		Target:     "payment-api-7f8b9c",
		ApprovedBy: "sre-lead@example.com",
		IssuedAt:   time.Now().Add(-5 * time.Second),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
		DryRun:     true,
	}
	SignTaskEd25519(&task, privKey)

	verifier := NewEd25519Verifier(pubKey)
	if err := verifier.Verify(task); err != nil {
		t.Fatalf("expected valid Ed25519 signature to pass, got: %v", err)
	}
}

func TestEd25519Verifier_TamperedPayload(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)

	task := SignedActionTask{
		TaskID:     "task-ed25519-002",
		ActionType: "restart_container",
		Target:     "payment-api-7f8b9c",
		ApprovedBy: "sre-lead@example.com",
		IssuedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	}
	SignTaskEd25519(&task, privKey)

	// Tamper with the action type after signing
	task.ActionType = "delete_host"

	verifier := NewEd25519Verifier(pubKey)
	if err := verifier.Verify(task); err != ErrSignatureInvalid {
		t.Fatalf("expected ErrSignatureInvalid for tampered Ed25519, got: %v", err)
	}
}

// ---------- Audit Logger ----------

func TestAuditLoggerChainIntegrity(t *testing.T) {
	var buf bytes.Buffer
	logger := NewAuditLogger(&buf)

	task1 := SignedActionTask{TaskID: "t-1", ActionType: "restart_service", Target: "a.service", ApprovedBy: "op"}
	task2 := SignedActionTask{TaskID: "t-2", ActionType: "reload_service", Target: "b.service", ApprovedBy: "op"}
	task3 := SignedActionTask{TaskID: "t-3", ActionType: "flush_cache", Target: "c.service", ApprovedBy: "op"}

	rec1, err := logger.Log(task1, "executed", "")
	if err != nil {
		t.Fatalf("log task1: %v", err)
	}
	rec2, err := logger.Log(task2, "dry_run", "")
	if err != nil {
		t.Fatalf("log task2: %v", err)
	}
	rec3, err := logger.Log(task3, "rejected_signature", "invalid sig")
	if err != nil {
		t.Fatalf("log task3: %v", err)
	}

	// Verify chain linkage
	if rec1.Sequence != 1 || rec2.Sequence != 2 || rec3.Sequence != 3 {
		t.Fatalf("unexpected sequence numbers: %d, %d, %d", rec1.Sequence, rec2.Sequence, rec3.Sequence)
	}

	if rec2.PrevRecordHash != rec1.RecordHash {
		t.Fatal("record 2 does not chain to record 1")
	}
	if rec3.PrevRecordHash != rec2.RecordHash {
		t.Fatal("record 3 does not chain to record 2")
	}

	// Verify hashes are self-consistent
	for _, rec := range []AuditRecord{rec1, rec2, rec3} {
		recomputed := rec.ComputeHash()
		if recomputed != rec.RecordHash {
			t.Fatalf("hash mismatch on record %d: expected %s, got %s", rec.Sequence, rec.RecordHash, recomputed)
		}
	}

	// Verify JSONL output is parseable
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines in audit log, got %d", len(lines))
	}
	for i, line := range lines {
		var parsed AuditRecord
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Fatalf("failed to parse audit record line %d: %v", i+1, err)
		}
	}
}

func TestAuditLoggerGenesisHash(t *testing.T) {
	var buf bytes.Buffer
	logger := NewAuditLogger(&buf)

	task := SignedActionTask{TaskID: "t-genesis", ActionType: "restart_service", Target: "x", ApprovedBy: "op"}
	rec, _ := logger.Log(task, "executed", "")

	if !strings.HasPrefix(rec.PrevRecordHash, "GENESIS_ROOT") {
		t.Fatalf("first record should chain to GENESIS_ROOT, got %s", rec.PrevRecordHash)
	}
}

// ---------- Executor ----------

func TestSystemExecutor_DryRun(t *testing.T) {
	exec := NewSystemExecutor()

	task := SignedActionTask{
		TaskID:     "exec-dr-01",
		ActionType: "restart_service",
		Target:     "nginx.service",
		DryRun:     true,
	}

	output, err := exec.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("dry-run should not fail: %v", err)
	}
	if !strings.Contains(output, "DRY_RUN") {
		t.Fatalf("expected DRY_RUN marker in output, got: %s", output)
	}
}

func TestSystemExecutor_UnsupportedAction(t *testing.T) {
	exec := NewSystemExecutor()

	task := SignedActionTask{
		TaskID:     "exec-unsup-01",
		ActionType: "delete_entire_cluster",
		Target:     "prod-cluster",
	}

	_, err := exec.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("expected unsupported action to be rejected")
	}
	if !strings.Contains(err.Error(), "unsupported action type") {
		t.Fatalf("expected unsupported action type error, got: %v", err)
	}
}

func TestSystemExecutor_InvalidTarget(t *testing.T) {
	exec := NewSystemExecutor()

	task := SignedActionTask{
		TaskID:     "exec-inject-01",
		ActionType: "restart_service",
		Target:     "nginx; rm -rf /",
	}

	_, err := exec.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("expected command injection target to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid characters") {
		t.Fatalf("expected invalid characters error, got: %v", err)
	}
}

func TestSystemExecutor_CustomCommandRunner(t *testing.T) {
	var capturedCmd string
	var capturedArgs []string

	exec := &SystemExecutor{
		CommandRunner: func(ctx context.Context, name string, args ...string) (string, error) {
			capturedCmd = name
			capturedArgs = args
			return "unit restarted successfully", nil
		},
	}

	task := SignedActionTask{
		TaskID:     "exec-custom-01",
		ActionType: "restart_service",
		Target:     "postgresql.service",
	}

	output, err := exec.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("custom runner should not fail: %v", err)
	}
	if capturedCmd != "systemctl" {
		t.Fatalf("expected systemctl, got: %s", capturedCmd)
	}
	if len(capturedArgs) != 2 || capturedArgs[0] != "restart" || capturedArgs[1] != "postgresql.service" {
		t.Fatalf("unexpected args: %v", capturedArgs)
	}
	if output != "unit restarted successfully" {
		t.Fatalf("unexpected output: %s", output)
	}
}

// ---------- Full Actuator Pipeline ----------

func TestActuator_FullPipeline_ValidTask(t *testing.T) {
	key := []byte("integration-test-key-32-bytesss!")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var auditBuf bytes.Buffer

	task := SignedActionTask{
		TaskID:     "pipe-001",
		ActionType: "restart_service",
		Target:     "nginx.service",
		ApprovedBy: "sre@example.com",
		IssuedAt:   time.Now().Add(-5 * time.Second),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
		DryRun:     false,
	}
	SignTaskHMAC(&task, key)

	act := NewActuator(
		ActuatorConfig{
			AllowedActions: map[string]bool{
				"restart_service": true,
				"reload_service":  true,
			},
		},
		NewHMACVerifier(key),
		NewSystemExecutor(),
		NewAuditLogger(&auditBuf),
		logger,
	)

	result, err := act.Handle(context.Background(), task)
	if err != nil {
		t.Fatalf("expected successful execution, got: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected result.Success=true, got false: %s", result.ErrorMessage)
	}
	if result.DurationMs < 0 {
		t.Fatal("expected non-negative duration")
	}

	// Verify audit log was written
	if auditBuf.Len() == 0 {
		t.Fatal("expected audit log to contain a record")
	}
	var rec AuditRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(auditBuf.String())), &rec); err != nil {
		t.Fatalf("failed to parse audit record: %v", err)
	}
	if rec.Outcome != "executed" {
		t.Fatalf("expected outcome 'executed', got '%s'", rec.Outcome)
	}
}

func TestActuator_FullPipeline_DryRun(t *testing.T) {
	key := []byte("integration-test-key-32-bytesss!")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var auditBuf bytes.Buffer

	task := SignedActionTask{
		TaskID:     "pipe-dr-001",
		ActionType: "restart_service",
		Target:     "nginx.service",
		ApprovedBy: "sre@example.com",
		IssuedAt:   time.Now().Add(-5 * time.Second),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
		DryRun:     true,
	}
	SignTaskHMAC(&task, key)

	act := NewActuator(
		ActuatorConfig{
			AllowedActions: map[string]bool{"restart_service": true},
		},
		NewHMACVerifier(key),
		NewSystemExecutor(),
		NewAuditLogger(&auditBuf),
		logger,
	)

	result, err := act.Handle(context.Background(), task)
	if err != nil {
		t.Fatalf("dry-run should succeed, got: %v", err)
	}
	if !result.Success || !result.DryRun {
		t.Fatal("expected dry-run success")
	}
	if !strings.Contains(result.Output, "DRY_RUN") {
		t.Fatalf("expected DRY_RUN in output, got: %s", result.Output)
	}

	var rec AuditRecord
	json.Unmarshal([]byte(strings.TrimSpace(auditBuf.String())), &rec)
	if rec.Outcome != "dry_run" {
		t.Fatalf("expected audit outcome 'dry_run', got '%s'", rec.Outcome)
	}
}

func TestActuator_FullPipeline_PolicyDenial(t *testing.T) {
	key := []byte("integration-test-key-32-bytesss!")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var auditBuf bytes.Buffer

	task := SignedActionTask{
		TaskID:     "pipe-deny-001",
		ActionType: "delete_host",
		Target:     "prod-server-01",
		ApprovedBy: "rogue@example.com",
		IssuedAt:   time.Now().Add(-5 * time.Second),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	}
	SignTaskHMAC(&task, key)

	act := NewActuator(
		ActuatorConfig{
			AllowedActions: map[string]bool{
				"restart_service": true,
				// "delete_host" is NOT in the allowlist
			},
		},
		NewHMACVerifier(key),
		NewSystemExecutor(),
		NewAuditLogger(&auditBuf),
		logger,
	)

	result, err := act.Handle(context.Background(), task)
	if err == nil {
		t.Fatal("expected policy denial error")
	}
	if result.Success {
		t.Fatal("expected result.Success=false")
	}

	var rec AuditRecord
	json.Unmarshal([]byte(strings.TrimSpace(auditBuf.String())), &rec)
	if rec.Outcome != "denied_by_policy" {
		t.Fatalf("expected audit outcome 'denied_by_policy', got '%s'", rec.Outcome)
	}
}

func TestActuator_FullPipeline_BadSignature(t *testing.T) {
	key := []byte("integration-test-key-32-bytesss!")
	badKey := []byte("attacker-forged-key-xxxxxxxxxxx!")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var auditBuf bytes.Buffer

	task := SignedActionTask{
		TaskID:     "pipe-badsig-001",
		ActionType: "restart_service",
		Target:     "nginx.service",
		ApprovedBy: "attacker@evil.io",
		IssuedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	}
	SignTaskHMAC(&task, badKey) // signed with wrong key

	act := NewActuator(
		ActuatorConfig{
			AllowedActions: map[string]bool{"restart_service": true},
		},
		NewHMACVerifier(key), // verifies with correct key
		NewSystemExecutor(),
		NewAuditLogger(&auditBuf),
		logger,
	)

	result, err := act.Handle(context.Background(), task)
	if err == nil {
		t.Fatal("expected signature rejection")
	}
	if result.Success {
		t.Fatal("expected result.Success=false")
	}

	var rec AuditRecord
	json.Unmarshal([]byte(strings.TrimSpace(auditBuf.String())), &rec)
	if rec.Outcome != "rejected_signature" {
		t.Fatalf("expected audit outcome 'rejected_signature', got '%s'", rec.Outcome)
	}
}

func TestActuator_DryRunDefault_OverridesLiveTask(t *testing.T) {
	key := []byte("integration-test-key-32-bytesss!")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var auditBuf bytes.Buffer

	task := SignedActionTask{
		TaskID:     "pipe-drd-001",
		ActionType: "restart_service",
		Target:     "nginx.service",
		ApprovedBy: "sre@example.com",
		IssuedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
		DryRun:     false, // task says live execution
	}
	SignTaskHMAC(&task, key)

	act := NewActuator(
		ActuatorConfig{
			AllowedActions: map[string]bool{"restart_service": true},
			DryRunDefault:  true, // but config forces dry-run
		},
		NewHMACVerifier(key),
		NewSystemExecutor(),
		NewAuditLogger(&auditBuf),
		logger,
	)

	result, err := act.Handle(context.Background(), task)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if !result.DryRun {
		t.Fatal("expected DryRunDefault to override task's DryRun=false")
	}
}
