package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/core/ports"
	"github.com/onisimchukv/uptimefoundry/internal/mcp/protocol"
	"github.com/onisimchukv/uptimefoundry/internal/rag/graphrag"
	"github.com/onisimchukv/uptimefoundry/internal/telemetry"
)

// Config configures the MCP server.
type Config struct {
	AuthToken string
}

// Server handles Model Context Protocol requests over stdio and SSE.
type Server struct {
	cfg           Config
	resourceRepo  ports.ResourceRepository
	incidentRepo  ports.IncidentRepository
	remediationRepo ports.RemediationRepository
	knowledgeRepo ports.KnowledgeRepository
	metrics       telemetry.MetricsProvider
	logs          telemetry.LogsProvider
	graphRAG      *graphrag.Engine
	logger        *slog.Logger
}

// NewServer constructs an MCP Server.
func NewServer(
	cfg Config,
	resourceRepo ports.ResourceRepository,
	incidentRepo ports.IncidentRepository,
	remediationRepo ports.RemediationRepository,
	knowledgeRepo ports.KnowledgeRepository,
	metrics telemetry.MetricsProvider,
	logs telemetry.LogsProvider,
	graphRAG *graphrag.Engine,
	logger *slog.Logger,
) *Server {
	return &Server{
		cfg:             cfg,
		resourceRepo:    resourceRepo,
		incidentRepo:    incidentRepo,
		remediationRepo: remediationRepo,
		knowledgeRepo:   knowledgeRepo,
		metrics:         metrics,
		logs:            logs,
		graphRAG:        graphRAG,
		logger:          logger,
	}
}

// HandleRequest dispatches an incoming JSON-RPC request to the corresponding MCP handler.
func (s *Server) HandleRequest(ctx context.Context, req protocol.Request) protocol.Response {
	resp := protocol.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		var params protocol.InitializeParams
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &params)
		}
		resp.Result = protocol.InitializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: protocol.ServerInfo{
				Name:    "uptimefoundry-mcp-gateway",
				Version: "1.2.0",
			},
			Capabilities: protocol.ServerCapabilities{
				Tools:     &protocol.ToolsCapability{ListChanged: false},
				Resources: &protocol.ResourcesCapability{Subscribe: false, ListChanged: false},
				Prompts:   &protocol.PromptsCapability{ListChanged: false},
			},
		}

	case "ping":
		resp.Result = map[string]any{"status": "pong"}

	case "tools/list":
		resp.Result = protocol.ToolListResult{
			Tools: s.listTools(),
		}

	case "tools/call":
		var params protocol.CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &protocol.Error{Code: protocol.ErrCodeInvalidParams, Message: "invalid tool call params"}
			return resp
		}
		result, err := s.callTool(ctx, params)
		if err != nil {
			resp.Result = protocol.CallToolResult{
				IsError: true,
				Content: []protocol.ToolContent{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
			}
		} else {
			resp.Result = result
		}

	case "resources/list":
		resp.Result = protocol.ResourceListResult{
			Resources: s.listResources(),
		}

	case "resources/read":
		var params protocol.ReadResourceParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &protocol.Error{Code: protocol.ErrCodeInvalidParams, Message: "invalid read resource params"}
			return resp
		}
		res, err := s.readResource(ctx, params.URI)
		if err != nil {
			resp.Error = &protocol.Error{Code: protocol.ErrCodeInternal, Message: err.Error()}
		} else {
			resp.Result = res
		}

	case "prompts/list":
		resp.Result = protocol.PromptListResult{
			Prompts: s.listPrompts(),
		}

	case "prompts/get":
		var params protocol.GetPromptParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &protocol.Error{Code: protocol.ErrCodeInvalidParams, Message: "invalid get prompt params"}
			return resp
		}
		res, err := s.getPrompt(ctx, params)
		if err != nil {
			resp.Error = &protocol.Error{Code: protocol.ErrCodeInternal, Message: err.Error()}
		} else {
			resp.Result = res
		}

	default:
		resp.Error = &protocol.Error{
			Code:    protocol.ErrCodeMethodNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
	}

	return resp
}

