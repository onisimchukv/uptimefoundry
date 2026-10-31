package fallback

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/graph"
)

// Engine implements graph.GraphEngine via PostgreSQL recursive CTEs.
type Engine struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

var _ graph.GraphEngine = (*Engine)(nil)

// New creates a new fallback graph engine.
func New(pool *pgxpool.Pool, logger *slog.Logger) *Engine {
	return &Engine{
		pool:   pool,
		logger: logger,
	}
}

func (e *Engine) Name() string {
	return "recursive_cte"
}

// UpsertNode is a no-op because relational resources table already stores vertices.
func (e *Engine) UpsertNode(ctx context.Context, res *models.Resource) error {
	return nil
}

// UpsertEdge is a no-op because relational relationships table already stores edges.
func (e *Engine) UpsertEdge(ctx context.Context, rel *models.Relationship) error {
	return nil
}

// GetSubgraph performs recursive Common Table Expression traversal out to N hops.
func (e *Engine) GetSubgraph(ctx context.Context, rootID string, hops int) ([]*models.Resource, []*models.Relationship, error) {
	if hops <= 0 {
		hops = 1
	}
	if hops > 5 {
		hops = 5
	}

	cteQuery := `
		WITH RECURSIVE traverse AS (
			SELECT source_resource_id, target_resource_id, 1 as depth
			FROM relationships
			WHERE (source_resource_id = $1 OR target_resource_id = $1) AND valid_to IS NULL
			UNION
			SELECT r.source_resource_id, r.target_resource_id, t.depth + 1
			FROM relationships r
			JOIN traverse t ON (
				r.source_resource_id = t.target_resource_id OR 
				r.source_resource_id = t.source_resource_id OR
				r.target_resource_id = t.source_resource_id OR
				r.target_resource_id = t.target_resource_id
			)
			WHERE t.depth < $2 AND r.valid_to IS NULL
		)
		SELECT DISTINCT id FROM (
			SELECT $1 as id
			UNION
			SELECT source_resource_id as id FROM traverse
			UNION
			SELECT target_resource_id as id FROM traverse
		) s;
	`

	rows, err := e.pool.Query(ctx, cteQuery, rootID, hops)
	if err != nil {
		return nil, nil, fmt.Errorf("recursive cte query: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil && id != "" {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		ids = append(ids, rootID)
	}

	// Retrieve full resource and relationship structs
	return e.fetchEntities(ctx, ids)
}

func (e *Engine) fetchEntities(ctx context.Context, resourceIDs []string) ([]*models.Resource, []*models.Relationship, error) {
	if len(resourceIDs) == 0 {
		return nil, nil, nil
	}

	// 1. Fetch Resources
	queryRes := `
		SELECT id, environment_id, kind, provider, external_id, name, coalesce(namespace, ''),
		       labels, attributes, status, source, coalesce(hierarchy_path::text, ''),
		       first_seen_at, last_seen_at, created_at, updated_at
		FROM resources
		WHERE id = ANY($1)
	`
	rows, err := e.pool.Query(ctx, queryRes, resourceIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch resources for subgraph: %w", err)
	}
	defer rows.Close()

	var resources []*models.Resource
	for rows.Next() {
		var res models.Resource
		var hierarchyStr string
		err := rows.Scan(
			&res.ID, &res.EnvironmentID, &res.Kind, &res.Provider, &res.ExternalID,
			&res.Name, &res.Namespace, &res.Labels, &res.Attributes, &res.Status,
			&res.Source, &hierarchyStr, &res.FirstSeenAt, &res.LastSeenAt,
			&res.CreatedAt, &res.UpdatedAt,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("scan resource row: %w", err)
		}
		res.HierarchyPath = hierarchyStr
		resources = append(resources, &res)
	}

	// 2. Fetch Interconnecting Relationships
	queryRel := `
		SELECT id, environment_id, source_resource_id, target_resource_id, relationship_type,
		       confidence, evidence_source, valid_from, valid_to, weight, attributes, created_at
		FROM relationships
		WHERE source_resource_id = ANY($1) AND target_resource_id = ANY($1) AND valid_to IS NULL
	`
	relRows, err := e.pool.Query(ctx, queryRel, resourceIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch relationships for subgraph: %w", err)
	}
	defer relRows.Close()

	var relationships []*models.Relationship
	for relRows.Next() {
		var rel models.Relationship
		err := relRows.Scan(
			&rel.ID, &rel.EnvironmentID, &rel.SourceResourceID, &rel.TargetResourceID,
			&rel.RelationshipType, &rel.Confidence, &rel.EvidenceSource,
			&rel.ValidFrom, &rel.ValidTo, &rel.Weight, &rel.Attributes, &rel.CreatedAt,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("scan relationship row: %w", err)
		}
		relationships = append(relationships, &rel)
	}

	return resources, relationships, nil
}

// QueryCypher returns an error indicating Apache AGE is required for openCypher.
func (e *Engine) QueryCypher(ctx context.Context, cypher string) ([]map[string]any, error) {
	return nil, errors.New("openCypher execution requires PostgreSQL with Apache AGE extension enabled; current engine is recursive_cte fallback")
}
