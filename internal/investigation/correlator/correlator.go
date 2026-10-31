package correlator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/core/ports"
	"github.com/onisimchukv/uptimefoundry/internal/rag/graphrag"
	"github.com/onisimchukv/uptimefoundry/internal/telemetry"
)

// EvidenceBundle contains deterministic operational evidence collected for an incident.
type EvidenceBundle struct {
	Incident       *models.Incident        `json:"incident"`
	Evidence       []models.EvidenceRecord `json:"evidence"`
	GroundedRAG    *graphrag.GroundedContext `json:"grounded_rag"`
	TopologyNodes  []*models.Resource      `json:"topology_nodes"`
	TopologyRels   []*models.Relationship  `json:"topology_relationships"`
}

// Correlator coordinates multimodal evidence gathering across topology, metrics, logs, and GraphRAG.
type Correlator struct {
	resourceRepo ports.ResourceRepository
	metrics      telemetry.MetricsProvider
	logs         telemetry.LogsProvider
	graphRAG     *graphrag.Engine
	logger       *slog.Logger
}

// NewCorrelator constructs a new multimodal evidence correlator.
func NewCorrelator(
	resourceRepo ports.ResourceRepository,
	metrics telemetry.MetricsProvider,
	logs telemetry.LogsProvider,
	graphRAG *graphrag.Engine,
	logger *slog.Logger,
) *Correlator {
	return &Correlator{
		resourceRepo: resourceRepo,
		metrics:      metrics,
		logs:         logs,
		graphRAG:     graphRAG,
		logger:       logger,
	}
}

// CollectEvidence gathers deterministic facts for an incident before involving AI models.
func (c *Correlator) CollectEvidence(ctx context.Context, inc *models.Incident) (*EvidenceBundle, error) {
	bundle := &EvidenceBundle{
		Incident: inc,
		Evidence: make([]models.EvidenceRecord, 0),
	}

	startWindow := inc.DetectedAt.Add(-30 * time.Minute)
	endWindow := inc.DetectedAt.Add(15 * time.Minute)
	if endWindow.After(time.Now()) {
		endWindow = time.Now()
	}

	// 1. Topology Subgraph Extraction
	var primaryResName string
	if inc.PrimaryResourceID != "" && c.resourceRepo != nil {
		nodes, rels, err := c.resourceRepo.GetTopologySubgraph(ctx, inc.PrimaryResourceID, 2)
		if err == nil {
			bundle.TopologyNodes = nodes
			bundle.TopologyRels = rels
			for _, n := range nodes {
				if n.ID == inc.PrimaryResourceID {
					primaryResName = n.Name
				}
			}

			bundle.Evidence = append(bundle.Evidence, models.EvidenceRecord{
				ID:              "ev-topo-01",
				EvidenceType:    "topology_subgraph",
				Provider:        "apache_age",
				ResourceIDs:     []string{inc.PrimaryResourceID},
				TimestampRange:  [2]time.Time{inc.DetectedAt, inc.DetectedAt},
				QueryOrMetadata: fmt.Sprintf("GetTopologySubgraph(id=%s, hops=2)", inc.PrimaryResourceID),
				Summary:         fmt.Sprintf("Discovered %d neighbor resources and %d relationship edges.", len(nodes), len(rels)),
				IsAnomaly:       false,
				Confidence:      1.0,
			})
		}
	}

	// 2. Telemetry Metrics Sweep
	if c.metrics != nil {
		metricQuery := fmt.Sprintf(`sum(rate(http_requests_total{status=~"5.."}[5m])) or rate(node_network_transmit_errs_total[5m])`)
		if primaryResName != "" {
			metricQuery = fmt.Sprintf(`sum(rate(http_requests_total{app="%s",status=~"5.."}[5m]))`, primaryResName)
		}

		res, err := c.metrics.QueryInstant(ctx, metricQuery, inc.DetectedAt)
		if err == nil && len(res.Series) > 0 {
			var val float64
			if len(res.Series[0].Points) > 0 {
				val = res.Series[0].Points[0].Value
			}
			isAnomaly := val > 0.05
			bundle.Evidence = append(bundle.Evidence, models.EvidenceRecord{
				ID:              "ev-metric-01",
				EvidenceType:    "metric_timeseries",
				Provider:        "victoriametrics",
				ResourceIDs:     []string{inc.PrimaryResourceID},
				TimestampRange:  [2]time.Time{startWindow, endWindow},
				QueryOrMetadata: metricQuery,
				Summary:         fmt.Sprintf("HTTP 5xx error rate at incident time: %.3f req/s", val),
				IsAnomaly:       isAnomaly,
				Confidence:      0.95,
			})
		}
	}

	// 3. Operational Log Sweep
	if c.logs != nil {
		logQuery := `{app=~".+"} |= "error"`
		if primaryResName != "" {
			logQuery = fmt.Sprintf(`{app="%s"} |= "error"`, primaryResName)
		}

		res, err := c.logs.QueryLogs(ctx, telemetry.LogQuery{
			Query: logQuery,
			Start: startWindow,
			End:   endWindow,
			Limit: 20,
		})
		if err == nil && len(res.Entries) > 0 {
			summary := fmt.Sprintf("Found %d error log lines during incident window.", len(res.Entries))
			snippet := ""
			for i, ent := range res.Entries {
				if i >= 3 {
					break
				}
				snippet += ent.Line + "\n"
			}
			bundle.Evidence = append(bundle.Evidence, models.EvidenceRecord{
				ID:                "ev-logs-01",
				EvidenceType:      "log_stream",
				Provider:          "loki",
				ResourceIDs:       []string{inc.PrimaryResourceID},
				TimestampRange:    [2]time.Time{startWindow, endWindow},
				QueryOrMetadata:   logQuery,
				Summary:           summary,
				RawPayloadSnippet: snippet,
				IsAnomaly:         true,
				Confidence:        0.90,
			})
		}
	}

	// 4. Grounded GraphRAG Retrieval
	if c.graphRAG != nil {
		queryText := fmt.Sprintf("%s %s", inc.Title, inc.Description)
		grounded, err := c.graphRAG.AssembleContext(ctx, inc.PrimaryResourceID, queryText, 2)
		if err == nil && len(grounded.MatchedDocuments) > 0 {
			bundle.GroundedRAG = grounded
			bundle.Evidence = append(bundle.Evidence, models.EvidenceRecord{
				ID:              "ev-rag-01",
				EvidenceType:    "graphrag_grounding",
				Provider:        "pgvector",
				ResourceIDs:     []string{inc.PrimaryResourceID},
				TimestampRange:  [2]time.Time{inc.DetectedAt, inc.DetectedAt},
				QueryOrMetadata: queryText,
				Summary:         fmt.Sprintf("Retrieved %d topologically relevant runbook procedures with citations.", len(grounded.MatchedDocuments)),
				IsAnomaly:       false,
				Confidence:      1.0,
			})
		}
	}

	return bundle, nil
}
