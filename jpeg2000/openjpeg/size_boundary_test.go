package openjpeg

import (
	"fmt"
	"testing"
)

// TestSizeBoundary tests different image sizes to find exact failure boundary
func TestSizeBoundary(t *testing.T) {
	sizes := []int{
		8, 12, 16, 20, 24, 28, 32, 36, 40, 48, 64,
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size, size), func(t *testing.T) {
			// Create gradient test data
			numPixels := size * size
			componentData := make([][]int32, 1)
			componentData[0] = make([]int32, numPixels)

			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					idx := y*size + x
					componentData[0][idx] = int32((x + y) % 256)
				}
			}

			// Encode with 0 levels (no wavelet transform)
			params := DefaultEncodeParams(size, size, 1, 8, false)
			params.NumLevels = 0
			encoder := NewEncoder(params)

			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			// Decode
			decoder := NewDecoder()
			err = decoder.Decode(encoded)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			// Verify
			decodedData, err := decoder.GetComponentData(0)
			if err != nil {
				t.Fatalf("GetComponentData failed: %v", err)
			}
			if len(decodedData) != numPixels {
				t.Fatalf("Decoded data length mismatch: got %d, want %d", len(decodedData), numPixels)
			}

			// Count errors
			errorCount := 0
			maxError := int32(0)
			for i := 0; i < numPixels; i++ {
				diff := componentData[0][i] - decodedData[i]
				if diff < 0 {
					diff = -diff
				}
				if diff != 0 {
					errorCount++
					if diff > maxError {
						maxError = diff
					}
				}
			}

			errorRate := float64(errorCount) / float64(numPixels) * 100
			if errorCount == 0 {
				t.Logf("✓ %dx%d: Perfect reconstruction, %d bytes", size, size, len(encoded))
			} else {
				t.Logf("✗ %dx%d: %.1f%% errors (%d/%d), max error: %d, %d bytes",
					size, size, errorRate, errorCount, numPixels, maxError, len(encoded))
			}

			// Fail if not perfect for lossless
			if errorCount > 0 {
				t.Errorf("Expected lossless reconstruction, got %d errors", errorCount)
			}
		})
	}
}
