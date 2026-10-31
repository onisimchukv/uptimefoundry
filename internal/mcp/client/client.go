package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/forgeops/forgeops/internal/mcp/protocol"
)

// Client communicates with an external MCP server over stdio subprocess.
type Client struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	pending map[uint64]chan protocol.Response
	mu      sync.Mutex
	reqID   uint64
	closed  bool
}

// StartSubprocess launches an external MCP server process and initializes stdio pipes.
func StartSubprocess(ctx context.Context, command string, args []string, env map[string]string) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if len(env) > 0 {
		for k, v := range env {
			cmd.Env = append(cmd.Environ(), fmt.Sprintf("%s=%s", k, v))
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start subprocess %q: %w", command, err)
	}

	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		pending: make(map[uint64]chan protocol.Response),
	}

	go c.listenStdout()

	// Perform initialize handshake
	initParams, _ := json.Marshal(protocol.InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: protocol.ClientInfo{
			Name:    "forgeops-mcp-client",
			Version: "1.2.0",
		},
	})
	_, err = c.Call(ctx, "initialize", initParams)
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("initialize handshake failed: %w", err)
	}

	return c, nil
}

func (c *Client) listenStdout() {
	scanner := bufio.NewScanner(c.stdout)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		var resp protocol.Response
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}

		// Match pending request ID
		var idNum uint64
		switch v := resp.ID.(type) {
		case float64:
			idNum = uint64(v)
		case uint64:
			idNum = v
		}

		c.mu.Lock()
		ch, exists := c.pending[idNum]
		if exists {
			delete(c.pending, idNum)
		}
		c.mu.Unlock()

		if exists {
			ch <- resp
		}
	}

	c.mu.Lock()
	c.closed = true
	for _, ch := range c.pending {
		close(ch)
	}
	c.pending = make(map[uint64]chan protocol.Response)
	c.mu.Unlock()
}

// Call sends a JSON-RPC request and awaits the server response.
func (c *Client) Call(ctx context.Context, method string, params json.RawMessage) (*protocol.Response, error) {
	id := atomic.AddUint64(&c.reqID, 1)
	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	ch := make(chan protocol.Response, 1)

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("client connection is closed")
	}
	c.pending[id] = ch
	c.mu.Unlock()

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	c.mu.Lock()
	_, err = fmt.Fprintf(c.stdin, "%s\n", string(reqBytes))
	c.mu.Unlock()

	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("write request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("connection terminated while waiting for response")
		}
		return &resp, nil
	}
}

// Close terminates the subprocess.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	_ = c.stdin.Close()
	_ = c.stdout.Close()
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	return nil
}
