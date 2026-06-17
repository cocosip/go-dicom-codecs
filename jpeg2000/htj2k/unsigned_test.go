package htj2k

import (
	"testing"
)

// TestHTEncoderUnsignedValues tests HTEncoder/HTDecoder with unsigned values
func TestHTEncoderUnsignedValues(t *testing.T) {
	tests := []struct {
		name   string
		values []int32
	}{
		{"AllZeros", make([]int32, 64)}, // All zeros
		{"AllOnes", makeConstant(64, 1)},
		{"Sequence_0to63", makeSequence(0, 64)},
		{"Sequence_128to191", makeSequence(128, 64)},
		{"Mixed_Signed", makeMixedSigned(64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width := 8
			height := 8

			t.Logf("Input (first 16): %v", tt.values[:16])

			_, decoded := encodeDecodeOpenJPHCleanupForTest(t, width, height, tt.values)

			t.Logf("Decoded (first 16): %v", decoded[:16])

			// Check
			errors := 0
			for i := 0; i < len(tt.values); i++ {
				if tt.values[i] != decoded[i] {
					errors++
					if errors <= 5 {
						t.Errorf("Pixel %d: expected %d, got %d", i, tt.values[i], decoded[i])
					}
				}
			}
			if errors > 0 {
				t.Errorf("Total errors: %d/%d", errors, len(tt.values))
			} else {
				t.Logf("✓ Perfect reconstruction")
			}
		})
	}
}

func makeConstant(size int, value int32) []int32 {
	data := make([]int32, size)
	for i := range data {
		data[i] = value
	}
	return data
}

func makeSequence(start int, size int) []int32 {
	data := make([]int32, size)
	for i := range data {
		data[i] = int32(start + i)
	}
	return data
}

func makeMixedSigned(size int) []int32 {
	data := make([]int32, size)
	for i := range data {
		data[i] = int32(i - size/2)
	}
	return data
}
