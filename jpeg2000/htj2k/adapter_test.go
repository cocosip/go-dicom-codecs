package htj2k

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"

	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
)

func TestPrepareFrameForEncodeMatchesFoDicomYBRConversion(t *testing.T) {
	tests := []struct {
		name        string
		photometric string
		width       uint16
		input       []byte
		want        []byte
	}{
		{
			name:        "YBR FULL",
			photometric: photometricYBRFull,
			width:       2,
			input:       []byte{16, 128, 128, 50, 128, 128},
			want:        []byte{16, 16, 16, 50, 50, 50},
		},
		{
			name:        "YBR FULL fo-dicom expression order",
			photometric: photometricYBRFull,
			width:       1,
			input:       []byte{32, 112, 144},
			want:        []byte{54, 26, 4},
		},
		{
			name:        "YBR FULL 422",
			photometric: "YBR_FULL_422",
			width:       2,
			input:       []byte{16, 50, 128, 128},
			want:        []byte{16, 16, 16, 50, 50, 50},
		},
		{
			name:        "YBR FULL 422 fo-dicom expression order",
			photometric: "YBR_FULL_422",
			width:       2,
			input:       []byte{40, 42, 116, 140},
			want:        []byte{57, 36, 19, 59, 38, 21},
		},
		{
			name:        "YBR conversion error falls back to original",
			photometric: photometricYBRFull,
			width:       1,
			input:       []byte{16, 128},
			want:        []byte{16, 128},
		},
		{
			name:        "RGB remains unchanged",
			photometric: "RGB",
			width:       1,
			input:       []byte{1, 2, 3},
			want:        []byte{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &imagetypes.FrameInfo{
				Width:                     tt.width,
				PhotometricInterpretation: tt.photometric,
			}
			got := prepareFrameForEncode(tt.input, info)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("prepareFrameForEncode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodecEncodeRejectsOutputThatIsNotSmallerThanSource(t *testing.T) {
	info := &imagetypes.FrameInfo{
		Width:                     4,
		Height:                    4,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PhotometricInterpretation: "MONOCHROME2",
	}
	source := codecHelpers.NewTestPixelData(info)
	if err := source.AddFrame([]byte{
		10, 20, 30, 40,
		15, 25, 35, 45,
		12, 22, 32, 42,
		18, 28, 38, 48,
	}); err != nil {
		t.Fatalf("AddFrame() error = %v", err)
	}
	destination := codecHelpers.NewTestPixelData(info)

	err := NewLosslessCodec().Encode(source, destination, nil)
	if err == nil || !strings.Contains(err.Error(), "not smaller") {
		t.Fatalf("Encode() error = %v, want output-not-smaller error", err)
	}
	if destination.FrameCount() != 0 {
		t.Fatalf("destination FrameCount() = %d, want 0", destination.FrameCount())
	}
}

func TestOpenJPHEncodeParamsMatchFoDicomFrameMapping(t *testing.T) {
	tests := []struct {
		name        string
		info        *imagetypes.FrameInfo
		params      *Parameters
		lossless    bool
		wantMCT     bool
		wantQuality int
	}{
		{
			name: "signed 16-bit mono lossless",
			info: &imagetypes.FrameInfo{
				Width: 17, Height: 9, BitsAllocated: 16,
				SamplesPerPixel: 1, PixelRepresentation: 1,
			},
			params:   NewHTJ2KLosslessParameters(),
			lossless: true,
			wantMCT:  false,
		},
		{
			name: "unsigned 8-bit two-component lossy",
			info: &imagetypes.FrameInfo{
				Width: 17, Height: 9, BitsAllocated: 8,
				SamplesPerPixel: 2, PixelRepresentation: 0,
			},
			params: func() *Parameters {
				p := NewHTJ2KParameters()
				p.Quality = 37
				return p
			}(),
			wantMCT:     true,
			wantQuality: 37,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := openJPHEncodeParams(tt.info, tt.params, tt.lossless, false)
			if got.Width != 17 || got.Height != 9 || got.Components != int(tt.info.SamplesPerPixel) ||
				got.BitDepth != int(tt.info.BitsAllocated) || got.IsSigned != (tt.info.PixelRepresentation != 0) {
				t.Fatalf("frame mapping = %+v, want width=17 height=9 components=%d bitDepth=%d signed=%v",
					got, tt.info.SamplesPerPixel, tt.info.BitsAllocated, tt.info.PixelRepresentation != 0)
			}
			if got.EnableMCT != tt.wantMCT || got.Lossless != tt.lossless {
				t.Fatalf("mode mapping: MCT=%v lossless=%v, want %v %v",
					got.EnableMCT, got.Lossless, tt.wantMCT, tt.lossless)
			}
			if !tt.lossless && got.Quality != tt.wantQuality {
				t.Fatalf("lossy quality = %d, want %d", got.Quality, tt.wantQuality)
			}
		})
	}
}
