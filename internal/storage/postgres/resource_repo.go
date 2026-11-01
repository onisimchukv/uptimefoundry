// SPDX-License-Identifier: MIT

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/core/ports"
	"github.com/onisimchukv/uptimefoundry/internal/graph"
)

// ResourceRepository implements ports.ResourceRepository on PostgreSQL.
type ResourceRepository struct {
	db          *DB
	graphEngine graph.GraphEngine
	logger      *slog.Logger
}

var _ ports.ResourceRepository = (*ResourceRepository)(nil)

// NewResourceRepository creates a new ResourceRepository backed by PostgreSQL and a GraphEngine.
func NewResourceRepository(db *DB, graphEngine graph.GraphEngine, logger *slog.Logger) *ResourceRepository {
	return &ResourceRepository{
		db:          db,
		graphEngine: graphEngine,
		logger:      logger,
	}
}

// SanitizeLtreePath sanitizes a hierarchical string into valid PostgreSQL ltree dot-separated labels.
// Each label must contain only alphanumeric characters and underscores [A-Za-z0-9_].
func SanitizeLtreePath(path string) string {
	if path == "" {
		return ""
	}

	parts := strings.Split(path, ".")
	var cleanParts []string

	for _, part := range parts {
		var sb strings.Builder
		for _, r := range part {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				sb.WriteRune(r)
			} else {
				sb.WriteRune('_')
			}
		}
		cleanLabel := sb.String()
		cleanLabel = strings.Trim(cleanLabel, "_")
		if cleanLabel != "" {
			cleanParts = append(cleanParts, cleanLabel)
		}
	}

	if len(cleanParts) == 0 {
		return ""
	}
	return strings.Join(cleanParts, ".")
}

// UpsertResource inserts or updates an infrastructure asset.
func (r *ResourceRepository) UpsertResource(ctx context.Context, res *models.Resource) error {
	if res.ID == "" {
		res.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if res.FirstSeenAt.IsZero() {
		res.FirstSeenAt = now
	}
	res.LastSeenAt = now
	res.UpdatedAt = now
	if res.CreatedAt.IsZero() {
		res.CreatedAt = now
	}

	labelsJSON, err := json.Marshal(res.Labels)
	if err != nil {
		labelsJSON = []byte("{}")
	}

	attributesJSON, err := json.Marshal(res.Attributes)
	if err != nil {
		attributesJSON = []byte("{}")
	}

	sanitizedPath := SanitizeLtreePath(res.HierarchyPath)
	res.HierarchyPath = sanitizedPath

	query := `
		INSERT INTO resources (
			id, environment_id, kind, provider, external_id, name, namespace,
			labels, attributes, status, source, hierarchy_path,
			first_seen_at, last_seen_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 
			CASE WHEN $12 = '' THEN NULL ELSE $12::ltree END,
			$13, $14, $15, $16
		)
		ON CONFLICT (environment_id, kind, provider, external_id) DO UPDATE SET
			name = EXCLUDED.name,
			namespace = EXCLUDED.namespace,
			labels = EXCLUDED.labels,
			attributes = EXCLUDED.attributes,
			status = EXCLUDED.status,
			source = EXCLUDED.source,
			hierarchy_path = EXCLUDED.hierarchy_path,
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at = EXCLUDED.updated_at
		RETURNING id, first_seen_at, created_at, updated_at;
	`

	err = r.db.Pool.QueryRow(ctx, query,
		res.ID, res.EnvironmentID, res.Kind, res.Provider, res.ExternalID,
		res.Name, res.Namespace, labelsJSON, attributesJSON, res.Status, res.Source,
		sanitizedPath, res.FirstSeenAt, res.LastSeenAt, res.CreatedAt, res.UpdatedAt,
	).Scan(&res.ID, &res.FirstSeenAt, &res.CreatedAt, &res.UpdatedAt)

	if err != nil {
		return fmt.Errorf("upsert resource in postgres: %w", err)
	}

	// Synchronize vertex to property graph
	if r.graphEngine != nil {
		if err := r.graphEngine.UpsertNode(ctx, res); err != nil {
			r.logger.Warn("failed to sync vertex to property graph",
				slog.String("resource_id", res.ID),
				slog.String("kind", string(res.Kind)),
				slog.Any("error", err),
			)
			// Non-blocking: Relational write is authoritative
		}
	}

	return nil
}

// GetResourceByID retrieves a single resource by its stable UUID.
func (r *ResourceRepository) GetResourceByID(ctx context.Context, id string) (*models.Resource, error) {
	query := `
		SELECT id, environment_id, kind, provider, external_id, name, coalesce(namespace, ''),
		       labels, attributes, status, source, coalesce(hierarchy_path::text, ''),
		       first_seen_at, last_seen_at, stale_since, created_at, updated_at
		FROM resources
		WHERE id = $1;
	`

	var res models.Resource
	var hierarchyStr string
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&res.ID, &res.EnvironmentID, &res.Kind, &res.Provider, &res.ExternalID,
		&res.Name, &res.Namespace, &res.Labels, &res.Attributes, &res.Status,
		&res.Source, &hierarchyStr, &res.FirstSeenAt, &res.LastSeenAt, &res.StaleSince,
		&res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("resource %s not found", id)
		}
		return nil, fmt.Errorf("query resource %s: %w", id, err)
	}
	res.HierarchyPath = hierarchyStr

	return &res, nil
}

