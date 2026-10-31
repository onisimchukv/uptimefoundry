package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/core/ports"
)

// DiscoveryRepository implements ports.DiscoveryRepository on PostgreSQL.
type DiscoveryRepository struct {
	db *DB
}

var _ ports.DiscoveryRepository = (*DiscoveryRepository)(nil)

// NewDiscoveryRepository creates a new DiscoveryRepository.
func NewDiscoveryRepository(db *DB) *DiscoveryRepository {
	return &DiscoveryRepository{db: db}
}

// CreateRun initializes a new discovery run record.
func (d *DiscoveryRepository) CreateRun(ctx context.Context, run *models.DiscoveryRun) error {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if run.Status == "" {
		run.Status = models.DiscoveryStatusRunning
	}

	errorsJSON, err := json.Marshal(run.Errors)
	if err != nil {
		errorsJSON = []byte("[]")
	}

	metaJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	query := `
		INSERT INTO discovery_runs (
			id, environment_id, adapter_type, started_at, status,
			resources_discovered, relationships_discovered, errors, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
		RETURNING id, started_at;
	`

	return d.db.Pool.QueryRow(ctx, query,
		run.ID, run.EnvironmentID, run.AdapterType, run.StartedAt, run.Status,
		run.ResourcesDiscovered, run.RelationshipsDiscovered, errorsJSON, metaJSON,
	).Scan(&run.ID, &run.StartedAt)
}

// UpdateRun records completion state, counts, and errors for a discovery run.
func (d *DiscoveryRepository) UpdateRun(ctx context.Context, run *models.DiscoveryRun) error {
	errorsJSON, err := json.Marshal(run.Errors)
	if err != nil {
		errorsJSON = []byte("[]")
	}

	metaJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	query := `
		UPDATE discovery_runs SET
			completed_at = $2,
			status = $3,
			resources_discovered = $4,
			relationships_discovered = $5,
			errors = $6,
			metadata = $7
		WHERE id = $1;
	`

	_, err = d.db.Pool.Exec(ctx, query,
		run.ID, run.CompletedAt, run.Status,
		run.ResourcesDiscovered, run.RelationshipsDiscovered,
		errorsJSON, metaJSON,
	)
	if err != nil {
		return fmt.Errorf("update discovery run %s: %w", run.ID, err)
	}

	return nil
}

// GetRunByID retrieves details of a specific discovery run.
func (d *DiscoveryRepository) GetRunByID(ctx context.Context, id string) (*models.DiscoveryRun, error) {
	query := `
		SELECT id, environment_id, adapter_type, started_at, completed_at, status,
		       resources_discovered, relationships_discovered, errors, metadata
		FROM discovery_runs
		WHERE id = $1;
	`

	var run models.DiscoveryRun
	err := d.db.Pool.QueryRow(ctx, query, id).Scan(
		&run.ID, &run.EnvironmentID, &run.AdapterType, &run.StartedAt, &run.CompletedAt,
		&run.Status, &run.ResourcesDiscovered, &run.RelationshipsDiscovered,
		&run.Errors, &run.Metadata,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("discovery run %s not found", id)
		}
		return nil, fmt.Errorf("query discovery run %s: %w", id, err)
	}

	return &run, nil
}

// ListRuns lists past discovery runs with pagination.
func (d *DiscoveryRepository) ListRuns(ctx context.Context, envID string, limit, offset int) ([]*models.DiscoveryRun, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	query := `
		SELECT id, environment_id, adapter_type, started_at, completed_at, status,
		       resources_discovered, relationships_discovered, errors, metadata
		FROM discovery_runs
		WHERE ($1 = '' OR environment_id = $1::uuid)
		ORDER BY started_at DESC
		LIMIT $2 OFFSET $3;
	`

	rows, err := d.db.Pool.Query(ctx, query, envID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list discovery runs: %w", err)
	}
	defer rows.Close()

	var runs []*models.DiscoveryRun
	for rows.Next() {
		var run models.DiscoveryRun
		err := rows.Scan(
			&run.ID, &run.EnvironmentID, &run.AdapterType, &run.StartedAt, &run.CompletedAt,
			&run.Status, &run.ResourcesDiscovered, &run.RelationshipsDiscovered,
			&run.Errors, &run.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("scan discovery run: %w", err)
		}
		runs = append(runs, &run)
	}

	return runs, nil
}
