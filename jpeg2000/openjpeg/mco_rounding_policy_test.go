package openjpeg

import "testing"

func TestMCTFixedPointRoundTrip(t *testing.T) {
	w, h, comps := 8, 8, 2
	n := w * h
	src := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		src[c] = make([]int32, n)
		for i := 0; i < n; i++ {
			src[c][i] = int32((i%7 + 1) * 2)
		}
	}
	p := DefaultEncodeParams(w, h, comps, 8, true)
	matrix := [][]float64{{0.5, 0}, {0, 0.5}}
	inv := [][]float64{{2, 0}, {0, 2}}
	b := MCTBindingParams{ComponentIDs: []uint16{0, 1}, Matrix: matrix, Inverse: inv, ElementType: 1}
	p.MCTBindings = []MCTBindingParams{b}
	p.NumLevels = 0
	data, err := NewEncoder(p).EncodeComponents(src)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	dec := NewDecoder()
	if err := dec.Decode(data); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		if got := dec.data[0][i]; got != src[0][i] {
			t.Fatalf("comp0 mismatch i=%d got=%d want=%d", i, got, src[0][i])
		}
		if got := dec.data[1][i]; got != src[1][i] {
			t.Fatalf("comp1 mismatch i=%d got=%d want=%d", i, got, src[1][i])
		}
	}
}
