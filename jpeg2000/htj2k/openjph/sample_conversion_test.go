package openjph

import (
	"math"
	"testing"
)

func TestOpenJPHSampleConversionVectors(t *testing.T) {
	for _, tt := range []struct {
		name      string
		sample    int32
		bitDepth  int
		isSigned  bool
		wantRev   int32
		wantFloat uint32
	}{
		{name: "u8 zero", sample: 0, bitDepth: 8, wantRev: -128, wantFloat: 0xbf000000},
		{name: "u8 midpoint", sample: 128, bitDepth: 8, wantRev: 0, wantFloat: 0x00000000},
		{name: "u8 maximum", sample: 255, bitDepth: 8, wantRev: 127, wantFloat: 0x3efe0000},
		{name: "s8 minimum", sample: -128, bitDepth: 8, isSigned: true, wantRev: -128, wantFloat: 0xbf000000},
		{name: "s16 maximum", sample: 32767, bitDepth: 16, isSigned: true, wantRev: 32767, wantFloat: 0x3efffe00},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := reversibleSampleToInternal(tt.sample, tt.bitDepth, tt.isSigned); got != tt.wantRev {
				t.Fatalf("reversibleSampleToInternal() = %d, want %d", got, tt.wantRev)
			}
			if got := math.Float32bits(irreversibleSampleToInternal(tt.sample, tt.bitDepth, tt.isSigned)); got != tt.wantFloat {
				t.Fatalf("irreversibleSampleToInternal() bits = %08x, want %08x", got, tt.wantFloat)
			}
		})
	}
}

func TestOpenJPHIrreversibleSampleRoundingAndClamp(t *testing.T) {
	for _, tt := range []struct {
		name     string
		value    float32
		isSigned bool
		want     int32
	}{
		{name: "signed positive half rounds away", value: 0.5 / 256, isSigned: true, want: 1},
		{name: "signed negative half rounds away", value: -0.5 / 256, isSigned: true, want: -1},
		{name: "signed upper clamp", value: 0.75, isSigned: true, want: 127},
		{name: "signed lower clamp", value: -0.75, isSigned: true, want: -128},
		{name: "unsigned midpoint", value: 0, want: 128},
		{name: "unsigned upper clamp", value: 0.75, want: 255},
		{name: "unsigned lower clamp", value: -0.75, want: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := irreversibleInternalToSample(tt.value, 8, tt.isSigned); got != tt.want {
				t.Fatalf("irreversibleInternalToSample() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPrepareIrreversibleDataNormalizesBeforeOpenJPHICT(t *testing.T) {
	params := DefaultEncodeParams(1, 1, 3, 8, false)
	params.Lossless = false
	params.EnableMCT = true
	encoder := NewEncoder(params)
	encoder.data = [][]int32{{0}, {128}, {255}}

	got := encoder.prepareIrreversibleData()
	want := [3]uint32{0xbdbe5a1c, 0x3eaa3247, 0xbe94a742}
	for component := range got {
		if bits := math.Float32bits(got[component][0]); bits != want[component] {
			t.Fatalf("component %d bits = %08x, want %08x", component, bits, want[component])
		}
	}

	params.Components = 1
	params.EnableMCT = false
	encoder = NewEncoder(params)
	encoder.data = [][]int32{{0}}
	if bits := math.Float32bits(encoder.prepareIrreversibleData()[0][0]); bits != 0xbf000000 {
		t.Fatalf("mono normalized bits = %08x, want bf000000", bits)
	}
}
