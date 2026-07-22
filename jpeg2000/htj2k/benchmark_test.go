package htj2k

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

func benchmarkCodecEncode(b *testing.B, c *Codec) {
	src, frameInfo, rawBytes := benchmarkPixelData(b, 256, 256)

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

func benchmarkCodecDecode(b *testing.B, c *Codec) {
	src, frameInfo, rawBytes := benchmarkPixelData(b, 256, 256)
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Encode(src, encoded, nil); err != nil {
		b.Fatal(err)
	}
	encodedFrame, err := encoded.GetFrame(0)
	if err != nil {
		b.Fatal(err)
	}
	inputs := make([]*codecHelpers.TestPixelData, b.N)
	for i := range inputs {
		input := codecHelpers.NewTestPixelData(frameInfo)
		if err := input.AddFrame(append([]byte(nil), encodedFrame...)); err != nil {
			b.Fatal(err)
		}
		inputs[i] = input
	}

	b.ReportAllocs()
	b.SetBytes(rawBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := codecHelpers.NewTestPixelData(frameInfo)
		if err := c.Decode(inputs[i], dst, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCodecEncode(b *testing.B) {
	for _, benchmark := range []struct {
		name  string
		codec *Codec
	}{
		{name: "Lossless", codec: NewLosslessCodec()},
		{name: "LosslessRPCL", codec: NewLosslessRPCLCodec()},
		{name: "Lossy", codec: NewCodec(80)},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			benchmarkCodecEncode(b, benchmark.codec)
		})
	}
}

func BenchmarkCodecDecode(b *testing.B) {
	for _, benchmark := range []struct {
		name  string
		codec *Codec
	}{
		{name: "Lossless", codec: NewLosslessCodec()},
		{name: "LosslessRPCL", codec: NewLosslessRPCLCodec()},
		{name: "Lossy", codec: NewCodec(80)},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			benchmarkCodecDecode(b, benchmark.codec)
		})
	}
}
