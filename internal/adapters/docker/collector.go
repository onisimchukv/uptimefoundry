// SPDX-License-Identifier: MIT

package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/onisimchukv/uptimefoundry/internal/adapters"
	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

// Config configures the Docker and Docker Swarm discovery adapter.
type Config struct {
	Host       string        `yaml:"host"`        // "unix:///var/run/docker.sock" or "tcp://127.0.0.1:2375"
	APIVersion string        `yaml:"api_version"` // e.g. "v1.43"
	Timeout    time.Duration `yaml:"timeout"`
}

// Adapter discovers Docker hosts, containers, networks, volumes, and Docker Swarm clusters/services/tasks.
type Adapter struct {
	cfg        Config
	httpClient *http.Client
	baseURL    string
	logger     *slog.Logger
}

var _ adapters.DiscoveryAdapter = (*Adapter)(nil)

// New creates a Docker and Docker Swarm discovery adapter.
func New(cfg Config, logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg.Host == "" {
		if val := os.Getenv("DOCKER_HOST"); val != "" {
			cfg.Host = val
		} else {
			cfg.Host = "unix:///var/run/docker.sock"
		}
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = "v1.43"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}

	apiVersion := strings.Trim(cfg.APIVersion, "/")
	var (
		client  *http.Client
		baseURL string
	)

	if strings.HasPrefix(cfg.Host, "unix://") {
		socketPath := strings.TrimPrefix(cfg.Host, "unix://")
		dialer := net.Dialer{Timeout: cfg.Timeout}
		client = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return dialer.DialContext(ctx, "unix", socketPath)
				},
			},
			Timeout: cfg.Timeout,
		}
		baseURL = "http://docker/" + apiVersion
	} else if strings.HasPrefix(cfg.Host, "tcp://") {
		client = &http.Client{Timeout: cfg.Timeout}
		baseURL = "http://" + strings.TrimPrefix(cfg.Host, "tcp://") + "/" + apiVersion
	} else {
		client = &http.Client{Timeout: cfg.Timeout}
		baseURL = strings.TrimRight(cfg.Host, "/") + "/" + apiVersion
	}

	return &Adapter{
		cfg:        cfg,
		httpClient: client,
		baseURL:    baseURL,
		logger:     logger,
	}
}

// SetHTTPClient overrides the HTTP client for mock/unit testing.
func (a *Adapter) SetHTTPClient(client *http.Client) {
	a.httpClient = client
}

// SetBaseURL overrides the base URL for testing against a mock HTTP server.
func (a *Adapter) SetBaseURL(u string) {
	a.baseURL = strings.TrimRight(u, "/")
}

func (a *Adapter) Name() string {
	return "docker_swarm"
}

// Docker API schema structures
type dockerInfo struct {
	ID              string `json:"ID"`
	Name            string `json:"Name"`
	ServerVersion   string `json:"ServerVersion"`
	OperatingSystem string `json:"OperatingSystem"`
	NCPU            int    `json:"NCPU"`
	MemTotal        int64  `json:"MemTotal"`
	Swarm           struct {
		NodeID           string `json:"NodeID"`
		LocalNodeState   string `json:"LocalNodeState"` // "active", "inactive", "locked"
		ControlAvailable bool   `json:"ControlAvailable"`
		Cluster          struct {
			ID   string `json:"ID"`
			Spec struct {
				Name string `json:"Name"`
			} `json:"Spec"`
		} `json:"Cluster"`
	} `json:"Swarm"`
}

type dockerNode struct {
	ID   string `json:"ID"`
	Spec struct {
		Role         string            `json:"Role"`         // "manager" or "worker"
		Availability string            `json:"Availability"` // "active", "pause", "drain"
		Labels       map[string]string `json:"Labels"`
	} `json:"Spec"`
	Description struct {
		Hostname string `json:"Hostname"`
		Platform struct {
			OS           string `json:"OS"`
			Architecture string `json:"Architecture"`
		} `json:"Platform"`
		Resources struct {
			NanoCPUs    int64 `json:"NanoCPUs"`
			MemoryBytes int64 `json:"MemoryBytes"`
		} `json:"Resources"`
		Engine struct {
			EngineVersion string `json:"EngineVersion"`
		} `json:"Engine"`
	} `json:"Description"`
	Status struct {
		State string `json:"State"` // "ready", "down"
		Addr  string `json:"Addr"`
	} `json:"Status"`
	ManagerStatus *struct {
		Leader       bool   `json:"Leader"`
		Reachability string `json:"Reachability"` // "reachable", "unreachable"
		Addr         string `json:"Addr"`
	} `json:"ManagerStatus,omitempty"`
}

