package t1

import (
	"testing"
)

// TestMinimalSingleCoeff tests encoding/decoding of a single coefficient
// This is the most minimal test case to debug synchronization issues
func TestMinimalSingleCoeff(t *testing.T) {
	tests := []struct {
		name  string
		value int32
	}{
		{"Value -4", -4},
		{"Value 8", 8},
		{"Value 1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create 1x1 block with single value
			width, height := 1, 1
			data := []int32{tt.value}

			// Encode
			enc := NewT1Encoder(width, height, 0)

			// Find max bitplane from input data (before encoding)
			maxAbs := tt.value
			if maxAbs < 0 {
				maxAbs = -maxAbs
			}
			maxBitplane := 0
			for maxAbs > 0 {
				maxAbs >>= 1
				maxBitplane++
			}
			maxBitplane = maxBitplane - 1
			if maxBitplane < 0 {
				maxBitplane = 0
			}
			numPasses := (maxBitplane * 3) + 1 // Full pass sequencing for all bitplanes

			t.Logf("Input value: %d, maxBitplane: %d, numPasses: %d", tt.value, maxBitplane, numPasses)

			encoded, err := enc.Encode(data, numPasses, 0)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			t.Logf("Encoded %d bytes", len(encoded))

			// Decode
			dec := NewT1Decoder(width, height, 0)
			err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			decoded := dec.GetData()
			if len(decoded) != 1 {
				t.Fatalf("Expected 1 value, got %d", len(decoded))
			}

			if decoded[0] != tt.value {
				t.Errorf("Value mismatch: got %d, want %d", decoded[0], tt.value)
			} else {
				t.Logf("閴?Correct value: %d", decoded[0])
			}
		})
	}
}

// TestMinimalTwoCoeffs tests with two coefficients to see neighbor effects
func TestMinimalTwoCoeffs(t *testing.T) {
	width, height := 2, 1
	data := []int32{-4, 0}

	// Encode
	enc := NewT1Encoder(width, height, 0)

	// Calculate maxBitplane from input data
	maxAbs := int32(0)
	for _, val := range data {
		if val < 0 {
			val = -val
		}
		if val > maxAbs {
			maxAbs = val
		}
	}
	maxBitplane := 0
	if maxAbs > 0 {
		for maxAbs > 0 {
			maxAbs >>= 1
			maxBitplane++
		}
		maxBitplane--
	}
	numPasses := (maxBitplane * 3) + 1

	t.Logf("Input: %v, maxBitplane: %d", data, maxBitplane)

	encoded, err := enc.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded %d bytes", len(encoded))

	// Decode
	dec := NewT1Decoder(width, height, 0)
	err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded := dec.GetData()

	for i := range data {
		if decoded[i] != data[i] {
			t.Errorf("Index %d: got %d, want %d", i, decoded[i], data[i])
		}
	}
}

