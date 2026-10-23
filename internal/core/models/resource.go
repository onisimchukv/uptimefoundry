package models

import (
	"time"
)

// ResourceKind defines the type of an infrastructure or software entity.
type ResourceKind string

const (
	// Infrastructure
	KindEnvironment      ResourceKind = "Environment"
	KindHost             ResourceKind = "Host"
	KindNetworkInterface ResourceKind = "NetworkInterface"
	KindFilesystem       ResourceKind = "Filesystem"
	KindDiskDevice       ResourceKind = "DiskDevice"

	// Containers
	KindDockerHost      ResourceKind = "DockerHost"
	KindDockerContainer ResourceKind = "DockerContainer"
	KindDockerImage     ResourceKind = "DockerImage"
	KindDockerNetwork   ResourceKind = "DockerNetwork"
	KindDockerVolume    ResourceKind = "DockerVolume"

	// Kubernetes
	KindK8sCluster               ResourceKind = "K8sCluster"
	KindK8sNode                  ResourceKind = "K8sNode"
	KindK8sNamespace             ResourceKind = "K8sNamespace"
	KindK8sPod                   ResourceKind = "K8sPod"
	KindK8sDeployment            ResourceKind = "K8sDeployment"
	KindK8sStatefulSet           ResourceKind = "K8sStatefulSet"
	KindK8sDaemonSet             ResourceKind = "K8sDaemonSet"
	KindK8sReplicaSet            ResourceKind = "K8sReplicaSet"
	KindK8sService               ResourceKind = "K8sService"
	KindK8sIngress               ResourceKind = "K8sIngress"
	KindK8sPersistentVolume      ResourceKind = "K8sPersistentVolume"
	KindK8sPersistentVolumeClaim ResourceKind = "K8sPersistentVolumeClaim"
	KindK8sEvent                 ResourceKind = "K8sEvent"

	// Docker Swarm
	KindSwarmCluster ResourceKind = "SwarmCluster"
	KindSwarmManager ResourceKind = "SwarmManager"
	KindSwarmWorker  ResourceKind = "SwarmWorker"
	KindSwarmService ResourceKind = "SwarmService"
	KindSwarmTask    ResourceKind = "SwarmTask"

	// Observability & Telemetry
	KindTelemetryEndpoint ResourceKind = "TelemetryEndpoint"
	KindOTelCollector     ResourceKind = "OTelCollector"
	KindMetricsBackend    ResourceKind = "MetricsBackend"
	KindLogBackend        ResourceKind = "LogBackend"
	KindTraceBackend      ResourceKind = "TraceBackend"
)

// Resource represents a normalized asset discovered across infrastructure.
type Resource struct {
	ID            string         `json:"id"`
	EnvironmentID string         `json:"environment_id"`
	Kind          ResourceKind   `json:"kind"`
	Provider      string         `json:"provider"`
	ExternalID    string         `json:"external_id"`
	Name          string         `json:"name"`
	Namespace     string         `json:"namespace,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Attributes    map[string]any    `json:"attributes,omitempty"`
	Status        string         `json:"status"`
	Source        string         `json:"source"`
	HierarchyPath string         `json:"hierarchy_path,omitempty"` // ltree representation (e.g. "prod.k8s_cluster_1.default.billing_pod")
	FirstSeenAt   time.Time      `json:"first_seen_at"`
	LastSeenAt    time.Time      `json:"last_seen_at"`
	StaleSince    *time.Time     `json:"stale_since,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// RelationshipType defines directional edges in the property graph.
type RelationshipType string

const (
	RelRunsOn           RelationshipType = "RUNS_ON"
	RelContains         RelationshipType = "CONTAINS"
	RelConnectsTo       RelationshipType = "CONNECTS_TO"
	RelDependsOn        RelationshipType = "DEPENDS_ON"
	RelExposes          RelationshipType = "EXPOSES"
	RelMounts           RelationshipType = "MOUNTS"
	RelScheduledOn      RelationshipType = "SCHEDULED_ON"
	RelMonitoredBy      RelationshipType = "MONITORED_BY"
	RelEmitsTelemetryTo RelationshipType = "EMITS_TELEMETRY_TO"
	RelObservedBy       RelationshipType = "OBSERVED_BY"
	RelRelatedTo        RelationshipType = "RELATED_TO"
)

// Relationship represents an edge connecting two resources in the topology.
type Relationship struct {
	ID               string           `json:"id"`
	EnvironmentID    string           `json:"environment_id"`
	SourceResourceID string           `json:"source_resource_id"`
	TargetResourceID string           `json:"target_resource_id"`
	RelationshipType RelationshipType `json:"relationship_type"`
	Confidence       float64          `json:"confidence"` // 0.0 to 1.0 (e.g. 1.0 for deterministic K8s API, 0.85 for inferred trace)
	EvidenceSource   string           `json:"evidence_source"` // e.g. "k8s_api", "ebpf_socket", "trace"
	ValidFrom        time.Time        `json:"valid_from"`
	ValidTo          *time.Time       `json:"valid_to,omitempty"`
	Weight           float64          `json:"weight"` // e.g. network round-trip time (RTT in ms) or traffic volume
	Attributes       map[string]any   `json:"attributes,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
}
