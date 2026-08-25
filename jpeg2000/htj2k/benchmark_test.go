package htj2k

import (
	"testing"

	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func benchmarkPixelData(b *testing.B, width, height uint16, bitsAllocated, samplesPerPixel uint16) (*codecHelpers.TestPixelData, *imagetypes.FrameInfo, int64) {
	b.Helper()

	bytesPerSample := int(bitsAllocated / 8)
	pixels := make([]byte, int(width)*int(height)*int(samplesPerPixel)*bytesPerSample)
	for sample := 0; sample < len(pixels)/bytesPerSample; sample++ {
		value := uint16(sample*31 + sample/257)
		offset := sample * bytesPerSample
		pixels[offset] = byte(value)
		if bytesPerSample == 2 {
			pixels[offset+1] = byte(value >> 8)
		}
	}
	photometric := photometricMonochrome2
	if samplesPerPixel == 3 {
		photometric = photometricRGB
	}
	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             bitsAllocated,
		BitsStored:                bitsAllocated,
		HighBit:                   bitsAllocated - 1,
		SamplesPerPixel:           samplesPerPixel,
		PhotometricInterpretation: photometric,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixels); err != nil {
		b.Fatal(err)
	}
	return src, frameInfo, int64(len(pixels))
}

func benchmarkCodecEncode(b *testing.B, c *Codec, bitsAllocated, samplesPerPixel uint16) {
	src, frameInfo, rawBytes := benchmarkPixelData(b, 256, 256, bitsAllocated, samplesPerPixel)

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

func benchmarkCodecDecode(b *testing.B, c *Codec, bitsAllocated, samplesPerPixel uint16) {
	src, frameInfo, rawBytes := benchmarkPixelData(b, 256, 256, bitsAllocated, samplesPerPixel)
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
		{name: losslessTestName, codec: NewLosslessCodec()},
		{name: "LosslessRPCL", codec: NewLosslessRPCLCodec()},
		{name: lossyTestName, codec: NewCodec(80)},
	} {
		for _, image := range benchmarkImages() {
			b.Run(benchmark.name+"/"+image.name, func(b *testing.B) {
				benchmarkCodecEncode(b, benchmark.codec, image.bitsAllocated, image.samplesPerPixel)
			})
		}
	}
}

func BenchmarkCodecDecode(b *testing.B) {
	for _, benchmark := range []struct {
		name  string
		codec *Codec
	}{
		{name: losslessTestName, codec: NewLosslessCodec()},
		{name: "LosslessRPCL", codec: NewLosslessRPCLCodec()},
		{name: lossyTestName, codec: NewCodec(80)},
	} {
		for _, image := range benchmarkImages() {
			b.Run(benchmark.name+"/"+image.name, func(b *testing.B) {
				benchmarkCodecDecode(b, benchmark.codec, image.bitsAllocated, image.samplesPerPixel)
			})
		}
	}
}

func benchmarkImages() []struct {
	name                           string
	bitsAllocated, samplesPerPixel uint16
} {
	return []struct {
		name                           string
		bitsAllocated, samplesPerPixel uint16
	}{
		{name: "Mono8", bitsAllocated: 8, samplesPerPixel: 1},
		{name: "Mono16", bitsAllocated: 16, samplesPerPixel: 1},
		{name: "RGB8", bitsAllocated: 8, samplesPerPixel: 3},
		{name: "RGB16", bitsAllocated: 16, samplesPerPixel: 3},
	}
}
