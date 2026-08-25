package t1

import (
	"testing"
)

// TestT1EncoderCreation tests encoder creation
func TestT1EncoderCreation(t *testing.T) {
	enc := NewT1Encoder(32, 32, 0)
	if enc == nil {
		t.Fatal("Failed to create T1 encoder")
	}

	if enc.width != 32 || enc.height != 32 {
		t.Errorf("Dimensions mismatch: got %dx%d, want 32x32", enc.width, enc.height)
	}
}

// TestT1EncodeDecodeRoundTrip tests encode-decode round trip
func TestT1EncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		data      []int32
		numPasses int
	}{
		{
			name:      "Single non-zero",
			width:     4,
			height:    4,
			data:      makeTestData(4, 4, []int32{0, 0, 0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}),
			numPasses: 10, // maxBitplane=3 -> 10 passes
		},
		{
			name:      "Simple pattern",
			width:     4,
			height:    4,
			data:      makeTestData(4, 4, []int32{1, 0, 0, 0, 0, 2, 0, 0, 0, 0, 4, 0, 0, 0, 0, 8}),
			numPasses: 10,
		},
		{
			name:      "Negative values",
			width:     4,
			height:    4,
			data:      makeTestData(4, 4, []int32{-4, 0, 0, 0, 0, 8, 0, 0, 0, 0, -2, 0, 0, 0, 0, 1}),
			numPasses: 10,
		},
		{
			name:      testNameAllZeros,
			width:     4,
			height:    4,
			data:      makeZeroData(4, 4),
			numPasses: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			enc := NewT1Encoder(tt.width, tt.height, 0)
			encoded, err := enc.Encode(tt.data, tt.numPasses, 0)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Get max bitplane from encoder
			maxBitplane := enc.findMaxBitplane()
			t.Logf("Encoded %d bytes, maxBitplane=%d", len(encoded), maxBitplane)

			// Decode with explicit maxBitplane
			dec := NewT1Decoder(tt.width, tt.height, 0)
			err = dec.DecodeWithBitplane(encoded, tt.numPasses, maxBitplane, 0)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			decoded := dec.GetData()

			// Compare
			if len(decoded) != len(tt.data) {
				t.Fatalf("Length mismatch: got %d, want %d", len(decoded), len(tt.data))
			}

			mismatchCount := 0
			for i := range tt.data {
				if decoded[i] != tt.data[i] {
					if mismatchCount < 5 {
						t.Errorf("Data mismatch at index %d: got %d, want %d",
							i, decoded[i], tt.data[i])
					}
					mismatchCount++
				}
			}

			if mismatchCount > 0 {
				t.Errorf("Total mismatches: %d / %d", mismatchCount, len(tt.data))
			}
		})
	}
}

// TestT1FindMaxBitplane tests max bitplane detection
func TestT1FindMaxBitplane(t *testing.T) {
	tests := []struct {
		name     string
		data     []int32
		expected int
	}{
		{
			name:     testNameAllZeros,
			data:     []int32{0, 0, 0, 0},
			expected: -1,
		},
		{
			name:     "Max value 1",
			data:     []int32{0, 1, 0, 0},
			expected: 0,
		},
		{
			name:     "Max value 7",
			data:     []int32{0, 7, 0, 0},
			expected: 2,
		},
		{
			name:     "Max value 8",
			data:     []int32{0, 8, 0, 0},
			expected: 3,
		},
		{
			name:     "Max value 255",
			data:     []int32{0, 255, 0, 0},
			expected: 7,
		},
		{
			name:     "Negative max",
			data:     []int32{0, -16, 0, 0},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewT1Encoder(2, 2, 0)
			enc.data = make([]int32, 16) // 4x4 with padding

			// Copy data with padding
			paddedWidth := 4
			for i := 0; i < 4; i++ {
				idx := ((i/2)+1)*paddedWidth + ((i % 2) + 1)
				enc.data[idx] = tt.data[i]
			}

			maxBP := enc.findMaxBitplane()
			if maxBP != tt.expected {
				t.Errorf("Max bitplane mismatch: got %d, want %d", maxBP, tt.expected)
			}
		})
	}
}

// TestT1EncodeEmptyBlock tests encoding an all-zero block
func TestT1EncodeEmptyBlock(t *testing.T) {
	enc := NewT1Encoder(8, 8, 0)
	data := makeZeroData(8, 8)

	encoded, err := enc.Encode(data, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	maxBitplane := enc.findMaxBitplane()
	t.Logf("Encoded empty block: %d bytes, maxBitplane=%d", len(encoded), maxBitplane)

	// Decode and verify
	dec := NewT1Decoder(8, 8, 0)
	err = dec.DecodeWithBitplane(encoded, 1, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded := dec.GetData()
	for i, val := range decoded {
		if val != 0 {
			t.Errorf("Index %d: expected 0, got %d", i, val)
		}
	}
}

// TestT1Quantization tests quantization function
func TestT1Quantization(t *testing.T) {
	data := []int32{100, -100, 50, -50, 25, -25}
	original := make([]int32, len(data))
	copy(original, data)

	stepSize := 10.0
	SetQuantization(data, stepSize)

	expected := []int32{10, -10, 5, -5, 2, -2}
	for i := range data {
		if data[i] != expected[i] {
			t.Errorf("Index %d: got %d, want %d (original %d)",
				i, data[i], expected[i], original[i])
		}
	}
}

// Helper functions

func makeTestData(width, height int, values []int32) []int32 {
	if len(values) != width*height {
		panic("values length mismatch")
	}
	return values
}

func makeZeroData(width, height int) []int32 {
	return make([]int32, width*height)
}
