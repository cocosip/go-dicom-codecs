package lossy_test

import (
	"fmt"
	"log"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"
	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

// ExampleCodec_Encode demonstrates basic lossy encoding
func ExampleCodec_Encode() {
	// Create a simple 16x16 grayscale test image
	width := uint16(16)
	height := uint16(16)
	pixelData := make([]byte, int(width)*int(height))
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Prepare source pixel data
	frameInfo := &dicomcodec.FrameInfo{
		Width:  width,
		Height: height,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		log.Fatalf("AddFrame failed: %v", err)
	}

	// Create codec with default rate (80)
	c := lossy.NewCodecWithRate(80)

	// Encode
	dst := codecHelpers.NewTestPixelData(frameInfo)
	err := c.Encode(context.Background(), src, dst, nil)
	if err != nil {
		log.Fatalf("Encoding failed: %v", err)
	}

	srcData, _ := src.Frame(context.Background(), 0)
	dstData, _ := dst.Frame(context.Background(), 0)
	fmt.Printf("Original size: %d bytes\n", len(srcData))
	fmt.Printf("Encoded size: %d bytes\n", len(dstData))
	fmt.Printf("Compression ratio: %.2f:1\n", float64(len(srcData))/float64(len(dstData)))
}

// ExampleCodec_Encode_withRateParameter demonstrates encoding with custom rate
func ExampleCodec_Encode_withRateParameter() {
	// Create test image
	width := uint16(64)
	height := uint16(64)
	pixelData := make([]byte, int(width)*int(height))
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	frameInfo := &dicomcodec.FrameInfo{
		Width:  width,
		Height: height,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		log.Fatalf("AddFrame failed: %v", err)
	}

	// Test different rate levels
	qualities := []int{100, 80, 50}

	for _, rate := range qualities {
		// Create codec with specific rate
		c := lossy.NewCodecWithRate(rate)

		// Alternatively, you can override rate via parameters:
		// c := lossy.NewCodecWithRate(80) // default
		// params := codec.NewParameters()
		// params.SetParameter("rate", rate)
		// err := c.Encode(src, dst, params)

		dst := codecHelpers.NewTestPixelData(frameInfo)
		err := c.Encode(context.Background(), src, dst, nil)
		if err != nil {
			log.Fatalf("Encoding failed: %v", err)
		}

		// Decode to check quality
		decoded := codecHelpers.NewTestPixelData(frameInfo)
		err = c.Decode(context.Background(), dst, decoded, nil)
		if err != nil {
			log.Fatalf("Decoding failed: %v", err)
		}

		// Calculate error
		srcData, _ := src.Frame(context.Background(), 0)
		decodedData, _ := decoded.Frame(context.Background(), 0)
		var maxError int
		for i := range srcData {
			diff := int(decodedData[i]) - int(srcData[i])
			if diff < 0 {
				diff = -diff
			}
			if diff > maxError {
				maxError = diff
			}
		}

		dstData, _ := dst.Frame(context.Background(), 0)
		compressionRatio := float64(len(srcData)) / float64(len(dstData))
		fmt.Printf("Rate %d: Compression %.2f:1, Max error: %d\n",
			rate, compressionRatio, maxError)
	}

	// Expected approximate results (may vary slightly):
	// Rate 100: Compression ~3:1, Max error: 鈮?
	// Rate 80: Compression ~3-4:1, Max error: 鈮?
	// Rate 50: Compression ~5-6:1, Max error: ~12-15
}

// ExampleCodec_Decode demonstrates basic lossy decoding
func ExampleCodec_Decode() {
	// Assume we have encoded data (in real use, this comes from DICOM file)
	// For this example, we'll encode first
	width := uint16(16)
	height := uint16(16)
	pixelData := make([]byte, int(width)*int(height))
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	frameInfo := &dicomcodec.FrameInfo{
		Width:  width,
		Height: height,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		log.Fatalf("AddFrame failed: %v", err)
	}

	// Encode
	c := lossy.NewCodecWithRate(80)
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	_ = c.Encode(context.Background(), src, encoded, nil)

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err := c.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		log.Fatalf("Decoding failed: %v", err)
	}

	fmt.Printf("Decoded width: %d\n", decoded.FrameInfo().Width)
	fmt.Printf("Decoded height: %d\n", decoded.FrameInfo().Height)
	decodedData, _ := decoded.Frame(context.Background(), 0)
	fmt.Printf("Decoded data size: %d bytes\n", len(decodedData))
}
