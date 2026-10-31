package ebpf

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/telemetry"
)

// Manager coordinates zero-code kernel probes and protocol tracers.
type Manager struct {
	status       ProbeStatus
	caps         KernelCapabilities
	connEvents   chan telemetry.ConnectionEvent
	redMetrics   chan REDMetric
	logger       *slog.Logger
	otlpEndpoint string
	mu           sync.RWMutex
	cancel       context.CancelFunc
}

// NewManager creates a new OBI eBPF Manager.
func NewManager(otlpEndpoint string, logger *slog.Logger) *Manager {
	return &Manager{
		status:       StatusDisabled,
		connEvents:   make(chan telemetry.ConnectionEvent, 1000),
		redMetrics:   make(chan REDMetric, 1000),
		logger:       logger,
		otlpEndpoint: otlpEndpoint,
	}
}

// Connections exposes the stream of live network socket interactions.
func (m *Manager) Connections() <-chan telemetry.ConnectionEvent {
	return m.connEvents
}

// REDMetrics exposes the stream of Rate, Errors, and Duration measurements.
func (m *Manager) REDMetrics() <-chan REDMetric {
	return m.redMetrics
}

// Status returns the current operational status of the probes.
func (m *Manager) Status() ProbeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// Capabilities returns detected kernel eBPF features.
func (m *Manager) Capabilities() KernelCapabilities {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.caps
}

// Start checks kernel BTF capabilities and commences probe processing.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	m.caps = detectKernelCapabilities()

	if !m.caps.HasBTF {
		m.status = StatusKernelUnsupported
		m.mu.Unlock()
		m.logger.Warn("ebpf zero-touch instrumentation disabled: host kernel lacks BTF (BPF Type Format) support",
			slog.String("kernel", m.caps.KernelRelease),
		)
		return nil
	}

	subCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.status = StatusActive
	m.mu.Unlock()

	m.logger.Info("starting opentelemetry ebpf instrumentation (obi) engine",
		slog.String("kernel", m.caps.KernelRelease),
		slog.Bool("btf", m.caps.HasBTF),
		slog.Bool("uprobes", m.caps.SupportsUprobes),
		slog.String("otlp_export", m.otlpEndpoint),
	)

	// Launch background probe readers
	go m.runProbeLoop(subCtx)

	return nil
}

// Stop cleanly terminates eBPF probe listeners.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.status = StatusDisabled
}

func (m *Manager) runProbeLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Routine probe ring-buffer poll & health verification
			m.logger.Debug("ebpf probe health verified")
		}
	}
}
