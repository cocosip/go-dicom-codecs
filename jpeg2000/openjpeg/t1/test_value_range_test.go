package t1

import (
	"testing"
)

// TestValueRange tests if specific value ranges cause issues
func TestValueRange(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		values []int32
	}{
		{
			name:   "4x4_with_5x5_values",
			width:  4,
			height: 4,
			// Use the same first 16 values as 5x5 gradient
			values: []int32{
				-128, -127, -126, -125,
				-124, -123, -122, -121,
				-120, -119, -118, -117,
				-116, -115, -114, -113,
			},
		},
		{
			name:   "5x5_gradient",
			width:  5,
			height: 5,
			// Full 5x5 gradient
			values: func() []int32 {
				v := make([]int32, 25)
				for i := 0; i < 25; i++ {
					v[i] = int32(i%256) - 128
				}
				return v
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxBitplane := 7
			numPasses := (maxBitplane * 3) + 1

			// Encode
			encoder := NewT1Encoder(tt.width, tt.height, 0)
			encoded, err := encoder.Encode(tt.values, numPasses, 0)
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
			for i := range tt.values {
				if decoded[i] != tt.values[i] {
					errorCount++
					t.Logf("[%d] expected=%d, got=%d", i, tt.values[i], decoded[i])
				}
			}

			if errorCount > 0 {
				t.Errorf("%s: %d/%d errors", tt.name, errorCount, len(tt.values))
			} else {
				t.Logf("%s: PASS", tt.name)
			}
		})
	}
}

