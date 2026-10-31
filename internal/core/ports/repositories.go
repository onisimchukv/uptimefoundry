package ports

import (
	"context"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

// ResourceRepository defines persistence operations for resources and relationships.
type ResourceRepository interface {
	UpsertResource(ctx context.Context, res *models.Resource) error
	GetResourceByID(ctx context.Context, id string) (*models.Resource, error)
	ListResources(ctx context.Context, envID string, kind models.ResourceKind, limit, offset int) ([]*models.Resource, error)
	UpsertRelationship(ctx context.Context, rel *models.Relationship) error
	GetRelationships(ctx context.Context, resourceID string) ([]*models.Relationship, error)
	
	// Property Graph specific operations (delegated to Apache AGE or in-memory fallback)
	GetTopologySubgraph(ctx context.Context, rootResourceID string, hops int) ([]*models.Resource, []*models.Relationship, error)
	ExecuteCypher(ctx context.Context, query string) (any, error)
}

// IncidentRepository defines operations for incident management.
type IncidentRepository interface {
	CreateIncident(ctx context.Context, inc *models.Incident) error
	GetIncidentByID(ctx context.Context, id string) (*models.Incident, error)
	ListIncidents(ctx context.Context, envID string, status models.IncidentStatus, limit, offset int) ([]*models.Incident, error)
	UpdateIncidentStatus(ctx context.Context, id string, status models.IncidentStatus) error
}

// InvestigationRepository manages evidence, hypotheses, and diagnostic runs.
type InvestigationRepository interface {
	CreateInvestigation(ctx context.Context, inv *models.Investigation) error
	GetInvestigationByID(ctx context.Context, id string) (*models.Investigation, error)
	SaveInvestigationResults(ctx context.Context, inv *models.Investigation) error
}

// KnowledgeRepository defines vector search and operational document indexing.
type KnowledgeRepository interface {
	IndexDocument(ctx context.Context, doc *models.KnowledgeDocument) error
	SearchHybrid(ctx context.Context, query string, embedding []float32, topK int, similarityThreshold float64) ([]*models.KnowledgeSearchResult, error)
	SearchWithTopologyFilter(ctx context.Context, embedding []float32, resourceKinds []string, topK int, similarityThreshold float64) ([]*models.KnowledgeSearchResult, error)
}

// RemediationRepository manages proposals, human reviews, and execution logs.
type RemediationRepository interface {
	CreateProposal(ctx context.Context, rem *models.Remediation) error
	GetProposalByID(ctx context.Context, id string) (*models.Remediation, error)
	ApproveProposal(ctx context.Context, id, approver string) error
	RecordExecution(ctx context.Context, id string, result map[string]any, err error) error
}

// AuditRepository records security and operational changes.
type AuditRepository interface {
	RecordEvent(ctx context.Context, event *models.AuditEvent) error
}

// DiscoveryRepository manages historical discovery execution runs.
type DiscoveryRepository interface {
	CreateRun(ctx context.Context, run *models.DiscoveryRun) error
	UpdateRun(ctx context.Context, run *models.DiscoveryRun) error
	GetRunByID(ctx context.Context, id string) (*models.DiscoveryRun, error)
	ListRuns(ctx context.Context, envID string, limit, offset int) ([]*models.DiscoveryRun, error)
}

