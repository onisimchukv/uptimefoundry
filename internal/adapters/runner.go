package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/forgeops/forgeops/internal/core/models"
	"github.com/forgeops/forgeops/internal/core/ports"
)

// Runner orchestrates discovery adapters and persists discovered infrastructure to storage.
type Runner struct {
	adapters      []DiscoveryAdapter
	resourceRepo  ports.ResourceRepository
	discoveryRepo ports.DiscoveryRepository
	logger        *slog.Logger
}

// NewRunner creates a new Discovery Runner.
func NewRunner(
	adapters []DiscoveryAdapter,
	resourceRepo ports.ResourceRepository,
	discoveryRepo ports.DiscoveryRepository,
	logger *slog.Logger,
) *Runner {
	return &Runner{
		adapters:      adapters,
		resourceRepo:  resourceRepo,
		discoveryRepo: discoveryRepo,
		logger:        logger,
	}
}

// RunAll executes all registered discovery adapters for the given environment.
func (r *Runner) RunAll(ctx context.Context, envID string) (*models.DiscoveryRun, error) {
	if envID == "" {
		envID = "00000000-0000-0000-0000-000000000001" // Default environment UUID if none specified
	}

	run := &models.DiscoveryRun{
		ID:            uuid.NewString(),
		EnvironmentID: envID,
		AdapterType:   "all",
		StartedAt:     time.Now().UTC(),
		Status:        models.DiscoveryStatusRunning,
		Metadata:      make(map[string]any),
	}

	if r.discoveryRepo != nil {
		if err := r.discoveryRepo.CreateRun(ctx, run); err != nil {
			r.logger.Warn("failed to record discovery run start", slog.Any("error", err))
		}
	}

	r.logger.Info("commencing discovery run",
		slog.String("run_id", run.ID),
		slog.String("env_id", envID),
		slog.Int("adapters_count", len(r.adapters)),
	)

	totalResources := 0
	totalRelationships := 0
	var collectedErrors []string

	for _, adapter := range r.adapters {
		r.logger.Debug("running discovery adapter", slog.String("adapter", adapter.Name()))

		res, err := adapter.Discover(ctx, envID)
		if err != nil {
			errStr := fmt.Sprintf("adapter %s critical error: %v", adapter.Name(), err)
			collectedErrors = append(collectedErrors, errStr)
			r.logger.Error("adapter failure", slog.String("adapter", adapter.Name()), slog.Any("error", err))
			continue
		}

		if len(res.Errors) > 0 {
			collectedErrors = append(collectedErrors, res.Errors...)
		}

		// Persist Discovered Resources
		for _, resource := range res.Resources {
			if r.resourceRepo != nil {
				if err := r.resourceRepo.UpsertResource(ctx, resource); err != nil {
					collectedErrors = append(collectedErrors, fmt.Sprintf("upsert resource %s: %v", resource.Name, err))
					continue
				}
			}
			totalResources++
		}

		// Persist Discovered Relationships
		for _, rel := range res.Relationships {
			if r.resourceRepo != nil {
				if err := r.resourceRepo.UpsertRelationship(ctx, rel); err != nil {
					collectedErrors = append(collectedErrors, fmt.Sprintf("upsert relationship %s: %v", rel.ID, err))
					continue
				}
			}
			totalRelationships++
		}
	}

	completedAt := time.Now().UTC()
	run.CompletedAt = &completedAt
	run.ResourcesDiscovered = totalResources
	run.RelationshipsDiscovered = totalRelationships
	run.Errors = collectedErrors

	if len(collectedErrors) > 0 && totalResources == 0 {
		run.Status = models.DiscoveryStatusFailed
	} else {
		run.Status = models.DiscoveryStatusCompleted
	}

	run.Metadata["duration_ms"] = completedAt.Sub(run.StartedAt).Milliseconds()

	if r.discoveryRepo != nil {
		if err := r.discoveryRepo.UpdateRun(ctx, run); err != nil {
			r.logger.Warn("failed to record discovery run completion", slog.Any("error", err))
		}
	}

	r.logger.Info("discovery run finished",
		slog.String("run_id", run.ID),
		slog.String("status", string(run.Status)),
		slog.Int("resources", run.ResourcesDiscovered),
		slog.Int("relationships", run.RelationshipsDiscovered),
		slog.Int("errors", len(run.Errors)),
	)

	return run, nil
}
