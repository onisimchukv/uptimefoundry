package redaction

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	// Match UUIDs: 8-4-4-4-12 hex format
	reUUID = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
	// Match integer IDs
	reIntID = regexp.MustCompile(`^\d+$`)
	// Match long hex hashes (SHA-1, MD5, SHA-256)
	reHexHash = regexp.MustCompile(`^[a-fA-F0-9]{24,64}$`)
)

// HTTPSummary represents redacted and route-templatized HTTP transaction metadata.
type HTTPSummary struct {
	Method         string `json:"method"`          // GET, POST, PUT, DELETE, etc.
	RouteTemplate  string `json:"route_template"`  // e.g. /api/v1/users/:id/orders/:id
	StatusCode     int    `json:"status_code"`     // e.g. 200, 404, 500
	StatusClass    string `json:"status_class"`    // 2xx, 3xx, 4xx, 5xx
	DurationMs     float64`json:"duration_ms"`
	RequestBytes   uint64 `json:"request_bytes"`
	ResponseBytes  uint64 `json:"response_bytes"`
	Protocol       string `json:"protocol"`        // HTTP/1.1, HTTP/2
	ContentTrust   string `json:"content_trust"`   // Always "untrusted"
}

// SanitizeHTTP extracts safe RED metric metadata from an HTTP request.
// It strips all query strings, headers, cookies, and tokens, converting raw URIs into bounded route templates.
func SanitizeHTTP(method, rawURI string, statusCode int, durationMs float64, reqBytes, respBytes uint64) HTTPSummary {
	// 1. Strip Query String (never retain ?token=..., ?password=...)
	path := rawURI
	if idx := strings.IndexByte(rawURI, '?'); idx != -1 {
		path = rawURI[:idx]
	}
	if parsed, err := url.Parse(rawURI); err == nil && parsed.Path != "" {
		path = parsed.Path
	}

	// 2. Templatize Route
	routeTemplate := TemplatizeRoute(path)

	// 3. Classify Status
	statusClass := "unknown"
	switch {
	case statusCode >= 200 && statusCode < 300:
		statusClass = "2xx"
	case statusCode >= 300 && statusCode < 400:
		statusClass = "3xx"
	case statusCode >= 400 && statusCode < 500:
		statusClass = "4xx"
	case statusCode >= 500 && statusCode < 600:
		statusClass = "5xx"
	}

	return HTTPSummary{
		Method:        strings.ToUpper(method),
		RouteTemplate: routeTemplate,
		StatusCode:    statusCode,
		StatusClass:   statusClass,
		DurationMs:    durationMs,
		RequestBytes:  reqBytes,
		ResponseBytes: respBytes,
		Protocol:      "HTTP/1.1",
		ContentTrust:  "untrusted",
	}
}

// TemplatizeRoute collapses high-cardinality dynamic path segments (IDs, UUIDs, hashes) into bounded route templates.
func TemplatizeRoute(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return "/"
	}

	parts := strings.Split(path, "/")
	var cleanParts []string

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check if segment is a dynamic identifier
		if reIntID.MatchString(part) {
			cleanParts = append(cleanParts, ":id")
		} else if reUUID.MatchString(part) {
			cleanParts = append(cleanParts, ":id")
		} else if reHexHash.MatchString(part) {
			cleanParts = append(cleanParts, ":hash")
		} else if len(part) > 32 {
			cleanParts = append(cleanParts, ":id")
		} else {
			cleanParts = append(cleanParts, part)
		}
	}

	// Cardinality Protection: Cap depth at 6 segments
	if len(cleanParts) > 6 {
		cleanParts = append(cleanParts[:6], "*")
	}

	return "/" + strings.Join(cleanParts, "/")
}
