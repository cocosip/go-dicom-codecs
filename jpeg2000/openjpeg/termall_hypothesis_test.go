package openjpeg

import (
	"testing"
)

// TestTERMALLHypothesis tests if TERMALL mode itself causes decoding errors
// This verifies that the issue is with TERMALL mode compatibility, not multi-layer logic
func TestTERMALLHypothesis(t *testing.T) {
	width, height := 8, 8
	pixelData := make([]byte, width*height)
	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	// Encode with NumLayers=1 (should use old Encode method - works perfectly)
	params1 := DefaultEncodeParams(width, height, 1, 8, false)
	params1.Lossless = true
	params1.NumLayers = 1
	params1.NumLevels = 2 // Use 2 levels for 8x8 image

	encoder1 := NewEncoder(params1)
	encoded1, err := encoder1.Encode(pixelData)
	if err != nil {
		t.Fatalf("Normal encoding failed: %v", err)
	}

	decoder1 := NewDecoder()
	if err := decoder1.Decode(encoded1); err != nil {
		t.Fatalf("Normal decoding failed: %v", err)
	}
	decoded1 := decoder1.GetPixelData()

	// Calculate error
	maxError1 := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decoded1[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError1 {
			maxError1 = diff
		}
	}

	t.Logf("Normal mode (old Encode): error=%d", maxError1)

	// Now test if we force NumLayers=2 to use EncodeLayered with TERMALL
	params2 := DefaultEncodeParams(width, height, 1, 8, false)
	params2.Lossless = true
	params2.NumLayers = 2 // Force use of EncodeLayered
	params2.NumLevels = 2 // Use 2 levels for 8x8 image

	encoder2 := NewEncoder(params2)
	encoded2, err := encoder2.Encode(pixelData)
	if err != nil {
		t.Fatalf("TERMALL encoding failed: %v", err)
	}

	decoder2 := NewDecoder()
	if err := decoder2.Decode(encoded2); err != nil {
		t.Fatalf("TERMALL decoding failed: %v", err)
	}
	decoded2 := decoder2.GetPixelData()

	maxError2 := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decoded2[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError2 {
			maxError2 = diff
		}
	}

	t.Logf("TERMALL mode (EncodeLayered): error=%d", maxError2)

	// If hypothesis is correct, error2 should be non-zero
	if maxError2 > 0 {
		t.Logf("✓ Hypothesis confirmed: TERMALL mode causes decoding errors when decoder doesn't expect it")
	} else {
		t.Logf("✗ Hypothesis rejected: TERMALL mode works correctly")
	}
}
