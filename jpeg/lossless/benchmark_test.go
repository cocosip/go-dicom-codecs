package lossless

import (
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

func benchmarkPixelData(b *testing.B, width, height uint16) (*codecHelpers.TestPixelData, *dicomcodec.FrameInfo, int64) {
	b.Helper()

	pixels := make([]byte, int(width)*int(height))
	for i := range pixels {
		pixels[i] = byte(i*31 + i/257)
	}
	frameInfo := &dicomcodec.FrameInfo{
		Width:  width,
		Height: height,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixels); err != nil {
		b.Fatal(err)
	}
	return src, frameInfo, int64(len(pixels))
}

func BenchmarkCodecEncode(b *testing.B) {
	src, frameInfo, rawBytes := benchmarkPixelData(b, 512, 512)
	c := NewLosslessCodec(1)

	b.ReportAllocs()
	b.SetBytes(rawBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := codecHelpers.NewTestPixelData(frameInfo)
		if err := c.Encode(context.Background(), src, dst, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCodecDecode(b *testing.B) {
	src, frameInfo, rawBytes := benchmarkPixelData(b, 512, 512)
	c := NewLosslessCodec(1)
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Encode(context.Background(), src, encoded, nil); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.SetBytes(rawBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := codecHelpers.NewTestPixelData(frameInfo)
		if err := c.Decode(context.Background(), encoded, dst, nil); err != nil {
			b.Fatal(err)
		}
	}
}
