// Package main demonstrates "全部编解码器" usage examples for DICOM JPEG codecs.
package main

import (
	"fmt"

	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom-codecs/jpeg/baseline"
	"github.com/cocosip/go-dicom-codecs/jpeg/lossless"
	"github.com/cocosip/go-dicom-codecs/jpeg/lossless14sv1"
	"github.com/cocosip/go-dicom-codecs/rle"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func main() {
	fmt.Println("=== DICOM Image Codecs - Complete Example ===")

	// Register all codecs
	fmt.Println("Registering all JPEG and RLE codecs...")
	baseline.RegisterBaselineCodec(85)       // Quality 85
	lossless14sv1.RegisterLosslessSV1Codec() // Predictor 1 only
	lossless.RegisterLosslessCodec(4)        // Predictor 4
	rle.RegisterRLECodec()
	fmt.Println("鉁?All codecs registered")

	// Create test image data (64x64 grayscale)
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y*2) % 256)
		}
	}

	fmt.Printf("Test image: %dx%d grayscale, %d bytes\n\n", width, height, len(pixelData))

	// Prepare source PixelData
	frameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		fmt.Printf("AddFrame error: %v\n", err)
		return
	}

	// Get global registry
	registry := codec.GetGlobalRegistry()

	// Test JPEG Baseline (Lossy)
	fmt.Println("--- JPEG Baseline (Process 1) - Lossy ---")
	testCodec(registry, transfer.JPEGBaseline8Bit, src, true)

	// Test JPEG Lossless SV1
	fmt.Println("\n--- JPEG Lossless SV1 (Predictor 1) - Lossless ---")
	testCodec(registry, transfer.JPEGLosslessSV1, src, false)

	// Test JPEG Lossless (All Predictors)
	fmt.Println("\n--- JPEG Lossless (Predictor 4) - Lossless ---")
	testCodec(registry, transfer.JPEGLossless, src, false)

	// Comparison
	fmt.Println("\n=== Compression Comparison ===")
	compareCodecs(registry, src)

	// RGB Example
	fmt.Println("\n=== RGB Image Example ===")
	testRGBImage(registry)
}

func testCodec(registry *codec.Registry, ts *transfer.Syntax, src imagetypes.PixelData, isLossy bool) {
	// Get codec from registry
	c, exists := registry.GetCodec(ts)
	if !exists {
		fmt.Printf("鉁?Codec not found for %s\n", ts.UID().UID())
		return
	}

	fmt.Printf("Codec: %s\n", c.Name())
	fmt.Printf("Transfer Syntax: %s\n", ts.UID().UID())

	// Encode
	encoded := codecHelpers.NewTestPixelData(src.GetFrameInfo())
	err := c.Encode(src, encoded, nil)
	if err != nil {
		fmt.Printf("鉁?Encode failed: %v\n", err)
		return
	}

	srcData, _ := src.GetFrame(0)
	encodedData, _ := encoded.GetFrame(0)
	ratio := float64(len(srcData)) / float64(len(encodedData))
	fmt.Printf("Compressed: %d bytes (%.2fx)\n", len(encodedData), ratio)

	// Decode
	decoded := codecHelpers.NewTestPixelData(src.GetFrameInfo())
	err = c.Decode(encoded, decoded, nil)
	if err != nil {
		fmt.Printf("鉁?Decode failed: %v\n", err)
		return
	}

	// Verify
	decodedData, _ := decoded.GetFrame(0)
	if isLossy {
		// For lossy, check quality
		maxDiff := 0
		totalDiff := 0
		for i := 0; i < len(srcData); i++ {
			diff := int(srcData[i]) - int(decodedData[i])
			if diff < 0 {
				diff = -diff
			}
			if diff > maxDiff {
				maxDiff = diff
			}
			totalDiff += diff
		}
		avgDiff := float64(totalDiff) / float64(len(srcData))
		fmt.Printf("Quality: Max diff=%d, Avg diff=%.2f\n", maxDiff, avgDiff)
		fmt.Println("鉁?Lossy compression completed")
	} else {
		// For lossless, check perfect reconstruction
		errors := 0
		for i := 0; i < len(srcData); i++ {
			if decodedData[i] != srcData[i] {
				errors++
			}
		}
		if errors == 0 {
			fmt.Printf("鉁?Perfect reconstruction: all %d pixels match\n", len(srcData))
		} else {
			fmt.Printf("鉁?Errors: %d pixels differ\n", errors)
		}
	}
}

