package openjpeg

import (
	"testing"
)

// TestMultiPrecinctGradient tests multi-precinct with gradient pattern
func TestMultiPrecinctGradient(t *testing.T) {
	// Same as TestSimpleMultiPrecinct but with gradient instead of uniform
	width, height := 64, 64
	precinctWidth, precinctHeight := 32, 32
	numLevels := 1

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = numLevels
	params.PrecinctWidth = precinctWidth
	params.PrecinctHeight = precinctHeight

	// Gradient pattern
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	t.Logf("Test: %dx%d gradient, %dx%d precincts, %d levels",
		width, height, precinctWidth, precinctHeight, numLevels)

	// Encode
	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

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
				x, y := i%width, i/width
				t.Logf("First error at pixel %d (x=%d,y=%d): expected=%d, got=%d, diff=%d",
					i, x, y, pixelData[i], decodedBytes[i], diff)
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

// TestMultiPrecinctTwoLevels tests with 2 decomposition levels
func TestMultiPrecinctTwoLevels(t *testing.T) {
	width, height := 64, 64
	precinctWidth, precinctHeight := 32, 32
	numLevels := 2 // More levels

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = numLevels
	params.PrecinctWidth = precinctWidth
	params.PrecinctHeight = precinctHeight

	// Uniform data for simplicity
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = 128
	}

	t.Logf("Test: %dx%d uniform, %dx%d precincts, %d levels",
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
				x, y := i%width, i/width
				t.Logf("First error at pixel %d (x=%d,y=%d): expected=%d, got=%d, diff=%d",
					i, x, y, pixelData[i], decodedBytes[i], diff)
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
