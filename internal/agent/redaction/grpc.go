package redaction

import (
	"strings"
)

// GRPCSummary represents redacted gRPC RPC metadata.
type GRPCSummary struct {
	Service       string  `json:"service"`        // e.g. billing.v1.PaymentService
	Method        string  `json:"method"`         // e.g. ProcessPayment
	StatusCode    int     `json:"status_code"`    // e.g. 0 (OK), 14 (UNAVAILABLE)
	StatusString  string  `json:"status_string"`  // e.g. "OK", "UNAVAILABLE"
	DurationMs    float64 `json:"duration_ms"`
	RequestBytes  uint64  `json:"request_bytes"`
	ResponseBytes uint64  `json:"response_bytes"`
	ContentTrust  string  `json:"content_trust"`  // Always "untrusted"
}

// SanitizeGRPC extracts safe RED metric metadata from a gRPC call path.
// It drops all metadata trailers, authentication tokens, and request/response payloads.
func SanitizeGRPC(fullPath string, statusCode int, durationMs float64, reqBytes, respBytes uint64) GRPCSummary {
	// Full path format: /package.Service/Method
	fullPath = strings.TrimPrefix(fullPath, "/")
	parts := strings.Split(fullPath, "/")

	service := "UnknownService"
	method := "UnknownMethod"

	if len(parts) >= 2 {
		service = parts[0]
		method = parts[1]
	} else if len(parts) == 1 {
		service = parts[0]
	}

	return GRPCSummary{
		Service:       service,
		Method:        method,
		StatusCode:    statusCode,
		StatusString:  grpcStatusString(statusCode),
		DurationMs:    durationMs,
		RequestBytes:  reqBytes,
		ResponseBytes: respBytes,
		ContentTrust:  "untrusted",
	}
}

func grpcStatusString(code int) string {
	switch code {
	case 0:
		return "OK"
	case 1:
		return "CANCELLED"
	case 2:
		return "UNKNOWN"
	case 3:
		return "INVALID_ARGUMENT"
	case 4:
		return "DEADLINE_EXCEEDED"
	case 5:
		return "NOT_FOUND"
	case 6:
		return "ALREADY_EXISTS"
	case 7:
		return "PERMISSION_DENIED"
	case 8:
		return "RESOURCE_EXHAUSTED"
	case 9:
		return "FAILED_PRECONDITION"
	case 10:
		return "ABORTED"
	case 11:
		return "OUT_OF_RANGE"
	case 12:
		return "UNIMPLEMENTED"
	case 13:
		return "INTERNAL"
	case 14:
		return "UNAVAILABLE"
	case 15:
		return "DATA_LOSS"
	case 16:
		return "UNAUTHENTICATED"
	default:
		return "UNKNOWN"
	}
}
