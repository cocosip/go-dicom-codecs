package openjpeg

import (
	"testing"
)

// TestPrecinctComparison compares results with and without precincts
func TestPrecinctComparison(t *testing.T) {
	width, height := 64, 64

	// Create gradient test data
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	// Test 1: No precincts (default)
	t.Run("NoPrecincts", func(t *testing.T) {
		params := DefaultEncodeParams(width, height, 1, 8, false)
		params.NumLevels = 1
		// PrecinctWidth = 0, PrecinctHeight = 0 means use default (very large)

		encoder := NewEncoder(params)
		encoded, err := encoder.Encode(pixelData)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		decoder := NewDecoder()
		if err := decoder.Decode(encoded); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		decodedBytes := decoder.GetPixelData()
		errors := 0
		for i := range pixelData {
			if pixelData[i] != decodedBytes[i] {
				errors++
			}
		}

		t.Logf("Encoded: %d bytes, Errors: %d", len(encoded), errors)
		if errors > 0 {
			t.Errorf("Expected 0 errors, got %d", errors)
		}
	})

	// Test 2: With precincts (32x32)
	t.Run("WithPrecincts32x32", func(t *testing.T) {
		params := DefaultEncodeParams(width, height, 1, 8, false)
		params.NumLevels = 1
		params.PrecinctWidth = 32
		params.PrecinctHeight = 32

		encoder := NewEncoder(params)
		encoded, err := encoder.Encode(pixelData)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		decoder := NewDecoder()
		if err := decoder.Decode(encoded); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		decodedBytes := decoder.GetPixelData()
		errors := 0
		maxError := 0
		for i := range pixelData {
			diff := int(pixelData[i]) - int(decodedBytes[i])
			if diff < 0 {
				diff = -diff
			}
			if diff > 0 {
				errors++
				if diff > maxError {
					maxError = diff
				}
			}
		}

		t.Logf("Encoded: %d bytes, Errors: %d, MaxError: %d", len(encoded), errors, maxError)
		if errors > 0 {
			t.Errorf("Expected 0 errors, got %d (max=%d)", errors, maxError)
		}
	})

	// Test 3: With large precincts (should be same as no precincts)
	t.Run("WithLargePrecincts", func(t *testing.T) {
		params := DefaultEncodeParams(width, height, 1, 8, false)
		params.NumLevels = 1
		params.PrecinctWidth = 128 // Larger than image
		params.PrecinctHeight = 128

		encoder := NewEncoder(params)
		encoded, err := encoder.Encode(pixelData)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		decoder := NewDecoder()
		if err := decoder.Decode(encoded); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		decodedBytes := decoder.GetPixelData()
		errors := 0
		for i := range pixelData {
			if pixelData[i] != decodedBytes[i] {
				errors++
			}
		}

		t.Logf("Encoded: %d bytes, Errors: %d", len(encoded), errors)
		if errors > 0 {
			t.Errorf("Expected 0 errors, got %d", errors)
		}
	})
}
