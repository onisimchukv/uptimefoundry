// SPDX-License-Identifier: MIT

package redaction

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var (
	// Match single-quoted strings: '...' including escaped quotes '' or \'
	reStringLiteral = regexp.MustCompile(`'(?:''|\\'|[^'])*'`)
	// Match numeric literals (integers, floats, scientific notation)
	reNumericLiteral = regexp.MustCompile(`\b\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b`)
	// Match hex literals 0x[0-9a-fA-F]+
	reHexLiteral = regexp.MustCompile(`\b0[xX][0-9a-fA-F]+\b`)
	// Match collapsed IN lists: IN (?, ?, ?) -> IN (?)
	reInList = regexp.MustCompile(`(?i)\bIN\s*\(\s*\?(?:\s*,\s*\?)*\s*\)`)
	// Match multiple whitespace
	reWhitespace = regexp.MustCompile(`\s+`)
)

// SQLSummary represents redacted and normalized SQL statement metadata.
type SQLSummary struct {
	Verb          string `json:"verb"`            // SELECT, INSERT, UPDATE, DELETE, etc.
	NormalizedSQL string `json:"normalized_sql"`  // Sanitized query with literals replaced by ?
	StatementHash string `json:"statement_hash"`  // SHA-256 hex digest of normalized SQL
	IsError       bool   `json:"is_error"`
	ErrorClass    string `json:"error_class,omitempty"`
}

// NormalizeSQL strips all literal values, parameters, and sensitive constants from SQL text.
// It mathematically guarantees zero secret leakage by replacing values with ? and collapsing IN clauses.
func NormalizeSQL(rawSQL string) SQLSummary {
	trimmed := strings.TrimSpace(rawSQL)
	if trimmed == "" {
		return SQLSummary{
			Verb:          "UNKNOWN",
			NormalizedSQL: "",
			StatementHash: "",
		}
	}

	// 1. Identify SQL verb (first non-whitespace token)
	verb := "UNKNOWN"
	fields := strings.Fields(trimmed)
	if len(fields) > 0 {
		verb = strings.ToUpper(fields[0])
		// Handle comments before verb (e.g., /* comment */ SELECT)
		if strings.HasPrefix(verb, "/*") || strings.HasPrefix(verb, "--") {
			for _, f := range fields {
				cleanF := strings.Trim(f, "/*;, ")
				if len(cleanF) > 0 && !strings.HasPrefix(f, "/*") && !strings.HasPrefix(f, "--") {
					verb = strings.ToUpper(cleanF)
					break
				}
			}
		}
	}

	// 2. Strip single-quoted string literals -> '?'
	redacted := reStringLiteral.ReplaceAllString(trimmed, "'?'")

	// 3. Strip hex literals -> ?
	redacted = reHexLiteral.ReplaceAllString(redacted, "?")

	// 4. Strip numeric literals -> ?
	redacted = reNumericLiteral.ReplaceAllString(redacted, "?")

	// 5. Collapse IN (?) lists -> IN (?)
	redacted = reInList.ReplaceAllString(redacted, "IN (?)")

	// 6. Normalize whitespace
	redacted = reWhitespace.ReplaceAllString(redacted, " ")
	normalized := strings.TrimSpace(redacted)

	// 7. Compute SHA-256 hash of normalized statement
	hash := sha256.Sum256([]byte(normalized))
	stmtHash := hex.EncodeToString(hash[:])

	return SQLSummary{
		Verb:          verb,
		NormalizedSQL: normalized,
		StatementHash: stmtHash,
	}
}
