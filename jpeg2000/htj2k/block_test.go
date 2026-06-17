package htj2k

import (
	"testing"
)

// TestHTBlockEncoderDecoder tests HTEncoder and HTDecoder directly
func TestHTBlockEncoderDecoder(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"2x2", 2, 2},
		{"3x3", 3, 3},
		{"4x4", 4, 4},
		{"5x5", 5, 5},
		{"8x8", 8, 8},
		{"16x16", 16, 16},
		{"32x32", 32, 32},
		{"64x64", 64, 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test coefficients
			size := tt.width * tt.height
			testCoeffs := make([]int32, size)
			for i := 0; i < size; i++ {
				testCoeffs[i] = int32(i - size/2) // Range: [-size/2, size/2]
			}

			encoded, decodedCoeffs := encodeDecodeOpenJPHCleanupForTest(t, tt.width, tt.height, testCoeffs)

			t.Logf("Original coeffs: %d int32s (%d bytes equivalent)", len(testCoeffs), len(testCoeffs)*4)
			t.Logf("Encoded size: %d bytes", len(encoded))

			// Compare
			if len(decodedCoeffs) != len(testCoeffs) {
				t.Fatalf("Decoded size mismatch: got %d, want %d", len(decodedCoeffs), len(testCoeffs))
			}

			errors := 0
			maxError := int32(0)
			for i := 0; i < len(testCoeffs); i++ {
				diff := testCoeffs[i] - decodedCoeffs[i]
				if diff < 0 {
					diff = -diff
				}
				if diff > 0 {
					errors++
					if diff > maxError {
						maxError = diff
					}
					if errors <= 5 {
						t.Logf("Error at [%d]: orig=%d, decoded=%d, diff=%d", i, testCoeffs[i], decodedCoeffs[i], diff)
					}
				}
			}

			t.Logf("Errors: %d/%d", errors, len(testCoeffs))
			t.Logf("Max error: %d", maxError)

			if errors > 0 {
				t.Errorf("HTJ2K block codec should have perfect reconstruction, got %d errors", errors)
			}
		})
	}
}

// TestHTBlockZeroCoeffs tests encoding/decoding of all-zero coefficients
func TestHTBlockZeroCoeffs(t *testing.T) {
	width := 8
	height := 8
	size := width * height

	// All zeros
	testCoeffs := make([]int32, size)

	encoded, decodedCoeffs := encodeDecodeOpenJPHCleanupForTest(t, width, height, testCoeffs)

	t.Logf("Zero coeffs encoded size: %d bytes", len(encoded))

	// Verify all zeros
	for i := 0; i < len(decodedCoeffs); i++ {
		if decodedCoeffs[i] != 0 {
			t.Errorf("Expected zero at [%d], got %d", i, decodedCoeffs[i])
		}
	}
}

// TestHTBlockSingleNonZero tests encoding/decoding with single non-zero coefficient
func TestHTBlockSingleNonZero(t *testing.T) {
	width := 8
	height := 8
	size := width * height

	testCoeffs := make([]int32, size)
	testCoeffs[0] = 100 // Single non-zero at top-left

	encoded, decodedCoeffs := encodeDecodeOpenJPHCleanupForTest(t, width, height, testCoeffs)

	t.Logf("Single non-zero encoded size: %d bytes", len(encoded))

	// Verify reconstruction
	if decodedCoeffs[0] != 100 {
		t.Errorf("Expected 100 at [0], got %d", decodedCoeffs[0])
	}

	for i := 1; i < len(decodedCoeffs); i++ {
		if decodedCoeffs[i] != 0 {
			t.Errorf("Expected zero at [%d], got %d", i, decodedCoeffs[i])
		}
	}
}

func TestHTBlockRightEdgeHighPassCoefficient(t *testing.T) {
	width := 64
	height := 64
	coeffs := make([]int32, width*height)
	coeffs[63] = 64

	_, decoded := encodeDecodeOpenJPHCleanupForTest(t, width, height, coeffs)

	if decoded[63] != coeffs[63] {
		t.Fatalf("right-edge coefficient mismatch: got %d, want %d", decoded[63], coeffs[63])
	}
}
