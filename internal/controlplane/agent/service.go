package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/forgeops/forgeops/internal/agent/protocol"
	"github.com/forgeops/forgeops/internal/core/models"
	"github.com/forgeops/forgeops/internal/core/ports"
)

// ActiveAgent tracks a live agent connection on the control plane.
type ActiveAgent struct {
	AgentID      string
	LastSeen     time.Time
	Tier         int
	HealthStatus string
	Capabilities map[string]any
	DropCounters map[string]uint64
	WALBytes     int64
	ShedLevel    int
}

// IngestionService coordinates agent messages, persistence, and policy distribution.
type IngestionService struct {
	ca           *CA
	resourceRepo ports.ResourceRepository
	activeAgents map[string]*ActiveAgent
	remotePolicy *protocol.PolicyPayload
	logger       *slog.Logger
	mu           sync.RWMutex
}

// NewIngestionService creates a new agent ingestion coordinator.
func NewIngestionService(ca *CA, resourceRepo ports.ResourceRepository, logger *slog.Logger) *IngestionService {
	// Default Remote Policy
	defaultPolicy := &protocol.PolicyPayload{
		Version:     1,
		ConfigJSON:  `{"modules":{"discovery":{"allowed":true},"flows":{"allowed":true}},"ebpf":{"max_tier":1}}`,
		GeneratedAt: time.Now().UTC(),
	}

	return &IngestionService{
		ca:           ca,
		resourceRepo: resourceRepo,
		activeAgents: make(map[string]*ActiveAgent),
		remotePolicy: defaultPolicy,
		logger:       logger,
	}
}

// HandleMessage processes a multiplexed message from an agent and produces the appropriate response.
func (s *IngestionService) HandleMessage(ctx context.Context, msg *protocol.AgentMessage) (*protocol.ServerMessage, error) {
	if msg == nil {
		return nil, fmt.Errorf("nil message received")
	}

	resp := &protocol.ServerMessage{
		AckSequence: msg.Sequence,
		AckClass:    msg.Class,
	}

	switch msg.Class {
	case protocol.ClassRegister:
		return s.handleRegister(ctx, msg, resp)

	case protocol.ClassHeartbeat:
		return s.handleHeartbeat(ctx, msg, resp)

	case protocol.ClassInventory:
		return s.handleInventory(ctx, msg, resp)

	case protocol.ClassFlow:
		return s.handleFlow(ctx, msg, resp)

	case protocol.ClassCertRenew:
		return s.handleCertRenew(ctx, msg, resp)

	case protocol.ClassTask:
		// Task execution result acknowledged
		return resp, nil

	default:
		return resp, nil
	}
}

func (s *IngestionService) handleRegister(ctx context.Context, msg *protocol.AgentMessage, resp *protocol.ServerMessage) (*protocol.ServerMessage, error) {
	reg := msg.Register
	if reg == nil {
		return nil, fmt.Errorf("missing register payload")
	}

	s.mu.Lock()
	s.activeAgents[msg.AgentID] = &ActiveAgent{
		AgentID:      msg.AgentID,
		LastSeen:     time.Now().UTC(),
		Capabilities: reg.Capabilities,
		HealthStatus: "registered",
	}
	policy := s.remotePolicy
	s.mu.Unlock()

	s.logger.Info("agent registered successfully",
		slog.String("agent_id", msg.AgentID),
		slog.String("version", reg.AgentVersion),
		slog.String("host_fingerprint", reg.HostFingerprint),
	)

	// Send current remote policy to agent
	resp.Policy = policy
	return resp, nil
}

func (s *IngestionService) handleHeartbeat(ctx context.Context, msg *protocol.AgentMessage, resp *protocol.ServerMessage) (*protocol.ServerMessage, error) {
	hb := msg.Heartbeat
	if hb == nil {
		return resp, nil
	}

	s.mu.Lock()
	if agent, ok := s.activeAgents[msg.AgentID]; ok {
		agent.LastSeen = time.Now().UTC()
		agent.Tier = hb.CurrentTier
		agent.HealthStatus = hb.HealthStatus
		agent.DropCounters = hb.DropCounters
		agent.WALBytes = hb.WALBytes
		agent.ShedLevel = hb.ShedLevel
	}
	s.mu.Unlock()

	s.logger.Debug("received agent heartbeat",
		slog.String("agent_id", msg.AgentID),
		slog.String("health", hb.HealthStatus),
		slog.Int("tier", hb.CurrentTier),
		slog.Int64("wal_bytes", hb.WALBytes),
	)

	return resp, nil
}

