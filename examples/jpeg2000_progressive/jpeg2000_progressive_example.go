// Package main demonstrates progressive/multi-layer JPEG2000 encoding.
package main

import (
	"fmt"

	"math/rand"

	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"

	"context"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"
	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

const photometricMonochrome2 = "MONOCHROME2"

// This example demonstrates progressive/multi-layer JPEG 2000 encoding
// Multi-layer encoding allows progressive refinement: start with low quality
// preview and progressively improve as more layers are decoded

func main() {
	fmt.Println("=== JPEG 2000 Progressive/Multi-layer Example ===")

	// Create sample image (512x512)
	width, height := 512, 512
	pixelData := generateSampleImage(width, height)

	// Example 1: Basic multi-layer encoding
	fmt.Println("1. Basic Multi-layer Encoding")
	basicMultiLayerExample(pixelData, width, height)
	fmt.Println()

	// Example 2: Multi-layer with target ratio
	fmt.Println("2. Multi-layer with Target Compression Ratio")
	multiLayerWithRatioExample(pixelData, width, height)
	fmt.Println()

	// Example 3: Progressive decoding simulation
	fmt.Println("3. Progressive Decoding Simulation")
	progressiveDecodingExample(pixelData, width, height)
}

// basicMultiLayerExample demonstrates basic multi-layer encoding
func basicMultiLayerExample(pixelData []byte, width, height int) {
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

	// Create parameters with 5 quality layers
	params := lossy.NewLossyParameters().
		WithRate(85).
		WithNumLayers(5). // 5 progressive quality layers
		WithNumLevels(5)

	// Encode
	encoder := lossy.NewCodecWithRate(85)
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := encoder.Encode(context.Background(), src, dst, params)
	if err != nil {
		fmt.Printf("   ERROR: %v\n", err)
		return
	}

	// Report results
	srcData, _ := src.Frame(context.Background(), 0)
	dstData, _ := dst.Frame(context.Background(), 0)
	ratio := float64(len(srcData)) / float64(len(dstData))

	fmt.Printf("   Image size: %dx%d\n", width, height)
	fmt.Printf("   Number of layers: %d\n", params.NumLayers)
	fmt.Printf("   Quality: %d\n", params.Rate)
	fmt.Printf("   Compressed size: %d bytes\n", len(dstData))
	fmt.Printf("   Compression ratio: %.2fx\n", ratio)
	fmt.Println()
	fmt.Println("   Layer distribution (approximate):")
	fmt.Println("   - Layer 0: Base quality (fast preview)")
	fmt.Println("   - Layer 1-3: Progressive refinement")
	fmt.Println("   - Layer 4: Full quality")
	fmt.Println()
	fmt.Println("   鉁?Multi-layer encoding complete")
	fmt.Println("   馃挕 Decoder can stop at any layer for progressive display")

	// Decode (all layers)
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = encoder.Decode(context.Background(), dst, decoded, nil)
	if err != nil {
		fmt.Printf("   Decode ERROR: %v\n", err)
		return
	}

	decodedFrameInfo := decoded.FrameInfo()
	fmt.Printf("   Decoded (full quality): %dx%d\n",
		decodedFrameInfo.Width, decodedFrameInfo.Height)
}

// multiLayerWithRatioExample combines multi-layer with target ratio
func multiLayerWithRatioExample(pixelData []byte, width, height int) {
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

	// Create parameters with layers + target ratio
	params := lossy.NewLossyParameters().
		WithTargetRatio(8.0). // Target 8:1 overall compression
		WithNumLayers(4).     // 4 quality layers
		WithNumLevels(5)

	// Encode
	encoder := lossy.NewCodecWithRate(80)
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := encoder.Encode(context.Background(), src, dst, params)
	if err != nil {
		fmt.Printf("   ERROR: %v\n", err)
		return
	}

	// Report results
	srcData, _ := src.Frame(context.Background(), 0)
	dstData, _ := dst.Frame(context.Background(), 0)
	targetBytes := len(srcData) / 8
	actualRatio := float64(len(srcData)) / float64(len(dstData))
	deviation := (actualRatio - 8.0) / 8.0 * 100

	fmt.Printf("   Target ratio: %.1fx (target size: ~%d bytes)\n",
		8.0, targetBytes)
	fmt.Printf("   Actual ratio: %.2fx (actual size: %d bytes)\n",
		actualRatio, len(dstData))
	fmt.Printf("   Deviation: %.1f%%\n", deviation)
	fmt.Printf("   Number of layers: %d\n", params.NumLayers)
	fmt.Println()
	fmt.Println("   PCRD (Post-Compression Rate-Distortion) optimization:")
	fmt.Println("   - Automatically distributes bits across layers")
	fmt.Println("   - Each layer adds meaningful quality improvement")
	fmt.Println("   - Respects target compression ratio constraint")
	fmt.Println()
	fmt.Println("   鉁?Multi-layer encoding with rate control complete")
}

// progressiveDecodingExample simulates progressive decoding
func progressiveDecodingExample(pixelData []byte, width, height int) {
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

	// Encode with 5 layers
	params := lossy.NewLossyParameters().
		WithRate(85).
		WithNumLayers(5).
		WithNumLevels(5)

	encoder := lossy.NewCodecWithRate(85)
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := encoder.Encode(context.Background(), src, dst, params)
	if err != nil {
		fmt.Printf("   ERROR: %v\n", err)
		return
	}

	fmt.Println("   Simulating progressive image loading:")
	fmt.Println()

	// Simulate progressive display
	// Note: This is a simulation. Actual layer-by-layer decoding
	// requires using the lower-level decoder API (t2/tile_decoder.go)

	dstData, _ := dst.Frame(context.Background(), 0)
	totalBytes := len(dstData)
	layerSizes := []int{
		totalBytes / 10,     // Layer 0: ~10% (fast preview)
		totalBytes * 3 / 10, // Layer 1: ~30% cumulative
		totalBytes * 5 / 10, // Layer 2: ~50% cumulative
		totalBytes * 8 / 10, // Layer 3: ~80% cumulative
		totalBytes,          // Layer 4: 100% (full quality)
	}

	for i, bytes := range layerSizes {
		percent := float64(bytes) / float64(totalBytes) * 100
		quality := []string{
			"Low quality preview",
			"Improved preview",
			"Medium quality",
			"High quality",
			"Full quality",
		}[i]

		fmt.Printf("   Layer %d: %6d bytes (%.0f%%) - %s\n",
			i, bytes, percent, quality)
	}

	fmt.Println()
	fmt.Println("   Use cases for progressive encoding:")
	fmt.Println("   1. Web/Mobile: Show preview while downloading full image")
	fmt.Println("   2. Telemedicine: Quick preview for triage, full quality for diagnosis")
	fmt.Println("   3. Streaming: Adapt quality to network bandwidth")
	fmt.Println("   4. Storage: Single file with multiple quality versions")
	fmt.Println()
	fmt.Println("   鉁?Progressive encoding demonstrated")
	fmt.Println()
	fmt.Println("   馃挕 Note: Full layer-by-layer decoding requires using")
	fmt.Println("      the lower-level tile decoder API for maximum control")

	// Full decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = encoder.Decode(context.Background(), dst, decoded, nil)
	if err != nil {
		fmt.Printf("   Decode ERROR: %v\n", err)
		return
	}

	srcData, _ := src.Frame(context.Background(), 0)
	decodedData, _ := decoded.Frame(context.Background(), 0)
	maxError := calculateMaxError(srcData, decodedData)
	avgError := calculateAvgError(srcData, decodedData)

	fmt.Println()
	fmt.Printf("   Full quality decode:\n")
	fmt.Printf("   - Max pixel error: %d\n", maxError)
	fmt.Printf("   - Avg pixel error: %.2f\n", avgError)
}

// Helper functions

func generateSampleImage(width, height int) []byte {
	// Generate gradient with details
	data := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Gradient
			gradient := uint8((x + y) * 255 / (width + height))

			// Add some high-frequency details
			detail := uint8((x % 8) * (y % 8) * 2)

			// Add noise
			noise := uint8(rand.Intn(10) - 5)

			value := clamp(int(gradient) + int(detail) + int(noise))
			data[y*width+x] = value
		}
	}
	return data
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

func clamp(val int) uint8 {
	if val < 0 {
		return 0
	}
	if val > 255 {
		return 255
	}
	return uint8(val)
}
