package embeddings

import (
	"math"
	"testing"
)

func TestCosineSimilarityIdentical(t *testing.T) {
	v := []float32{1, 2, 3}
	sim := CosineSimilarity(v, v)
	if math.Abs(sim-1.0) > 1e-6 {
		t.Fatalf("identical vectors: got %f, want 1.0", sim)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim) > 1e-6 {
		t.Fatalf("orthogonal vectors: got %f, want 0.0", sim)
	}
}

func TestCosineSimilarityOpposite(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim+1.0) > 1e-6 {
		t.Fatalf("opposite vectors: got %f, want -1.0", sim)
	}
}

func TestCosineSimilarityDifferentLengths(t *testing.T) {
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Fatalf("different length vectors: got %f, want 0.0", sim)
	}
}

func TestCosineSimilarityEmpty(t *testing.T) {
	sim := CosineSimilarity(nil, nil)
	if sim != 0 {
		t.Fatalf("empty vectors: got %f, want 0.0", sim)
	}
}

func TestCosineSimilarityZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Fatalf("zero vector: got %f, want 0.0", sim)
	}
}

func TestNoopProvider(t *testing.T) {
	p := &noopProvider{}
	if p.Dimensions() != 0 {
		t.Fatalf("noop dims: got %d, want 0", p.Dimensions())
	}
	vec, err := p.Embed("hello")
	if err != nil {
		t.Fatalf("noop embed error: %v", err)
	}
	if vec != nil {
		t.Fatalf("noop embed: got %v, want nil", vec)
	}
}

func TestToFloat32(t *testing.T) {
	in := []float64{1.5, 2.5, 3.5}
	out := toFloat32(in)
	if len(out) != 3 {
		t.Fatalf("len: got %d, want 3", len(out))
	}
	for i, want := range []float32{1.5, 2.5, 3.5} {
		if out[i] != want {
			t.Fatalf("index %d: got %f, want %f", i, out[i], want)
		}
	}
}

// --- Provider selection tests ---
// These call the unexported get() function directly to verify the
// env-var-driven provider selection chain without making real API calls.

func TestProviderSelectionNoop(t *testing.T) {
	// Force an unrecognized provider name — should fall through to noop
	t.Setenv("TOT_EMBED_PROVIDER", "nonexistent_provider_xyz")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("VOYAGE_API_KEY", "")
	t.Setenv("OLLAMA_BASE_URL", "")
	p := get()
	if _, ok := p.(*noopProvider); !ok {
		t.Fatalf("expected noopProvider with invalid explicit provider, got %T", p)
	}
}

func TestProviderSelectionOpenAI(t *testing.T) {
	t.Setenv("TOT_EMBED_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "test-key-not-real")
	t.Setenv("TOT_EMBED_MODEL", "")
	p := get()
	op, ok := p.(*openaiProvider)
	if !ok {
		t.Fatalf("expected openaiProvider, got %T", p)
	}
	if op.model != "text-embedding-3-small" {
		t.Fatalf("default model: got %q, want text-embedding-3-small", op.model)
	}
	if op.Dimensions() != 1536 {
		t.Fatalf("dimensions: got %d, want 1536", op.Dimensions())
	}
}

func TestProviderSelectionOpenAILarge(t *testing.T) {
	t.Setenv("TOT_EMBED_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("TOT_EMBED_MODEL", "text-embedding-3-large")
	p := get()
	op := p.(*openaiProvider)
	if op.Dimensions() != 3072 {
		t.Fatalf("large model dimensions: got %d, want 3072", op.Dimensions())
	}
}

func TestProviderSelectionVoyage(t *testing.T) {
	t.Setenv("TOT_EMBED_PROVIDER", "voyage")
	t.Setenv("VOYAGE_API_KEY", "test-key-not-real")
	t.Setenv("TOT_EMBED_MODEL", "")
	p := get()
	vp, ok := p.(*voyageProvider)
	if !ok {
		t.Fatalf("expected voyageProvider, got %T", p)
	}
	if vp.model != "voyage-3-lite" {
		t.Fatalf("default model: got %q", vp.model)
	}
	if vp.Dimensions() != 512 {
		t.Fatalf("dimensions: got %d, want 512", vp.Dimensions())
	}
}

func TestProviderSelectionOllama(t *testing.T) {
	t.Setenv("TOT_EMBED_PROVIDER", "ollama")
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:99999")
	t.Setenv("TOT_EMBED_MODEL", "")
	p := get()
	op, ok := p.(*ollamaProvider)
	if !ok {
		t.Fatalf("expected ollamaProvider, got %T", p)
	}
	if op.model != "mxbai-embed-large" {
		t.Fatalf("default model: got %q", op.model)
	}
	if op.Dimensions() != 1024 {
		t.Fatalf("dimensions: got %d, want 1024", op.Dimensions())
	}
}

func TestProviderSelectionAutoDetect(t *testing.T) {
	// No explicit provider, but OPENAI_API_KEY is set — should auto-select openai
	t.Setenv("TOT_EMBED_PROVIDER", "")
	t.Setenv("OPENAI_API_KEY", "auto-detect-key")
	t.Setenv("VOYAGE_API_KEY", "")
	t.Setenv("TOT_EMBED_MODEL", "")
	p := get()
	if _, ok := p.(*openaiProvider); !ok {
		t.Fatalf("expected openaiProvider via auto-detect, got %T", p)
	}
}

func TestProviderSelectionCustomModel(t *testing.T) {
	t.Setenv("TOT_EMBED_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("TOT_EMBED_MODEL", "custom-model-v2")
	p := get()
	op := p.(*openaiProvider)
	if op.model != "custom-model-v2" {
		t.Fatalf("custom model: got %q, want custom-model-v2", op.model)
	}
}