func (s *IngestionService) handleInventory(ctx context.Context, msg *protocol.AgentMessage, resp *protocol.ServerMessage) (*protocol.ServerMessage, error) {
	inv := msg.Inventory
	if inv == nil || s.resourceRepo == nil {
		return resp, nil
	}

	// Persist upserted resources
	for _, res := range inv.UpsertedResources {
		if err := s.resourceRepo.UpsertResource(ctx, res); err != nil {
			s.logger.Warn("failed to persist resource from agent inventory",
				slog.String("agent_id", msg.AgentID),
				slog.String("resource", res.Name),
				slog.Any("error", err),
			)
		}
	}

	// Persist relationships
	for _, rel := range inv.Relationships {
		if err := s.resourceRepo.UpsertRelationship(ctx, rel); err != nil {
			s.logger.Warn("failed to persist relationship from agent inventory",
				slog.String("agent_id", msg.AgentID),
				slog.String("rel_id", rel.ID),
				slog.Any("error", err),
			)
		}
	}

	s.logger.Info("persisted agent inventory delta",
		slog.String("agent_id", msg.AgentID),
		slog.Int("resources", len(inv.UpsertedResources)),
		slog.Int("relationships", len(inv.Relationships)),
	)

	return resp, nil
}

func (s *IngestionService) handleFlow(ctx context.Context, msg *protocol.AgentMessage, resp *protocol.ServerMessage) (*protocol.ServerMessage, error) {
	flow := msg.Flow
	if flow == nil || s.resourceRepo == nil {
		return resp, nil
	}

	now := time.Now().UTC()
	for _, edge := range flow.Edges {
		rel := &models.Relationship{
			ID:               uuid.NewMD5(uuid.NameSpaceDNS, []byte(edge.SourceWorkload+":"+edge.DestWorkload+":"+edge.Protocol)).String(),
			SourceResourceID: edge.SourceWorkload,
			TargetResourceID: edge.DestWorkload,
			RelationshipType: models.RelConnectsTo,
			Confidence:       1.0,
			EvidenceSource:   "ebpf_flow_tracker",
			ValidFrom:        edge.FirstSeen,
			Weight:           edge.MeanRTTMs, // Network latency RTT in ms
			Attributes: map[string]any{
				"min_rtt_ms":   edge.MinRTTMs,
				"p95_rtt_ms":   edge.P95RTTMs,
				"bytes_sent":   edge.BytesSent,
				"bytes_recv":   edge.BytesRecv,
				"retransmits":  edge.Retransmits,
				"dest_port":    edge.DestPort,
				"protocol":     edge.Protocol,
				"service_vip":  edge.ServiceVIP,
				"last_updated": now,
			},
		}

		if err := s.resourceRepo.UpsertRelationship(ctx, rel); err != nil {
			s.logger.Warn("failed to upsert flow edge",
				slog.String("source", edge.SourceWorkload),
				slog.String("dest", edge.DestWorkload),
				slog.Any("error", err),
			)
		}
	}

	s.logger.Info("persisted agent flow delta",
		slog.String("agent_id", msg.AgentID),
		slog.Int("edges", len(flow.Edges)),
	)

	return resp, nil
}

func (s *IngestionService) handleCertRenew(ctx context.Context, msg *protocol.AgentMessage, resp *protocol.ServerMessage) (*protocol.ServerMessage, error) {
	renewReq := msg.CertRenew
	if renewReq == nil {
		return nil, fmt.Errorf("missing cert renew payload")
	}

	certPEM, expiresAt, err := s.ca.RenewCertificate(msg.AgentID, "", renewReq.CSR)
	if err != nil {
		return nil, fmt.Errorf("certificate renewal failed: %w", err)
	}

	resp.CertRenewResp = &protocol.CertRenewResponse{
		CertificateChain: [][]byte{certPEM},
		ExpiresAt:        expiresAt,
	}

	s.logger.Info("renewed agent certificate",
		slog.String("agent_id", msg.AgentID),
		slog.Time("expires_at", expiresAt),
	)

	return resp, nil
}

// ActiveAgents returns a snapshot of connected agents and their health.
func (s *IngestionService) ActiveAgents() []*ActiveAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*ActiveAgent
	for _, a := range s.activeAgents {
		list = append(list, a)
	}
	return list
}
