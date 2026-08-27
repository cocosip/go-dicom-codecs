package htj2k

import (
	"bytes"
	"testing"

	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k/openjph"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func TestCodecDecodeSupportsStoredAndAllocatedPrecisionCodestreams(t *testing.T) {
	tests := []struct {
		name     string
		signed   bool
		samples  []uint16
		wantSsiz uint8
	}{
		{
			name:     "unsigned",
			samples:  []uint16{0x000, 0x001, 0x7ff, 0x800, 0xfff},
			wantSsiz: 0x0b,
		},
		{
			name:     "signed",
			signed:   true,
			samples:  []uint16{0x000, 0x001, 0x7ff, 0x800, 0xfff},
			wantSsiz: 0x8b,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const width, height = 64, 64
			pixels := makePrecisionTestPixels(width*height, tt.samples)
			frameInfo := &imagetypes.FrameInfo{
				Width:                     width,
				Height:                    height,
				BitsAllocated:             16,
				BitsStored:                12,
				HighBit:                   11,
				SamplesPerPixel:           1,
				PhotometricInterpretation: photometricMonochrome2,
			}
			if tt.signed {
				frameInfo.PixelRepresentation = 1
			}

			for _, streamPrecision := range []int{12, 16} {
				name := "standard stored precision"
				if streamPrecision == 16 {
					name = "legacy allocated precision"
				}
				t.Run(name, func(t *testing.T) {
					params := openjph.DefaultEncodeParams(width, height, 1, streamPrecision, tt.signed)
					params.NumLevels = 3
					encoded, err := openjph.NewEncoder(params).Encode(pixels)
					if err != nil {
						t.Fatalf("encode %d-bit compatibility codestream: %v", streamPrecision, err)
					}

					parsed, err := codestream.NewParser(bytes.Clone(encoded)).Parse()
					if err != nil {
						t.Fatalf("parse %d-bit compatibility codestream: %v", streamPrecision, err)
					}
					wantSsiz := tt.wantSsiz
					if streamPrecision == 16 {
						wantSsiz = 0x0f
						if tt.signed {
							wantSsiz = 0x8f
						}
					}
					if got := parsed.SIZ.Components[0].Ssiz; got != wantSsiz {
						t.Fatalf("Ssiz = 0x%02X, want 0x%02X", got, wantSsiz)
					}

					source := codecHelpers.NewTestPixelData(frameInfo)
					if err := source.AddFrame(encoded); err != nil {
						t.Fatalf("add encoded frame: %v", err)
					}
					destination := codecHelpers.NewTestPixelData(frameInfo)
					if err := NewLosslessCodec().Decode(source, destination, nil); err != nil {
						t.Fatalf("decode %d-bit compatibility codestream: %v", streamPrecision, err)
					}
					decoded, err := destination.GetFrame(0)
					if err != nil {
						t.Fatalf("read decoded frame: %v", err)
					}
					if !bytes.Equal(decoded, pixels) {
						t.Fatalf("decoded %d-bit compatibility pixels differ at byte %d", streamPrecision, firstByteDiff(decoded, pixels))
					}
				})
			}
		})
	}
}

func makePrecisionTestPixels(count int, samples []uint16) []byte {
	pixels := make([]byte, count*2)
	for index := 0; index < count; index++ {
		value := samples[index%len(samples)]
		pixels[index*2] = byte(value)
		pixels[index*2+1] = byte(value >> 8)
	}
	return pixels
}

func firstByteDiff(left, right []byte) int {
	limit := min(len(left), len(right))
	for index := 0; index < limit; index++ {
		if left[index] != right[index] {
			return index
		}
	}
	return limit
}
