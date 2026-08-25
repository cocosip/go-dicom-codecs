package openjpeg

import (
	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
	"testing"
)

func TestMCT_RCT_LosslessRoundTrip(t *testing.T) {
	width, height := 32, 32
	numPixels := width * height
	comps := make([][]int32, 3)
	for c := 0; c < 3; c++ {
		comps[c] = make([]int32, numPixels)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				idx := y*width + x
				comps[c][idx] = int32((x*3 + y*5 + c*17) % 256)
			}
		}
	}

	params := DefaultEncodeParams(width, height, 3, 8, false)
	params.NumLevels = 1
	params.Lossless = true
	enc := NewEncoder(params)
	encoded, err := enc.EncodeComponents(comps)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	parser := codestream.NewParser(encoded)
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cs.COD == nil || cs.COD.MultipleComponentTransform != 1 {
		t.Fatalf("MCT flag not set")
	}
	dec := NewDecoder()
	if err := dec.Decode(encoded); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for c := 0; c < 3; c++ {
		got, err := dec.GetComponentData(c)
		if err != nil {
			t.Fatalf("get component %d failed: %v", c, err)
		}
		if len(got) != len(comps[c]) {
			t.Fatalf("len mismatch")
		}
		for i := range got {
			if got[i] != comps[c][i] {
				t.Fatalf("mismatch at c=%d i=%d: got %d want %d", c, i, got[i], comps[c][i])
			}
		}
	}
}

func TestMCT_ICT_LossyDecodeRGB(t *testing.T) {
	width, height := 64, 64
	numPixels := width * height
	comps := make([][]int32, 3)
	for c := 0; c < 3; c++ {
		comps[c] = make([]int32, numPixels)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				idx := y*width + x
				comps[c][idx] = int32((x*7 + y*11 + c*23) % 256)
			}
		}
	}
	params := DefaultEncodeParams(width, height, 3, 8, false)
	params.Lossless = false
	params.NumLevels = 1
	params.Quality = 80
	enc := NewEncoder(params)
	encoded, err := enc.EncodeComponents(comps)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	dec := NewDecoder()
	if err := dec.Decode(encoded); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	maxErr := int32(0)
	for c := 0; c < 3; c++ {
		got, err := dec.GetComponentData(c)
		if err != nil {
			t.Fatalf("get component %d failed: %v", c, err)
		}
		for i := range got {
			d := got[i] - comps[c][i]
			if d < 0 {
				d = -d
			}
			if d > maxErr {
				maxErr = d
			}
		}
	}
	if maxErr > 16 {
		t.Fatalf("max error too high: %d", maxErr)
	}
}