func compareCodecs(registry *codec.Registry, src imagetypes.PixelData) {
	codecs := []struct {
		name string
		ts   *transfer.Syntax
	}{
		{"JPEG Baseline", transfer.JPEGBaseline8Bit},
		{"JPEG Lossless SV1", transfer.JPEGLosslessSV1},
		{"JPEG Lossless", transfer.JPEGLossless},
	}

	fmt.Printf("%-20s %12s %10s %12s\n", "Codec", "Size (bytes)", "Ratio", "Type")
	fmt.Println("鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€")

	for _, entry := range codecs {
		c, exists := registry.GetCodec(entry.ts)
		if !exists {
			continue
		}

		encoded := codecHelpers.NewTestPixelData(src.GetFrameInfo())
		err := c.Encode(src, encoded, nil)
		if err != nil {
			fmt.Printf("%-20s %12s %10s %12s\n", entry.name, "ERROR", "-", "-")
			continue
		}

		srcData, _ := src.GetFrame(0)
		encodedData, _ := encoded.GetFrame(0)
		ratio := float64(len(srcData)) / float64(len(encodedData))
		codecType := "Lossless"
		if entry.ts == transfer.JPEGBaseline8Bit {
			codecType = "Lossy"
		}

		fmt.Printf("%-20s %12d %9.2fx %12s\n", entry.name, len(encodedData), ratio, codecType)
	}
}

func testRGBImage(registry *codec.Registry) {
	// Create RGB test data (32x32)
	width, height := 32, 32
	components := 3
	pixelData := make([]byte, width*height*components)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * components
			pixelData[offset+0] = byte(x * 8)       // R
			pixelData[offset+1] = byte(y * 8)       // G
			pixelData[offset+2] = byte((x + y) * 4) // B
		}
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           uint16(components),
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "RGB",
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		fmt.Printf("AddFrame error: %v\n", err)
		return
	}

	fmt.Printf("RGB image: %dx%d, %d bytes\n\n", width, height, len(pixelData))

	// Test with Baseline (best for RGB photos)
	c, exists := registry.GetCodec(transfer.JPEGBaseline8Bit)
	if exists {
		encoded := codecHelpers.NewTestPixelData(frameInfo)
		err := c.Encode(src, encoded, nil)
		if err != nil {
			fmt.Printf("RGB Baseline encode failed: %v\n", err)
		} else {
			srcData, _ := src.GetFrame(0)
			encodedData, _ := encoded.GetFrame(0)
			ratio := float64(len(srcData)) / float64(len(encodedData))
			fmt.Printf("JPEG Baseline: %d bytes (%.2fx compression)\n", len(encodedData), ratio)

			decoded := codecHelpers.NewTestPixelData(frameInfo)
			err = c.Decode(encoded, decoded, nil)
			if err != nil {
				fmt.Printf("RGB Baseline decode failed: %v\n", err)
			} else {
				fmt.Println("鉁?RGB lossy compression successful")
			}
		}
	}

	// Test with Lossless SV1
	c, exists = registry.GetCodec(transfer.JPEGLosslessSV1)
	if exists {
		encoded := codecHelpers.NewTestPixelData(frameInfo)
		err := c.Encode(src, encoded, nil)
		if err != nil {
			fmt.Printf("RGB Lossless SV1 encode failed: %v\n", err)
		} else {
			srcData, _ := src.GetFrame(0)
			encodedData, _ := encoded.GetFrame(0)
			ratio := float64(len(srcData)) / float64(len(encodedData))
			fmt.Printf("JPEG Lossless SV1: %d bytes (%.2fx compression)\n", len(encodedData), ratio)

			decoded := codecHelpers.NewTestPixelData(frameInfo)
			err = c.Decode(encoded, decoded, nil)
			if err != nil {
				fmt.Printf("RGB Lossless SV1 decode failed: %v\n", err)
			} else {
				// Check if lossless
				decodedData, _ := decoded.GetFrame(0)
				errors := 0
				for i := 0; i < len(srcData); i++ {
					if decodedData[i] != srcData[i] {
						errors++
					}
				}
				if errors == 0 {
					fmt.Println("鉁?RGB perfect lossless reconstruction")
				} else {
					fmt.Printf("RGB had %d pixel differences\n", errors)
				}
			}
		}
	}
}
