// SPDX-License-Identifier: MIT

package embeddings

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

// Provider produces dense vector embeddings for text chunks.
type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// MockProvider generates deterministic, unit-normalized 768-dimensional embeddings
// based on the SHA-256 hash of the input text. Ideal for offline environments and testing.
type MockProvider struct {
	dim int
}

// NewMockProvider creates a new mock embedding provider.
func NewMockProvider(dim int) *MockProvider {
	if dim <= 0 {
		dim = 768
	}
	return &MockProvider{dim: dim}
}

// Dimension returns the vector dimension.
func (m *MockProvider) Dimension() int {
	return m.dim
}

// Embed generates a deterministic unit vector for the given text.
func (m *MockProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vec := make([]float32, m.dim)
	h := sha256.New()
	h.Write([]byte(text))
	hashBytes := h.Sum(nil)

	// Expand hash deterministically across dimension
	var sumSq float64
	for i := 0; i < m.dim; i++ {
		idx := (i * 4) % (len(hashBytes) - 4)
		val := binary.LittleEndian.Uint32(hashBytes[idx : idx+4])
		f := float32((float64(val)/float64(math.MaxUint32))*2.0 - 1.0)
		vec[i] = f
		sumSq += float64(f * f)
	}

	// Normalize to unit length for cosine similarity
	norm := float32(math.Sqrt(sumSq))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec, nil
}

// EmbedBatch embeds multiple texts.
func (m *MockProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, t := range texts {
		vec, err := m.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		results[i] = vec
	}
	return results, nil
}

// HTTPProvider connects to a local Text Embeddings Inference (TEI) or Ollama server.
type HTTPProvider struct {
	endpoint   string
	httpClient *http.Client
	dim        int
}

// NewHTTPProvider creates a provider targeting a local TEI or Ollama `/embed` endpoint.
func NewHTTPProvider(endpoint string, dim int) *HTTPProvider {
	if dim <= 0 {
		dim = 768
	}
	return &HTTPProvider{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		dim: dim,
	}
}

// Dimension returns the expected vector dimension.
func (h *HTTPProvider) Dimension() int {
	return h.dim
}

type teiRequest struct {
	Inputs any `json:"inputs"`
}

// Embed requests a vector embedding from the HTTP endpoint.
func (h *HTTPProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody, err := json.Marshal(teiRequest{Inputs: text})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding endpoint returned status %d", resp.StatusCode)
	}

	// TEI returns a 1D float array or a 2D array if wrapped in array
	var raw []float32
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		// Try array of arrays
		var batch [][]float32
		if err2 := json.NewDecoder(resp.Body).Decode(&batch); err2 == nil && len(batch) > 0 {
			raw = batch[0]
		} else {
			return nil, fmt.Errorf("decode embedding response: %w", err)
		}
	}

	return raw, nil
}

// EmbedBatch embeds multiple texts.
func (h *HTTPProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody, err := json.Marshal(teiRequest{Inputs: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("batch embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding endpoint returned status %d", resp.StatusCode)
	}

	var batch [][]float32
	if err := json.NewDecoder(resp.Body).Decode(&batch); err != nil {
		return nil, fmt.Errorf("decode batch embedding: %w", err)
	}

	return batch, nil
}
