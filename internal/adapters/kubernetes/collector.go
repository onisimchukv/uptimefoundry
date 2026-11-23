// SPDX-License-Identifier: MIT

package kubernetes

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/onisimchukv/uptimefoundry/internal/adapters"
	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

// Config configures the Kubernetes discovery adapter.
type Config struct {
	BaseURL     string        `yaml:"base_url"`
	Token       string        `yaml:"token"`
	InCluster   bool          `yaml:"in_cluster"`
	ClusterName string        `yaml:"cluster_name"`
	Timeout     time.Duration `yaml:"timeout"`
}

// Adapter discovers Kubernetes cluster assets: Nodes, Namespaces, Deployments, Services, and Pods.
type Adapter struct {
	cfg        Config
	httpClient *http.Client
	logger     *slog.Logger
}

var _ adapters.DiscoveryAdapter = (*Adapter)(nil)

// New creates a Kubernetes discovery adapter.
func New(cfg Config, logger *slog.Logger) *Adapter {
	if cfg.ClusterName == "" {
		cfg.ClusterName = "k8s-cluster"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // In-cluster CA or user-configured
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   cfg.Timeout,
	}

	if cfg.InCluster && cfg.BaseURL == "" {
		cfg.BaseURL = "https://kubernetes.default.svc"
		tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if err == nil {
			cfg.Token = strings.TrimSpace(string(tokenBytes))
		}
	}

	return &Adapter{
		cfg:        cfg,
		httpClient: client,
		logger:     logger,
	}
}

// SetHTTPClient allows overriding the client for mock/unit testing.
func (a *Adapter) SetHTTPClient(client *http.Client) {
	a.httpClient = client
}

func (a *Adapter) Name() string {
	return "kubernetes"
}

// Minimal Kubernetes API resource structures for JSON unmarshaling
type k8sListMeta struct {
	ResourceVersion string `json:"resourceVersion"`
}

type k8sObjectMeta struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	UID               string            `json:"uid"`
	Labels            map[string]string `json:"labels"`
	CreationTimestamp string            `json:"creationTimestamp"`
}

type k8sNode struct {
	Metadata k8sObjectMeta `json:"metadata"`
	Status   struct {
		Phase string `json:"phase"`
		NodeInfo struct {
			OperatingSystem string `json:"operatingSystem"`
			Architecture    string `json:"architecture"`
			KubeletVersion  string `json:"kubeletVersion"`
		} `json:"nodeInfo"`
	} `json:"status"`
}

type k8sNodeList struct {
	Items []k8sNode `json:"items"`
}