// ListResources lists resources with optional filtering and pagination.
func (r *ResourceRepository) ListResources(ctx context.Context, envID string, kind models.ResourceKind, limit, offset int) ([]*models.Resource, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	baseQuery := `
		SELECT id, environment_id, kind, provider, external_id, name, coalesce(namespace, ''),
		       labels, attributes, status, source, coalesce(hierarchy_path::text, ''),
		       first_seen_at, last_seen_at, stale_since, created_at, updated_at
		FROM resources
		WHERE ($1 = '' OR environment_id = $1::uuid)
		  AND ($2 = '' OR kind = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4;
	`

	rows, err := r.db.Pool.Query(ctx, baseQuery, envID, string(kind), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()

	var results []*models.Resource
	for rows.Next() {
		var res models.Resource
		var hierarchyStr string
		err := rows.Scan(
			&res.ID, &res.EnvironmentID, &res.Kind, &res.Provider, &res.ExternalID,
			&res.Name, &res.Namespace, &res.Labels, &res.Attributes, &res.Status,
			&res.Source, &hierarchyStr, &res.FirstSeenAt, &res.LastSeenAt, &res.StaleSince,
			&res.CreatedAt, &res.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan resource row: %w", err)
		}
		res.HierarchyPath = hierarchyStr
		results = append(results, &res)
	}

	return results, nil
}

// UpsertRelationship creates or updates a directional topology edge.
func (r *ResourceRepository) UpsertRelationship(ctx context.Context, rel *models.Relationship) error {
	if rel.ID == "" {
		rel.ID = uuid.NewString()
	}
	if rel.ValidFrom.IsZero() {
		rel.ValidFrom = time.Now().UTC()
	}
	if rel.CreatedAt.IsZero() {
		rel.CreatedAt = time.Now().UTC()
	}

	attributesJSON, err := json.Marshal(rel.Attributes)
	if err != nil {
		attributesJSON = []byte("{}")
	}

	query := `
		INSERT INTO relationships (
			id, environment_id, source_resource_id, target_resource_id, relationship_type,
			confidence, evidence_source, valid_from, valid_to, weight, attributes, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		RETURNING id, created_at;
	`

	err = r.db.Pool.QueryRow(ctx, query,
		rel.ID, rel.EnvironmentID, rel.SourceResourceID, rel.TargetResourceID,
		rel.RelationshipType, rel.Confidence, rel.EvidenceSource,
		rel.ValidFrom, rel.ValidTo, rel.Weight, attributesJSON, rel.CreatedAt,
	).Scan(&rel.ID, &rel.CreatedAt)

	if err != nil {
		return fmt.Errorf("upsert relationship in postgres: %w", err)
	}

	// Synchronize edge to property graph
	if r.graphEngine != nil {
		if err := r.graphEngine.UpsertEdge(ctx, rel); err != nil {
			r.logger.Warn("failed to sync edge to property graph",
				slog.String("source", rel.SourceResourceID),
				slog.String("target", rel.TargetResourceID),
				slog.String("type", string(rel.RelationshipType)),
				slog.Any("error", err),
			)
		}
	}

	return nil
}

// GetRelationships retrieves active relationships directly touching a resource.
func (r *ResourceRepository) GetRelationships(ctx context.Context, resourceID string) ([]*models.Relationship, error) {
	query := `
		SELECT id, environment_id, source_resource_id, target_resource_id, relationship_type,
		       confidence, evidence_source, valid_from, valid_to, weight, attributes, created_at
		FROM relationships
		WHERE (source_resource_id = $1 OR target_resource_id = $1)
		  AND valid_to IS NULL
		ORDER BY created_at DESC;
	`

	rows, err := r.db.Pool.Query(ctx, query, resourceID)
	if err != nil {
		return nil, fmt.Errorf("get relationships for %s: %w", resourceID, err)
	}
	defer rows.Close()

	var rels []*models.Relationship
	for rows.Next() {
		var rel models.Relationship
		err := rows.Scan(
			&rel.ID, &rel.EnvironmentID, &rel.SourceResourceID, &rel.TargetResourceID,
			&rel.RelationshipType, &rel.Confidence, &rel.EvidenceSource,
			&rel.ValidFrom, &rel.ValidTo, &rel.Weight, &rel.Attributes, &rel.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan relationship: %w", err)
		}
		rels = append(rels, &rel)
	}

	return rels, nil
}

// GetTopologySubgraph queries the multi-hop neighborhood using the configured GraphEngine.
func (r *ResourceRepository) GetTopologySubgraph(ctx context.Context, rootResourceID string, hops int) ([]*models.Resource, []*models.Relationship, error) {
	if r.graphEngine == nil {
		return nil, nil, errors.New("no graph engine configured")
	}
	return r.graphEngine.GetSubgraph(ctx, rootResourceID, hops)
}

// ExecuteCypher runs an openCypher statement against the graph engine.
func (r *ResourceRepository) ExecuteCypher(ctx context.Context, query string) (any, error) {
	if r.graphEngine == nil {
		return nil, errors.New("no graph engine configured")
	}
	return r.graphEngine.QueryCypher(ctx, query)
}
