package openjpeg

import (
	"testing"
)

// TestSingleLayerLossy verifies single-layer lossy encode/decode accuracy.
func TestSingleLayerLossy(t *testing.T) {
	width, height := 8, 8
	pixelData := make([]byte, width*height)
	for i := 0; i < width*height; i++ {
		pixelData[i] = byte(i % 16)
	}

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLayers = 1
	params.Lossless = false
	params.NumLevels = 2

	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	decoder := NewDecoder()
	if err := decoder.Decode(encoded); err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetPixelData()

	t.Logf("Original: %v", pixelData)
	t.Logf("Decoded:  %v", decoded)

	maxError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decoded[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("Max error: %d", maxError)

	if maxError > 20 {
		t.Errorf("Single-layer lossy has too much error: max=%d", maxError)
	}
}
