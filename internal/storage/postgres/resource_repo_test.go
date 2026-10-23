package postgres

import (
	"testing"
)

func TestSanitizeLtreePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "clean alphanumeric path",
			input:    "prod.k8s_cluster.default.billing",
			expected: "prod.k8s_cluster.default.billing",
		},
		{
			name:     "path with hyphens and special chars",
			input:    "prod-east.k8s-cluster-1.billing-service:v2.pod/78fa",
			expected: "prod_east.k8s_cluster_1.billing_service_v2.pod_78fa",
		},
		{
			name:     "path with leading and trailing delimiters",
			input:    ".prod..cluster-1.",
			expected: "prod.cluster_1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeLtreePath(tc.input)
			if result != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}
