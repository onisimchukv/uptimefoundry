// SPDX-License-Identifier: MIT

package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/adapters"
	"github.com/onisimchukv/uptimefoundry/internal/adapters/host"
	"github.com/onisimchukv/uptimefoundry/internal/agent/capability"
	"github.com/onisimchukv/uptimefoundry/internal/agent/flows"
	"github.com/onisimchukv/uptimefoundry/internal/agent/identity"
	"github.com/onisimchukv/uptimefoundry/internal/agent/obi"
	"github.com/onisimchukv/uptimefoundry/internal/agent/policy"
	"github.com/onisimchukv/uptimefoundry/internal/agent/protocol"
	"github.com/onisimchukv/uptimefoundry/internal/agent/wal"
)

// Config configures the supervisor daemon.
type Config struct {
	ControlPlaneURL string
	LocalPolicyPath string
	StateDir        string
	WALMaxMB        int
	SocketPath      string
	BootstrapToken  string
	EnvironmentID   string
}

// Supervisor coordinates agent lifecycle, unprivileged discovery, WAL, and control plane streaming.
type Supervisor struct {
	cfg            Config
	id             *identity.Identity
	localPolicy    *policy.LocalPolicy
	effective      *policy.EffectivePolicy
	prober         *capability.Prober
	capability     *capability.Report
	wal            *wal.WAL
	hostAdapter    adapters.DiscoveryAdapter
	obiWrapper     *obi.Wrapper
	flowTable      *flows.Table
	dnsTracker     *flows.DNSTracker
	watchdog       *Watchdog
	ipcListener    net.Listener
	logger         *slog.Logger
	httpClient     *http.Client
	connected      bool
	lastContact    time.Time
	streamEpoch    uint64
	mu             sync.RWMutex
	cancel         context.CancelFunc
}

// New creates a new agent supervisor.
func New(cfg Config, logger *slog.Logger) (*Supervisor, error) {
	if cfg.StateDir == "" {
		cfg.StateDir = "state"
	}
	if cfg.SocketPath == "" {
		cfg.SocketPath = filepath.Join(cfg.StateDir, "agent.sock")
	}

	_ = os.MkdirAll(cfg.StateDir, 0700)

	// 1. Load Local Security Ceiling Policy
	locPol, err := policy.LoadLocalPolicy(cfg.LocalPolicyPath)
	if err != nil {
		return nil, fmt.Errorf("load local policy: %w", err)
	}

	// 2. Initialize WAL
	w, err := wal.New(filepath.Join(cfg.StateDir, "wal"), cfg.WALMaxMB)
	if err != nil {
		return nil, fmt.Errorf("init wal: %w", err)
	}

	prober := capability.NewProber(logger)
	hostAdapter := host.New(logger)
	effPolicy := locPol.Intersect(nil)
	obiWrapper := obi.NewWrapper(effPolicy, logger)
	flowTable := flows.NewTable(nil, nil, logger)
	dnsTracker := flows.NewDNSTracker(effPolicy.DNS.FullNames)

	s := &Supervisor{
		cfg:         cfg,
		localPolicy: locPol,
		effective:   effPolicy,
		prober:      prober,
		wal:         w,
		hostAdapter: hostAdapter,
		obiWrapper:  obiWrapper,
		flowTable:   flowTable,
		dnsTracker:  dnsTracker,
		logger:      logger,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}

	wcfg := DefaultWatchdogConfig()
	wcfg.OnShedChange = s.handleShedChange
	s.watchdog = NewWatchdog(wcfg, nil, logger)

	return s, nil
}

