package embeddings

import (
	"context"
	"math"
	"testing"
)

func TestMockProvider_NormalizationAndDimension(t *testing.T) {
	provider := NewMockProvider(768)
	if provider.Dimension() != 768 {
		t.Fatalf("expected dim 768, got %d", provider.Dimension())
	}

	ctx := context.Background()
	vec, err := provider.Embed(ctx, "High HTTP 504 gateway timeout on Payment Service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(vec) != 768 {
		t.Fatalf("expected 768 elements, got %d", len(vec))
	}

	// Verify unit normalization: sqrt(sum(v_i^2)) ≈ 1.0
	var sumSq float64
	for _, v := range vec {
		sumSq += float64(v * v)
	}
	norm := math.Sqrt(sumSq)
	if math.Abs(norm-1.0) > 1e-4 {
		t.Fatalf("expected unit norm (1.0), got %f", norm)
	}

	// Determinism check: same string yields identical vector
	vec2, _ := provider.Embed(ctx, "High HTTP 504 gateway timeout on Payment Service")
	for i := range vec {
		if vec[i] != vec2[i] {
			t.Fatalf("expected deterministic output, element %d differed: %f vs %f", i, vec[i], vec2[i])
		}
	}
}

func TestMockProvider_Batch(t *testing.T) {
	provider := NewMockProvider(768)
	texts := []string{"Doc 1", "Doc 2", "Doc 3"}
	batch, err := provider.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batch) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(batch))
	}
}
