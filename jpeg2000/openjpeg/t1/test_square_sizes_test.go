package t1

import (
	"fmt"
	"testing"
)

// TestSquareSizes tests various square sizes
func TestSquareSizes(t *testing.T) {
	sizes := []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 30, 32, 64}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size, size), func(t *testing.T) {
			numPixels := size * size
			input := make([]int32, numPixels)
			for i := 0; i < numPixels; i++ {
				input[i] = int32(i%256) - 128
			}

			maxBitplane := CalculateMaxBitplane(input)
			numPasses := (maxBitplane * 3) + 1

			// Encode
			encoder := NewT1Encoder(size, size, 0)
			encoded, err := encoder.Encode(input, numPasses, 0)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			// Decode
			decoder := NewT1Decoder(size, size, 0)
			err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			decoded := decoder.GetData()

			// Check
			errorCount := 0
			for i := range input {
				if decoded[i] != input[i] {
					errorCount++
				}
			}

			errorRate := float64(errorCount) / float64(numPixels) * 100
			if errorCount > 0 {
				t.Errorf("%dx%d: %d/%d errors (%.1f%%)", size, size, errorCount, numPixels, errorRate)
			}
		})
	}
}

