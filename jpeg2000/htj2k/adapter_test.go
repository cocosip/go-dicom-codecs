package htj2k

import (
	"bytes"
	"strings"
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
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
			info := &dicomcodec.FrameInfo{
				Width: tt.width, BitDepth: pixel.BitDepth{BitsAllocated: 0, BitsStored: 0, HighBit: 0, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(tt.photometric),
			}
			got := prepareFrameForEncode(tt.input, *info)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("prepareFrameForEncode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodecEncodeRejectsOutputThatIsNotSmallerThanSource(t *testing.T) {
	info := &dicomcodec.FrameInfo{
		Width:  4,
		Height: 4,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}
	source := codecHelpers.NewTestPixelData(info)
	if err := source.AddFrame(context.Background(), []byte{
		10, 20, 30, 40,
		15, 25, 35, 45,
		12, 22, 32, 42,
		18, 28, 38, 48,
	}); err != nil {
		t.Fatalf("AddFrame() error = %v", err)
	}
	destination := codecHelpers.NewTestPixelData(info)

	err := NewLosslessCodec().Encode(context.Background(), source, destination, nil)
	if err == nil || !strings.Contains(err.Error(), "not smaller") {
		t.Fatalf("Encode() error = %v, want output-not-smaller error", err)
	}
	if destination.FrameCount() != 0 {
		t.Fatalf("destination FrameCount() = %d, want 0", destination.FrameCount())
	}
}

func TestCodecEncodeRejectsInvalidPixelBitMetadata(t *testing.T) {
	tests := []struct {
		name string
		info *dicomcodec.FrameInfo
		want string
	}{
		{
			name: "zero bits stored",
			info: &dicomcodec.FrameInfo{BitDepth: pixel.BitDepth{BitsAllocated: 16, BitsStored: 0, HighBit: 0, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2")},
			want: "BitsStored must be between 1 and BitsAllocated",
		},
		{
			name: "bits stored exceeds container",
			info: &dicomcodec.FrameInfo{BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 12, HighBit: 11, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2")},
			want: "BitsStored must be between 1 and BitsAllocated",
		},
		{
			name: "high bit does not identify stored precision",
			info: &dicomcodec.FrameInfo{BitDepth: pixel.BitDepth{BitsAllocated: 16, BitsStored: 12, HighBit: 12, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2")},
			want: "HighBit must equal BitsStored - 1",
		},
		{
			name: "unsupported input container",
			info: &dicomcodec.FrameInfo{BitDepth: pixel.BitDepth{BitsAllocated: 12, BitsStored: 12, HighBit: 11, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2")},
			want: "BitsAllocated must be 8 or 16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.info.Width = 1
			tt.info.Height = 1
			tt.info.SamplesPerPixel = 1
			tt.info.PhotometricInterpretation = *pixel.Monochrome2
			source := codecHelpers.NewTestPixelData(tt.info)
			destination := codecHelpers.NewTestPixelData(tt.info)

			err := NewLosslessCodec().Encode(context.Background(), source, destination, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Encode() error = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestOpenJPHEncodeParamsSeparateStoredPrecisionFromAllocatedContainer(t *testing.T) {
	tests := []struct {
		name        string
		info        *dicomcodec.FrameInfo
		params      *Parameters
		lossless    bool
		wantMCT     bool
		wantQuality int
	}{
		{
			name: "signed 12-bit mono in 16-bit container",
			info: &dicomcodec.FrameInfo{
				Width: 17, Height: 9,
				SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 16, BitsStored: 12, HighBit: 11, IsSigned: pixel.Representation(1).IsSigned()}, PixelRepresentation: pixel.Representation(1), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
			},
			params:   NewHTJ2KLosslessParameters(),
			lossless: true,
			wantMCT:  false,
		},
		{
			name: "unsigned 8-bit two-component lossy",
			info: &dicomcodec.FrameInfo{
				Width: 17, Height: 9,
				SamplesPerPixel: 2, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
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
			got := openJPHEncodeParams(*tt.info, tt.params, tt.lossless, false)
			if got.Width != 17 || got.Height != 9 || got.Components != int(tt.info.SamplesPerPixel) ||
				got.BitDepth != int(tt.info.BitDepth.BitsStored) || got.InputBitsAllocated != int(tt.info.BitDepth.BitsAllocated) ||
				got.IsSigned != (tt.info.PixelRepresentation != 0) {
				t.Fatalf("frame mapping = %+v, want width=17 height=9 components=%d bitDepth=%d inputBitsAllocated=%d signed=%v",
					got, tt.info.SamplesPerPixel, tt.info.BitDepth.BitsStored, tt.info.BitDepth.BitsAllocated,
					tt.info.PixelRepresentation != 0)
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
