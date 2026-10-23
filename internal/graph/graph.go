package graph

import (
	"context"

	"github.com/forgeops/forgeops/internal/core/models"
)

// GraphEngine abstracts graph topology storage and traversal operations.
type GraphEngine interface {
	// UpsertNode ensures a resource exists as a vertex in the property graph.
	UpsertNode(ctx context.Context, res *models.Resource) error

	// UpsertEdge ensures a relationship exists as a directional edge in the property graph.
	UpsertEdge(ctx context.Context, rel *models.Relationship) error

	// GetSubgraph traverses out to N hops from rootID and returns reachable resources and relationships.
	GetSubgraph(ctx context.Context, rootID string, hops int) ([]*models.Resource, []*models.Relationship, error)

	// QueryCypher executes an openCypher query directly against the graph.
	QueryCypher(ctx context.Context, cypher string) ([]map[string]any, error)

	// Name returns the identifier of the engine (e.g. "apache_age", "recursive_cte").
	Name() string
}
