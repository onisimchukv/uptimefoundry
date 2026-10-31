package supervisor

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"time"
)

// GenerateDiagnosticsBundle creates a compressed tar.gz archive per Section 14.5.
// Contains: capability report, effective policy (masked), supervisor status, and buffer stats.
// Excludes private keys, auth tokens, payloads, or captured text.
func (s *Supervisor) GenerateDiagnosticsBundle() ([]byte, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("diagnostics-%s-%s.tar.gz", s.id.AgentID, timestamp)

	addJSONFile := func(name string, v any) error {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Name:    name,
			Mode:    0600,
			Size:    int64(len(data)),
			ModTime: time.Now().UTC(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	}

	// 1. Capability Report
	if err := addJSONFile("capability_report.json", s.capability); err != nil {
		return nil, "", fmt.Errorf("archive capability report: %w", err)
	}

	// 2. Effective Policy (already masked, no private secrets)
	if err := addJSONFile("effective_policy.json", s.effective); err != nil {
		return nil, "", fmt.Errorf("archive effective policy: %w", err)
	}

	// 3. Supervisor Status Snapshot
	walBytes, unacked, drops, _ := s.wal.Stats()
	shed := int(s.watchdog.CurrentLevel())
	status := map[string]any{
		"agent_id":           s.id.AgentID,
		"host_fingerprint":   s.id.HostFingerprint,
		"tier":               s.capability.Tier.String(),
		"connected":          s.connected,
		"last_contact":       s.lastContact,
		"wal_bytes":          walBytes,
		"unacked_messages":   unacked,
		"shed_level":         shed,
		"shed_level_name":    s.watchdog.CurrentLevel().String(),
		"drop_counters":      drops,
		"policy_conflicts":   s.effective.Conflicts,
		"kill_switch":        s.effective.KillSwitchActive,
		"certificate_expiry": s.id.ExpiresAt,
	}
	if err := addJSONFile("supervisor_status.json", status); err != nil {
		return nil, "", fmt.Errorf("archive supervisor status: %w", err)
	}

	// 4. Buffer & Flow Statistics
	bufferStats := map[string]any{
		"wal_bytes":     walBytes,
		"unacked":       unacked,
		"drop_counters": drops,
		"active_flows":  s.flowTable.ActiveFlowCount(),
	}
	if err := addJSONFile("buffer_stats.json", bufferStats); err != nil {
		return nil, "", fmt.Errorf("archive buffer stats: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, "", err
	}
	if err := gzw.Close(); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), filename, nil
}
