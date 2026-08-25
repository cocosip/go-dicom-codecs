package openjpeg

import (
	"testing"
)

// Verify int32 element matrix is written and parsed, with identity matrix
func TestMCTInt32MatrixIdentity(t *testing.T) {
	w, h, comps := 8, 8, 3
	n := w * h
	src := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		src[c] = make([]int32, n)
		for i := 0; i < n; i++ {
			src[c][i] = int32((i + c) % 127)
		}
	}
	params := DefaultEncodeParams(w, h, comps, 8, true)
	I := make([][]float64, comps)
	for i := 0; i < comps; i++ {
		I[i] = make([]float64, comps)
		for j := 0; j < comps; j++ {
			if i == j {
				I[i][j] = 1
			}
		}
	}
	params.MCTMatrix = I
	params.InverseMCTMatrix = I
	params.MCTReversible = true
	params.MCTMatrixElementType = 0
	params.NumLevels = 0
	data, err := NewEncoder(params).EncodeComponents(src)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	dec := NewDecoder()
	if err := dec.Decode(data); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for c := 0; c < comps; c++ {
		got, err := dec.GetComponentData(c)
		if err != nil {
			t.Fatalf("get comp %d: %v", c, err)
		}
		for i := 0; i < 16 && i < len(got); i++ {
			if got[i] != src[c][i] {
				t.Fatalf("mismatch comp=%d i=%d got=%d want=%d", c, i, got[i], src[c][i])
			}
		}
	}
}
