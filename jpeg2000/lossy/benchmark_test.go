package lossy

import (
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

func benchmarkPixelData(b *testing.B, width, height uint16, bitsAllocated, samplesPerPixel uint16) (*codecHelpers.TestPixelData, *dicomcodec.FrameInfo, int64) {
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
	frameInfo := &dicomcodec.FrameInfo{
		Width:  width,
		Height: height,

		SamplesPerPixel: samplesPerPixel, BitDepth: pixel.BitDepth{BitsAllocated: bitsAllocated, BitsStored: bitsAllocated, HighBit: bitsAllocated - 1, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometric),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixels); err != nil {
		b.Fatal(err)
	}
	return src, frameInfo, int64(len(pixels))
}

func BenchmarkCodecEncode(b *testing.B) {
	for _, image := range benchmarkImages() {
		b.Run(image.name, func(b *testing.B) {
			src, frameInfo, rawBytes := benchmarkPixelData(b, 128, 128, image.bitsAllocated, image.samplesPerPixel)
			c := NewCodec()
			b.ReportAllocs()
			b.SetBytes(rawBytes)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dst := codecHelpers.NewTestPixelData(frameInfo)
				if err := c.Encode(context.Background(), src, dst, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCodecDecode(b *testing.B) {
	for _, image := range benchmarkImages() {
		b.Run(image.name, func(b *testing.B) {
			src, frameInfo, rawBytes := benchmarkPixelData(b, 128, 128, image.bitsAllocated, image.samplesPerPixel)
			c := NewCodec()
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
		})
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
