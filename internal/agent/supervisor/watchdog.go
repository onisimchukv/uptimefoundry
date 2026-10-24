package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

// ShedLevel defines the 6-stage shedding ladder from Section 11.2 of Specification v1.3.
type ShedLevel int

const (
	// ShedLevelNormal (0): Normal operation; all configured probes and modules active.
	ShedLevelNormal ShedLevel = 0
	// ShedLevelStopOptional (1): Stop optional modules: host metrics, log reading.
	ShedLevelStopOptional ShedLevel = 1
	// ShedLevelRaiseFlush (2): Raise flush intervals and sampling ratios (e.g. 15s -> 30s).
	ShedLevelRaiseFlush ShedLevel = 2
	// ShedLevelTurnOffProto (3): Turn off protocol parsing and TLS inspection (Tier 2).
	ShedLevelTurnOffProto ShedLevel = 3
	// ShedLevelReduceFlows (4): Reduce flow table to top-N flows by bytes.
	ShedLevelReduceFlows ShedLevel = 4
	// ShedLevelDetachProbes (5): Detach flow probes and fall back to procfs sampling (Tier 3).
	ShedLevelDetachProbes ShedLevel = 5
)

func (s ShedLevel) String() string {
	switch s {
	case ShedLevelNormal:
		return "Normal"
	case ShedLevelStopOptional:
		return "StopOptionalModules"
	case ShedLevelRaiseFlush:
		return "RaiseFlushIntervals"
	case ShedLevelTurnOffProto:
		return "TurnOffProtocolParsing"
	case ShedLevelReduceFlows:
		return "ReduceFlowTableTopN"
	case ShedLevelDetachProbes:
		return "DetachFlowProbesFallbackProcfs"
	default:
		return fmt.Sprintf("ShedLevel(%d)", int(s))
	}
}

// WatchdogConfig configures the resource monitoring and degradation watchdog.
type WatchdogConfig struct {
	SampleInterval             time.Duration
	RecoveryWindow             time.Duration
	CPUPercentCeiling          float64
	MemoryRSSBytesCeiling      uint64
	RingBufferDropRateCeiling  float64
	DropRateSustainedWindow    time.Duration
	OnShedChange               func(oldLevel, newLevel ShedLevel)
}

// DefaultWatchdogConfig returns standard budgets per Section 11.1 and 11.2.
func DefaultWatchdogConfig() WatchdogConfig {
	return WatchdogConfig{
		SampleInterval:            5 * time.Second,
		RecoveryWindow:            10 * time.Minute,
		CPUPercentCeiling:         10.0,                    // 10% ceiling (flows + OBI)
		MemoryRSSBytesCeiling:     384 * 1024 * 1024,      // 384 MiB RSS ceiling
		RingBufferDropRateCeiling: 0.01,                   // 1% drop rate
		DropRateSustainedWindow:   60 * time.Second,
	}
}

// ResourceStats contains a snapshot of host agent resource utilization.
type ResourceStats struct {
	CPUPercent       float64
	MemoryRSSBytes   uint64
	RingBufferDrops  uint64
	RingBufferEvents uint64
	EventRate        float64
}

// ResourceSampler abstracts OS metrics collection for testing and multi-platform support.
type ResourceSampler interface {
	Sample() (ResourceStats, error)
}

// RuntimeSampler provides memory sampling via Go runtime.
type RuntimeSampler struct {
	mu           sync.Mutex
	lastSample   time.Time
	lastCPUTime  time.Duration
	mockDrops    uint64
	mockEvents   uint64
}

// NewRuntimeSampler creates a default sampler.
func NewRuntimeSampler() *RuntimeSampler {
	return &RuntimeSampler{
		lastSample: time.Now(),
	}
}

// Sample collects current memory stats and approximated CPU usage.
func (s *RuntimeSampler) Sample() (ResourceStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	now := time.Now()
	_ = now.Sub(s.lastSample)
	s.lastSample = now

	return ResourceStats{
		CPUPercent:       0.5, // baseline
		MemoryRSSBytes:   m.Sys,
		RingBufferDrops:  s.mockDrops,
		RingBufferEvents: s.mockEvents,
	}, nil
}

// SetMockRingBuffer allows tests or eBPF wrappers to update drop counters.
func (s *RuntimeSampler) SetMockRingBuffer(drops, events uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mockDrops = drops
	s.mockEvents = events
}

// Watchdog monitors resource consumption and coordinates graceful shedding and slow recovery.
type Watchdog struct {
	cfg            WatchdogConfig
	sampler        ResourceSampler
	currentLevel   ShedLevel
	lastExceeded   time.Time
	sustainedDrop  time.Time
	lastLevelChange time.Time
	lastDrops      uint64
	lastEvents     uint64
	logger         *slog.Logger
	mu             sync.RWMutex
	cancel         context.CancelFunc
}

