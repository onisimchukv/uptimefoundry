// SPDX-License-Identifier: MIT

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

// IncidentRepo manages persistence for incidents, investigations, and remediation proposals.
type IncidentRepo struct {
	db     *DB
	logger *slog.Logger
}

// NewIncidentRepo constructs an IncidentRepo.
func NewIncidentRepo(db *DB, logger *slog.Logger) *IncidentRepo {
	return &IncidentRepo{
		db:     db,
		logger: logger,
	}
}

// CreateIncident creates a new incident.
func (r *IncidentRepo) CreateIncident(ctx context.Context, inc *models.Incident) error {
	attrsJSON, _ := json.Marshal(inc.Attributes)
	now := time.Now().UTC()
	if inc.DetectedAt.IsZero() {
		inc.DetectedAt = now
	}

	query := `
		INSERT INTO incidents (
			environment_id, title, description, severity, status, source, primary_resource_id, detected_at, attributes
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		) RETURNING id
	`

	err := r.db.Pool.QueryRow(ctx, query,
		inc.EnvironmentID,
		inc.Title,
		inc.Description,
		string(inc.Severity),
		string(inc.Status),
		inc.Source,
		inc.PrimaryResourceID,
		inc.DetectedAt,
		attrsJSON,
	).Scan(&inc.ID)

	if err != nil {
		return fmt.Errorf("create incident: %w", err)
	}

	return nil
}

// GetIncidentByID retrieves an incident by UUID.
func (r *IncidentRepo) GetIncidentByID(ctx context.Context, id string) (*models.Incident, error) {
	query := `
		SELECT id, environment_id, title, description, severity, status, source, primary_resource_id, detected_at, resolved_at, attributes
		FROM incidents
		WHERE id = $1
	`

	var inc models.Incident
	var primaryRes *string
	var sev, stat string
	var attrsRaw []byte

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&inc.ID,
		&inc.EnvironmentID,
		&inc.Title,
		&inc.Description,
		&sev,
		&stat,
		&inc.Source,
		&primaryRes,
		&inc.DetectedAt,
		&inc.ResolvedAt,
		&attrsRaw,
	)
	if err != nil {
		return nil, fmt.Errorf("get incident by id %q: %w", id, err)
	}

	inc.Severity = models.IncidentSeverity(sev)
	inc.Status = models.IncidentStatus(stat)
	if primaryRes != nil {
		inc.PrimaryResourceID = *primaryRes
	}
	if len(attrsRaw) > 0 {
		_ = json.Unmarshal(attrsRaw, &inc.Attributes)
	}

	return &inc, nil
}

// ListIncidents lists incidents filtered by environment and status.
func (r *IncidentRepo) ListIncidents(ctx context.Context, envID string, status models.IncidentStatus, limit, offset int) ([]*models.Incident, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, environment_id, title, description, severity, status, source, primary_resource_id, detected_at, resolved_at, attributes
		FROM incidents
		WHERE ($1 = '' OR environment_id = $1::uuid)
		  AND ($2 = '' OR status = $2)
		ORDER BY detected_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Pool.Query(ctx, query, envID, string(status), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var incidents []*models.Incident
	for rows.Next() {
		var inc models.Incident
		var primaryRes *string
		var sev, stat string
		var attrsRaw []byte

		if err := rows.Scan(
			&inc.ID,
			&inc.EnvironmentID,
			&inc.Title,
			&inc.Description,
			&sev,
			&stat,
			&inc.Source,
			&primaryRes,
			&inc.DetectedAt,
			&inc.ResolvedAt,
			&attrsRaw,
		); err != nil {
			return nil, err
		}

		inc.Severity = models.IncidentSeverity(sev)
		inc.Status = models.IncidentStatus(stat)
		if primaryRes != nil {
			inc.PrimaryResourceID = *primaryRes
		}
		if len(attrsRaw) > 0 {
			_ = json.Unmarshal(attrsRaw, &inc.Attributes)
		}
		incidents = append(incidents, &inc)
	}

	return incidents, rows.Err()
}

// UpdateIncidentStatus modifies an incident status.
func (r *IncidentRepo) UpdateIncidentStatus(ctx context.Context, id string, status models.IncidentStatus) error {
	var resolvedAt *time.Time
	if status == models.StatusResolved || status == models.StatusClosed {
		now := time.Now().UTC()
		resolvedAt = &now
	}

	query := `
		UPDATE incidents
		SET status = $1, resolved_at = COALESCE($2, resolved_at)
		WHERE id = $3
	`
	_, err := r.db.Pool.Exec(ctx, query, string(status), resolvedAt, id)
	return err
}

// CreateInvestigation creates an investigation run for an incident.
func (r *IncidentRepo) CreateInvestigation(ctx context.Context, inv *models.Investigation) error {
	hypoJSON, _ := json.Marshal(inv.Hypotheses)
	bundleJSON, _ := json.Marshal(inv.EvidenceBundle)
	metaJSON, _ := json.Marshal(inv.ModelMetadata)

	now := time.Now().UTC()
	if inv.StartedAt.IsZero() {
		inv.StartedAt = now
	}

	query := `
		INSERT INTO investigations (
			incident_id, status, started_at, hypotheses, evidence_bundle, summary, confidence, model_metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		) RETURNING id
	`

	return r.db.Pool.QueryRow(ctx, query,
		inv.IncidentID,
		string(inv.Status),
		inv.StartedAt,
		hypoJSON,
		bundleJSON,
		inv.Summary,
		inv.Confidence,
		metaJSON,
	).Scan(&inv.ID)
}

// GetInvestigationByID retrieves an investigation record.
func (r *IncidentRepo) GetInvestigationByID(ctx context.Context, id string) (*models.Investigation, error) {
	query := `
		SELECT id, incident_id, status, started_at, completed_at, hypotheses, evidence_bundle, summary, confidence, model_metadata
		FROM investigations
		WHERE id = $1
	`

	var inv models.Investigation
	var stat string
	var hypoRaw, bundleRaw, metaRaw []byte

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&inv.ID,
		&inv.IncidentID,
		&stat,
		&inv.StartedAt,
		&inv.CompletedAt,
		&hypoRaw,
		&bundleRaw,
		&inv.Summary,
		&inv.Confidence,
		&metaRaw,
	)
	if err != nil {
		return nil, fmt.Errorf("get investigation %q: %w", id, err)
	}

	inv.Status = models.InvestigationStatus(stat)
	if len(hypoRaw) > 0 {
		_ = json.Unmarshal(hypoRaw, &inv.Hypotheses)
	}
	if len(bundleRaw) > 0 {
		_ = json.Unmarshal(bundleRaw, &inv.EvidenceBundle)
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &inv.ModelMetadata)
	}

	return &inv, nil
}

