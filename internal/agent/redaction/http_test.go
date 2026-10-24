package redaction

import (
	"strings"
	"testing"
)

func TestSanitizeHTTPAndSecretLeakage(t *testing.T) {
	// Raw URI containing sensitive token and integer IDs
	rawURI := "/api/v1/customers/98214/accounts/6ba7b810-9dad-11d1-80b4-00c04fd430c8?token=super_secret_jwt_token_12345&api_key=secret_key_88"

	summary := SanitizeHTTP("get", rawURI, 200, 14.5, 256, 1024)

	if summary.Method != "GET" {
		t.Fatalf("expected method GET, got %s", summary.Method)
	}

	// Secret Leakage Assertions (Criterion 7)
	secretTokens := []string{
		"super_secret_jwt_token_12345",
		"secret_key_88",
		"token=",
		"api_key=",
		"98214",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
	}

	for _, secret := range secretTokens {
		if strings.Contains(summary.RouteTemplate, secret) {
			t.Fatalf("CRITICAL SECRET LEAK: route template contains sensitive token %q: %s", secret, summary.RouteTemplate)
		}
	}

	expectedTemplate := "/api/v1/customers/:id/accounts/:id"
	if summary.RouteTemplate != expectedTemplate {
		t.Fatalf("expected route template %q, got %q", expectedTemplate, summary.RouteTemplate)
	}

	if summary.StatusClass != "2xx" {
		t.Fatalf("expected status class 2xx, got %s", summary.StatusClass)
	}

	if summary.ContentTrust != "untrusted" {
		t.Fatalf("expected content trust to be untrusted, got %s", summary.ContentTrust)
	}
}

func TestSanitizeGRPC(t *testing.T) {
	rawPath := "/billing.v1.PaymentService/ChargeCustomer"
	summary := SanitizeGRPC(rawPath, 0, 8.2, 128, 512)

	if summary.Service != "billing.v1.PaymentService" {
		t.Fatalf("expected service billing.v1.PaymentService, got %s", summary.Service)
	}
	if summary.Method != "ChargeCustomer" {
		t.Fatalf("expected method ChargeCustomer, got %s", summary.Method)
	}
	if summary.StatusString != "OK" {
		t.Fatalf("expected status string OK, got %s", summary.StatusString)
	}
}

func FuzzTemplatizeRoute(f *testing.F) {
	f.Add("/api/v1/users/123/profile")
	f.Add("/items/4f5a6b7c-8d9e-1011-1213-141516171819")
	f.Add("/very/deep/nested/path/with/too/many/levels/and/random/tokens")
	f.Add("/?token=secret")

	f.Fuzz(func(t *testing.T, input string) {
		res := TemplatizeRoute(input)
		if len(res) == 0 {
			t.Fatal("templatized route should not be empty")
		}
		if res[0] != '/' {
			t.Fatalf("templatized route should start with /, got %s", res)
		}
	})
}
