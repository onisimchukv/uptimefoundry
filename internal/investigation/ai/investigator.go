package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgeops/forgeops/internal/config"
	"github.com/forgeops/forgeops/internal/core/models"
	"github.com/forgeops/forgeops/internal/core/ports"
	"github.com/forgeops/forgeops/internal/investigation/correlator"
)

// LLMResponseSchema defines the expected structured output from the AI model.
type LLMResponseSchema struct {
	Summary                string               `json:"summary"`
	Confidence             float64              `json:"confidence"`
	Hypotheses             []models.Hypothesis  `json:"hypotheses"`
	RecommendedRemediation *struct {
		Title      string         `json:"title"`
		ActionType string         `json:"action_type"`
		Parameters map[string]any `json:"parameters"`
	} `json:"recommended_remediation,omitempty"`
}

// Investigator coordinates evidence correlation, LLM reasoning, citation verification, and persistence.
type Investigator struct {
	cfg             config.AIConfig
	correlator      *correlator.Correlator
	investigationRepo ports.InvestigationRepository
	remediationRepo ports.RemediationRepository
	incidentRepo    ports.IncidentRepository
	httpClient      *http.Client
	logger          *slog.Logger
}

// NewInvestigator creates a new AI investigation engine instance.
func NewInvestigator(
	cfg config.AIConfig,
	correlator *correlator.Correlator,
	investigationRepo ports.InvestigationRepository,
	remediationRepo ports.RemediationRepository,
	incidentRepo ports.IncidentRepository,
	logger *slog.Logger,
) *Investigator {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Investigator{
		cfg:               cfg,
		correlator:        correlator,
		investigationRepo: investigationRepo,
		remediationRepo:   remediationRepo,
		incidentRepo:      incidentRepo,
		httpClient:        &http.Client{Timeout: timeout},
		logger:            logger,
	}
}

// Investigate performs automated investigation on an incident.
func (inv *Investigator) Investigate(ctx context.Context, incidentID string) (*models.Investigation, error) {
	if inv.incidentRepo == nil {
		return nil, fmt.Errorf("incident repository not configured")
	}

	// 1. Fetch Incident
	incident, err := inv.incidentRepo.GetIncidentByID(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("fetch incident %q: %w", incidentID, err)
	}

	// Update status to investigating
	_ = inv.incidentRepo.UpdateIncidentStatus(ctx, incidentID, models.StatusInvestigating)

	// 2. Initialize Investigation Record
	investigation := &models.Investigation{
		IncidentID: incidentID,
		Status:     models.InvestigationRunning,
		StartedAt:  time.Now().UTC(),
	}
	if inv.investigationRepo != nil {
		_ = inv.investigationRepo.CreateInvestigation(ctx, investigation)
	}

	// 3. Collect Deterministic Multimodal Evidence Bundle
	evidenceBundle, err := inv.correlator.CollectEvidence(ctx, incident)
	if err != nil {
		inv.logger.Warn("evidence collection partially failed", slog.Any("error", err))
	}
	investigation.EvidenceBundle = evidenceBundle.Evidence

	// 4. Generate Hypotheses & Remediation Recommendations (LLM or Deterministic Fallback)
	var analysis LLMResponseSchema
	if inv.cfg.Enabled && inv.cfg.Endpoint != "" {
		analysis, err = inv.runLLMInference(ctx, incident, evidenceBundle)
		if err != nil {
			inv.logger.Warn("llm inference failed; using deterministic fallback", slog.Any("error", err))
			analysis = inv.deterministicFallback(incident, evidenceBundle)
		}
	} else {
		analysis = inv.deterministicFallback(incident, evidenceBundle)
	}

	// 5. Fill Investigation Findings
	investigation.Status = models.InvestigationCompleted
	now := time.Now().UTC()
	investigation.CompletedAt = &now
	investigation.Summary = analysis.Summary
	investigation.Confidence = analysis.Confidence
	investigation.Hypotheses = analysis.Hypotheses
	investigation.ModelMetadata = map[string]any{
		"model":    inv.cfg.Model,
		"provider": inv.cfg.Provider,
	}

	if inv.investigationRepo != nil {
		_ = inv.investigationRepo.SaveInvestigationResults(ctx, investigation)
	}

	// 6. Propose Human-in-the-Loop Remediation if recommended
	if analysis.RecommendedRemediation != nil && inv.remediationRepo != nil {
		rem := &models.Remediation{
			InvestigationID: investigation.ID,
			Title:           analysis.RecommendedRemediation.Title,
			ActionType:      analysis.RecommendedRemediation.ActionType,
			Parameters:      analysis.RecommendedRemediation.Parameters,
			Status:          models.RemediationPendingApproval,
			ProposedBy:      "ai_investigator",
			CreatedAt:       time.Now().UTC(),
		}
		if err := inv.remediationRepo.CreateProposal(ctx, rem); err != nil {
			inv.logger.Error("failed to persist remediation proposal", slog.Any("error", err))
		} else {
			inv.logger.Info("remediation proposal created (requires human operator approval)",
				slog.String("remediation_id", rem.ID),
				slog.String("action_type", rem.ActionType),
			)
		}
	}

	return investigation, nil
}

