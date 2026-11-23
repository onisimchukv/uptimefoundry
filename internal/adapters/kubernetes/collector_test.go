// SPDX-License-Identifier: MIT

package kubernetes

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

func TestKubernetesAdapter_Discover(t *testing.T) {
	// Mock Kubernetes API Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/nodes":
			_ = json.NewEncoder(w).Encode(k8sNodeList{
				Items: []k8sNode{
					{
						Metadata: k8sObjectMeta{Name: "worker-01", UID: "uid-node-1"},
						Status: struct {
							Phase    string `json:"phase"`
							NodeInfo struct {
								OperatingSystem string `json:"operatingSystem"`
								Architecture    string `json:"architecture"`
								KubeletVersion  string `json:"kubeletVersion"`
							} `json:"nodeInfo"`
						}{
							Phase: "Ready",
							NodeInfo: struct {
								OperatingSystem string `json:"operatingSystem"`
								Architecture    string `json:"architecture"`
								KubeletVersion  string `json:"kubeletVersion"`
							}{OperatingSystem: "linux", Architecture: "amd64", KubeletVersion: "v1.30.0"},
						},
					},
				},
			})

		case "/api/v1/namespaces":
			_ = json.NewEncoder(w).Encode(k8sNamespaceList{
				Items: []k8sNamespace{
					{
						Metadata: k8sObjectMeta{Name: "production", UID: "uid-ns-1"},
						Status:   struct{ Phase string `json:"phase"` }{Phase: "Active"},
					},
				},
			})

		case "/apis/apps/v1/deployments":
			replicas := int32(2)
			_ = json.NewEncoder(w).Encode(k8sDeploymentList{
				Items: []k8sDeployment{
					{
						Metadata: k8sObjectMeta{Name: "payment-api", Namespace: "production", UID: "uid-dep-1"},
						Spec:     struct{ Replicas *int32 `json:"replicas"` }{Replicas: &replicas},
						Status: struct {
							Replicas          int32 `json:"replicas"`
							AvailableReplicas int32 `json:"availableReplicas"`
						}{Replicas: 2, AvailableReplicas: 2},
					},
				},
			})

		case "/api/v1/services":
			_ = json.NewEncoder(w).Encode(k8sServiceList{
				Items: []k8sService{
					{
						Metadata: k8sObjectMeta{Name: "payment-svc", Namespace: "production", UID: "uid-svc-1"},
						Spec: struct {
							ClusterIP string            `json:"clusterIP"`
							Type      string            `json:"type"`
							Selector  map[string]string `json:"selector"`
						}{
							ClusterIP: "10.96.0.100",
							Type:      "ClusterIP",
							Selector:  map[string]string{"app": "payment"},
						},
					},
				},
			})

		case "/api/v1/pods":
			_ = json.NewEncoder(w).Encode(k8sPodList{
				Items: []k8sPod{
					{
						Metadata: k8sObjectMeta{
							Name:      "payment-api-abcde",
							Namespace: "production",
							UID:       "uid-pod-1",
							Labels:    map[string]string{"app": "payment"},
						},
						Spec: struct {
							NodeName string `json:"nodeName"`
						}{NodeName: "worker-01"},
						Status: struct {
							Phase  string `json:"phase"`
							PodIP  string `json:"podIP"`
							HostIP string `json:"hostIP"`
						}{Phase: "Running", PodIP: "10.244.1.5", HostIP: "192.168.1.10"},
					},
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	adapter := New(Config{
		BaseURL:     server.URL,
		ClusterName: "test-k8s-cluster",
	}, logger)
	adapter.SetHTTPClient(server.Client())

	if adapter.Name() != "kubernetes" {
		t.Errorf("expected adapter name 'kubernetes', got %s", adapter.Name())
	}

	res, err := adapter.Discover(context.Background(), "env-prod-01")
	if err != nil {
		t.Fatalf("unexpected discover error: %v", err)
	}

	if len(res.Errors) > 0 {
		t.Errorf("expected 0 errors, got %v", res.Errors)
	}

	// Should discover: 1 cluster, 1 node, 1 namespace, 1 deployment, 1 service, 1 pod = 6 resources
	if len(res.Resources) != 6 {
		t.Fatalf("expected 6 resources, got %d", len(res.Resources))
	}

	resourceKinds := make(map[models.ResourceKind]int)
	for _, r := range res.Resources {
		resourceKinds[r.Kind]++
	}
	if resourceKinds[models.KindK8sCluster] != 1 {
		t.Errorf("expected 1 K8sCluster, got %d", resourceKinds[models.KindK8sCluster])
	}
	if resourceKinds[models.KindK8sNode] != 1 {
		t.Errorf("expected 1 K8sNode, got %d", resourceKinds[models.KindK8sNode])
	}
	if resourceKinds[models.KindK8sNamespace] != 1 {
		t.Errorf("expected 1 K8sNamespace, got %d", resourceKinds[models.KindK8sNamespace])
	}
	if resourceKinds[models.KindK8sDeployment] != 1 {
		t.Errorf("expected 1 K8sDeployment, got %d", resourceKinds[models.KindK8sDeployment])
	}
	if resourceKinds[models.KindK8sService] != 1 {
		t.Errorf("expected 1 K8sService, got %d", resourceKinds[models.KindK8sService])
	}
	if resourceKinds[models.KindK8sPod] != 1 {
		t.Errorf("expected 1 K8sPod, got %d", resourceKinds[models.KindK8sPod])
	}

	// Should discover relationships:
	// Cluster CONTAINS Node
	// Cluster CONTAINS Namespace
	// Namespace CONTAINS Deployment
	// Namespace CONTAINS Service
	// Pod SCHEDULED_ON Node
	// Service EXPOSES Pod
	if len(res.Relationships) != 6 {
		t.Fatalf("expected 6 relationships, got %d", len(res.Relationships))
	}

	relTypes := make(map[models.RelationshipType]int)
	for _, rel := range res.Relationships {
		relTypes[rel.RelationshipType]++
	}

	if relTypes[models.RelContains] != 4 {
		t.Errorf("expected 4 CONTAINS relationships, got %d", relTypes[models.RelContains])
	}
	if relTypes[models.RelScheduledOn] != 1 {
		t.Errorf("expected 1 SCHEDULED_ON relationship, got %d", relTypes[models.RelScheduledOn])
	}
	if relTypes[models.RelExposes] != 1 {
		t.Errorf("expected 1 EXPOSES relationship, got %d", relTypes[models.RelExposes])
	}
}

func TestKubernetesAdapter_EmptyBaseURL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	adapter := New(Config{BaseURL: ""}, logger)

	res, err := adapter.Discover(context.Background(), "env-01")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res.Errors) == 0 {
		t.Error("expected error message when base_url is empty")
	}
}
