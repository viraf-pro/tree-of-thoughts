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
