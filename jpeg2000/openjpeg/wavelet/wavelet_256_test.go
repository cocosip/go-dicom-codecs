package wavelet

import (
	"testing"
)

func TestWavelet192x192_1Level(t *testing.T) {
	width := 192
	height := 192
	numLevels := 1

	// Create gradient pattern
	data := make([]int32, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			data[y*width+x] = int32((x + y) % 256)
		}
	}

	// Save original for comparison
	original := make([]int32, len(data))
	copy(original, data)

	// Forward transform
	ForwardMultilevel(data, width, height, numLevels)

	// Inverse transform
	InverseMultilevel(data, width, height, numLevels)

	// Verify perfect reconstruction
	mismatches := 0
	for i := 0; i < len(data); i++ {
		if data[i] != original[i] {
			x := i % width
			y := i / width
			if mismatches < 20 {
				t.Logf("Mismatch at (%d,%d): expected=%d got=%d diff=%d",
					x, y, original[i], data[i], data[i]-original[i])
			}
			mismatches++
		}
	}

	if mismatches > 0 {
		t.Errorf("Found %d mismatches in 192x192 1-level wavelet round-trip", mismatches)
	} else {
		t.Log("✓ Perfect reconstruction for 192x192 1-level wavelet")
	}
}

func TestWavelet256x256_2Levels(t *testing.T) {
	width := 256
	height := 256
	numLevels := 2

	// Create gradient pattern
	data := make([]int32, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			data[y*width+x] = int32((x + y) % 256)
		}
	}

	// Save original for comparison
	original := make([]int32, len(data))
	copy(original, data)

	// Forward transform
	ForwardMultilevel(data, width, height, numLevels)

	// Inverse transform
	InverseMultilevel(data, width, height, numLevels)

	// Verify perfect reconstruction
	mismatches := 0
	for i := 0; i < len(data); i++ {
		if data[i] != original[i] {
			x := i % width
			y := i / width
			if mismatches < 20 {
				t.Logf("Mismatch at (%d,%d): expected=%d got=%d diff=%d",
					x, y, original[i], data[i], data[i]-original[i])
			}
			mismatches++
		}
	}

	if mismatches > 0 {
		t.Errorf("Found %d mismatches in 256x256 2-level wavelet round-trip", mismatches)
	} else {
		t.Log("✓ Perfect reconstruction for 256x256 2-level wavelet")
	}
}
