package openjpeg

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

// Verify MCO stage ordering matches MCC index list.
func TestMCOStageOrdering(t *testing.T) {
	w, h, comps := 8, 8, 3
	n := w * h
	src := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		src[c] = make([]int32, n)
		for i := 0; i < n; i++ {
			src[c][i] = int32(i % 100)
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
	params.NumLevels = 0
	enc := NewEncoder(params)
	data, err := enc.EncodeComponents(src)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	cs, err := codestream.NewParser(data).Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(cs.MCO) == 0 || len(cs.MCC) == 0 {
		t.Fatalf("expected MCO/MCC present")
	}
	if cs.MCO[0].NumStages != 1 || len(cs.MCO[0].StageIndices) != 1 {
		t.Fatalf("unexpected MCO stage count")
	}
	if cs.MCO[0].StageIndices[0] != cs.MCC[0].Index {
		t.Fatalf("MCO stage index mismatch")
	}
}
