package adapters

import (
	"context"

	"github.com/forgeops/forgeops/internal/core/models"
)

// DiscoveryResult contains the resources and relationships collected during a single adapter scan.
type DiscoveryResult struct {
	AdapterName   string
	Resources     []*models.Resource
	Relationships []*models.Relationship
	Errors        []string
	Metadata      map[string]any
}

// DiscoveryAdapter defines the contract for infrastructure discovery providers (Host, K8s, Swarm).
type DiscoveryAdapter interface {
	Name() string
	Discover(ctx context.Context, envID string) (*DiscoveryResult, error)
}
