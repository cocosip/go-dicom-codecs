package openjpeg

import "testing"

func TestMCTMCCApplyOrder(t *testing.T) {
	width, height, comps := 16, 16, 3
	n := width * height
	src := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		src[c] = make([]int32, n)
		for i := 0; i < n; i++ {
			src[c][i] = int32((i + c*3) % 200)
		}
	}
	params := DefaultEncodeParams(width, height, comps, 8, false)
	I := make([][]float64, comps)
	inv := make([][]float64, comps)
	for i := 0; i < comps; i++ {
		I[i] = make([]float64, comps)
		inv[i] = make([]float64, comps)
		for j := 0; j < comps; j++ {
			if i == j {
				I[i][j] = 1
				inv[i][j] = 1
			}
		}
	}
	params.MCTMatrix = I
	params.InverseMCTMatrix = inv
	params.MCTReversible = true
	params.MCTOffsets = []int32{10, 20, 30}
	params.NumLevels = 0
	enc := NewEncoder(params)
	data, err := enc.EncodeComponents(src)
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
			t.Fatalf("get comp %d failed: %v", c, err)
		}
		for i := 0; i < 16 && i < len(got); i++ {
			want := src[c][i]
			if got[i] != want {
				t.Fatalf("comp=%d i=%d got=%d want=%d", c, i, got[i], want)
			}
		}
	}
}
