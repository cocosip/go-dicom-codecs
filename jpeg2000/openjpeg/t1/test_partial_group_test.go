package t1

import (
	"testing"
)

// TestPartialGroup tests images where height is not multiple of 4
func TestPartialGroup(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"1x5", 1, 5},
		{"5x1", 5, 1},
		{"5x4", 5, 4},
		{"5x5", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			numPixels := tt.width * tt.height
			input := make([]int32, numPixels)
			for i := 0; i < numPixels; i++ {
				input[i] = int32(i%256) - 128
			}

			maxBitplane := 7
			numPasses := (maxBitplane * 3) + 1

			// Encode
			encoder := NewT1Encoder(tt.width, tt.height, 0)
			encoded, err := encoder.Encode(input, numPasses, 0)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			// Decode
			decoder := NewT1Decoder(tt.width, tt.height, 0)
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
				t.Errorf("%s: %d/%d errors (%.1f%%)", tt.name, errorCount, numPixels, errorRate)
			} else {
				t.Logf("%s: PASS", tt.name)
			}
		})
	}
}