// Start launches the supervisor, initiates capability probing, identity enrollment, and IPC listeners.
func (s *Supervisor) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// 1. Capability Probe
	s.capability = s.prober.Probe(ctx)

	// 2. Load or Generate Cryptographic Identity
	keyPath := filepath.Join(s.cfg.StateDir, "identity", "key.pem")
	privKey, err := identity.GenerateOrLoadKey(keyPath)
	if err != nil {
		return fmt.Errorf("initialize agent keypair: %w", err)
	}

	fingerprint := identity.ComputeHostFingerprint()
	s.id = &identity.Identity{
		PrivateKey:      privKey,
		HostFingerprint: fingerprint,
		KeyPath:         keyPath,
		CertPath:        filepath.Join(s.cfg.StateDir, "identity", "cert.pem"),
	}

	// 3. Load Certificate if already enrolled
	cert, rawPEM, err := identity.LoadCertificate(s.id.CertPath)
	if err == nil {
		s.id.Certificate = cert
		s.id.RawCertPEM = rawPEM
		s.id.ExpiresAt = cert.NotAfter
		s.id.AgentID = cert.Subject.CommonName
		s.logger.Info("loaded existing agent identity",
			slog.String("agent_id", s.id.AgentID),
			slog.Time("expires_at", s.id.ExpiresAt),
		)
	} else if s.cfg.BootstrapToken != "" {
		s.logger.Info("enrolling agent with bootstrap token",
			slog.String("fingerprint", fingerprint),
		)
		// Perform Enrollment against control plane
		if err := s.enroll(ctx); err != nil {
			s.logger.Warn("enrollment attempt failed; will retry in background", slog.Any("error", err))
		}
	} else {
		s.logger.Warn("agent un-enrolled and no bootstrap token provided; running in local inspection mode")
	}

	// 4. Start Local IPC Socket for uptimefoundry-agentctl
	if err := s.startIPC(); err != nil {
		s.logger.Warn("failed to start local unix ipc socket; agentctl disabled", slog.Any("error", err))
	}

	// 5. Start Periodic Discovery Loop
	go s.runDiscoveryLoop(ctx)

	// 6. Start Heartbeat Loop
	go s.runHeartbeatLoop(ctx)

	// 7. Start Flow Delta Flush Loop (Section 7.3)
	go s.runFlowFlushLoop(ctx)

	// 8. Start Watchdog Resource Monitor & Shedding Engine (Section 11.2)
	s.watchdog.Start(ctx)

	// 9. Start WAL Egress Streaming Loop
	go s.runEgressLoop(ctx)

	return nil
}

// Stop cleanly terminates supervisor workers and deletes the local IPC socket.
func (s *Supervisor) Stop() {
	if s.watchdog != nil {
		s.watchdog.Stop()
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.ipcListener != nil {
		_ = s.ipcListener.Close()
		_ = os.Remove(s.cfg.SocketPath)
	}
}

func (s *Supervisor) enroll(ctx context.Context) error {
	csrPEM, err := identity.CreateCSR(s.id.PrivateKey, "pending-enrollment")
	if err != nil {
		return fmt.Errorf("create enrollment csr: %w", err)
	}

	enrollURL := strings.TrimRight(s.cfg.ControlPlaneURL, "/") + "/api/v1/agent/enroll"
	reqBody := map[string]string{
		"token":            s.cfg.BootstrapToken,
		"host_fingerprint": s.id.HostFingerprint,
		"csr_pem":          string(csrPEM),
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal enroll request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, enrollURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create enroll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("enrollment http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("enrollment rejected (status %d): %s", resp.StatusCode, errResp["error"])
	}

	var enrollResp struct {
		AgentID        string    `json:"agent_id"`
		CertificatePEM string    `json:"certificate_pem"`
		CACertPEM      string    `json:"ca_cert_pem"`
		ExpiresAt      time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&enrollResp); err != nil {
		return fmt.Errorf("decode enrollment response: %w", err)
	}

	_ = os.MkdirAll(filepath.Dir(s.id.CertPath), 0700)
	if err := os.WriteFile(s.id.CertPath, []byte(enrollResp.CertificatePEM), 0600); err != nil {
		return fmt.Errorf("write agent cert: %w", err)
	}

	cert, rawPEM, err := identity.LoadCertificate(s.id.CertPath)
	if err != nil {
		return fmt.Errorf("load enrolled cert: %w", err)
	}

	s.mu.Lock()
	s.id.Certificate = cert
	s.id.RawCertPEM = rawPEM
	s.id.AgentID = enrollResp.AgentID
	s.id.ExpiresAt = enrollResp.ExpiresAt
	s.mu.Unlock()

	s.logger.Info("agent successfully enrolled with control plane",
		slog.String("agent_id", s.id.AgentID),
		slog.Time("expires_at", s.id.ExpiresAt),
	)
	return nil
}

func (s *Supervisor) runEgressLoop(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	endpoint := strings.TrimRight(s.cfg.ControlPlaneURL, "/") + "/api/v1/agent/message"

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.drainWAL(ctx, endpoint)
		}
	}
}

