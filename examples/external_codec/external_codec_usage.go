// Package main provides examples demonstrating "澶栭儴缂栬В鐮佸櫒" usage with JPEG Lossless.
package main

import (
	"fmt"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom-codecs/jpeg/lossless"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

const photometricMonochrome2 = "MONOCHROME2"

func main() {
	fmt.Println("=== JPEG Lossless Codec Usage Example (External Interface) ===")

	// Example 1: Direct codec usage
	fmt.Println("Example 1: Direct codec usage")
	directUsage()
	fmt.Println()

	// Example 2: Registry-based usage
	fmt.Println("Example 2: Registry-based usage")
	registryUsage()
	fmt.Println()

	// Example 3: Using parameters
	fmt.Println("Example 3: Using parameters to specify predictor")
	parametersUsage()
	fmt.Println()
}

func directUsage() {
	// Create test image data (64x64 grayscale)
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y*2) % 256)
		}
	}

	// Create source PixelData
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		fmt.Printf("AddFrame error: %v\n", err)
		return
	}

	// Create codec with predictor 4 (Ra + Rb - Rc)
	losslessCodec := lossless.NewLosslessCodec(4)
	fmt.Printf("Codec: %s\n", losslessCodec.Name())

	// Encode
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := losslessCodec.Encode(context.Background(), src, encoded, nil)
	if err != nil {
		fmt.Printf("Encode error: %v\n", err)
		return
	}

	srcData, _ := src.Frame(context.Background(), 0)
	encodedData, _ := encoded.Frame(context.Background(), 0)
	fmt.Printf("Original size: %d bytes\n", len(srcData))
	fmt.Printf("Compressed size: %d bytes\n", len(encodedData))
	fmt.Printf("Compression ratio: %.2fx\n", float64(len(srcData))/float64(len(encodedData)))

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = losslessCodec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}

	// Verify lossless reconstruction
	decodedData, _ := decoded.Frame(context.Background(), 0)
	errors := 0
	for i := 0; i < len(srcData); i++ {
		if decodedData[i] != srcData[i] {
			errors++
		}
	}

	if errors == 0 {
		fmt.Printf("鉁?Perfect lossless reconstruction: all %d pixels match\n", len(srcData))
	} else {
		fmt.Printf("鉁?Reconstruction errors: %d pixels differ\n", errors)
	}
}

func registryUsage() {
	// Register codec with the global registry
	lossless.RegisterLosslessCodec(1) // Register with predictor 1

	// Get codec from registry
	registry := codec.GlobalRegistry()
	retrievedCodec, exists := registry.Lookup(transfer.JPEGLossless)
	if !exists {
		fmt.Println("Codec not found in registry")
		return
	}

	fmt.Printf("Retrieved codec: %s\n", retrievedCodec.Name())

	// Create test data
	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		fmt.Printf("AddFrame error: %v\n", err)
		return
	}

	// Encode using retrieved codec
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := retrievedCodec.Encode(context.Background(), src, encoded, nil)
	if err != nil {
		fmt.Printf("Encode error: %v\n", err)
		return
	}

	srcData, _ := src.Frame(context.Background(), 0)
	encodedData, _ := encoded.Frame(context.Background(), 0)
	fmt.Printf("Compressed size: %d bytes (%.2fx)\n",
		len(encodedData), float64(len(srcData))/float64(len(encodedData)))

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = retrievedCodec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}

	// Verify
	decodedData, _ := decoded.Frame(context.Background(), 0)
	errors := 0
	for i := 0; i < len(srcData); i++ {
		if decodedData[i] != srcData[i] {
			errors++
		}
	}

	if errors == 0 {
		fmt.Println("鉁?Registry codec works perfectly")
	} else {
		fmt.Printf("鉁?Errors: %d\n", errors)
	}
}

func parametersUsage() {
	// Create codec with auto-select (predictor 0)
	losslessCodec := lossless.NewLosslessCodec(0)

	// Create test data
	width, height := 48, 48
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x*3 + y*5) % 256)
		}
	}

	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		fmt.Printf("AddFrame error: %v\n", err)
		return
	}

	// Create parameters and override predictor
	params := lossless.NewLosslessParameters().WithPredictor(5)

	// Encode with parameters
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := losslessCodec.Encode(context.Background(), src, encoded, params)
	if err != nil {
		fmt.Printf("Encode error: %v\n", err)
		return
	}

	srcData, _ := src.Frame(context.Background(), 0)
	encodedData, _ := encoded.Frame(context.Background(), 0)
	fmt.Printf("Codec default: %s\n", losslessCodec.Name())
	fmt.Printf("Using predictor from parameters: 5 (Ra + ((Rb - Rc) >> 1))\n")
	fmt.Printf("Compressed size: %d bytes (%.2fx)\n",
		len(encodedData), float64(len(srcData))/float64(len(encodedData)))

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = losslessCodec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}

	// Verify
	decodedData, _ := decoded.Frame(context.Background(), 0)
	errors := 0
	for i := 0; i < len(srcData); i++ {
		if decodedData[i] != srcData[i] {
			errors++
		}
	}

	if errors == 0 {
		fmt.Println("鉁?Parameters override works correctly")
	} else {
		fmt.Printf("鉁?Errors: %d\n", errors)
	}
}