type dockerService struct {
	ID   string `json:"ID"`
	Spec struct {
		Name         string            `json:"Name"`
		Labels       map[string]string `json:"Labels"`
		TaskTemplate struct {
			ContainerSpec struct {
				Image string `json:"Image"`
			} `json:"ContainerSpec"`
		} `json:"TaskTemplate"`
		Mode struct {
			Replicated *struct {
				Replicas *int64 `json:"Replicas"`
			} `json:"Replicated,omitempty"`
		} `json:"Mode"`
		EndpointSpec struct {
			Ports []struct {
				Protocol      string `json:"Protocol"`
				TargetPort    int    `json:"TargetPort"`
				PublishedPort int    `json:"PublishedPort"`
			} `json:"Ports"`
		} `json:"EndpointSpec"`
	} `json:"Spec"`
	Endpoint struct {
		VirtualIPs []struct {
			NetworkID string `json:"NetworkID"`
			Addr      string `json:"Addr"`
		} `json:"VirtualIPs"`
	} `json:"Endpoint"`
}

type dockerTask struct {
	ID           string            `json:"ID"`
	ServiceID    string            `json:"ServiceID"`
	NodeID       string            `json:"NodeID"`
	Slot         int               `json:"Slot"`
	DesiredState string            `json:"DesiredState"`
	Status       struct {
		State           string `json:"State"` // "running", "complete", "failed"
		ContainerStatus struct {
			ContainerID string `json:"ContainerID"`
		} `json:"ContainerStatus"`
	} `json:"Status"`
}

type dockerNetwork struct {
	ID         string            `json:"Id"`
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Scope      string            `json:"Scope"` // "swarm", "local", "global"
	Internal   bool              `json:"Internal"`
	Labels     map[string]string `json:"Labels"`
	IPAM       struct {
		Config []struct {
			Subnet  string `json:"Subnet"`
			Gateway string `json:"Gateway"`
		} `json:"Config"`
	} `json:"IPAM"`
}

type dockerContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	Command string            `json:"Command"`
	Created int64             `json:"Created"`
	State   string            `json:"State"`  // "running", "exited"
	Status  string            `json:"Status"` // "Up 2 hours"
	Labels  map[string]string `json:"Labels"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
	NetworkSettings struct {
		Networks map[string]struct {
			NetworkID string `json:"NetworkID"`
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []struct {
		Name        string `json:"Name"`
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		Driver      string `json:"Driver"`
	} `json:"Mounts"`
}

type dockerVolumeList struct {
	Volumes []struct {
		Name       string            `json:"Name"`
		Driver     string            `json:"Driver"`
		Mountpoint string            `json:"Mountpoint"`
		Scope      string            `json:"Scope"`
		Labels     map[string]string `json:"Labels"`
	} `json:"Volumes"`
}

// Discover queries the Docker Engine and Swarm APIs and builds a normalized topology graph.
func (a *Adapter) Discover(ctx context.Context, envID string) (*adapters.DiscoveryResult, error) {
	result := &adapters.DiscoveryResult{
		AdapterName: a.Name(),
		Metadata:    map[string]any{"docker_host": a.cfg.Host},
	}

	now := time.Now().UTC()

	// 1. Fetch /info
	var info dockerInfo
	if err := a.get(ctx, "/info", &info); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("fetch docker info: %v", err))
		return result, nil
	}

	hostName := info.Name
	if hostName == "" {
		hostName = "docker-host"
	}
	hostExtID := info.ID
	if hostExtID == "" {
		hostExtID = hostName
	}
	dockerHostID := uuid.NewMD5(uuid.NameSpaceDNS, []byte("docker-host/"+hostExtID)).String()

	dockerHostRes := &models.Resource{
		ID:            dockerHostID,
		EnvironmentID: envID,
		Kind:          models.KindDockerHost,
		Provider:      "docker",
		ExternalID:    hostExtID,
		Name:          hostName,
		Status:        "active",
		Source:        "docker_api",
		HierarchyPath: hostName,
		Attributes: map[string]any{
			"server_version":   info.ServerVersion,
			"operating_system": info.OperatingSystem,
			"ncpu":             info.NCPU,
			"mem_total_bytes":  info.MemTotal,
		},
		FirstSeenAt: now,
		LastSeenAt:  now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	result.Resources = append(result.Resources, dockerHostRes)

	// Maps for relationship resolution
	swarmNodeMap := make(map[string]string)     // NodeID -> ResourceID
	swarmServiceMap := make(map[string]string)  // ServiceID -> ResourceID
	swarmTaskMap := make(map[string]string)     // TaskID -> ResourceID
	containerMap := make(map[string]string)     // ContainerID -> ResourceID
	networkMap := make(map[string]string)       // NetworkID -> ResourceID

	// 2. Swarm Discovery (if cluster active)
	isSwarmActive := info.Swarm.LocalNodeState == "active"
	var swarmClusterID string

	if isSwarmActive {
		clusterName := info.Swarm.Cluster.Spec.Name
		if clusterName == "" {
			clusterName = "swarm-cluster"
		}
		clusterExtID := info.Swarm.Cluster.ID
		if clusterExtID == "" {
			clusterExtID = clusterName
		}
		swarmClusterID = uuid.NewMD5(uuid.NameSpaceDNS, []byte("swarm-cluster/"+clusterExtID)).String()

		clusterRes := &models.Resource{
			ID:            swarmClusterID,
			EnvironmentID: envID,
			Kind:          models.KindSwarmCluster,
			Provider:      "docker_swarm",
			ExternalID:    clusterExtID,
			Name:          clusterName,
			Status:        "active",
			Source:        "docker_swarm_api",
			HierarchyPath: clusterName,
			Attributes: map[string]any{
				"cluster_id":        clusterExtID,
				"control_available": info.Swarm.ControlAvailable,
				"local_node_id":     info.Swarm.NodeID,
			},
			FirstSeenAt: now,
			LastSeenAt:  now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		result.Resources = append(result.Resources, clusterRes)

		// Relationship: DockerHost RELATED_TO SwarmCluster
		result.Relationships = append(result.Relationships, &models.Relationship{
			ID:               uuid.New().String(),
			EnvironmentID:    envID,
			SourceResourceID: dockerHostID,
			TargetResourceID: swarmClusterID,
			RelationshipType: models.RelRelatedTo,
			Confidence:       1.0,
			EvidenceSource:   "docker_swarm_api",
			ValidFrom:        now,
			CreatedAt:        now,
		})

		// Discover Swarm Nodes if manager or control available
		var nodeList []dockerNode
		if err := a.get(ctx, "/nodes", &nodeList); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("fetch swarm nodes: %v", err))
		} else {
			for _, n := range nodeList {
				nodeResID := uuid.NewMD5(uuid.NameSpaceDNS, []byte("swarm-node/"+n.ID)).String()
				swarmNodeMap[n.ID] = nodeResID

				kind := models.KindSwarmWorker
				isLeader := false
				if n.Spec.Role == "manager" {
					kind = models.KindSwarmManager
					if n.ManagerStatus != nil && n.ManagerStatus.Leader {
						isLeader = true
					}
				}

				nodeRes := &models.Resource{
					ID:            nodeResID,
					EnvironmentID: envID,
					Kind:          kind,
					Provider:      "docker_swarm",
					ExternalID:    n.ID,
					Name:          n.Description.Hostname,
					Labels:        n.Spec.Labels,
					Status:        n.Status.State,
					Source:        "docker_swarm_api",
					HierarchyPath: fmt.Sprintf("%s.nodes.%s", clusterName, n.Description.Hostname),
					Attributes: map[string]any{
						"role":         n.Spec.Role,
						"availability": n.Spec.Availability,
						"addr":         n.Status.Addr,
						"leader":       isLeader,
						"os":           n.Description.Platform.OS,
						"arch":         n.Description.Platform.Architecture,
						"nano_cpus":    n.Description.Resources.NanoCPUs,
						"memory_bytes": n.Description.Resources.MemoryBytes,
					},
					FirstSeenAt: now,
					LastSeenAt:  now,
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				result.Resources = append(result.Resources, nodeRes)

				// Relationship: SwarmCluster CONTAINS SwarmNode
				result.Relationships = append(result.Relationships, &models.Relationship{
					ID:               uuid.New().String(),
					EnvironmentID:    envID,
					SourceResourceID: swarmClusterID,
					TargetResourceID: nodeResID,
					RelationshipType: models.RelContains,
					Confidence:       1.0,
					EvidenceSource:   "docker_swarm_api",
					ValidFrom:        now,
					CreatedAt:        now,
				})

				// If node hostname matches the local docker host name, link RUNS_ON
				if n.Description.Hostname == hostName {
					result.Relationships = append(result.Relationships, &models.Relationship{
						ID:               uuid.New().String(),
						EnvironmentID:    envID,
						SourceResourceID: nodeResID,
						TargetResourceID: dockerHostID,
						RelationshipType: models.RelRunsOn,
						Confidence:       1.0,
						EvidenceSource:   "docker_swarm_api",
						ValidFrom:        now,
						CreatedAt:        now,
					})
				}
			}
		}

		// Discover Swarm Services
		var serviceList []dockerService
		if err := a.get(ctx, "/services", &serviceList); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("fetch swarm services: %v", err))
		} else {
			for _, svc := range serviceList {
				svcResID := uuid.NewMD5(uuid.NameSpaceDNS, []byte("swarm-service/"+svc.ID)).String()
				swarmServiceMap[svc.ID] = svcResID

				var replicas int64 = 1
				if svc.Spec.Mode.Replicated != nil && svc.Spec.Mode.Replicated.Replicas != nil {
					replicas = *svc.Spec.Mode.Replicated.Replicas
				}

				svcRes := &models.Resource{
					ID:            svcResID,
					EnvironmentID: envID,
					Kind:          models.KindSwarmService,
					Provider:      "docker_swarm",
					ExternalID:    svc.ID,
					Name:          svc.Spec.Name,
					Labels:        svc.Spec.Labels,
					Status:        "active",
					Source:        "docker_swarm_api",
					HierarchyPath: fmt.Sprintf("%s.services.%s", clusterName, svc.Spec.Name),
					Attributes: map[string]any{
						"image":       svc.Spec.TaskTemplate.ContainerSpec.Image,
						"replicas":    replicas,
						"virtual_ips": svc.Endpoint.VirtualIPs,
						"ports":       svc.Spec.EndpointSpec.Ports,
					},
					FirstSeenAt: now,
					LastSeenAt:  now,
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				result.Resources = append(result.Resources, svcRes)

				// Relationship: SwarmCluster CONTAINS SwarmService
				result.Relationships = append(result.Relationships, &models.Relationship{
					ID:               uuid.New().String(),
					EnvironmentID:    envID,
					SourceResourceID: swarmClusterID,
					TargetResourceID: svcResID,
					RelationshipType: models.RelContains,
					Confidence:       1.0,
					EvidenceSource:   "docker_swarm_api",
					ValidFrom:        now,
					CreatedAt:        now,
				})
			}
		}

		// Discover Swarm Tasks
		var taskList []dockerTask
		if err := a.get(ctx, "/tasks", &taskList); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("fetch swarm tasks: %v", err))
		} else {
			for _, tsk := range taskList {
				taskResID := uuid.NewMD5(uuid.NameSpaceDNS, []byte("swarm-task/"+tsk.ID)).String()
				swarmTaskMap[tsk.ID] = taskResID

				taskName := fmt.Sprintf("task-%s", tsk.ID[:8])
				if svcID, ok := swarmServiceMap[tsk.ServiceID]; ok {
					taskName = fmt.Sprintf("task-%s-slot%d", svcID[:8], tsk.Slot)
				}

				taskRes := &models.Resource{
					ID:            taskResID,
					EnvironmentID: envID,
					Kind:          models.KindSwarmTask,
					Provider:      "docker_swarm",
					ExternalID:    tsk.ID,
					Name:          taskName,
					Status:        tsk.Status.State,
					Source:        "docker_swarm_api",
					HierarchyPath: fmt.Sprintf("%s.tasks.%s", clusterName, tsk.ID),
					Attributes: map[string]any{
						"service_id":   tsk.ServiceID,
						"node_id":      tsk.NodeID,
						"slot":         tsk.Slot,
						"desired_state": tsk.DesiredState,
						"container_id": tsk.Status.ContainerStatus.ContainerID,
					},
					FirstSeenAt: now,
					LastSeenAt:  now,
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				result.Resources = append(result.Resources, taskRes)

				// Relationship: SwarmService CONTAINS SwarmTask
				if svcResID, ok := swarmServiceMap[tsk.ServiceID]; ok {
					result.Relationships = append(result.Relationships, &models.Relationship{
						ID:               uuid.New().String(),
						EnvironmentID:    envID,
						SourceResourceID: svcResID,
						TargetResourceID: taskResID,
						RelationshipType: models.RelContains,
						Confidence:       1.0,
						EvidenceSource:   "docker_swarm_api",
						ValidFrom:        now,
						CreatedAt:        now,
					})
				}

				// Relationship: SwarmTask SCHEDULED_ON SwarmNode
				if nodeResID, ok := swarmNodeMap[tsk.NodeID]; ok {
					result.Relationships = append(result.Relationships, &models.Relationship{
						ID:               uuid.New().String(),
						EnvironmentID:    envID,
						SourceResourceID: taskResID,
						TargetResourceID: nodeResID,
						RelationshipType: models.RelScheduledOn,
						Confidence:       1.0,
						EvidenceSource:   "docker_swarm_api",
						ValidFrom:        now,
						CreatedAt:        now,
					})
				}
			}
		}
	}

	// 3. Discover Docker Networks (/networks)
	var networkList []dockerNetwork
	if err := a.get(ctx, "/networks", &networkList); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("fetch docker networks: %v", err))
	} else {
		for _, net := range networkList {
			netResID := uuid.NewMD5(uuid.NameSpaceDNS, []byte("docker-network/"+net.ID)).String()
			networkMap[net.ID] = netResID
			networkMap[net.Name] = netResID // allow lookup by name as well

			netRes := &models.Resource{
				ID:            netResID,
				EnvironmentID: envID,
				Kind:          models.KindDockerNetwork,
				Provider:      "docker",
				ExternalID:    net.ID,
				Name:          net.Name,
				Labels:        net.Labels,
				Status:        "active",
				Source:        "docker_api",
				HierarchyPath: fmt.Sprintf("%s.networks.%s", hostName, net.Name),
				Attributes: map[string]any{
					"driver":   net.Driver,
					"scope":    net.Scope,
					"internal": net.Internal,
					"ipam":     net.IPAM.Config,
				},
				FirstSeenAt: now,
				LastSeenAt:  now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			result.Resources = append(result.Resources, netRes)

			parentID := dockerHostID
			if net.Scope == "swarm" && swarmClusterID != "" {
				parentID = swarmClusterID
			}

			// Relationship: Host/Cluster CONTAINS Network
			result.Relationships = append(result.Relationships, &models.Relationship{
				ID:               uuid.New().String(),
				EnvironmentID:    envID,
				SourceResourceID: parentID,
				TargetResourceID: netResID,
				RelationshipType: models.RelContains,
				Confidence:       1.0,
				EvidenceSource:   "docker_api",
				ValidFrom:        now,
				CreatedAt:        now,
			})
		}
	}

	// 4. Discover Docker Containers (/containers/json?all=1)
	var containerList []dockerContainer
	if err := a.get(ctx, "/containers/json?all=1", &containerList); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("fetch docker containers: %v", err))
	} else {
		for _, c := range containerList {
			cResID := uuid.NewMD5(uuid.NameSpaceDNS, []byte("docker-container/"+c.ID)).String()
			containerMap[c.ID] = cResID
			if len(c.ID) >= 12 {
				containerMap[c.ID[:12]] = cResID
			}

			cName := c.ID[:12]
			if len(c.Names) > 0 {
				cName = strings.TrimPrefix(c.Names[0], "/")
			}

			cRes := &models.Resource{
				ID:            cResID,
				EnvironmentID: envID,
				Kind:          models.KindDockerContainer,
				Provider:      "docker",
				ExternalID:    c.ID,
				Name:          cName,
				Labels:        c.Labels,
				Status:        c.State,
				Source:        "docker_api",
				HierarchyPath: fmt.Sprintf("%s.containers.%s", hostName, cName),
				Attributes: map[string]any{
					"image":   c.Image,
					"command": c.Command,
					"status":  c.Status,
					"ports":   c.Ports,
					"mounts":  c.Mounts,
					"created": c.Created,
				},
				FirstSeenAt: now,
				LastSeenAt:  now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			result.Resources = append(result.Resources, cRes)

			// Relationship: DockerHost CONTAINS DockerContainer
			result.Relationships = append(result.Relationships, &models.Relationship{
				ID:               uuid.New().String(),
				EnvironmentID:    envID,
				SourceResourceID: dockerHostID,
				TargetResourceID: cResID,
				RelationshipType: models.RelContains,
				Confidence:       1.0,
				EvidenceSource:   "docker_api",
				ValidFrom:        now,
				CreatedAt:        now,
			})

			// Relationship: DockerNetwork CONTAINS DockerContainer
			for netName := range c.NetworkSettings.Networks {
				if netResID, ok := networkMap[netName]; ok {
					result.Relationships = append(result.Relationships, &models.Relationship{
						ID:               uuid.New().String(),
						EnvironmentID:    envID,
						SourceResourceID: netResID,
						TargetResourceID: cResID,
						RelationshipType: models.RelContains,
						Confidence:       1.0,
						EvidenceSource:   "docker_api",
						ValidFrom:        now,
						CreatedAt:        now,
					})
				}
			}

			// If container was spawned by a Swarm task:
			// Link: SwarmTask RUNS_ON DockerContainer
			if tskID, ok := c.Labels["com.docker.swarm.task.id"]; ok {
				if taskResID, found := swarmTaskMap[tskID]; found {
					result.Relationships = append(result.Relationships, &models.Relationship{
						ID:               uuid.New().String(),
						EnvironmentID:    envID,
						SourceResourceID: taskResID,
						TargetResourceID: cResID,
						RelationshipType: models.RelRunsOn,
						Confidence:       1.0,
						EvidenceSource:   "docker_swarm_labels",
						ValidFrom:        now,
						CreatedAt:        now,
					})
				}
			}

			// Link: SwarmService EXPOSES DockerContainer
			if svcID, ok := c.Labels["com.docker.swarm.service.id"]; ok {
				if svcResID, found := swarmServiceMap[svcID]; found {
					result.Relationships = append(result.Relationships, &models.Relationship{
						ID:               uuid.New().String(),
						EnvironmentID:    envID,
						SourceResourceID: svcResID,
						TargetResourceID: cResID,
						RelationshipType: models.RelExposes,
						Confidence:       1.0,
						EvidenceSource:   "docker_swarm_labels",
						ValidFrom:        now,
						CreatedAt:        now,
					})
				}
			}
		}
	}

	// 5. Discover Docker Volumes (/volumes)
	var volumeList dockerVolumeList
	if err := a.get(ctx, "/volumes", &volumeList); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("fetch docker volumes: %v", err))
	} else {
		for _, v := range volumeList.Volumes {
			vResID := uuid.NewMD5(uuid.NameSpaceDNS, []byte("docker-volume/"+v.Name)).String()

			vRes := &models.Resource{
				ID:            vResID,
				EnvironmentID: envID,
				Kind:          models.KindDockerVolume,
				Provider:      "docker",
				ExternalID:    v.Name,
				Name:          v.Name,
				Labels:        v.Labels,
				Status:        "active",
				Source:        "docker_api",
				HierarchyPath: fmt.Sprintf("%s.volumes.%s", hostName, v.Name),
				Attributes: map[string]any{
					"driver":     v.Driver,
					"mountpoint": v.Mountpoint,
					"scope":      v.Scope,
				},
				FirstSeenAt: now,
				LastSeenAt:  now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			result.Resources = append(result.Resources, vRes)

			// Relationship: DockerHost CONTAINS DockerVolume
			result.Relationships = append(result.Relationships, &models.Relationship{
				ID:               uuid.New().String(),
				EnvironmentID:    envID,
				SourceResourceID: dockerHostID,
				TargetResourceID: vResID,
				RelationshipType: models.RelContains,
				Confidence:       1.0,
				EvidenceSource:   "docker_api",
				ValidFrom:        now,
				CreatedAt:        now,
			})
		}
	}

	a.logger.Info("docker and swarm discovery scan completed",
		slog.String("docker_host", hostName),
		slog.Int("resources", len(result.Resources)),
		slog.Int("relationships", len(result.Relationships)),
		slog.Int("errors", len(result.Errors)),
	)

	return result, nil
}

func (a *Adapter) get(ctx context.Context, path string, target any) error {
	url := a.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d from %s: %s", resp.StatusCode, path, strings.TrimSpace(string(body)))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}