func (s *Server) listTools() []protocol.Tool {
	return []protocol.Tool{
		{
			Name:        "uptimefoundry_get_topology",
			Description: "Query subgraphs around a resource up to N hops using openCypher / property graph.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"resource_id": {Type: "string", Description: "UUID or unique ID of the target resource"},
					"hops":        {Type: "integer", Description: "Traversal hop distance (default 2)", Default: 2},
				},
				Required: []string{"resource_id"},
			},
		},
		{
			Name:        "uptimefoundry_query_metrics",
			Description: "Execute bounded instant PromQL or MetricsQL queries against VictoriaMetrics.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"query":    {Type: "string", Description: "PromQL or MetricsQL query expression"},
					"duration": {Type: "string", Description: "Lookback window, e.g. 15m, 1h", Default: "15m"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "uptimefoundry_query_logs",
			Description: "Search structured operational logs in Loki within an incident window.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"query": {Type: "string", Description: "LogQL query expression (e.g. {app=\"payment-api\"} |= \"error\")"},
					"limit": {Type: "integer", Description: "Max lines to return (default 50)", Default: 50},
					"since": {Type: "string", Description: "Duration to look back, e.g. 1h", Default: "1h"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "uptimefoundry_search_knowledge",
			Description: "Perform GraphRAG vector and hybrid searches over internal runbooks and historical post-mortems.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"query":          {Type: "string", Description: "Problem description or search query"},
					"resource_kinds": {Type: "array", Description: "Optional list of infrastructure kinds (e.g. k8s_deployment, database)"},
					"top_k":          {Type: "integer", Description: "Number of runbook excerpts to return (default 4)", Default: 4},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "uptimefoundry_get_incident_evidence",
			Description: "Retrieve all correlated telemetry, topology changes, and events for an active incident.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"incident_id": {Type: "string", Description: "UUID of the active incident"},
				},
				Required: []string{"incident_id"},
			},
		},
		{
			Name:        "uptimefoundry_propose_remediation",
			Description: "Create a draft remediation proposal requiring human operator approval. Does not execute changes directly.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.PropertyDef{
					"investigation_id": {Type: "string", Description: "UUID of the investigation proposing this action"},
					"title":            {Type: "string", Description: "Human-readable title describing the action"},
					"action_type":      {Type: "string", Description: "Action kind (e.g. restart_pod, scale_deployment, flush_cache)"},
					"parameters":       {Type: "object", Description: "Action parameters (target, replicas, etc.)"},
				},
				Required: []string{"investigation_id", "title", "action_type"},
			},
		},
	}
}

