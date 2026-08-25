package openjpeg

import (
	"testing"
)

// TestPrecinctDetailedDebug tests precinct partitioning with very detailed logging
// SKIP: Multi-precinct spatial partitioning not yet fully implemented
func TestPrecinctDetailedDebug(t *testing.T) {
	t.Skip("Multi-precinct spatial partitioning in progress - see PRECINCT_TODO.md")
	// Test case: 128x128 image with 32x32 precincts (should have multiple precincts)
	width, height := 128, 128
	precinctWidth, precinctHeight := 32, 32
	numLevels := 2

	// Create test parameters
	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = numLevels
	params.PrecinctWidth = precinctWidth
	params.PrecinctHeight = precinctHeight

	// Create simple test data (gradient)
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	t.Logf("Image: %dx%d, Precincts: %dx%d, Levels: %d", width, height, precinctWidth, precinctHeight, numLevels)

	// Calculate expected precincts
	for res := 0; res <= numLevels; res++ {
		var resWidth, resHeight int
		if res == 0 {
			resWidth = width >> numLevels
			resHeight = height >> numLevels
		} else {
			level := numLevels - res + 1
			resWidth = width >> level
			resHeight = height >> level
		}

		numPX := (resWidth + precinctWidth - 1) / precinctWidth
		numPY := (resHeight + precinctHeight - 1) / precinctHeight
		totalPrecincts := numPX * numPY

		t.Logf("Resolution %d: size=%dx%d, precincts=%dx%d (total=%d)",
			res, resWidth, resHeight, numPX, numPY, totalPrecincts)
	}

	// Encode
	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded size: %d bytes", len(encoded))

	// Decode
	decoder := NewDecoder()
	if err := decoder.Decode(encoded); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify pixel data
	decodedBytes := decoder.GetPixelData()
	errorCount := 0
	maxError := 0
	firstErrorIdx := -1

	for i := range pixelData {
		diff := int(pixelData[i]) - int(decodedBytes[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > 0 {
			if firstErrorIdx == -1 {
				firstErrorIdx = i
				t.Logf("First error at pixel %d (x=%d, y=%d): expected=%d, got=%d, diff=%d",
					i, i%width, i/width, pixelData[i], decodedBytes[i], diff)
			}
			errorCount++
			if diff > maxError {
				maxError = diff
			}
		}
	}

	t.Logf("Errors: %d pixels, max error: %d", errorCount, maxError)

	if errorCount > 0 {
		t.Errorf("Expected 0 pixel errors for lossless, got %d (max error: %d)", errorCount, maxError)
	}
}
