// Package main demonstrates basic JPEG2000 lossless and lossy compression.
package main

import (
	"fmt"
	"math/rand"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/lossless"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"
	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

const photometricMonochrome2 = "MONOCHROME2"

// This example demonstrates basic JPEG 2000 lossless and lossy compression

func main() {
	fmt.Println("=== JPEG 2000 Basic Example ===")

	// Create sample image data (512x512 grayscale)
	width, height := 512, 512
	pixelData := generateSampleImage(width, height)

	// Example 1: Lossless compression
	fmt.Println("1. Lossless Compression (Perfect Reconstruction)")
	losslessExample(pixelData, width, height)
	fmt.Println()

	// Example 2: Lossy compression with different quality levels
	fmt.Println("2. Lossy Compression (Quality Control)")
	lossyQualityExample(pixelData, width, height)
	fmt.Println()

	// Example 3: Target compression ratio
	fmt.Println("3. Lossy Compression (Target Ratio)")
	lossyRatioExample(pixelData, width, height)
}

// losslessExample demonstrates lossless JPEG 2000 compression
func losslessExample(pixelData []byte, width, height int) {
	// Create source pixel data
	frameInfo := &dicomcodec.FrameInfo{
		Width:           uint16(width),
		Height:          uint16(height),
		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		fmt.Printf("   ERROR: AddFrame failed: %v\n", err)
		return
	}

	// Create parameters (default NumLevels=5)
	params := lossless.NewLosslessParameters().WithNumLevels(5)

	// Create codec and encode
	encoder := lossless.NewCodec()
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := encoder.Encode(context.Background(), src, dst, params)
	if err != nil {
		fmt.Printf("   ERROR: Encode failed: %v\n", err)
		return
	}

	// Report compression
	srcData, _ := src.Frame(context.Background(), 0)
	dstData, _ := dst.Frame(context.Background(), 0)
	ratio := float64(len(srcData)) / float64(len(dstData))
	fmt.Printf("   Original size: %d bytes\n", len(srcData))
	fmt.Printf("   Compressed size: %d bytes\n", len(dstData))
	fmt.Printf("   Compression ratio: %.2fx\n", ratio)

	// Decode back
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = encoder.Decode(context.Background(), dst, decoded, nil)
	if err != nil {
		fmt.Printf("   ERROR: Decode failed: %v\n", err)
		return
	}

	// Verify perfect reconstruction
	decodedData, _ := decoded.Frame(context.Background(), 0)
	errors := countPixelErrors(srcData, decodedData)
	fmt.Printf("   Pixel errors: %d (should be 0 for lossless)\n", errors)
	if errors == 0 {
		fmt.Println("   鉁?Perfect reconstruction!")
	} else {
		fmt.Println("   鉂?Reconstruction errors detected!")
	}
}

// lossyQualityExample demonstrates lossy compression with different quality levels
func lossyQualityExample(pixelData []byte, width, height int) {
	frameInfo := &dicomcodec.FrameInfo{
		Width:           uint16(width),
		Height:          uint16(height),
		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		fmt.Printf("   ERROR: AddFrame failed: %v\n", err)
		return
	}

	// Try different quality levels
	qualities := []int{95, 85, 70, 50}

	for _, quality := range qualities {
		// Create parameters
		params := lossy.NewLossyParameters().
			WithRate(quality).
			WithNumLevels(5)

		// Create codec and encode
		encoder := lossy.NewCodecWithRate(quality)
		dst := codecHelpers.NewTestPixelData(frameInfo)

		err := encoder.Encode(context.Background(), src, dst, params)
		if err != nil {
			fmt.Printf("   Quality %d: ERROR: %v\n", quality, err)
			continue
		}

		// Decode
		decoded := codecHelpers.NewTestPixelData(frameInfo)
		err = encoder.Decode(context.Background(), dst, decoded, nil)
		if err != nil {
			fmt.Printf("   Quality %d: Decode ERROR: %v\n", quality, err)
			continue
		}

		// Calculate metrics
		srcData, _ := src.Frame(context.Background(), 0)
		dstData, _ := dst.Frame(context.Background(), 0)
		decodedData, _ := decoded.Frame(context.Background(), 0)
		ratio := float64(len(srcData)) / float64(len(dstData))
		maxError := calculateMaxError(srcData, decodedData)
		avgError := calculateAvgError(srcData, decodedData)

		fmt.Printf("   Quality %d: %.2fx compression, max error=%d, avg error=%.2f\n",
			quality, ratio, maxError, avgError)
	}
}

// lossyRatioExample demonstrates target compression ratio
func lossyRatioExample(pixelData []byte, width, height int) {
	frameInfo := &dicomcodec.FrameInfo{
		Width:           uint16(width),
		Height:          uint16(height),
		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		fmt.Printf("   ERROR: AddFrame failed: %v\n", err)
		return
	}

	// Try different target ratios
	targetRatios := []float64{5.0, 8.0, 10.0}

	for _, targetRatio := range targetRatios {
		// Create parameters with target ratio
		params := lossy.NewLossyParameters().
			WithTargetRatio(targetRatio).
			WithNumLevels(5)

		// Create codec
		encoder := lossy.NewCodecWithRate(80) // Default quality
		dst := codecHelpers.NewTestPixelData(frameInfo)

		err := encoder.Encode(context.Background(), src, dst, params)
		if err != nil {
			fmt.Printf("   Target %.1fx: ERROR: %v\n", targetRatio, err)
			continue
		}

		// Calculate actual ratio
		srcData, _ := src.Frame(context.Background(), 0)
		dstData, _ := dst.Frame(context.Background(), 0)
		actualRatio := float64(len(srcData)) / float64(len(dstData))
		deviation := (actualRatio - targetRatio) / targetRatio * 100

		fmt.Printf("   Target %.1fx: Actual %.2fx (%.1f%% deviation)\n",
			targetRatio, actualRatio, deviation)
	}

	fmt.Println("   Note: Actual ratio may vary 卤5-20% depending on image complexity")
}

// Helper functions

func generateSampleImage(width, height int) []byte {
	// Generate gradient pattern
	data := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Gradient with some noise
			gradient := uint8(x * 255 / width)
			noise := uint8(rand.Intn(10) - 5)
			data[y*width+x] = gradient + noise
		}
	}
	return data
}

func countPixelErrors(original, decoded []byte) int {
	if len(original) != len(decoded) {
		return len(original) // All pixels wrong
	}

	errors := 0
	for i := range original {
		if original[i] != decoded[i] {
			errors++
		}
	}
	return errors
}

func calculateMaxError(original, decoded []byte) int {
	if len(original) != len(decoded) {
		return 255
	}

	maxError := 0
	for i := range original {
		diff := int(original[i]) - int(decoded[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}
	return maxError
}

func calculateAvgError(original, decoded []byte) float64 {
	if len(original) != len(decoded) {
		return 255.0
	}

	sum := 0
	for i := range original {
		diff := int(original[i]) - int(decoded[i])
		if diff < 0 {
			diff = -diff
		}
		sum += diff
	}
	return float64(sum) / float64(len(original))
}
