package openjpeg

import (
	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
	"testing"
)

func TestMCTBindingBuilderCreatesMarkers(t *testing.T) {
	w, h, comps := 8, 8, 2
	n := w * h
	src := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		src[c] = make([]int32, n)
		for i := 0; i < n; i++ {
			src[c][i] = int32((i + c) % 50)
		}
	}
	b := NewMCTBinding().Assoc(2).Components([]uint16{0, 1}).Matrix([][]float64{{1, 0}, {0, 1}}).Inverse([][]float64{{1, 0}, {0, 1}}).Offsets([]int32{3, -3}).ElementType(1).MCOPrecision(1).Build()
	p := DefaultEncodeParams(w, h, comps, 8, true)
	p.MCTBindings = []MCTBindingParams{b}
	p.NumLevels = 0
	data, err := NewEncoder(p).EncodeComponents(src)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	cs, err := codestream.NewParser(data).Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(cs.MCT) == 0 || len(cs.MCC) == 0 {
		t.Fatalf("markers missing")
	}
}
