package openjpeg

import (
	"fmt"
	"testing"
)

// TestSimpleMultiPrecinct tests the simplest possible multi-precinct case
func TestSimpleMultiPrecinct(t *testing.T) {
	// Simplest case: 64x64 image, 32x32 precincts, 1 decomposition level
	// This should create 2x2 precincts at the highest resolution only
	width, height := 64, 64
	precinctWidth, precinctHeight := 32, 32
	numLevels := 1

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = numLevels
	params.PrecinctWidth = precinctWidth
	params.PrecinctHeight = precinctHeight

	// Create uniform image (all zeros) for simplicity
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = 128 // Mid-gray
	}

	t.Logf("Test: %dx%d image, %dx%d precincts, %d levels",
		width, height, precinctWidth, precinctHeight, numLevels)

	// Calculate expected precincts
	for res := 0; res <= numLevels; res++ {
		divisor := numLevels - res
		resWidth := width >> divisor
		resHeight := height >> divisor

		numPX := (resWidth + precinctWidth - 1) / precinctWidth
		numPY := (resHeight + precinctHeight - 1) / precinctHeight

		t.Logf("  Res %d: %dx%d, precincts=%dx%d (total=%d)",
			res, resWidth, resHeight, numPX, numPY, numPX*numPY)
	}

	// Encode
	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded: %d bytes", len(encoded))

	// Decode
	decoder := NewDecoder()
	if err := decoder.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify
	decodedBytes := decoder.GetPixelData()
	errors := 0
	maxError := 0

	for i := range pixelData {
		diff := int(pixelData[i]) - int(decodedBytes[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > 0 {
			if errors == 0 {
				t.Logf("First error at pixel %d: expected=%d, got=%d",
					i, pixelData[i], decodedBytes[i])
			}
			errors++
			if diff > maxError {
				maxError = diff
			}
		}
	}

	if errors > 0 {
		t.Errorf("FAIL: %d pixels with errors (max=%d)", errors, maxError)
	} else {
		t.Logf("PASS: Perfect reconstruction (0 errors)")
	}
}

// TestMultiPrecinctDebug tests multi-precinct with detailed packet logging
func TestMultiPrecinctDebug(t *testing.T) {
	t.Skip("For manual debugging only")

	width, height := 64, 64
	precinctWidth, precinctHeight := 32, 32
	numLevels := 1

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = numLevels
	params.PrecinctWidth = precinctWidth
	params.PrecinctHeight = precinctHeight

	// Gradient pattern to make errors visible
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y*2) % 256)
		}
	}

	encoder := NewEncoder(params)

	// TODO: Add hooks to log precinct assignments during encoding

	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoder := NewDecoder()
	if err := decoder.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decodedBytes := decoder.GetPixelData()

	// Show error pattern
	fmt.Println("Error map (first 8x8 block):")
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			idx := y*width + x
			diff := int(pixelData[idx]) - int(decodedBytes[idx])
			if diff < 0 {
				diff = -diff
			}
			if diff == 0 {
				fmt.Print(" . ")
			} else {
				fmt.Printf("%3d", diff)
			}
		}
		fmt.Println()
	}
}
