// SPDX-License-Identifier: MIT

package traverser

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/graph"
)

// BlastRadiusReport summarizes the ripple effect of a component degradation.
type BlastRadiusReport struct {
	RootResourceID   string               `json:"root_resource_id"`
	TotalImpacted    int                  `json:"total_impacted"`
	SeverityScore    float64              `json:"severity_score"` // 0.0 to 100.0
	ImpactedWorkloads []string             `json:"impacted_workloads"`
	ImpactedServices  []string             `json:"impacted_services"`
	ImpactedNodes     []string             `json:"impacted_nodes"`
	Resources        []*models.Resource   `json:"resources"`
	Relationships    []*models.Relationship `json:"relationships"`
}

// TopologyTraverser executes SRE topological analysis on top of a GraphEngine.
type TopologyTraverser struct {
	engine graph.GraphEngine
	logger *slog.Logger
}

// New creates a new TopologyTraverser.
func New(engine graph.GraphEngine, logger *slog.Logger) *TopologyTraverser {
	return &TopologyTraverser{
		engine: engine,
		logger: logger,
	}
}

// ComputeBlastRadius assesses all downstream and colocated resources affected if rootResourceID fails.
func (t *TopologyTraverser) ComputeBlastRadius(ctx context.Context, rootResourceID string, maxHops int) (*BlastRadiusReport, error) {
	if maxHops <= 0 {
		maxHops = 2
	}

	resources, rels, err := t.engine.GetSubgraph(ctx, rootResourceID, maxHops)
	if err != nil {
		return nil, fmt.Errorf("traverse blast radius: %w", err)
	}

	report := &BlastRadiusReport{
		RootResourceID: rootResourceID,
		TotalImpacted:  len(resources),
		Resources:      resources,
		Relationships:  rels,
	}

	var score float64
	for _, res := range resources {
		if res.ID == rootResourceID {
			continue
		}

		switch res.Kind {
		case models.KindK8sService, models.KindK8sIngress:
			report.ImpactedServices = append(report.ImpactedServices, res.Name)
			score += 15.0
		case models.KindK8sDeployment, models.KindK8sStatefulSet, models.KindSwarmService:
			report.ImpactedWorkloads = append(report.ImpactedWorkloads, res.Name)
			score += 10.0
		case models.KindK8sPod, models.KindDockerContainer:
			score += 2.0
		case models.KindK8sNode, models.KindHost:
			report.ImpactedNodes = append(report.ImpactedNodes, res.Name)
			score += 20.0
		default:
			score += 1.0
		}
	}

	if score > 100.0 {
		score = 100.0
	}
	report.SeverityScore = score

	t.logger.Info("blast radius computed",
		slog.String("root_resource", rootResourceID),
		slog.Int("total_impacted", report.TotalImpacted),
		slog.Float64("severity_score", report.SeverityScore),
	)

	return report, nil
}
