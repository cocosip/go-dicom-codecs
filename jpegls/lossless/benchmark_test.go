package lossless

import (
	"testing"

	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func benchmarkPixelData(b *testing.B, width, height uint16) (*codecHelpers.TestPixelData, *imagetypes.FrameInfo, int64) {
	b.Helper()

	pixels := make([]byte, int(width)*int(height))
	for i := range pixels {
		pixels[i] = byte(i*31 + i/257)
	}
	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PhotometricInterpretation: photometricMonochrome2,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixels); err != nil {
		b.Fatal(err)
	}
	return src, frameInfo, int64(len(pixels))
}

func BenchmarkCodecEncode(b *testing.B) {
	src, frameInfo, rawBytes := benchmarkPixelData(b, 512, 512)
	c := NewJPEGLSLosslessCodec()

	b.ReportAllocs()
	b.SetBytes(rawBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := codecHelpers.NewTestPixelData(frameInfo)
		if err := c.Encode(src, dst, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCodecDecode(b *testing.B) {
	src, frameInfo, rawBytes := benchmarkPixelData(b, 512, 512)
	c := NewJPEGLSLosslessCodec()
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Encode(src, encoded, nil); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.SetBytes(rawBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := codecHelpers.NewTestPixelData(frameInfo)
		if err := c.Decode(encoded, dst, nil); err != nil {
			b.Fatal(err)
		}
	}
}