// SaveInvestigationResults updates an investigation record with completed findings.
func (r *IncidentRepo) SaveInvestigationResults(ctx context.Context, inv *models.Investigation) error {
	hypoJSON, _ := json.Marshal(inv.Hypotheses)
	bundleJSON, _ := json.Marshal(inv.EvidenceBundle)
	metaJSON, _ := json.Marshal(inv.ModelMetadata)

	now := time.Now().UTC()
	inv.CompletedAt = &now

	query := `
		UPDATE investigations
		SET status = $1, completed_at = $2, hypotheses = $3, evidence_bundle = $4, summary = $5, confidence = $6, model_metadata = $7
		WHERE id = $8
	`

	_, err := r.db.Pool.Exec(ctx, query,
		string(inv.Status),
		inv.CompletedAt,
		hypoJSON,
		bundleJSON,
		inv.Summary,
		inv.Confidence,
		metaJSON,
		inv.ID,
	)
	return err
}

// CreateProposal creates a human-in-the-loop remediation proposal.
func (r *IncidentRepo) CreateProposal(ctx context.Context, rem *models.Remediation) error {
	paramsJSON, _ := json.Marshal(rem.Parameters)
	now := time.Now().UTC()
	if rem.CreatedAt.IsZero() {
		rem.CreatedAt = now
	}

	query := `
		INSERT INTO remediations (
			investigation_id, title, action_type, parameters, status, proposed_by, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) RETURNING id
	`

	return r.db.Pool.QueryRow(ctx, query,
		rem.InvestigationID,
		rem.Title,
		rem.ActionType,
		paramsJSON,
		string(rem.Status),
		rem.ProposedBy,
		rem.CreatedAt,
	).Scan(&rem.ID)
}

// GetProposalByID retrieves a remediation proposal by ID.
func (r *IncidentRepo) GetProposalByID(ctx context.Context, id string) (*models.Remediation, error) {
	query := `
		SELECT id, investigation_id, title, action_type, parameters, status, proposed_by, approved_by, approved_at, executed_at, execution_result, created_at
		FROM remediations
		WHERE id = $1
	`

	var rem models.Remediation
	var stat string
	var paramsRaw, resRaw []byte

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&rem.ID,
		&rem.InvestigationID,
		&rem.Title,
		&rem.ActionType,
		&paramsRaw,
		&stat,
		&rem.ProposedBy,
		&rem.ApprovedBy,
		&rem.ApprovedAt,
		&rem.ExecutedAt,
		&resRaw,
		&rem.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get remediation proposal %q: %w", id, err)
	}

	rem.Status = models.RemediationStatus(stat)
	if len(paramsRaw) > 0 {
		_ = json.Unmarshal(paramsRaw, &rem.Parameters)
	}
	if len(resRaw) > 0 {
		_ = json.Unmarshal(resRaw, &rem.ExecutionResult)
	}

	return &rem, nil
}

// ApproveProposal marks a proposal as approved.
func (r *IncidentRepo) ApproveProposal(ctx context.Context, id, approver string) error {
	now := time.Now().UTC()
	query := `
		UPDATE remediations
		SET status = $1, approved_by = $2, approved_at = $3
		WHERE id = $4 AND status = 'pending_approval'
	`
	res, err := r.db.Pool.Exec(ctx, query, string(models.RemediationApproved), approver, now, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("proposal %q not found or not in pending_approval state", id)
	}
	return nil
}

// RecordExecution marks a proposal as executed or failed.
func (r *IncidentRepo) RecordExecution(ctx context.Context, id string, result map[string]any, execErr error) error {
	now := time.Now().UTC()
	status := models.RemediationExecuted
	if execErr != nil {
		status = models.RemediationFailed
		if result == nil {
			result = make(map[string]any)
		}
		result["error"] = execErr.Error()
	}

	resJSON, _ := json.Marshal(result)
	query := `
		UPDATE remediations
		SET status = $1, executed_at = $2, execution_result = $3
		WHERE id = $4
	`
	_, err := r.db.Pool.Exec(ctx, query, string(status), now, resJSON, id)
	return err
}