func (s *Supervisor) drainWAL(ctx context.Context, endpoint string) {
	batch := s.wal.Peek(10)
	if len(batch) == 0 {
		return
	}

	for _, msg := range batch {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.mu.RLock()
		if msg.AgentID == "" && s.id != nil {
			msg.AgentID = s.id.AgentID
		}
		s.mu.RUnlock()

		data, err := json.Marshal(msg)
		if err != nil {
			s.logger.Error("failed to marshal wal message for egress", slog.Any("error", err))
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
		if err != nil {
			s.logger.Error("failed to create egress request", slog.Any("error", err))
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			s.mu.Lock()
			s.connected = false
			s.mu.Unlock()
			s.logger.Debug("control plane unreachable during wal egress", slog.Any("error", err))
			return
		}

		if resp.StatusCode == http.StatusOK {
			var srvMsg protocol.ServerMessage
			if err := json.NewDecoder(resp.Body).Decode(&srvMsg); err == nil {
				s.wal.Acknowledge(srvMsg.AckClass, srvMsg.AckSequence)
			}
			resp.Body.Close()

			s.mu.Lock()
			s.connected = true
			s.lastContact = time.Now()
			s.mu.Unlock()
		} else {
			resp.Body.Close()
			s.mu.Lock()
			s.connected = false
			s.mu.Unlock()
			s.logger.Warn("control plane rejected agent message",
				slog.Int("status", resp.StatusCode),
				slog.Uint64("seq", msg.Sequence),
			)
			return
		}
	}
}

func (s *Supervisor) runDiscoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Run initial discovery sweep immediately
	s.collectAndBufferDiscovery(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectAndBufferDiscovery(ctx)
		}
	}
}

func (s *Supervisor) collectAndBufferDiscovery(ctx context.Context) {
	s.mu.RLock()
	allowed := s.effective.Modules["discovery"].Allowed
	envID := s.cfg.EnvironmentID
	agentID := s.id.AgentID
	s.mu.RUnlock()

	if !allowed {
		s.logger.Debug("discovery module disabled by effective policy")
		return
	}

	res, err := s.hostAdapter.Discover(ctx, envID)
	if err != nil {
		s.logger.Error("local host discovery failed", slog.Any("error", err))
		return
	}

	// Package into InventoryDeltaPayload and push to WAL
	invPayload := &protocol.InventoryDeltaPayload{
		UpsertedResources: res.Resources,
		Relationships:     res.Relationships,
		IntervalStart:     time.Now().Add(-30 * time.Second),
		IntervalEnd:       time.Now(),
	}

	msg := &protocol.AgentMessage{
		AgentID:   agentID,
		Class:     protocol.ClassInventory,
		Inventory: invPayload,
	}

	if err := s.wal.Push(msg); err != nil {
		s.logger.Warn("failed to buffer inventory delta to wal", slog.Any("error", err))
	}
}

func (s *Supervisor) runHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sendHeartbeat()
		}
	}
}

func (s *Supervisor) sendHeartbeat() {
	s.mu.RLock()
	agentID := s.id.AgentID
	tier := int(s.capability.Tier)
	conflicts := len(s.effective.Conflicts)
	s.mu.RUnlock()

	walBytes, unacked, drops, _ := s.wal.Stats()
	shed := int(s.watchdog.CurrentLevel())

	probeStates := make(map[string]string)
	for k, v := range s.capability.ProbeStates {
		probeStates[k] = string(v.Status)
	}

	health := "healthy"
	if !s.connected {
		health = "disconnected"
	}
	if shed > 0 {
		health = "degraded"
	}

	hb := &protocol.HeartbeatPayload{
		HealthStatus:    health,
		CurrentTier:     tier,
		ProbeStates:     probeStates,
		DropCounters:    drops,
		WALBytes:        walBytes,
		UnackedMessages: unacked,
		ShedLevel:       shed,
		PolicyConflicts: conflicts,
	}

	msg := &protocol.AgentMessage{
		AgentID:   agentID,
		Class:     protocol.ClassHeartbeat,
		Heartbeat: hb,
	}

	_ = s.wal.Push(msg)
}

func (s *Supervisor) runFlowFlushLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flushFlowDelta()
		}
	}
}

func (s *Supervisor) flushFlowDelta() {
	s.mu.RLock()
	allowed := s.effective.Modules["flows"].Allowed
	agentID := s.id.AgentID
	s.mu.RUnlock()

	if !allowed || s.flowTable == nil {
		return
	}

	delta := s.flowTable.FlushDelta(15)
	if len(delta.Edges) == 0 {
		return
	}

	msg := &protocol.AgentMessage{
		AgentID: agentID,
		Class:   protocol.ClassFlow,
		Flow:    &delta,
	}

	if err := s.wal.Push(msg); err != nil {
		s.logger.Warn("failed to push flow delta to wal", slog.Any("error", err))
	}
}

