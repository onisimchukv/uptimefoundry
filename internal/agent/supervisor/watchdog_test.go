// SPDX-License-Identifier: MIT

package supervisor

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

type mockSampler struct {
	mu    sync.Mutex
	stats ResourceStats
}

func (m *mockSampler) Sample() (ResourceStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stats, nil
}

func (m *mockSampler) Set(stats ResourceStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats = stats
}

func TestWatchdog_NormalOperation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sampler := &mockSampler{}
	cfg := DefaultWatchdogConfig()
	cfg.CPUPercentCeiling = 10.0
	cfg.MemoryRSSBytesCeiling = 384 * 1024 * 1024

	wd := NewWatchdog(cfg, sampler, logger)

	// Sample within normal limits
	sampler.Set(ResourceStats{
		CPUPercent:     2.0,
		MemoryRSSBytes: 64 * 1024 * 1024,
	})

	level := wd.Evaluate(sampler.stats)
	if level != ShedLevelNormal {
		t.Fatalf("expected ShedLevelNormal (0), got %v", level)
	}
}

func TestWatchdog_SheddingLadderProgression(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sampler := &mockSampler{}
	cfg := DefaultWatchdogConfig()
	cfg.CPUPercentCeiling = 5.0

	var transitions [][2]ShedLevel
	var mu sync.Mutex
	cfg.OnShedChange = func(oldLevel, newLevel ShedLevel) {
		mu.Lock()
		defer mu.Unlock()
		transitions = append(transitions, [2]ShedLevel{oldLevel, newLevel})
	}

	wd := NewWatchdog(cfg, sampler, logger)

	// Exceed CPU threshold: should step up to Level 1
	lvl1 := wd.Evaluate(ResourceStats{CPUPercent: 12.0})
	if lvl1 != ShedLevelStopOptional {
		t.Fatalf("expected level 1 (StopOptional), got %v", lvl1)
	}

	// Exceed again: should step up to Level 2
	lvl2 := wd.Evaluate(ResourceStats{CPUPercent: 14.0})
	if lvl2 != ShedLevelRaiseFlush {
		t.Fatalf("expected level 2 (RaiseFlush), got %v", lvl2)
	}

	// Exceed again: should step up to Level 3
	lvl3 := wd.Evaluate(ResourceStats{CPUPercent: 15.0})
	if lvl3 != ShedLevelTurnOffProto {
		t.Fatalf("expected level 3 (TurnOffProto), got %v", lvl3)
	}

	// Exceed again: should step up to Level 4
	lvl4 := wd.Evaluate(ResourceStats{CPUPercent: 16.0})
	if lvl4 != ShedLevelReduceFlows {
		t.Fatalf("expected level 4 (ReduceFlows), got %v", lvl4)
	}

	// Exceed again: should step up to Level 5 (detach probes)
	lvl5 := wd.Evaluate(ResourceStats{CPUPercent: 18.0})
	if lvl5 != ShedLevelDetachProbes {
		t.Fatalf("expected level 5 (DetachProbes), got %v", lvl5)
	}

	// Exceeding beyond level 5 should remain at ceiling level 5
	lvl5Max := wd.Evaluate(ResourceStats{CPUPercent: 20.0})
	if lvl5Max != ShedLevelDetachProbes {
		t.Fatalf("expected level to remain capped at 5, got %v", lvl5Max)
	}
}

func TestWatchdog_SlowRecoveryHysteresis(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sampler := &mockSampler{}
	cfg := DefaultWatchdogConfig()
	cfg.CPUPercentCeiling = 10.0
	cfg.RecoveryWindow = 50 * time.Millisecond // fast window for unit test

	wd := NewWatchdog(cfg, sampler, logger)

	// Step up to Level 2
	wd.Evaluate(ResourceStats{CPUPercent: 15.0})
	wd.Evaluate(ResourceStats{CPUPercent: 15.0})
	if wd.CurrentLevel() != ShedLevelRaiseFlush {
		t.Fatalf("expected level 2, got %v", wd.CurrentLevel())
	}

	// Normal reading immediately: must NOT step down immediately
	lvl := wd.Evaluate(ResourceStats{CPUPercent: 2.0})
	if lvl != ShedLevelRaiseFlush {
		t.Fatalf("expected hysteresis to hold level 2, got %v", lvl)
	}

	// Sleep past RecoveryWindow
	time.Sleep(60 * time.Millisecond)

	// Evaluate again: must step down at most ONE step (2 -> 1)
	lvl = wd.Evaluate(ResourceStats{CPUPercent: 2.0})
	if lvl != ShedLevelStopOptional {
		t.Fatalf("expected single step down to level 1, got %v", lvl)
	}

	// Evaluate again immediately: must remain at level 1
	lvl = wd.Evaluate(ResourceStats{CPUPercent: 2.0})
	if lvl != ShedLevelStopOptional {
		t.Fatalf("expected level 1 to hold before second recovery window, got %v", lvl)
	}

	// Sleep past second recovery window
	time.Sleep(60 * time.Millisecond)

	// Evaluate again: steps down from 1 -> 0 (Normal)
	lvl = wd.Evaluate(ResourceStats{CPUPercent: 2.0})
	if lvl != ShedLevelNormal {
		t.Fatalf("expected recovery to level 0 (Normal), got %v", lvl)
	}
}

func TestWatchdog_RingBufferDropRateShedding(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sampler := &mockSampler{}
	cfg := DefaultWatchdogConfig()
	cfg.RingBufferDropRateCeiling = 0.01 // 1%
	cfg.DropRateSustainedWindow = 20 * time.Millisecond

	wd := NewWatchdog(cfg, sampler, logger)

	// Initial sample baseline
	wd.Evaluate(ResourceStats{
		RingBufferEvents: 1000,
		RingBufferDrops:  0,
	})

	// Sample with 5% drop rate (> 1%)
	wd.Evaluate(ResourceStats{
		RingBufferEvents: 1950,
		RingBufferDrops:  50, // 50 / 1000 = 5%
	})

	// Before sustained window, should not shed yet
	if wd.CurrentLevel() != ShedLevelNormal {
		t.Fatalf("expected level 0 before sustained window, got %v", wd.CurrentLevel())
	}

	// Sleep past sustained window
	time.Sleep(30 * time.Millisecond)

	// Next sample with high drop rate sustained: should trigger step up
	lvl := wd.Evaluate(ResourceStats{
		RingBufferEvents: 2900,
		RingBufferDrops:  100, // another 50 drops / 1000 events = 5%
	})

	if lvl != ShedLevelStopOptional {
		t.Fatalf("expected step up to level 1 on sustained drop rate, got %v", lvl)
	}
}
