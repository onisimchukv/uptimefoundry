// SPDX-License-Identifier: MIT

package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/mcp/protocol"
)

func TestClient_CallAndClose(t *testing.T) {
	// Setup in-memory pipes:
	// client writes to clientInWrite, read by serverInRead
	// server writes to serverOutWrite, read by clientOutRead
	serverInRead, clientInWrite := io.Pipe()
	clientOutRead, serverOutWrite := io.Pipe()

	client := NewIOClient(clientInWrite, clientOutRead)
	defer client.Close()

	// Mock server reading from serverInRead and responding to serverOutWrite
	go func() {
		scanner := bufio.NewScanner(serverInRead)
		for scanner.Scan() {
			var req protocol.Request
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				continue
			}

			var resp protocol.Response
			resp.JSONRPC = "2.0"
			resp.ID = req.ID

			if req.Method == "ping" {
				resp.Result = map[string]any{"status": "pong"}
			} else {
				resp.Error = &protocol.Error{Code: -32601, Message: "Method not found"}
			}

			data, _ := json.Marshal(resp)
			_, _ = fmt.Fprintf(serverOutWrite, "%s\n", string(data))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1. Call ping
	resp, err := client.Call(ctx, "ping", nil)
	if err != nil {
		t.Fatalf("call ping failed: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error response: %v", resp.Error)
	}

	resultMap, ok := resp.Result.(map[string]any)
	if !ok || resultMap["status"] != "pong" {
		t.Errorf("expected status: pong, got: %v", resp.Result)
	}

	// 2. Call unknown method
	respUnknown, err := client.Call(ctx, "unknown_method", nil)
	if err != nil {
		t.Fatalf("call unknown failed: %v", err)
	}
	if respUnknown.Error == nil || respUnknown.Error.Code != -32601 {
		t.Errorf("expected -32601 Method not found error, got: %v", respUnknown.Error)
	}

	// 3. Test context cancellation
	cancelledCtx, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	_, err = client.Call(cancelledCtx, "ping", nil)
	if err == nil {
		t.Error("expected context canceled error, got nil")
	}

	// 4. Test Close
	if err := client.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}

	// Call after close should fail
	_, err = client.Call(context.Background(), "ping", nil)
	if err == nil {
		t.Error("expected error calling after close, got nil")
	}
}
