package wal

import (
	"path/filepath"
	"testing"

	"github.com/onisimchukv/uptimefoundry/internal/agent/protocol"
)

func TestWALBufferingAndPriority(t *testing.T) {
	tempDir := t.TempDir()
	// Set very small max size (e.g. 1 KB) to trigger backpressure in unit test
	w, err := New(tempDir, 1) // 1 MB
	if err != nil {
		t.Fatalf("failed to create wal: %v", err)
	}

	// 1. Push High-Priority Heartbeat
	hbMsg := &protocol.AgentMessage{
		Class:     protocol.ClassHeartbeat,
		Heartbeat: &protocol.HeartbeatPayload{HealthStatus: "healthy"},
	}
	if err := w.Push(hbMsg); err != nil {
		t.Fatalf("failed to push heartbeat: %v", err)
	}
	if hbMsg.Sequence != 1 {
		t.Fatalf("expected sequence 1, got %d", hbMsg.Sequence)
	}

	// 2. Push Inventory Delta
	invMsg := &protocol.AgentMessage{
		Class:     protocol.ClassInventory,
		Inventory: &protocol.InventoryDeltaPayload{},
	}
	if err := w.Push(invMsg); err != nil {
		t.Fatalf("failed to push inventory: %v", err)
	}
	if invMsg.Sequence != 2 {
		t.Fatalf("expected sequence 2, got %d", invMsg.Sequence)
	}

	// 3. Peek
	records := w.Peek(10)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// 4. Acknowledge sequence 1 for Heartbeat
	w.Acknowledge(protocol.ClassHeartbeat, 1)

	recordsAfterAck := w.Peek(10)
	if len(recordsAfterAck) != 1 || recordsAfterAck[0].Class != protocol.ClassInventory {
		t.Fatalf("expected 1 remaining record of class inventory, got %d", len(recordsAfterAck))
	}
}

func TestSegmentDiskReadWrite(t *testing.T) {
	tempDir := t.TempDir()
	segPath := filepath.Join(tempDir, "00001.seg")

	records := []*Record{
		{Data: []byte(`{"msg":"first test record"}`)},
		{Data: []byte(`{"msg":"second test record"}`)},
	}

	if err := WriteSegment(segPath, records); err != nil {
		t.Fatalf("failed to write segment: %v", err)
	}

	readBack, err := ReadSegment(segPath)
	if err != nil {
		t.Fatalf("failed to read segment: %v", err)
	}

	if len(readBack) != 2 {
		t.Fatalf("expected 2 records read back, got %d", len(readBack))
	}
	if string(readBack[0].Data) != `{"msg":"first test record"}` {
		t.Fatalf("unexpected record data: %s", string(readBack[0].Data))
	}
}
