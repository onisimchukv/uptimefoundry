package flows

import (
	"testing"
)

func TestVIPResolver(t *testing.T) {
	resolver := NewVIPResolver()

	// Register Kubernetes ClusterIP Service with backing pods
	resolver.RegisterService(
		"k8s_service:billing-svc",
		"10.96.0.100",
		8080,
		[]Endpoint{
			{WorkloadName: "pod:billing-api-74bf", IP: "10.244.1.15", Port: 8080},
		},
	)

	// 1. Resolve registered VIP
	workload, vip, conf := resolver.Resolve("10.96.0.100", 8080)
	if workload != "pod:billing-api-74bf" {
		t.Fatalf("expected resolved workload pod:billing-api-74bf, got %s", workload)
	}
	if vip != "10.96.0.100" {
		t.Fatalf("expected service VIP 10.96.0.100, got %s", vip)
	}
	if conf != 1.0 {
		t.Fatalf("expected confidence 1.0, got %f", conf)
	}

	// 2. Resolve unregistered direct IP
	directWorkload, directVIP, directConf := resolver.Resolve("192.168.1.50", 5432)
	if directWorkload != "192.168.1.50" {
		t.Fatalf("expected direct IP 192.168.1.50, got %s", directWorkload)
	}
	if directVIP != "" {
		t.Fatalf("expected empty VIP for direct IP, got %s", directVIP)
	}
	if directConf != 1.0 {
		t.Fatalf("expected confidence 1.0, got %f", directConf)
	}
}
