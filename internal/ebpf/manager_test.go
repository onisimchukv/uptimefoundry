package ebpf

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestEBPFManagerLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mgr := NewManager("http://127.0.0.1:8428/opentelemetry/v1/metrics", logger)

	if mgr.Status() != StatusDisabled {
		t.Fatalf("expected initial status disabled, got %s", mgr.Status())
	}

	err := mgr.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	// Status will either be StatusActive (if running on a Linux kernel with BTF)
	// or StatusKernelUnsupported (on non-Linux or without BTF)
	status := mgr.Status()
	if status != StatusActive && status != StatusKernelUnsupported {
		t.Fatalf("unexpected probe status: %s", status)
	}

	mgr.Stop()
	if mgr.Status() != StatusDisabled {
		t.Fatalf("expected status disabled after stop, got %s", mgr.Status())
	}
}