func (s *Server) callTool(ctx context.Context, params protocol.CallToolParams) (protocol.CallToolResult, error) {
	switch params.Name {
	case "uptimefoundry_get_topology":
		resID, _ := params.Arguments["resource_id"].(string)
		hops := 2
		if h, ok := params.Arguments["hops"].(float64); ok && h > 0 {
			hops = int(h)
		}
		if s.resourceRepo == nil {
			return protocol.CallToolResult{}, fmt.Errorf("resource repository not configured")
		}
		nodes, rels, err := s.resourceRepo.GetTopologySubgraph(ctx, resID, hops)
		if err != nil {
			return protocol.CallToolResult{}, err
		}
		out, _ := json.MarshalIndent(map[string]any{
			"root_id":       resID,
			"hops":          hops,
			"nodes_count":   len(nodes),
			"rels_count":    len(rels),
			"nodes":         nodes,
			"relationships": rels,
		}, "", "  ")
		return protocol.CallToolResult{Content: []protocol.ToolContent{{Type: "text", Text: string(out)}}}, nil

	case "uptimefoundry_query_metrics":
		query, _ := params.Arguments["query"].(string)
		if s.metrics == nil {
			return protocol.CallToolResult{}, fmt.Errorf("metrics provider not configured")
		}
		res, err := s.metrics.QueryInstant(ctx, query, time.Now())
		if err != nil {
			return protocol.CallToolResult{}, err
		}
		out, _ := json.MarshalIndent(res, "", "  ")
		return protocol.CallToolResult{Content: []protocol.ToolContent{{Type: "text", Text: string(out)}}}, nil

	case "uptimefoundry_query_logs":
		query, _ := params.Arguments["query"].(string)
		limit := 50
		if l, ok := params.Arguments["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if s.logs == nil {
			return protocol.CallToolResult{}, fmt.Errorf("logs provider not configured")
		}
		now := time.Now()
		start := now.Add(-1 * time.Hour)
		res, err := s.logs.QueryLogs(ctx, telemetry.LogQuery{
			Query: query,
			Start: start,
			End:   now,
			Limit: limit,
		})
		if err != nil {
			return protocol.CallToolResult{}, err
		}
		out, _ := json.MarshalIndent(res, "", "  ")
		return protocol.CallToolResult{Content: []protocol.ToolContent{{Type: "text", Text: string(out)}}}, nil

	case "uptimefoundry_search_knowledge":
		query, _ := params.Arguments["query"].(string)
		if s.knowledgeRepo == nil {
			return protocol.CallToolResult{}, fmt.Errorf("knowledge repository not configured")
		}
		topK := 4
		if k, ok := params.Arguments["top_k"].(float64); ok && k > 0 {
			topK = int(k)
		}
		results, err := s.knowledgeRepo.SearchHybrid(ctx, query, nil, topK, 0.5)
		if err != nil {
			return protocol.CallToolResult{}, err
		}
		out, _ := json.MarshalIndent(results, "", "  ")
		return protocol.CallToolResult{Content: []protocol.ToolContent{{Type: "text", Text: string(out)}}}, nil

	case "uptimefoundry_get_incident_evidence":
		incID, _ := params.Arguments["incident_id"].(string)
		if s.incidentRepo == nil {
			return protocol.CallToolResult{}, fmt.Errorf("incident repository not configured")
		}
		inc, err := s.incidentRepo.GetIncidentByID(ctx, incID)
		if err != nil {
			return protocol.CallToolResult{}, err
		}
		out, _ := json.MarshalIndent(inc, "", "  ")
		return protocol.CallToolResult{Content: []protocol.ToolContent{{Type: "text", Text: string(out)}}}, nil

	case "uptimefoundry_propose_remediation":
		invID, _ := params.Arguments["investigation_id"].(string)
		title, _ := params.Arguments["title"].(string)
		actType, _ := params.Arguments["action_type"].(string)
		paramsMap, _ := params.Arguments["parameters"].(map[string]any)

		if s.remediationRepo == nil {
			return protocol.CallToolResult{}, fmt.Errorf("remediation repository not configured")
		}

		rem := &models.Remediation{
			InvestigationID: invID,
			Title:           title,
			ActionType:      actType,
			Parameters:      paramsMap,
			Status:          models.RemediationPendingApproval,
			ProposedBy:      "mcp_agent",
			CreatedAt:       time.Now().UTC(),
		}
		if err := s.remediationRepo.CreateProposal(ctx, rem); err != nil {
			return protocol.CallToolResult{}, err
		}
		out := fmt.Sprintf("Remediation proposal created successfully (ID: %s, Status: pending_approval). Execution requires human approval.", rem.ID)
		return protocol.CallToolResult{Content: []protocol.ToolContent{{Type: "text", Text: out}}}, nil

	default:
		return protocol.CallToolResult{}, fmt.Errorf("unknown tool: %s", params.Name)
	}
}

func (s *Server) listResources() []protocol.Resource {
	return []protocol.Resource{
		{
			URI:         "uptimefoundry://environments/{env}/topology",
			Name:        "Environment Topology",
			Description: "Live property graph topology of an environment",
			MimeType:    "application/json",
		},
		{
			URI:         "uptimefoundry://incidents/{id}/evidence",
			Name:        "Incident Evidence Bundle",
			Description: "Multimodal evidence collected for an incident",
			MimeType:    "application/json",
		},
		{
			URI:         "uptimefoundry://resources/{id}/health",
			Name:        "Resource Health",
			Description: "Live health and eBPF socket metrics for a specific resource",
			MimeType:    "application/json",
		},
		{
			URI:         "uptimefoundry://runbooks/{slug}",
			Name:        "Verified Runbook",
			Description: "Operational runbook documentation markdown",
			MimeType:    "text/markdown",
		},
	}
}

