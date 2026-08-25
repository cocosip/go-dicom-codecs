package t2

import (
	"math"
	"reflect"
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

func TestOpenJPHIrreversibleDecodeTransferMatchesTxFromCodeblock32(t *testing.T) {
	decoder := &TileDecoder{
		cod: &codestream.CODSegment{Transformation: 0},
		qcd: &codestream.QCDSegment{
			Sqcd: 1<<5 | 2,
			SPqcd: []byte{
				0xb7, 0x18,
				0xb6, 0xea,
				0xb6, 0xea,
				0xb6, 0xbc,
			},
		},
	}

	steps := decoder.decodeQuantizationSteps(1)
	if len(steps) != 4 {
		t.Fatalf("decoded step count = %d, want 4", len(steps))
	}

	// OpenJPH tx_from_cb32 receives the full cleanup magnitude. This literal
	// excludes bin centering so the transfer input is exactly 12345 << 9.
	got := float32(12345<<9) * float32(steps[1])
	if bits := math.Float32bits(got); bits != 0x3c33cc86 {
		t.Fatalf("dequantized coefficient = %g (0x%08x), want 0.010974055 (0x3c33cc86)", got, bits)
	}
}

func TestOpenJPHIrreversibleICTRunsBeforeIntegerConversion(t *testing.T) {
	decoder := &TileDecoder{
		cod: &codestream.CODSegment{
			Transformation:             0,
			MultipleComponentTransform: 1,
		},
		siz: &codestream.SIZSegment{
			Csiz: 3,
			Components: []codestream.ComponentSize{
				{Ssiz: 7, XRsiz: 1, YRsiz: 1},
				{Ssiz: 7, XRsiz: 1, YRsiz: 1},
				{Ssiz: 7, XRsiz: 1, YRsiz: 1},
			},
		},
	}

	got := decoder.finalizeIrreversibleSamples([][]float32{{0.1}, {0.001}, {0.001}})
	want := [][]int32{{26}, {25}, {26}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("irreversible ICT output = %v, want %v", got, want)
	}
}

func TestOpenJPHIrreversibleNoDecompositionRetainsFloatSamples(t *testing.T) {
	decoder := &TileDecoder{
		cod: &codestream.CODSegment{Transformation: 0},
		siz: &codestream.SIZSegment{
			Csiz:       1,
			Components: []codestream.ComponentSize{{Ssiz: 7, XRsiz: 1, YRsiz: 1}},
		},
	}
	component := &ComponentDecoder{
		componentIdx: 0,
		width:        1,
		height:       1,
		numLevels:    0,
		coefficients: []int32{1},
	}

	if err := decoder.applyIDWT(component); err != nil {
		t.Fatalf("apply no-decomposition irreversible path: %v", err)
	}
	got := decoder.finalizeIrreversibleSamples([][]float32{component.floatSamples})
	want := [][]int32{{256}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("no-decomposition irreversible output = %v, want %v", got, want)
	}
}
