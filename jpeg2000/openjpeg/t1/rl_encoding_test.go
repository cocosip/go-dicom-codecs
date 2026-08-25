package t1

import (
	"fmt"
	"testing"
)

// TestRLEncodingPatterns tests RL encoding with different patterns
func TestRLEncodingPatterns(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		pattern func(x, y int) int32
	}{
		{
			name:   "8x8_gradient",
			width:  8,
			height: 8,
			pattern: func(x, y int) int32 {
				return int32((y*8+x)*4) - 128
			},
		},
		{
			name:   "16x16_gradient",
			width:  16,
			height: 16,
			pattern: func(x, y int) int32 {
				return int32(y*16+x) - 128
			},
		},
		{
			name:   "32x32_gradient",
			width:  32,
			height: 32,
			pattern: func(x, y int) int32 {
				return int32((y*32+x)/4) - 128
			},
		},
		{
			name:   "3x3_uniform",
			width:  3,
			height: 3,
			pattern: func(_, _ int) int32 {
				return -128
			},
		},
		{
			name:   "16x16_uniform",
			width:  16,
			height: 16,
			pattern: func(_, _ int) int32 {
				return -128
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			numPixels := tt.width * tt.height
			input := make([]int32, numPixels)

			// Generate pattern
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					input[y*tt.width+x] = tt.pattern(x, y)
				}
			}

			// Calculate actual maxBitplane from data (simulates T2 layer)
			maxBitplane := CalculateMaxBitplane(input)
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

			// Check mismatches
			mismatches := 0
			maxError := int32(0)
			for i := 0; i < numPixels; i++ {
				if decoded[i] != input[i] {
					mismatches++
					diff := decoded[i] - input[i]
					if diff < 0 {
						diff = -diff
					}
					if diff > maxError {
						maxError = diff
					}
				}
			}

			errorRate := float64(mismatches) / float64(numPixels) * 100
			if mismatches > 0 {
				t.Logf("Size: %dx%d, Encoded: %d bytes", tt.width, tt.height, len(encoded))
				t.Errorf("Errors: %d/%d (%.1f%%), max error: %d",
					mismatches, numPixels, errorRate, maxError)
			} else {
				t.Logf("鉁?Perfect: %dx%d, %d bytes", tt.width, tt.height, len(encoded))
			}
		})
	}
}

// TestRLBoundaryConditions tests RL encoding at 4-pixel boundaries
func TestRLBoundaryConditions(t *testing.T) {
	// Test different widths to understand RL boundary behavior
	widths := []int{3, 4, 5, 7, 8, 9, 11, 12, 13, 15, 16, 17}

	for _, width := range widths {
		height := width
		t.Run(fmt.Sprintf("%dx%d_gradient", width, height), func(t *testing.T) {
			numPixels := width * height
			input := make([]int32, numPixels)

			// Gradient pattern
			for i := 0; i < numPixels; i++ {
				input[i] = int32(i%256) - 128
			}

			// Calculate actual maxBitplane from data (simulates T2 layer)
			maxBitplane := CalculateMaxBitplane(input)
			numPasses := (maxBitplane * 3) + 1

			encoder := NewT1Encoder(width, height, 0)
			encoded, err := encoder.Encode(input, numPasses, 0)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			decoder := NewT1Decoder(width, height, 0)
			err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			decoded := decoder.GetData()

			// Count mismatches
			mismatches := 0
			for i := 0; i < numPixels; i++ {
				if decoded[i] != input[i] {
					mismatches++
				}
			}

			errorRate := float64(mismatches) / float64(numPixels) * 100
			if mismatches > 0 {
				t.Errorf("%dx%d: %.1f%% errors", width, height, errorRate)
			} else {
				t.Logf("%dx%d: 鉁?Perfect", width, height)
			}
		})
	}
}

