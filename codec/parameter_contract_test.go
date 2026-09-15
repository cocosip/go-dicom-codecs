package codec_test

import (
	"context"
	"errors"
	"testing"

	codecfixtures "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom-codecs/jpeg/baseline"
	"github.com/cocosip/go-dicom-codecs/jpeg/extended"
	"github.com/cocosip/go-dicom-codecs/jpeg/lossless"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k"
	j2klossless "github.com/cocosip/go-dicom-codecs/jpeg2000/lossless"
	j2klossy "github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"
	nearless "github.com/cocosip/go-dicom-codecs/jpegls/nearlossless"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

func TestConfigurableCodecsRejectParametersOfAnotherType(t *testing.T) {
	tests := []struct {
		name  string
		codec codec.Codec
	}{
		{name: "JPEG Baseline", codec: baseline.NewBaselineCodec(90)},
		{name: "JPEG Extended", codec: extended.NewExtendedCodec(8, 90)},
		{name: "JPEG Lossless", codec: lossless.NewLosslessCodec(1)},
		{name: "HTJ2K", codec: htj2k.NewLosslessCodec()},
		{name: "JPEG 2000 Lossless", codec: j2klossless.NewCodec()},
		{name: "JPEG 2000 Lossy", codec: j2klossy.NewCodec()},
		{name: "JPEG-LS Near-Lossless", codec: nearless.NewJPEGLSNearLosslessCodec(3)},
	}

	frameInfo := &codec.FrameInfo{
		Width:                     8,
		Height:                    8,
		SamplesPerPixel:           1,
		BitDepth:                  pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7},
		PhotometricInterpretation: *pixel.Monochrome2,
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := codecfixtures.NewTestPixelData(frameInfo)
			if err := source.AddFrame(context.Background(), make([]byte, 64)); err != nil {
				t.Fatal(err)
			}
			destination := codecfixtures.NewTestPixelData(frameInfo)

			err := test.codec.Encode(context.Background(), source, destination, codec.NoParameters{})
			if !errors.Is(err, codec.ErrInvalidParameters) {
				t.Fatalf("Encode() error = %v, want codec.ErrInvalidParameters", err)
			}
		})
	}
}
