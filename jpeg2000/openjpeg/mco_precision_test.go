package openjpeg

import (
	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
	"testing"
)

func TestMCOPrecisionReversibleFlag(t *testing.T) {
	w, h, comps := 16, 16, 3
	n := w * h
	src := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		src[c] = make([]int32, n)
		for i := 0; i < n; i++ {
			src[c][i] = int32((i + c) % 200)
		}
	}
	p := DefaultEncodeParams(w, h, comps, 8, true)
	I := make([][]float64, comps)
	for i := 0; i < comps; i++ {
		I[i] = make([]float64, comps)
		for j := 0; j < comps; j++ {
			if i == j {
				I[i][j] = 1
			}
		}
	}
	p.MCTMatrix = I
	p.InverseMCTMatrix = I
	p.MCTReversible = true
	p.MCTMatrixElementType = 0
	p.NumLevels = 0
	data, err := NewEncoder(p).EncodeComponents(src)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	cs, err := codestream.NewParser(data).Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(cs.MCC) == 0 {
		t.Fatalf("expected MCC present")
	}
	if !cs.MCC[0].Reversible {
		t.Fatalf("expected reversible flag set")
	}
	if len(cs.MCO) == 0 {
		t.Fatalf("expected MCO present")
	}
	if cs.MCO[0].NumStages != 1 || len(cs.MCO[0].StageIndices) != 1 {
		t.Fatalf("unexpected MCO stage count")
	}
	if cs.MCO[0].StageIndices[0] != cs.MCC[0].Index {
		t.Fatalf("MCO stage index mismatch")
	}
	dec := NewDecoder()
	if err := dec.Decode(data); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for c := 0; c < comps; c++ {
		got, _ := dec.GetComponentData(c)
		for i := 0; i < 16 && i < len(got); i++ {
			if got[i] != src[c][i] {
				t.Fatalf("mismatch comp=%d i=%d got=%d want=%d", c, i, got[i], src[c][i])
			}
		}
	}
}
