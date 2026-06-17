package htj2k

import (
	"testing"

	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func TestHTJ2KHeaderMatchesOpenJPHSignals(t *testing.T) {
	tests := []struct {
		name        string
		codec       *Codec
		wantProg    uint8
		wantXform   uint8
		wantHTStyle bool
	}{
		{
			name:        "lossless default",
			codec:       NewLosslessCodec(),
			wantProg:    2, // OpenJPH defaults to RPCL.
			wantXform:   1, // Reversible 5/3.
			wantHTStyle: true,
		},
		{
			name:        "lossless rpcl",
			codec:       NewLosslessRPCLCodec(),
			wantProg:    2,
			wantXform:   1,
			wantHTStyle: true,
		},
		{
			name:        "lossy",
			codec:       NewCodec(80),
			wantProg:    2,
			wantXform:   0, // Irreversible 9/7.
			wantHTStyle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frameInfo := &imagetypes.FrameInfo{
				Width:                     128,
				Height:                    128,
				BitsAllocated:             8,
				BitsStored:                8,
				HighBit:                   7,
				SamplesPerPixel:           1,
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: photometricMonochrome2,
			}
			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(makeGradient(128 * 128)); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}
			dst := codecHelpers.NewTestPixelData(frameInfo)
			if err := tt.codec.Encode(src, dst, nil); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			encoded, err := dst.GetFrame(0)
			if err != nil {
				t.Fatalf("GetFrame failed: %v", err)
			}
			cs, err := codestream.NewParser(encoded).Parse()
			if err != nil {
				t.Fatalf("Parse encoded codestream failed: %v", err)
			}
			if cs.COD == nil {
				t.Fatal("missing COD segment")
			}
			if cs.SIZ == nil {
				t.Fatal("missing SIZ segment")
			}
			if cs.SIZ.Rsiz&0x4000 == 0 {
				t.Fatalf("SIZ.Rsiz must signal HTJ2K capability bit 14; got 0x%04X", cs.SIZ.Rsiz)
			}
			if !containsMarker(encoded, 0xFF50) {
				t.Fatal("encoded HTJ2K codestream is missing CAP marker")
			}

			if cs.COD.Scod&0x40 != 0 {
				t.Fatalf("HTJ2K mode must not be signalled in Scod; got Scod=0x%02X", cs.COD.Scod)
			}
			gotHTStyle := cs.COD.CodeBlockStyle&0x40 != 0
			if gotHTStyle != tt.wantHTStyle {
				t.Fatalf("HTJ2K mode signal in code-block style = %v, want %v (style=0x%02X)",
					gotHTStyle, tt.wantHTStyle, cs.COD.CodeBlockStyle)
			}
			if cs.COD.NumberOfDecompositionLevels != 5 {
				t.Fatalf("decomposition levels = %d, want OpenJPH default 5", cs.COD.NumberOfDecompositionLevels)
			}
			if cs.COD.ProgressionOrder != tt.wantProg {
				t.Fatalf("progression order = %d, want %d", cs.COD.ProgressionOrder, tt.wantProg)
			}
			if cs.COD.Transformation != tt.wantXform {
				t.Fatalf("wavelet transform = %d, want %d", cs.COD.Transformation, tt.wantXform)
			}
		})
	}
}

func makeGradient(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i)
	}
	return data
}

func containsMarker(data []byte, marker uint16) bool {
	hi := byte(marker >> 8)
	lo := byte(marker)
	for i := 0; i+1 < len(data); i++ {
		if data[i] == hi && data[i+1] == lo {
			return true
		}
	}
	return false
}
