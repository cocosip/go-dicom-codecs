package t1

import (
	"testing"
)

// calculateMaxBitplane uses the same logic as encoder
func calculateMaxBitplane(data []int32) int {
	maxAbs := int32(0)
	for _, val := range data {
		absVal := val
		if absVal < 0 {
			absVal = -absVal
		}
		if absVal > maxAbs {
			maxAbs = absVal
		}
	}

	if maxAbs == 0 {
		return -1
	}

	// Find highest bit set
	bitplane := 0
	for maxAbs > 0 {
		maxAbs >>= 1
		bitplane++
	}

	return bitplane - 1
}

// TestPartialBlockRoundTrip tests T1 encoding/decoding of partial blocks
func TestPartialBlock32x64(t *testing.T) {
	width := 32
	height := 64

	// Create test data
	data := make([]int32, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			data[y*width+x] = int32((x + y) % 256 - 128) // -128 to 127 (DC level shifted)
		}
	}

	// Calculate max bitplane (same logic as encoder)
	maxBitplane := calculateMaxBitplane(data)
	numPasses := (maxBitplane * 3) + 1

	// Encode
	encoder := NewT1Encoder(width, height, 0)
	encoded, err := encoder.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	t.Logf("Encoded %dx%d block: %d coeffs 鈫?%d bytes, maxBP=%d", width, height, len(data), len(encoded), maxBitplane)

	// Decode
	decoder := NewT1Decoder(width, height, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	// Verify
	if len(decoded) != len(data) {
		t.Fatalf("Length mismatch: expected %d, got %d", len(data), len(decoded))
	}

	mismatchCount := 0
	for i := 0; i < len(data); i++ {
		if decoded[i] != data[i] {
			if mismatchCount < 5 {
				x := i % width
				y := i / width
				t.Errorf("Mismatch at (%d,%d): expected=%d got=%d diff=%d",
					x, y, data[i], decoded[i], decoded[i]-data[i])
			}
			mismatchCount++
		}
	}

	if mismatchCount > 0 {
		t.Errorf("Found %d/%d coefficient mismatches", mismatchCount, len(data))
	} else {
		t.Log("鉁?Perfect reconstruction")
	}
}

func TestPartialBlock64x32(t *testing.T) {
	width := 64
	height := 32

	// Create test data
	data := make([]int32, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			data[y*width+x] = int32((x + y) % 256 - 128)
		}
	}

	// Calculate max bitplane (same logic as encoder)
	maxBitplane := calculateMaxBitplane(data)
	numPasses := (maxBitplane * 3) + 1

	// Encode
	encoder := NewT1Encoder(width, height, 0)
	encoded, err := encoder.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	t.Logf("Encoded %dx%d block: %d coeffs 鈫?%d bytes, maxBP=%d", width, height, len(data), len(encoded), maxBitplane)

	// Decode
	decoder := NewT1Decoder(width, height, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	// Verify
	if len(decoded) != len(data) {
		t.Fatalf("Length mismatch: expected %d, got %d", len(data), len(decoded))
	}

	mismatchCount := 0
	for i := 0; i < len(data); i++ {
		if decoded[i] != data[i] {
			if mismatchCount < 5 {
				x := i % width
				y := i / width
				t.Errorf("Mismatch at (%d,%d): expected=%d got=%d diff=%d",
					x, y, data[i], decoded[i], decoded[i]-data[i])
			}
			mismatchCount++
		}
	}

	if mismatchCount > 0 {
		t.Errorf("Found %d/%d coefficient mismatches", mismatchCount, len(data))
	} else {
		t.Log("鉁?Perfect reconstruction")
	}
}

