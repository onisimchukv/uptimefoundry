package age

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forgeops/forgeops/internal/core/models"
	"github.com/forgeops/forgeops/internal/graph"
)

// Engine implements graph.GraphEngine via Apache AGE on PostgreSQL.
type Engine struct {
	pool      *pgxpool.Pool
	graphName string
	logger    *slog.Logger
}

var _ graph.GraphEngine = (*Engine)(nil)

// New creates a new Apache AGE graph engine.
func New(pool *pgxpool.Pool, graphName string, logger *slog.Logger) *Engine {
	if graphName == "" {
		graphName = "forgeops_graph"
	}
	return &Engine{
		pool:      pool,
		graphName: graphName,
		logger:    logger,
	}
}

func (e *Engine) Name() string {
	return "apache_age"
}

// sanitizeCypherString escapes single quotes and backslashes for safe embedding in openCypher string literals.
func sanitizeCypherString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "'", "\\'")
}

// sanitizeCypherIdentifier ensures label/relation types contain only alphanumeric characters and underscores.
func sanitizeCypherIdentifier(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	res := sb.String()
	if len(res) == 0 {
		return "Unknown"
	}
	return res
}

// UpsertNode creates or updates a vertex in the Apache AGE graph.
func (e *Engine) UpsertNode(ctx context.Context, res *models.Resource) error {
	label := sanitizeCypherIdentifier(string(res.Kind))
	id := sanitizeCypherString(res.ID)
	name := sanitizeCypherString(res.Name)
	env := sanitizeCypherString(res.EnvironmentID)
	status := sanitizeCypherString(res.Status)

	cypher := fmt.Sprintf(`
		MERGE (n:%s {id: '%s'})
		SET n.name = '%s', n.env = '%s', n.status = '%s', n.kind = '%s'
		RETURN n
	`, label, id, name, env, status, label)

	query := fmt.Sprintf(`
		SELECT * FROM ag_catalog.cypher('%s', $$
			%s
		$$) as (v ag_catalog.agtype);
	`, e.graphName, cypher)

	_, err := e.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("age upsert node (%s:%s): %w", label, res.ID, err)
	}

	return nil
}

// UpsertEdge creates or updates a directional edge in the Apache AGE graph.
func (e *Engine) UpsertEdge(ctx context.Context, rel *models.Relationship) error {
	relType := sanitizeCypherIdentifier(string(rel.RelationshipType))
	srcID := sanitizeCypherString(rel.SourceResourceID)
	targetID := sanitizeCypherString(rel.TargetResourceID)
	evidence := sanitizeCypherString(rel.EvidenceSource)

	cypher := fmt.Sprintf(`
		MATCH (s {id: '%s'}), (t {id: '%s'})
		MERGE (s)-[r:%s]->(t)
		SET r.confidence = %f, r.weight = %f, r.evidence_source = '%s'
		RETURN r
	`, srcID, targetID, relType, rel.Confidence, rel.Weight, evidence)

	query := fmt.Sprintf(`
		SELECT * FROM ag_catalog.cypher('%s', $$
			%s
		$$) as (e ag_catalog.agtype);
	`, e.graphName, cypher)

	_, err := e.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("age upsert edge (%s -[%s]-> %s): %w", rel.SourceResourceID, relType, rel.TargetResourceID, err)
	}

	return nil
}

// GetSubgraph queries all neighbor nodes within N hops using openCypher.
func (e *Engine) GetSubgraph(ctx context.Context, rootID string, hops int) ([]*models.Resource, []*models.Relationship, error) {
	if hops <= 0 {
		hops = 1
	}
	if hops > 5 {
		hops = 5 // Bound query depth to prevent unbounded graph traversal
	}

	cleanRootID := sanitizeCypherString(rootID)

	cypher := fmt.Sprintf(`
		MATCH (root {id: '%s'})-[r*1..%d]-(neighbor)
		RETURN DISTINCT neighbor.id
	`, cleanRootID, hops)

	query := fmt.Sprintf(`
		SELECT * FROM ag_catalog.cypher('%s', $$
			%s
		$$) as (id ag_catalog.agtype);
	`, e.graphName, cypher)

	rows, err := e.pool.Query(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("age query subgraph for %s: %w", rootID, err)
	}
	defer rows.Close()

	var neighborIDs []string
	neighborIDs = append(neighborIDs, rootID)

	for rows.Next() {
		var rawID any
		if err := rows.Scan(&rawID); err == nil {
			idStr := fmt.Sprintf("%v", rawID)
			// Apache AGE agtype strings can be quoted like "res-123"
			idStr = strings.Trim(idStr, "\"")
			if idStr != "" && idStr != rootID {
				neighborIDs = append(neighborIDs, idStr)
			}
		}
	}

	// Query full relational details for these discovered node IDs
	return e.fetchEntitiesFromRelational(ctx, neighborIDs)
}

// fetchEntitiesFromRelational retrieves full relational Resource and Relationship structs given resource IDs.
func (e *Engine) fetchEntitiesFromRelational(ctx context.Context, resourceIDs []string) ([]*models.Resource, []*models.Relationship, error) {
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

// QueryCypher runs arbitrary read-only openCypher queries through Apache AGE.
func (e *Engine) QueryCypher(ctx context.Context, cypher string) ([]map[string]any, error) {
	query := fmt.Sprintf(`
		SELECT * FROM ag_catalog.cypher('%s', $$
			%s
		$$) as (result ag_catalog.agtype);
	`, e.graphName, cypher)

	rows, err := e.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("execute cypher: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var raw any
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan cypher row: %w", err)
		}
		results = append(results, map[string]any{"result": raw})
	}

	return results, nil
}
