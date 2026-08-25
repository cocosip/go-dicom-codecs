package openjpeg

import (
	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
	"testing"
)

func TestMCTMCCMarkersPresent(t *testing.T) {
	width, height, comps := 32, 32, 3
	params := DefaultEncodeParams(width, height, comps, 8, false)
	params.NumLevels = 1
	// Identity matrix with explicit inverse
	I := make([][]float64, comps)
	inv := make([][]float64, comps)
	for i := 0; i < comps; i++ {
		I[i] = make([]float64, comps)
		inv[i] = make([]float64, comps)
		for j := 0; j < comps; j++ {
			v := 0.0
			if i == j {
				v = 1.0
			}
			I[i][j] = v
			inv[i][j] = v
		}
	}
	params.MCTMatrix = I
	params.InverseMCTMatrix = inv
	enc := NewEncoder(params)
	// Build constant component data
	n := width * height
	compsData := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		compsData[c] = make([]int32, n)
		for i := 0; i < n; i++ {
			compsData[c][i] = int32((i + c) % 256)
		}
	}
	data, err := enc.EncodeComponents(compsData)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	// Parse codestream and check markers
	p := codestream.NewParser(data)
	cs, err := p.Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(cs.MCT) == 0 {
		t.Fatalf("expected MCT marker present")
	}
	if len(cs.MCC) == 0 {
		t.Fatalf("expected MCC marker present")
	}
}
