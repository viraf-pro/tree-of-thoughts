package encoding

import "testing"

func TestFloat32BytesRoundtrip(t *testing.T) {
	original := []float32{1.5, -2.3, 0.0, 3.14159}
	encoded := Float32ToBytes(original)
	restored := BytesToFloat32(encoded)

	if len(restored) != len(original) {
		t.Fatalf("length: got %d, want %d", len(restored), len(original))
	}
	for i := range original {
		if restored[i] != original[i] {
			t.Fatalf("index %d: got %f, want %f", i, restored[i], original[i])
		}
	}
}

func TestBytesToFloat32KnownBytes(t *testing.T) {
	// 1.0 in IEEE 754 little-endian is 0x00, 0x00, 0x80, 0x3F
	b := []byte{0x00, 0x00, 0x80, 0x3F}
	got := BytesToFloat32(b)
	if len(got) != 1 || got[0] != 1.0 {
		t.Fatalf("got %v, want [1.0]", got)
	}
}

func TestFloat32BytesEmpty(t *testing.T) {
	encoded := Float32ToBytes(nil)
	if len(encoded) != 0 {
		t.Fatalf("nil input: got %d bytes", len(encoded))
	}
	restored := BytesToFloat32(nil)
	if len(restored) != 0 {
		t.Fatalf("nil input: got %d floats", len(restored))
	}
}