func (s *Server) readResource(ctx context.Context, uri string) (protocol.ReadResourceResult, error) {
	if strings.HasPrefix(uri, "uptimefoundry://environments/") && strings.HasSuffix(uri, "/topology") {
		parts := strings.Split(uri, "/")
		if len(parts) >= 4 {
			envID := parts[3]
			if s.resourceRepo != nil {
				res, err := s.resourceRepo.ListResources(ctx, envID, "", 100, 0)
				if err == nil {
					bytes, _ := json.MarshalIndent(res, "", "  ")
					return protocol.ReadResourceResult{
						Contents: []protocol.ResourceContent{{URI: uri, MimeType: "application/json", Text: string(bytes)}},
					}, nil
				}
			}
		}
	} else if strings.HasPrefix(uri, "uptimefoundry://incidents/") && strings.HasSuffix(uri, "/evidence") {
		parts := strings.Split(uri, "/")
		if len(parts) >= 4 {
			incID := parts[3]
			if s.incidentRepo != nil {
				inc, err := s.incidentRepo.GetIncidentByID(ctx, incID)
				if err == nil {
					bytes, _ := json.MarshalIndent(inc, "", "  ")
					return protocol.ReadResourceResult{
						Contents: []protocol.ResourceContent{{URI: uri, MimeType: "application/json", Text: string(bytes)}},
					}, nil
				}
			}
		}
	}

	return protocol.ReadResourceResult{
		Contents: []protocol.ResourceContent{{URI: uri, MimeType: "text/plain", Text: fmt.Sprintf("Resource at %s", uri)}},
	}, nil
}

func (s *Server) listPrompts() []protocol.Prompt {
	return []protocol.Prompt{
		{
			Name:        "triage_incident",
			Description: "Assembles target resource subgraph, active anomalies, log errors, and top-matching runbook steps into a zero-shot prompt optimized for SRE root-cause diagnosis.",
			Arguments: []protocol.PromptArgument{
				{Name: "incident_id", Description: "UUID of the incident to triage", Required: true},
			},
		},
	}
}

func (s *Server) getPrompt(ctx context.Context, params protocol.GetPromptParams) (protocol.GetPromptResult, error) {
	if params.Name != "triage_incident" {
		return protocol.GetPromptResult{}, fmt.Errorf("unknown prompt: %s", params.Name)
	}

	incID := params.Arguments["incident_id"]
	if incID == "" {
		return protocol.GetPromptResult{}, fmt.Errorf("missing required argument 'incident_id'")
	}

	var incTitle, incDesc, primaryResID string
	if s.incidentRepo != nil {
		inc, err := s.incidentRepo.GetIncidentByID(ctx, incID)
		if err == nil {
			incTitle = inc.Title
			incDesc = inc.Description
			primaryResID = inc.PrimaryResourceID
		}
	}

	promptText := fmt.Sprintf("You are an expert SRE investigator analyzing Incident %s: %s\nDescription: %s\n\n", incID, incTitle, incDesc)

	if s.graphRAG != nil && primaryResID != "" {
		grounded, err := s.graphRAG.AssembleContext(ctx, primaryResID, incTitle+" "+incDesc, 2)
		if err == nil {
			promptText += grounded.PromptContext
		}
	}

	promptText += "\n\nTask: Analyze the evidence above. Identify the most probable root cause, verify supporting signals, cite internal runbooks using markdown links, and propose non-destructive remediation steps requiring human approval."

	return protocol.GetPromptResult{
		Description: "Zero-shot SRE Triage Prompt",
		Messages: []protocol.PromptMessage{
			{
				Role:    "user",
				Content: protocol.ToolContent{Type: "text", Text: promptText},
			},
		},
	}, nil
}
