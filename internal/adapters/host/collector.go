package host

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/onisimchukv/uptimefoundry/internal/adapters"
	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

// Adapter implements adapters.DiscoveryAdapter for Linux / bare-metal hosts.
type Adapter struct {
	logger *slog.Logger
}

var _ adapters.DiscoveryAdapter = (*Adapter)(nil)

// New creates a new Host discovery adapter.
func New(logger *slog.Logger) *Adapter {
	return &Adapter{logger: logger}
}

func (a *Adapter) Name() string {
	return "host"
}

// Discover gathers host identity, hardware topology, and network interfaces.
func (a *Adapter) Discover(ctx context.Context, envID string) (*adapters.DiscoveryResult, error) {
	result := &adapters.DiscoveryResult{
		AdapterName: a.Name(),
		Metadata:    make(map[string]any),
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
		result.Errors = append(result.Errors, fmt.Sprintf("failed to get hostname: %v", err))
	}

	hostID := uuid.NewMD5(uuid.NameSpaceDNS, []byte(hostname)).String()
	now := time.Now().UTC()

	// 1. Gather Host OS and hardware attributes
	hostAttrs := map[string]any{
		"hostname":     hostname,
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"num_cpu":      runtime.NumCPU(),
		"discovered_by": "uptimefoundry-host-adapter",
	}

	// Enrich with platform-specific attributes (e.g. /etc/os-release on Linux)
	enrichHostAttributes(hostAttrs)

	hostRes := &models.Resource{
		ID:            hostID,
		EnvironmentID: envID,
		Kind:          models.KindHost,
		Provider:      "linux_host",
		ExternalID:    hostname,
		Name:          hostname,
		Attributes:    hostAttrs,
		Status:        "online",
		Source:        "host_adapter",
		HierarchyPath: fmt.Sprintf("%s.hosts.%s", envID, strings.ReplaceAll(hostname, ".", "_")),
		FirstSeenAt:   now,
		LastSeenAt:    now,
	}
	result.Resources = append(result.Resources, hostRes)

	// 2. Discover Network Interfaces
	ifaces, err := net.Interfaces()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list network interfaces: %v", err))
	} else {
		for _, iface := range ifaces {
			// Skip down or loopback interfaces
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}

			addrs, _ := iface.Addrs()
			var ipList []string
			for _, addr := range addrs {
				ipList = append(ipList, addr.String())
			}

			ifaceID := uuid.NewMD5(uuid.NameSpaceDNS, []byte(hostname+":"+iface.Name)).String()
			ifaceRes := &models.Resource{
				ID:            ifaceID,
				EnvironmentID: envID,
				Kind:          models.KindNetworkInterface,
				Provider:      "linux_host",
				ExternalID:    fmt.Sprintf("%s/%s", hostname, iface.Name),
				Name:          iface.Name,
				Attributes: map[string]any{
					"mac_address": iface.HardwareAddr.String(),
					"mtu":         iface.MTU,
					"ips":         ipList,
					"flags":       iface.Flags.String(),
				},
				Status:        "active",
				Source:        "host_adapter",
				HierarchyPath: fmt.Sprintf("%s.hosts.%s.net.%s", envID, strings.ReplaceAll(hostname, ".", "_"), iface.Name),
				FirstSeenAt:   now,
				LastSeenAt:    now,
			}
			result.Resources = append(result.Resources, ifaceRes)

			// Relationship: Host CONTAINS NetworkInterface
			rel := &models.Relationship{
				ID:               uuid.NewMD5(uuid.NameSpaceDNS, []byte(hostID+":"+ifaceID)).String(),
				EnvironmentID:    envID,
				SourceResourceID: hostID,
				TargetResourceID: ifaceID,
				RelationshipType: models.RelContains,
				Confidence:       1.0,
				EvidenceSource:   "host_net_sysfs",
				ValidFrom:        now,
				Weight:           1.0,
			}
			result.Relationships = append(result.Relationships, rel)
		}
	}

	result.Metadata["discovered_resources"] = len(result.Resources)
	result.Metadata["discovered_relationships"] = len(result.Relationships)

	a.logger.Info("host discovery completed",
		slog.String("host", hostname),
		slog.Int("resources", len(result.Resources)),
		slog.Int("relationships", len(result.Relationships)),
		slog.Int("errors", len(result.Errors)),
	)

	return result, nil
}
