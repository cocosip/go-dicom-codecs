package openjpeg

import (
	"math"
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

func TestMCONormScaleRoundTrip(t *testing.T) {
	w, h, comps := 16, 16, 3
	n := w * h
	src := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		src[c] = make([]int32, n)
		for i := 0; i < n; i++ {
			src[c][i] = int32((i*3 + c*7) % 255)
		}
	}
	params := DefaultEncodeParams(w, h, comps, 8, true)
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
	params.MCTMatrixElementType = 1
	params.NumLevels = 0
	enc := NewEncoder(params)
	data, err := enc.EncodeComponents(src)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	p := codestream.NewParser(data)
	cs, err := p.Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(cs.MCO) == 0 {
		t.Fatalf("expected MCO present")
	}
	if len(cs.MCT) == 0 {
		t.Fatalf("expected MCT present")
	}
	var m *codestream.MCTSegment
	for i := range cs.MCT {
		if cs.MCT[i].ArrayType == codestream.MCTArrayDecorrelate {
			m = &cs.MCT[i]
			break
		}
	}
	if m == nil {
		t.Fatalf("decorrelation MCT not found")
	}
	if m.ElementType != codestream.MCTElementFloat32 || m.ArrayType != codestream.MCTArrayDecorrelate {
		t.Fatalf("unexpected MCT types")
	}
	if len(m.Data) < comps*comps*4 {
		t.Fatalf("unexpected MCT data length")
	}
	for r := 0; r < comps; r++ {
		for c := 0; c < comps; c++ {
			off := (r*comps + c) * 4
			v := uint32(m.Data[off])<<24 | uint32(m.Data[off+1])<<16 | uint32(m.Data[off+2])<<8 | uint32(m.Data[off+3])
			f := math.Float32frombits(v)
			if r == c {
				if f < 0.99 || f > 1.01 {
					t.Fatalf("unexpected diag value: %f", f)
				}
			} else {
				if f != 0 {
					t.Fatalf("unexpected off-diag value: %f", f)
				}
			}
		}
	}
	if cs.MCO[0].NumStages != 1 || len(cs.MCO[0].StageIndices) != 1 {
		t.Fatalf("unexpected MCO stage count")
	}
	if cs.MCO[0].StageIndices[0] != cs.MCC[0].Index {
		t.Fatalf("MCO stage index mismatch")
	}
}
