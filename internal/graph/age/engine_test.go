package age

import (
	"testing"
)

func TestSanitizeCypherString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"O'Reilly", "O\\'Reilly"},
		{"with\\backslash", "with\\\\backslash"},
		{"mixed'\\quotes", "mixed\\'\\\\quotes"},
	}

	for _, tc := range tests {
		result := sanitizeCypherString(tc.input)
		if result != tc.expected {
			t.Fatalf("expected %q, got %q", tc.expected, result)
		}
	}
}

func TestSanitizeCypherIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Host", "Host"},
		{"K8sPod", "K8sPod"},
		{"RUNS_ON", "RUNS_ON"},
		{"CONNECTS-TO", "CONNECTS_TO"},
		{"custom@kind#1", "custom_kind_1"},
		{"", "Unknown"},
	}

	for _, tc := range tests {
		result := sanitizeCypherIdentifier(tc.input)
		if result != tc.expected {
			t.Fatalf("expected %q, got %q", tc.expected, result)
		}
	}
}
