package t1

import (
	"testing"
)

// TestTERMALLLayered tests the DecodeLayered method with TERMALL mode
func TestTERMALLLayered(t *testing.T) {
	// Use a small 8x8 block
	width, height := 8, 8
	data := make([]int32, width*height)

	// Simple pattern
	for i := 0; i < len(data); i++ {
		data[i] = int32(i % 16)
	}

	// Calculate max bitplane
	maxBitplane := 0
	for _, v := range data {
		abs := v
		if abs < 0 {
			abs = -abs
		}
		for b := 0; b < 32; b++ {
			if (abs >> b) == 0 {
				if b-1 > maxBitplane {
					maxBitplane = b - 1
				}
				break
			}
		}
	}

	numPasses := (maxBitplane * 3) + 1
	t.Logf("Testing %dx%d block, maxBitplane=%d, numPasses=%d", width, height, maxBitplane, numPasses)

	// Encode with TERMALL mode
	encoder := NewT1Encoder(width, height, 0)
	layerBoundaries := []int{numPasses} // Single layer
	passes, completeData, err := encoder.EncodeLayered(data, numPasses, 0, layerBoundaries, 0x04) // 0x04 = TERMALL flag
	if err != nil {
		t.Fatalf("TERMALL encode failed: %v", err)
	}
	t.Logf("TERMALL encoded: %d bytes, %d passes", len(completeData), len(passes))

	// Extract cumulative pass lengths
	passLengths := make([]int, len(passes))
	for i, p := range passes {
		passLengths[i] = p.ActualBytes
		passLen := p.ActualBytes
		if i > 0 {
			passLen = p.ActualBytes - passes[i-1].ActualBytes
		}
		t.Logf("  Pass %d: bitplane=%d type=%d actualBytes=%d (len=%d bytes)", i, p.Bitplane, p.PassType, p.ActualBytes, passLen)
	}

	// Decode with DecodeLayered
	decoder := NewT1Decoder(width, height, 0)
	if err := decoder.DecodeLayered(completeData, passLengths, maxBitplane, 0); err != nil {
		t.Fatalf("TERMALL decode failed: %v", err)
	}
	decoded := decoder.GetData()

	// Verify perfect reconstruction
	maxError := int32(0)
	errorCount := 0
	for i := 0; i < len(data); i++ {
		diff := data[i] - decoded[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > 0 {
			errorCount++
			if errorCount <= 3 {
				t.Logf("Error at i=%d: expected=%d got=%d diff=%d", i, data[i], decoded[i], diff)
			}
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("Reconstruction: maxError=%d, errorCount=%d/%d", maxError, errorCount, len(data))

	if maxError > 0 {
		t.Errorf("Expected perfect reconstruction (error=0), got error=%d", maxError)
	}
}

