package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/forgeops/forgeops/internal/adapters"
	"github.com/forgeops/forgeops/internal/adapters/host"
	"github.com/forgeops/forgeops/internal/agent/capability"
	"github.com/forgeops/forgeops/internal/agent/identity"
	"github.com/forgeops/forgeops/internal/agent/policy"
	"github.com/forgeops/forgeops/internal/agent/protocol"
	"github.com/forgeops/forgeops/internal/agent/wal"
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
	ipcListener    net.Listener
	logger         *slog.Logger
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

	return &Supervisor{
		cfg:         cfg,
		localPolicy: locPol,
		effective:   locPol.Intersect(nil),
		prober:      prober,
		wal:         w,
		hostAdapter: hostAdapter,
		logger:      logger,
	}, nil
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

	// 4. Start Local IPC Socket for forgeops-agentctl
	if err := s.startIPC(); err != nil {
		s.logger.Warn("failed to start local unix ipc socket; agentctl disabled", slog.Any("error", err))
	}

	// 5. Start Periodic Discovery Loop
	go s.runDiscoveryLoop(ctx)

	// 6. Start Heartbeat Loop
	go s.runHeartbeatLoop(ctx)

	return nil
}

// Stop cleanly terminates supervisor workers and deletes the local IPC socket.
func (s *Supervisor) Stop() {
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

	// Note: in full production with mTLS, this is an outbound TLS call to /api/v1/agent/enroll
	s.logger.Info("created enrollment csr", slog.Int("csr_len", len(csrPEM)))
	return nil
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

	walBytes, unacked, drops, shed := s.wal.Stats()

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

// Status returns a snapshot of agent state for forgeops-agentctl.
func (s *Supervisor) Status() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	walBytes, unacked, drops, shed := s.wal.Stats()

	status := map[string]any{
		"agent_id":          s.id.AgentID,
		"host_fingerprint":  s.id.HostFingerprint,
		"tier":              s.capability.Tier.String(),
		"tier_number":       int(s.capability.Tier),
		"connected":         s.connected,
		"last_contact":      s.lastContact,
		"wal_bytes":         walBytes,
		"unacked_messages":  unacked,
		"shed_level":        shed,
		"drop_counters":     drops,
		"policy_conflicts":  s.effective.Conflicts,
		"kill_switch":       s.effective.KillSwitchActive,
		"certificate_expiry": s.id.ExpiresAt,
	}

	return status
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

	go func() {
		_ = http.Serve(listener, mux)
	}()

	return nil
}