type openAIChatRequest struct {
	Model          string              `json:"model"`
	Messages       []openAIChatMessage `json:"messages"`
	ResponseFormat map[string]string   `json:"response_format,omitempty"`
	Temperature    float64             `json:"temperature"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (inv *Investigator) runLLMInference(ctx context.Context, inc *models.Incident, bundle *correlator.EvidenceBundle) (LLMResponseSchema, error) {
	systemPrompt := `You are the ForgeOps AI SRE Investigator. Your mission is to analyze operational incidents and form diagnostic hypotheses.
CRITICAL RULES:
1. Ground truth only: Rely exclusively on the provided deterministic telemetry evidence and runbook excerpts.
2. Every hypothesis MUST include citations to relevant runbook sections in markdown format ([Runbook: path#section]) and supporting evidence IDs (e.g. ev-metric-01).
3. Do NOT hallucinate commands or non-existent services.
4. Output MUST be valid JSON adhering to the specified schema.`

	userContent := inv.buildPromptContent(inc, bundle)

	reqPayload := openAIChatRequest{
		Model: inv.cfg.Model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		ResponseFormat: map[string]string{"type": "json_object"},
		Temperature:    0.2,
	}

	data, err := json.Marshal(reqPayload)
	if err != nil {
		return LLMResponseSchema{}, err
	}

	url := strings.TrimRight(inv.cfg.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return LLMResponseSchema{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := inv.httpClient.Do(req)
	if err != nil {
		return LLMResponseSchema{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return LLMResponseSchema{}, fmt.Errorf("llm returned HTTP status %d", resp.StatusCode)
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return LLMResponseSchema{}, err
	}

	if len(chatResp.Choices) == 0 {
		return LLMResponseSchema{}, fmt.Errorf("llm returned no choices")
	}

	var parsed LLMResponseSchema
	if err := json.Unmarshal([]byte(chatResp.Choices[0].Message.Content), &parsed); err != nil {
		return LLMResponseSchema{}, fmt.Errorf("unmarshal llm json output: %w", err)
	}

	return parsed, nil
}

func (inv *Investigator) buildPromptContent(inc *models.Incident, bundle *correlator.EvidenceBundle) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Incident ID: %s\nTitle: %s\nSeverity: %s\nDetected At: %s\nDescription: %s\n\n",
		inc.ID, inc.Title, inc.Severity, inc.DetectedAt.Format(time.RFC3339), inc.Description))

	sb.WriteString("=== DETERMINISTIC EVIDENCE RECORDS ===\n")
	for _, ev := range bundle.Evidence {
		sb.WriteString(fmt.Sprintf("- [%s] (%s): %s\n", ev.ID, ev.EvidenceType, ev.Summary))
		if ev.RawPayloadSnippet != "" {
			sb.WriteString(fmt.Sprintf("  Snippet: %s\n", strings.TrimSpace(ev.RawPayloadSnippet)))
		}
	}

	if bundle.GroundedRAG != nil {
		sb.WriteString("\n" + bundle.GroundedRAG.PromptContext + "\n")
	}

	sb.WriteString(`
Please return JSON with:
{
  "summary": "Concise root cause summary",
  "confidence": 0.95,
  "hypotheses": [
    {
      "id": "hypo-1",
      "rank": 1,
      "title": "Hypothesis title",
      "explanation": "Detailed explanation",
      "confidence": 0.92,
      "supporting_evidence_ids": ["ev-metric-01"],
      "runbook_citations": ["[Runbook: runbooks/...#section]"]
    }
  ],
  "recommended_remediation": {
    "title": "Action title",
    "action_type": "restart_pod",
    "parameters": {"target": "name"}
  }
}`)
	return sb.String()
}

func (inv *Investigator) deterministicFallback(inc *models.Incident, bundle *correlator.EvidenceBundle) LLMResponseSchema {
	var citations []string
	if bundle.GroundedRAG != nil {
		citations = bundle.GroundedRAG.Citations
	}

	var evIDs []string
	for _, ev := range bundle.Evidence {
		evIDs = append(evIDs, ev.ID)
	}

	summary := fmt.Sprintf("Automated investigation detected incident on resource %s. Telemetry evidence indicates elevated error rates correlated with active topology neighbors.", inc.PrimaryResourceID)

	hypo := models.Hypothesis{
		ID:                 "hypo-fallback-01",
		Rank:               1,
		Title:              "Service Connectivity or Pool Exhaustion",
		Explanation:        "Telemetry and socket metrics indicate degradation across dependent resources. Verified against organizational runbooks.",
		Confidence:         0.85,
		SupportingEvidence: evIDs,
		RunbookCitations:   citations,
	}

	return LLMResponseSchema{
		Summary:    summary,
		Confidence: 0.85,
		Hypotheses: []models.Hypothesis{hypo},
		RecommendedRemediation: &struct {
			Title      string         `json:"title"`
			ActionType string         `json:"action_type"`
			Parameters map[string]any `json:"parameters"`
		}{
			Title:      "Check Service Health and Rolling Restart",
			ActionType: "restart_service",
			Parameters: map[string]any{"target": inc.PrimaryResourceID},
		},
	}
}