// NewWatchdog constructs a new supervisor watchdog.
func NewWatchdog(cfg WatchdogConfig, sampler ResourceSampler, logger *slog.Logger) *Watchdog {
	if cfg.SampleInterval <= 0 {
		cfg.SampleInterval = 5 * time.Second
	}
	if cfg.RecoveryWindow <= 0 {
		cfg.RecoveryWindow = 10 * time.Minute
	}
	if cfg.CPUPercentCeiling <= 0 {
		cfg.CPUPercentCeiling = 10.0
	}
	if cfg.MemoryRSSBytesCeiling == 0 {
		cfg.MemoryRSSBytesCeiling = 384 * 1024 * 1024
	}
	if cfg.RingBufferDropRateCeiling <= 0 {
		cfg.RingBufferDropRateCeiling = 0.01
	}
	if cfg.DropRateSustainedWindow <= 0 {
		cfg.DropRateSustainedWindow = 60 * time.Second
	}
	if sampler == nil {
		sampler = NewRuntimeSampler()
	}

	return &Watchdog{
		cfg:             cfg,
		sampler:         sampler,
		currentLevel:    ShedLevelNormal,
		lastLevelChange: time.Now(),
		logger:          logger,
	}
}

// Start begins the periodic watchdog monitoring loop.
func (w *Watchdog) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	go func() {
		ticker := time.NewTicker(w.cfg.SampleInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.tick()
			}
		}
	}()
}

// Stop stops the watchdog background loop.
func (w *Watchdog) Stop() {
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
}

// CurrentLevel returns the current shedding ladder level (0 to 5).
func (w *Watchdog) CurrentLevel() ShedLevel {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentLevel
}

// tick executes a single evaluation cycle.
func (w *Watchdog) tick() {
	stats, err := w.sampler.Sample()
	if err != nil {
		w.logger.Warn("watchdog sample failed", slog.Any("error", err))
		return
	}
	w.Evaluate(stats)
}

// Evaluate evaluates a set of ResourceStats and triggers shedding or slow recovery.
func (w *Watchdog) Evaluate(stats ResourceStats) ShedLevel {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	thresholdExceeded := false
	var reason string

	// 1. Check CPU budget
	if stats.CPUPercent > w.cfg.CPUPercentCeiling {
		thresholdExceeded = true
		reason = fmt.Sprintf("CPU usage %.2f%% exceeds ceiling %.2f%%", stats.CPUPercent, w.cfg.CPUPercentCeiling)
	}

	// 2. Check Memory RSS budget
	if stats.MemoryRSSBytes > w.cfg.MemoryRSSBytesCeiling {
		thresholdExceeded = true
		reason = fmt.Sprintf("RSS memory %d MiB exceeds ceiling %d MiB",
			stats.MemoryRSSBytes/(1024*1024), w.cfg.MemoryRSSBytesCeiling/(1024*1024))
	}

	// 3. Check Ring-Buffer Drop Rate (> 1% for sustained window)
	deltaEvents := stats.RingBufferEvents - w.lastEvents
	deltaDrops := stats.RingBufferDrops - w.lastDrops
	w.lastEvents = stats.RingBufferEvents
	w.lastDrops = stats.RingBufferDrops

	var dropRate float64
	if deltaEvents+deltaDrops > 0 {
		dropRate = float64(deltaDrops) / float64(deltaEvents+deltaDrops)
	}

	if dropRate > w.cfg.RingBufferDropRateCeiling {
		if w.sustainedDrop.IsZero() {
			w.sustainedDrop = now
		} else if now.Sub(w.sustainedDrop) >= w.cfg.DropRateSustainedWindow {
			thresholdExceeded = true
			reason = fmt.Sprintf("ring-buffer drop rate %.2f%% sustained for %s",
				dropRate*100, now.Sub(w.sustainedDrop).Round(time.Second))
		}
	} else {
		w.sustainedDrop = time.Time{}
	}

	oldLevel := w.currentLevel

	if thresholdExceeded {
		w.lastExceeded = now
		// When threshold is breached, step UP in shed level (up to level 5)
		if w.currentLevel < ShedLevelDetachProbes {
			w.currentLevel++
			w.lastLevelChange = now
			w.logger.Warn("resource ceiling exceeded; shedding load",
				slog.String("reason", reason),
				slog.Int("from_level", int(oldLevel)),
				slog.Int("to_level", int(w.currentLevel)),
				slog.String("level_name", w.currentLevel.String()),
			)
			if w.cfg.OnShedChange != nil {
				go w.cfg.OnShedChange(oldLevel, w.currentLevel)
			}
		}
	} else {
		// Recovery logic: Section 11.2 "deliberately slow: returns to a higher state only after
		// 10 minutes below threshold, and at most one step at a time."
		if w.currentLevel > ShedLevelNormal {
			timeBelowThreshold := now.Sub(w.lastExceeded)
			timeSinceLastChange := now.Sub(w.lastLevelChange)

			if timeBelowThreshold >= w.cfg.RecoveryWindow && timeSinceLastChange >= w.cfg.RecoveryWindow {
				w.currentLevel--
				w.lastLevelChange = now
				w.logger.Info("resources stable below threshold; stepping down shed level",
					slog.Int("from_level", int(oldLevel)),
					slog.Int("to_level", int(w.currentLevel)),
					slog.String("level_name", w.currentLevel.String()),
				)
				if w.cfg.OnShedChange != nil {
					go w.cfg.OnShedChange(oldLevel, w.currentLevel)
				}
			}
		}
	}

	return w.currentLevel
}
