// SPDX-License-Identifier: MIT

package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

func TestDockerAdapter_SwarmDiscovery(t *testing.T) {
	// Setup mock Docker Engine & Swarm API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/info":
			w.Write([]byte(`{
				"ID": "host-docker-node-1",
				"Name": "manager-01",
				"ServerVersion": "24.0.7",
				"OperatingSystem": "Ubuntu 22.04.3 LTS",
				"NCPU": 8,
				"MemTotal": 16777216000,
				"Swarm": {
					"NodeID": "node-mgr-1",
					"LocalNodeState": "active",
					"ControlAvailable": true,
					"Cluster": {
						"ID": "swarm-cluster-alpha",
						"Spec": { "Name": "production-swarm" }
					}
				}
			}`))

		case "/nodes":
			w.Write([]byte(`[
				{
					"ID": "node-mgr-1",
					"Spec": { "Role": "manager", "Availability": "active", "Labels": {"zone": "us-east-1a"} },
					"Description": {
						"Hostname": "manager-01",
						"Platform": { "OS": "linux", "Architecture": "x86_64" },
						"Resources": { "NanoCPUs": 8000000000, "MemoryBytes": 16777216000 },
						"Engine": { "EngineVersion": "24.0.7" }
					},
					"Status": { "State": "ready", "Addr": "192.168.1.10" },
					"ManagerStatus": { "Leader": true, "Reachability": "reachable", "Addr": "192.168.1.10:2377" }
				},
				{
					"ID": "node-wrk-2",
					"Spec": { "Role": "worker", "Availability": "active", "Labels": {"zone": "us-east-1b"} },
					"Description": {
						"Hostname": "worker-02",
						"Platform": { "OS": "linux", "Architecture": "x86_64" },
						"Resources": { "NanoCPUs": 16000000000, "MemoryBytes": 33554432000 },
						"Engine": { "EngineVersion": "24.0.7" }
					},
					"Status": { "State": "ready", "Addr": "192.168.1.11" }
				}
			]`))

		case "/services":
			w.Write([]byte(`[
				{
					"ID": "svc-payments-99",
					"Spec": {
						"Name": "payments-api",
						"Labels": { "tier": "backend", "team": "fintech" },
						"TaskTemplate": {
							"ContainerSpec": { "Image": "registry.corp/payments:v2.1.0" }
						},
						"Mode": { "Replicated": { "Replicas": 2 } },
						"EndpointSpec": {
							"Ports": [{ "Protocol": "tcp", "TargetPort": 8080, "PublishedPort": 443 }]
						}
					},
					"Endpoint": {
						"VirtualIPs": [
							{ "NetworkID": "net-overlay-10", "Addr": "10.0.1.5/24" }
						]
					}
				}
			]`))

		case "/tasks":
			w.Write([]byte(`[
				{
					"ID": "task-pay-slot1-uuid",
					"ServiceID": "svc-payments-99",
					"NodeID": "node-mgr-1",
					"Slot": 1,
					"DesiredState": "running",
					"Status": {
						"State": "running",
						"ContainerStatus": { "ContainerID": "c-container-pay-01" }
					}
				},
				{
					"ID": "task-pay-slot2-uuid",
					"ServiceID": "svc-payments-99",
					"NodeID": "node-wrk-2",
					"Slot": 2,
					"DesiredState": "running",
					"Status": {
						"State": "running",
						"ContainerStatus": { "ContainerID": "c-container-pay-02" }
					}
				}
			]`))

		case "/networks":
			w.Write([]byte(`[
				{
					"Id": "net-overlay-10",
					"Name": "ingress_overlay",
					"Driver": "overlay",
					"Scope": "swarm",
					"Internal": false,
					"Labels": {},
					"IPAM": { "Config": [{ "Subnet": "10.0.1.0/24", "Gateway": "10.0.1.1" }] }
				},
				{
					"Id": "net-bridge-local",
					"Name": "bridge",
					"Driver": "bridge",
					"Scope": "local",
					"Internal": false,
					"Labels": {},
					"IPAM": { "Config": [{ "Subnet": "172.17.0.0/16", "Gateway": "172.17.0.1" }] }
				}
			]`))

		case "/containers/json":
			w.Write([]byte(`[
				{
					"Id": "c-container-pay-01",
					"Names": ["/payments-api.1.task-pay-slot1-uuid"],
					"Image": "registry.corp/payments:v2.1.0",
					"Command": "./server",
					"Created": 1700000000,
					"State": "running",
					"Status": "Up 5 days",
					"Labels": {
						"com.docker.swarm.service.id": "svc-payments-99",
						"com.docker.swarm.task.id": "task-pay-slot1-uuid"
					},
					"Ports": [{ "PrivatePort": 8080, "PublicPort": 443, "Type": "tcp" }],
					"NetworkSettings": {
						"Networks": {
							"ingress_overlay": { "NetworkID": "net-overlay-10", "IPAddress": "10.0.1.15" }
						}
					},
					"Mounts": [
						{ "Name": "vol-payments-data", "Type": "volume", "Source": "/var/lib/docker/volumes/vol-payments-data/_data", "Destination": "/data" }
					]
				}
			]`))

		case "/volumes":
			w.Write([]byte(`{
				"Volumes": [
					{
						"Name": "vol-payments-data",
						"Driver": "local",
						"Mountpoint": "/var/lib/docker/volumes/vol-payments-data/_data",
						"Scope": "local",
						"Labels": { "purpose": "transaction-logs" }
					}
				]
			}`))

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := New(Config{
		Host:       server.URL,
		APIVersion: "",
		Timeout:    5 * time.Second,
	}, nil)
	adapter.SetHTTPClient(server.Client())
	adapter.SetBaseURL(server.URL)

	if adapter.Name() != "docker_swarm" {
		t.Fatalf("expected adapter name docker_swarm, got %s", adapter.Name())
	}

	result, err := adapter.Discover(context.Background(), "env-test-01")
	if err != nil {
		t.Fatalf("unexpected discovery error: %v", err)
	}

	if len(result.Errors) > 0 {
		t.Fatalf("expected 0 discovery errors, got: %v", result.Errors)
	}

	// 1. Verify Resource Kinds
	kindCounts := make(map[models.ResourceKind]int)
	for _, res := range result.Resources {
		kindCounts[res.Kind]++
		if res.EnvironmentID != "env-test-01" {
			t.Errorf("resource %s has unexpected envID %s", res.Name, res.EnvironmentID)
		}
	}

	expectedKinds := map[models.ResourceKind]int{
		models.KindDockerHost:      1,
		models.KindSwarmCluster:    1,
		models.KindSwarmManager:    1,
		models.KindSwarmWorker:     1,
		models.KindSwarmService:    1,
		models.KindSwarmTask:       2,
		models.KindDockerNetwork:   2,
		models.KindDockerContainer: 1,
		models.KindDockerVolume:    1,
	}

	for kind, expectedCount := range expectedKinds {
		if count := kindCounts[kind]; count != expectedCount {
			t.Errorf("expected %d resources of kind %s, got %d", expectedCount, kind, count)
		}
	}

	// 2. Verify Relationships
	relCounts := make(map[models.RelationshipType]int)
	for _, rel := range result.Relationships {
		relCounts[rel.RelationshipType]++
	}

	if relCounts[models.RelContains] == 0 {
		t.Error("expected CONTAINS relationships")
	}
	if relCounts[models.RelScheduledOn] == 0 {
		t.Error("expected SCHEDULED_ON relationships for Swarm tasks")
	}
	if relCounts[models.RelRunsOn] == 0 {
		t.Error("expected RUNS_ON relationships linking tasks to containers")
	}
	if relCounts[models.RelExposes] == 0 {
		t.Error("expected EXPOSES relationships linking Swarm services to containers")
	}
}