// Status returns a snapshot of agent state for uptimefoundry-agentctl.
func (s *Supervisor) Status() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusLocked()
}

func (s *Supervisor) statusLocked() map[string]any {
	walBytes, unacked, drops, _ := s.wal.Stats()
	shed := int(s.watchdog.CurrentLevel())

	agentID := ""
	fingerprint := ""
	var certExpiry time.Time
	if s.id != nil {
		agentID = s.id.AgentID
		fingerprint = s.id.HostFingerprint
		certExpiry = s.id.ExpiresAt
	}

	tierStr := "unknown"
	tierNum := 0
	if s.capability != nil {
		tierStr = s.capability.Tier.String()
		tierNum = int(s.capability.Tier)
	}

	var conflicts []string
	killSwitch := false
	if s.effective != nil {
		conflicts = s.effective.Conflicts
		killSwitch = s.effective.KillSwitchActive
	}

	status := map[string]any{
		"agent_id":           agentID,
		"host_fingerprint":   fingerprint,
		"tier":               tierStr,
		"tier_number":        tierNum,
		"connected":          s.connected,
		"last_contact":       s.lastContact,
		"wal_bytes":          walBytes,
		"unacked_messages":   unacked,
		"shed_level":         shed,
		"shed_level_name":    s.watchdog.CurrentLevel().String(),
		"drop_counters":      drops,
		"policy_conflicts":   conflicts,
		"kill_switch":        killSwitch,
		"certificate_expiry": certExpiry,
	}

	return status
}

func (s *Supervisor) handleShedChange(oldLevel, newLevel ShedLevel) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("applying watchdog shed transition",
		slog.Int("old_level", int(oldLevel)),
		slog.Int("new_level", int(newLevel)),
		slog.String("level_name", newLevel.String()),
	)

	// Level 1: Stop optional modules: host metrics, log reading
	if newLevel >= ShedLevelStopOptional {
		if mod, ok := s.effective.Modules["logs"]; ok {
			mod.Allowed = false
		}
	} else {
		if mod, ok := s.localPolicy.Modules["logs"]; ok {
			s.effective.Modules["logs"] = mod
		}
	}

	// Level 3: Turn off protocol parsing and TLS inspection (Tier 2)
	if newLevel >= ShedLevelTurnOffProto {
		if s.capability.Tier == capability.TierFullEBPF {
			s.capability.Tier = capability.TierReducedEBPF
		}
	} else if newLevel < ShedLevelTurnOffProto {
		if s.capability.Tier == capability.TierReducedEBPF {
			s.capability.Tier = capability.TierFullEBPF
		}
	}

	// Level 4: Reduce flow table to top-N flows by bytes
	if newLevel >= ShedLevelReduceFlows {
		if s.flowTable != nil {
			s.flowTable.PruneToTopN(500)
			s.flowTable.SetMaxEdges(500)
		}
	} else if newLevel < ShedLevelReduceFlows {
		if s.flowTable != nil {
			s.flowTable.SetMaxEdges(2000)
		}
	}

	// Level 5: Detach flow probes and fall back to procfs sampling (Tier 3)
	if newLevel >= ShedLevelDetachProbes {
		s.capability.Tier = capability.TierProcfs
	}
}

// startIPC creates the local Unix domain socket HTTP listener.
func (s *Supervisor) startIPC() error {
	_ = os.Remove(s.cfg.SocketPath)
	_ = os.MkdirAll(filepath.Dir(s.cfg.SocketPath), 0755)

	listener, err := net.Listen("unix", s.cfg.SocketPath)
	if err != nil {
		return err
	}
	s.ipcListener = listener
	_ = os.Chmod(s.cfg.SocketPath, 0660)

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.Status())
	})

	mux.HandleFunc("/policy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s.mu.RLock()
		eff := s.effective
		s.mu.RUnlock()
		_ = json.NewEncoder(w).Encode(eff)
	})

	mux.HandleFunc("/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		bundleBytes, filename, err := s.GenerateDiagnosticsBundle()
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.Header().Set("X-Bundle-Filename", filename)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bundleBytes)
	})

	go func() {
		_ = http.Serve(listener, mux)
	}()

	return nil
}
