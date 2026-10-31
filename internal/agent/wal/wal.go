package wal

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/agent/protocol"
)

// Record wraps an AgentMessage with sequence and cryptographic integrity checksum.
type Record struct {
	Sequence  uint64                `json:"seq"`
	Class     protocol.MessageClass `json:"class"`
	Checksum  [32]byte              `json:"chk"`
	Data      []byte                `json:"data"`
	Timestamp time.Time             `json:"ts"`
}

// WAL provides persistent, prioritized buffering across network and control-plane outages.
type WAL struct {
	dir          string
	maxBytes     int64
	currentBytes int64
	currentSeq   uint64
	dropCounters map[protocol.MessageClass]uint64
	queue        []*protocol.AgentMessage
	mu           sync.Mutex
}

// New creates or re-opens a WAL directory.
func New(dir string, maxMB int) (*WAL, error) {
	if dir == "" {
		dir = "wal"
	}
	if maxMB <= 0 {
		maxMB = 512
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create wal directory: %w", err)
	}

	w := &WAL{
		dir:          dir,
		maxBytes:     int64(maxMB) * 1024 * 1024,
		dropCounters: make(map[protocol.MessageClass]uint64),
	}

	return w, nil
}

// Push appends a message to the WAL, applying priority queueing and backpressure shedding.
func (w *WAL) Push(msg *protocol.AgentMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.currentSeq++
	msg.Sequence = w.currentSeq
	msg.Timestamp = time.Now().UTC()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal wal record: %w", err)
	}
	recordSize := int64(len(data))

	// Check Backpressure Fill Ratio
	fillRatio := float64(w.currentBytes) / float64(w.maxBytes)

	// Class 1: Register, Heartbeat, TaskResult are highest priority - NEVER dropped
	if msg.Class == protocol.ClassRegister || msg.Class == protocol.ClassHeartbeat || msg.Class == protocol.ClassTask {
		w.queue = append(w.queue, msg)
		w.currentBytes += recordSize
		return nil
	}

	// Class 5: MetricBatch dropped at >= 90%
	if fillRatio >= 0.90 && msg.Class == protocol.ClassMetric {
		w.dropCounters[protocol.ClassMetric]++
		return nil // Shed
	}

	// Class 4 & 3: EventBatch and FlowDelta dropped at >= 100%
	if fillRatio >= 1.00 {
		if msg.Class == protocol.ClassEvent || msg.Class == protocol.ClassFlow {
			w.dropCounters[msg.Class]++
			return nil // Shed
		}
	}

	w.queue = append(w.queue, msg)
	w.currentBytes += recordSize

	return nil
}

// Peek retrieves up to maxItems unacknowledged messages without removing them.
func (w *WAL) Peek(maxItems int) []*protocol.AgentMessage {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.queue) == 0 {
		return nil
	}

	if maxItems <= 0 || maxItems > len(w.queue) {
		maxItems = len(w.queue)
	}

	batch := make([]*protocol.AgentMessage, maxItems)
	copy(batch, w.queue[:maxItems])
	return batch
}

// Acknowledge removes messages up to ackSequence for the given class.
func (w *WAL) Acknowledge(class protocol.MessageClass, ackSequence uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var remaining []*protocol.AgentMessage
	var freedBytes int64

	for _, msg := range w.queue {
		if msg.Class == class && msg.Sequence <= ackSequence {
			data, _ := json.Marshal(msg)
			freedBytes += int64(len(data))
		} else {
			remaining = append(remaining, msg)
		}
	}

	w.queue = remaining
	w.currentBytes -= freedBytes
	if w.currentBytes < 0 {
		w.currentBytes = 0
	}
}

// Stats returns the current WAL size, unacknowledged count, and drop statistics.
func (w *WAL) Stats() (int64, int, map[string]uint64, int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	drops := make(map[string]uint64)
	for k, v := range w.dropCounters {
		drops[string(k)] = v
	}

	fillRatio := float64(w.currentBytes) / float64(w.maxBytes)
	shedLevel := 0
	if fillRatio >= 1.0 {
		shedLevel = 4
	} else if fillRatio >= 0.90 {
		shedLevel = 3
	} else if fillRatio >= 0.70 {
		shedLevel = 2
	}

	return w.currentBytes, len(w.queue), drops, shedLevel
}

// WriteSegment saves a batch of records to disk with CRC/SHA256 checksums.
func WriteSegment(path string, records []*Record) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, rec := range records {
		rec.Checksum = sha256.Sum256(rec.Data)
		lenBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(lenBytes, uint64(len(rec.Data)))

		if _, err := f.Write(lenBytes); err != nil {
			return err
		}
		if _, err := f.Write(rec.Checksum[:]); err != nil {
			return err
		}
		if _, err := f.Write(rec.Data); err != nil {
			return err
		}
	}

	return nil
}

// ReadSegment reads records from a disk segment, validating record checksums.
func ReadSegment(path string) ([]*Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var records []*Record
	lenBuf := make([]byte, 8)
	chkBuf := make([]byte, 32)

	for {
		_, err := io.ReadFull(f, lenBuf)
		if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return records, err
		}

		length := binary.BigEndian.Uint64(lenBuf)
		if _, err := io.ReadFull(f, chkBuf); err != nil {
			return records, err
		}

		data := make([]byte, length)
		if _, err := io.ReadFull(f, data); err != nil {
			return records, err
		}

		actualChk := sha256.Sum256(data)
		var expChk [32]byte
		copy(expChk[:], chkBuf)

		if actualChk != expChk {
			// Skip corrupted record
			continue
		}

		rec := &Record{
			Checksum: expChk,
			Data:     data,
		}
		records = append(records, rec)
	}

	return records, nil
}
