package redaction

import (
	"strings"
	"testing"
)

func TestSQLNormalization(t *testing.T) {
	rawQuery := `SELECT id, name, credit_card FROM users WHERE id = 12345 AND email = 'alice@secret-domain.org' AND status IN (1, 2, 3)`

	summary := NormalizeSQL(rawQuery)

	if summary.Verb != "SELECT" {
		t.Fatalf("expected verb SELECT, got %s", summary.Verb)
	}

	// Secret Leakage Assertions (Criterion 7)
	secretTokens := []string{
		"12345",
		"alice@secret-domain.org",
		"1, 2, 3",
	}

	for _, secret := range secretTokens {
		if strings.Contains(summary.NormalizedSQL, secret) {
			t.Fatalf("CRITICAL SECRET LEAK: normalized sql contains sensitive token %q: %s", secret, summary.NormalizedSQL)
		}
	}

	expectedPattern := "SELECT id, name, credit_card FROM users WHERE id = ? AND email = '?' AND status IN (?)"
	if summary.NormalizedSQL != expectedPattern {
		t.Fatalf("expected normalized query %q, got %q", expectedPattern, summary.NormalizedSQL)
	}

	if len(summary.StatementHash) != 64 {
		t.Fatalf("expected 64-char sha256 statement hash, got %d (%s)", len(summary.StatementHash), summary.StatementHash)
	}
}

func TestSQLInsertNormalization(t *testing.T) {
	rawInsert := `INSERT INTO payments (account_id, amount, note) VALUES (9876, 54.20, 'confidential payment details')`
	summary := NormalizeSQL(rawInsert)

	if summary.Verb != "INSERT" {
		t.Fatalf("expected verb INSERT, got %s", summary.Verb)
	}

	if strings.Contains(summary.NormalizedSQL, "confidential payment details") {
		t.Fatal("secret leaked in INSERT normalization")
	}
	if strings.Contains(summary.NormalizedSQL, "9876") {
		t.Fatal("numeric ID leaked in INSERT normalization")
	}
}

func FuzzNormalizeSQL(f *testing.F) {
	// Seed fuzz corpus
	f.Add("SELECT * FROM table WHERE x = 123")
	f.Add("INSERT INTO table VALUES ('secret', 0xdeadbeef)")
	f.Add("DELETE FROM table WHERE id IN (1, 2, 3, 4)")
	f.Add("/* comment */ UPDATE users SET password = 'foo'")

	f.Fuzz(func(t *testing.T, input string) {
		summary := NormalizeSQL(input)
		if summary.Verb == "" {
			t.Fatal("verb should not be empty")
		}
	})
}