type k8sNamespace struct {
	Metadata k8sObjectMeta `json:"metadata"`
	Status   struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type k8sNamespaceList struct {
	Items []k8sNamespace `json:"items"`
}

type k8sService struct {
	Metadata k8sObjectMeta `json:"metadata"`
	Spec     struct {
		ClusterIP string            `json:"clusterIP"`
		Type      string            `json:"type"`
		Selector  map[string]string `json:"selector"`
	} `json:"spec"`
}

type k8sServiceList struct {
	Items []k8sService `json:"items"`
}

type k8sPod struct {
	Metadata k8sObjectMeta `json:"metadata"`
	Spec     struct {
		NodeName string `json:"nodeName"`
	} `json:"spec"`
	Status struct {
		Phase  string `json:"phase"`
		PodIP  string `json:"podIP"`
		HostIP string `json:"hostIP"`
	} `json:"status"`
}

type k8sPodList struct {
	Items []k8sPod `json:"items"`
}

type k8sDeployment struct {
	Metadata k8sObjectMeta `json:"metadata"`
	Spec     struct {
		Replicas *int32 `json:"replicas"`
	} `json:"spec"`
	Status struct {
		Replicas          int32 `json:"replicas"`
		AvailableReplicas int32 `json:"availableReplicas"`
	} `json:"status"`
}

type k8sDeploymentList struct {
	Items []k8sDeployment `json:"items"`
}

// Discover queries the Kubernetes API and builds a normalized topology graph.
func (a *Adapter) Discover(ctx context.Context, envID string) (*adapters.DiscoveryResult, error) {
	result := &adapters.DiscoveryResult{
		AdapterName: a.Name(),
		Metadata:    map[string]any{"cluster_name": a.cfg.ClusterName},
	}

	if a.cfg.BaseURL == "" {
		result.Errors = append(result.Errors, "kubernetes api base_url is not configured")
		return result, nil
	}

	now := time.Now().UTC()

	// 1. Cluster root resource
	clusterID := uuid.NewMD5(uuid.NameSpaceDNS, []byte(a.cfg.ClusterName)).String()
	clusterRes := &models.Resource{
		ID:            clusterID,
		EnvironmentID: envID,
		Kind:          models.KindK8sCluster,
		Provider:      "kubernetes",
		ExternalID:    a.cfg.ClusterName,
		Name:          a.cfg.ClusterName,
		Status:        "active",
		Source:        "k8s_api",
		HierarchyPath: a.cfg.ClusterName,
		FirstSeenAt:   now,
		LastSeenAt:    now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	result.Resources = append(result.Resources, clusterRes)

	// 2. Discover Nodes
	nodeMap := make(map[string]string) // nodeName -> resourceID
	var nodeList k8sNodeList
	if err := a.get(ctx, "/api/v1/nodes", &nodeList); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("fetch nodes: %v", err))
	} else {
		for _, n := range nodeList.Items {
			nodeID := uuid.NewMD5(uuid.NameSpaceDNS, []byte(fmt.Sprintf("%s/node/%s", a.cfg.ClusterName, n.Metadata.Name))).String()
			nodeMap[n.Metadata.Name] = nodeID

			res := &models.Resource{
				ID:            nodeID,
				EnvironmentID: envID,
				Kind:          models.KindK8sNode,
				Provider:      "kubernetes",
				ExternalID:    n.Metadata.UID,
				Name:          n.Metadata.Name,
				Labels:        n.Metadata.Labels,
				Status:        n.Status.Phase,
				Source:        "k8s_api",
				HierarchyPath: fmt.Sprintf("%s.nodes.%s", a.cfg.ClusterName, n.Metadata.Name),
				Attributes: map[string]any{
					"os":      n.Status.NodeInfo.OperatingSystem,
					"arch":    n.Status.NodeInfo.Architecture,
					"kubelet": n.Status.NodeInfo.KubeletVersion,
				},
				FirstSeenAt: now,
				LastSeenAt:  now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			result.Resources = append(result.Resources, res)

			// Relationship: Cluster CONTAINS Node
			result.Relationships = append(result.Relationships, &models.Relationship{
				ID:               uuid.New().String(),
				EnvironmentID:    envID,
				SourceResourceID: clusterID,
				TargetResourceID: nodeID,
				RelationshipType: models.RelContains,
				Confidence:       1.0,
				EvidenceSource:   "k8s_api",
				ValidFrom:        now,
				CreatedAt:        now,
			})
		}
	}

	// 3. Discover Namespaces
	nsMap := make(map[string]string) // nsName -> resourceID
	var nsList k8sNamespaceList
	if err := a.get(ctx, "/api/v1/namespaces", &nsList); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("fetch namespaces: %v", err))
	} else {
		for _, ns := range nsList.Items {
			nsID := uuid.NewMD5(uuid.NameSpaceDNS, []byte(fmt.Sprintf("%s/ns/%s", a.cfg.ClusterName, ns.Metadata.Name))).String()
			nsMap[ns.Metadata.Name] = nsID

			res := &models.Resource{
				ID:            nsID,
				EnvironmentID: envID,
				Kind:          models.KindK8sNamespace,
				Provider:      "kubernetes",
				ExternalID:    ns.Metadata.UID,
				Name:          ns.Metadata.Name,
				Labels:        ns.Metadata.Labels,
				Status:        ns.Status.Phase,
				Source:        "k8s_api",
				HierarchyPath: fmt.Sprintf("%s.namespaces.%s", a.cfg.ClusterName, ns.Metadata.Name),
				FirstSeenAt:   now,
				LastSeenAt:    now,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			result.Resources = append(result.Resources, res)

			// Relationship: Cluster CONTAINS Namespace
			result.Relationships = append(result.Relationships, &models.Relationship{
				ID:               uuid.New().String(),
				EnvironmentID:    envID,
				SourceResourceID: clusterID,
				TargetResourceID: nsID,
				RelationshipType: models.RelContains,
				Confidence:       1.0,
				EvidenceSource:   "k8s_api",
				ValidFrom:        now,
				CreatedAt:        now,
			})
		}
	}

	// 4. Discover Deployments
	var depList k8sDeploymentList
	if err := a.get(ctx, "/apis/apps/v1/deployments", &depList); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("fetch deployments: %v", err))
	} else {
		for _, d := range depList.Items {
			depID := uuid.NewMD5(uuid.NameSpaceDNS, []byte(fmt.Sprintf("%s/dep/%s/%s", a.cfg.ClusterName, d.Metadata.Namespace, d.Metadata.Name))).String()
			replicas := int32(1)
			if d.Spec.Replicas != nil {
				replicas = *d.Spec.Replicas
			}

			res := &models.Resource{
				ID:            depID,
				EnvironmentID: envID,
				Kind:          models.KindK8sDeployment,
				Provider:      "kubernetes",
				ExternalID:    d.Metadata.UID,
				Name:          d.Metadata.Name,
				Namespace:     d.Metadata.Namespace,
				Labels:        d.Metadata.Labels,
				Status:        fmt.Sprintf("%d/%d", d.Status.AvailableReplicas, replicas),
				Source:        "k8s_api",
				HierarchyPath: fmt.Sprintf("%s.%s.deployments.%s", a.cfg.ClusterName, d.Metadata.Namespace, d.Metadata.Name),
				Attributes: map[string]any{
					"replicas":           replicas,
					"available_replicas": d.Status.AvailableReplicas,
				},
				FirstSeenAt: now,
				LastSeenAt:  now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			result.Resources = append(result.Resources, res)

			// Relationship: Namespace CONTAINS Deployment
			if nsID, ok := nsMap[d.Metadata.Namespace]; ok {
				result.Relationships = append(result.Relationships, &models.Relationship{
					ID:               uuid.New().String(),
					EnvironmentID:    envID,
					SourceResourceID: nsID,
					TargetResourceID: depID,
					RelationshipType: models.RelContains,
					Confidence:       1.0,
					EvidenceSource:   "k8s_api",
					ValidFrom:        now,
					CreatedAt:        now,
				})
			}
		}
	}

	// 5. Discover Services
	services := make([]k8sService, 0)
	var svcList k8sServiceList
	if err := a.get(ctx, "/api/v1/services", &svcList); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("fetch services: %v", err))
	} else {
		services = svcList.Items
		for _, s := range services {
			svcID := uuid.NewMD5(uuid.NameSpaceDNS, []byte(fmt.Sprintf("%s/svc/%s/%s", a.cfg.ClusterName, s.Metadata.Namespace, s.Metadata.Name))).String()

			res := &models.Resource{
				ID:            svcID,
				EnvironmentID: envID,
				Kind:          models.KindK8sService,
				Provider:      "kubernetes",
				ExternalID:    s.Metadata.UID,
				Name:          s.Metadata.Name,
				Namespace:     s.Metadata.Namespace,
				Labels:        s.Metadata.Labels,
				Status:        "active",
				Source:        "k8s_api",
				HierarchyPath: fmt.Sprintf("%s.%s.services.%s", a.cfg.ClusterName, s.Metadata.Namespace, s.Metadata.Name),
				Attributes: map[string]any{
					"cluster_ip": s.Spec.ClusterIP,
					"type":       s.Spec.Type,
					"selector":   s.Spec.Selector,
				},
				FirstSeenAt: now,
				LastSeenAt:  now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			result.Resources = append(result.Resources, res)

			// Relationship: Namespace CONTAINS Service
			if nsID, ok := nsMap[s.Metadata.Namespace]; ok {
				result.Relationships = append(result.Relationships, &models.Relationship{
					ID:               uuid.New().String(),
					EnvironmentID:    envID,
					SourceResourceID: nsID,
					TargetResourceID: svcID,
					RelationshipType: models.RelContains,
					Confidence:       1.0,
					EvidenceSource:   "k8s_api",
					ValidFrom:        now,
					CreatedAt:        now,
				})
			}
		}
	}

	// 6. Discover Pods
	var podList k8sPodList
	if err := a.get(ctx, "/api/v1/pods", &podList); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("fetch pods: %v", err))
	} else {
		for _, p := range podList.Items {
			podID := uuid.NewMD5(uuid.NameSpaceDNS, []byte(fmt.Sprintf("%s/pod/%s/%s", a.cfg.ClusterName, p.Metadata.Namespace, p.Metadata.Name))).String()

			res := &models.Resource{
				ID:            podID,
				EnvironmentID: envID,
				Kind:          models.KindK8sPod,
				Provider:      "kubernetes",
				ExternalID:    p.Metadata.UID,
				Name:          p.Metadata.Name,
				Namespace:     p.Metadata.Namespace,
				Labels:        p.Metadata.Labels,
				Status:        p.Status.Phase,
				Source:        "k8s_api",
				HierarchyPath: fmt.Sprintf("%s.%s.pods.%s", a.cfg.ClusterName, p.Metadata.Namespace, p.Metadata.Name),
				Attributes: map[string]any{
					"pod_ip":    p.Status.PodIP,
					"host_ip":   p.Status.HostIP,
					"node_name": p.Spec.NodeName,
				},
				FirstSeenAt: now,
				LastSeenAt:  now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			result.Resources = append(result.Resources, res)

			// Relationship: Pod SCHEDULED_ON Node
			if nodeID, ok := nodeMap[p.Spec.NodeName]; ok {
				result.Relationships = append(result.Relationships, &models.Relationship{
					ID:               uuid.New().String(),
					EnvironmentID:    envID,
					SourceResourceID: podID,
					TargetResourceID: nodeID,
					RelationshipType: models.RelScheduledOn,
					Confidence:       1.0,
					EvidenceSource:   "k8s_api",
					ValidFrom:        now,
					CreatedAt:        now,
				})
			}

			// Relationship: Service EXPOSES Pod (via selector match)
			for _, s := range services {
				if s.Metadata.Namespace == p.Metadata.Namespace && matchesSelector(s.Spec.Selector, p.Metadata.Labels) {
					svcID := uuid.NewMD5(uuid.NameSpaceDNS, []byte(fmt.Sprintf("%s/svc/%s/%s", a.cfg.ClusterName, s.Metadata.Namespace, s.Metadata.Name))).String()
					result.Relationships = append(result.Relationships, &models.Relationship{
						ID:               uuid.New().String(),
						EnvironmentID:    envID,
						SourceResourceID: svcID,
						TargetResourceID: podID,
						RelationshipType: models.RelExposes,
						Confidence:       1.0,
						EvidenceSource:   "k8s_api",
						ValidFrom:        now,
						CreatedAt:        now,
					})
				}
			}
		}
	}

	return result, nil
}

func (a *Adapter) get(ctx context.Context, path string, target any) error {
	url := strings.TrimRight(a.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if a.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func matchesSelector(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}