func TestDockerAdapter_StandaloneHostDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/info":
			w.Write([]byte(`{
				"ID": "standalone-docker-host",
				"Name": "docker-standalone",
				"ServerVersion": "24.0.5",
				"OperatingSystem": "Debian GNU/Linux 12 (bookworm)",
				"NCPU": 4,
				"MemTotal": 8388608000,
				"Swarm": { "LocalNodeState": "inactive" }
			}`))

		case "/networks":
			w.Write([]byte(`[
				{ "Id": "bridge-net-id", "Name": "bridge", "Driver": "bridge", "Scope": "local" }
			]`))

		case "/containers/json":
			w.Write([]byte(`[
				{
					"Id": "standalone-redis-c1",
					"Names": ["/my-redis"],
					"Image": "redis:7.2-alpine",
					"State": "running",
					"Status": "Up 12 hours",
					"NetworkSettings": {
						"Networks": { "bridge": { "NetworkID": "bridge-net-id", "IPAddress": "172.17.0.2" } }
					}
				}
			]`))

		case "/volumes":
			w.Write([]byte(`{ "Volumes": [] }`))

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := New(Config{Host: server.URL}, nil)
	adapter.SetHTTPClient(server.Client())
	adapter.SetBaseURL(server.URL)

	result, err := adapter.Discover(context.Background(), "env-standalone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, res := range result.Resources {
		if res.Kind == models.KindSwarmCluster || res.Kind == models.KindSwarmService || res.Kind == models.KindSwarmTask {
			t.Errorf("unexpected Swarm resource %s (%s) on standalone host", res.Name, res.Kind)
		}
	}

	var foundContainer bool
	for _, res := range result.Resources {
		if res.Kind == models.KindDockerContainer && res.Name == "my-redis" {
			foundContainer = true
			break
		}
	}
	if !foundContainer {
		t.Fatal("expected container 'my-redis' to be discovered on standalone host")
	}
}

func TestDockerAdapter_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal daemon error", http.StatusInternalServerError)
	}))
	defer server.Close()

	adapter := New(Config{Host: server.URL}, nil)
	adapter.SetHTTPClient(server.Client())
	adapter.SetBaseURL(server.URL)

	result, err := adapter.Discover(context.Background(), "env-error")
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected discovery errors when API returns 500")
	}
}
