package encoding

import "testing"

func TestFloat32BytesRoundtrip(t *testing.T) {
	original := []float32{1.5, -2.3, 0.0, 3.14159}
	bytes := Float32ToBytes(original)
	restored := BytesToFloat32(bytes)

	if len(restored) != len(original) {
		t.Fatalf("length: got %d, want %d", len(restored), len(original))
	}
	for i := range original {
		if restored[i] != original[i] {
			t.Fatalf("index %d: got %f, want %f", i, restored[i], original[i])
		}
	}
}

func TestFloat32BytesEmpty(t *testing.T) {
	bytes := Float32ToBytes(nil)
	if len(bytes) != 0 {
		t.Fatalf("nil input: got %d bytes", len(bytes))
	}
	restored := BytesToFloat32(nil)
	if len(restored) != 0 {
		t.Fatalf("nil input: got %d floats", len(restored))
	}
}
