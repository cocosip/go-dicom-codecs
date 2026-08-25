package openjpeg

import "testing"

func TestMCCMultiSubsetBinding(t *testing.T) {
	w, h, comps := 8, 8, 2
	n := w * h
	src := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		src[c] = make([]int32, n)
		for i := 0; i < n; i++ {
			src[c][i] = int32((i + c) % 50)
		}
	}
	p := DefaultEncodeParams(w, h, comps, 8, true)
	b0 := MCTBindingParams{AssocType: 2, ComponentIDs: []uint16{0, 1}, Matrix: [][]float64{{1, 0}, {0, 1}}, Inverse: [][]float64{{1, 0}, {0, 1}}, Offsets: []int32{5, -5}, ElementType: 1, MCOPrecision: 0}
	p.MCTBindings = []MCTBindingParams{b0}
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
