package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/onisimchukv/uptimefoundry/internal/mcp/protocol"
)

// RunStdio listens for JSON-RPC 2.0 messages from stdin and writes responses to stdout.
func (s *Server) RunStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	// Buffer up to 1MB for large tool responses / embeddings
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	s.logger.Info("mcp server running on stdio transport")

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req protocol.Request
		if err := json.Unmarshal(line, &req); err != nil {
			resp := protocol.Response{
				JSONRPC: "2.0",
				Error: &protocol.Error{
					Code:    protocol.ErrCodeParse,
					Message: fmt.Sprintf("parse error: %v", err),
				},
			}
			data, _ := json.Marshal(resp)
			_, _ = fmt.Fprintf(out, "%s\n", string(data))
			continue
		}

		resp := s.HandleRequest(ctx, req)
		data, err := json.Marshal(resp)
		if err != nil {
			s.logger.Error("failed to marshal mcp response", slog.Any("error", err))
			continue
		}

		if _, err := fmt.Fprintf(out, "%s\n", string(data)); err != nil {
			return fmt.Errorf("write stdio response: %w", err)
		}
	}

	return scanner.Err()
}
