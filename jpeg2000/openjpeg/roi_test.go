package openjpeg

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

func TestEncodeAddsRGNSegment(t *testing.T) {
	width, height := 16, 16
	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.ROI = &ROIParams{
		X0:     0,
		Y0:     0,
		Width:  8,
		Height: 8,
		Shift:  3,
	}

	// Simple gradient data
	numPixels := width * height
	componentData := [][]int32{make([]int32, numPixels)}
	for i := 0; i < numPixels; i++ {
		componentData[0][i] = int32(i % 256)
	}

	enc := NewEncoder(params)
	stream, err := enc.EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	parser := codestream.NewParser(stream)
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse codestream failed: %v", err)
	}

	if len(cs.RGN) != params.Components {
		t.Fatalf("expected %d RGN segments, got %d", params.Components, len(cs.RGN))
	}
	rgn := cs.RGN[0]
	if rgn.Srgn != 0 {
		t.Fatalf("unexpected ROI style: got %d, want 0 (MaxShift)", rgn.Srgn)
	}
	if int(rgn.SPrgn) != params.ROI.Shift {
		t.Fatalf("unexpected ROI shift: got %d, want %d", rgn.SPrgn, params.ROI.Shift)
	}

	dec := NewDecoder()
	dec.SetROI(params.ROI)
	if err := dec.Decode(stream); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if dec.Width() != width || dec.Height() != height {
		t.Fatalf("decoded dimensions mismatch: got %dx%d, want %dx%d", dec.Width(), dec.Height(), width, height)
	}
}
