package openjpeg

import (
	"testing"
)

// TestMultiLayerLossy - test multi-layer lossy mode
func TestMultiLayerLossy(t *testing.T) {
	// Very small image
	width, height := 64, 64
	pixelData := make([]byte, width*height)

	// Simple gradient
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	// 2-layer lossy
	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLayers = 2
	params.Lossless = false
	params.NumLevels = 5
	params.Quality = 80 // Add quality parameter like TestMultiLayerLossyEncoding

	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}
	t.Logf("Encoded: %d bytes", len(encoded))

	// Decode
	decoder := NewDecoder()
	if err := decoder.Decode(encoded); err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	// Get decoded data
	decodedPixels := decoder.GetPixelData()

	// Calculate error
	maxError := 0
	errorCount := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedPixels[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
		if diff > 0 {
			errorCount++
		}
	}

	t.Logf("Max error: %d, error count: %d/%d", maxError, errorCount, len(pixelData))

	// Show first few original vs decoded values
	t.Logf("First 10 pixels - Original: %v", pixelData[:10])
	t.Logf("First 10 pixels - Decoded:  %v", decodedPixels[:10])

	// Lossy should have some error but not completely broken
	if maxError > 100 {
		t.Errorf("Multi-layer lossy has too much error: max=%d", maxError)
	}
	if errorCount == len(pixelData) {
		t.Errorf("All pixels are wrong - decoder is completely broken")
	}
}
