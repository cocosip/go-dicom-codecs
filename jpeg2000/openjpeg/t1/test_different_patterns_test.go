package t1

import (
	"fmt"
	"testing"
)

// TestDifferentPatterns_5x5 tests 5x5 with different data patterns
func TestDifferentPatterns_5x5(t *testing.T) {
	patterns := []struct {
		name string
		gen  func(i int) int32
	}{
		{
			name: "Sequential -128 to -104",
			gen:  func(i int) int32 { return int32(i - 128) },
		},
		{
			name: testNameAllZeros,
			gen:  func(_ int) int32 { return 0 },
		},
		{
			name: "All ones",
			gen:  func(_ int) int32 { return 1 },
		},
		{
			name: "Alternating 0 and 1",
			gen:  func(i int) int32 { return int32(i % 2) },
		},
		{
			name: "Small positive values 0-24",
			gen:  func(i int) int32 { return int32(i) },
		},
		{
			name: "Modulo pattern",
			gen:  func(i int) int32 { return int32(i%256 - 128) },
		},
	}

	for _, pattern := range patterns {
		t.Run(pattern.name, func(t *testing.T) {
			width, height := 5, 5
			data := make([]int32, width*height)
			for i := range data {
				data[i] = pattern.gen(i)
			}

			// Calculate actual numPasses based on data
			maxBitplane := CalculateMaxBitplane(data)
			numPasses := (maxBitplane * 3) + 1
			if numPasses < 0 {
				numPasses = 0 // All zeros case
			}

			// Encode
			encoder := NewT1Encoder(width, height, 0)
			encoded, err := encoder.Encode(data, numPasses, 0)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Decode using Decode() function
			decoder := NewT1Decoder(width, height, 0)
			err = decoder.Decode(encoded, numPasses, 0)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			decoded := decoder.GetData()

			// Compare
			errors := 0
			for i := range data {
				if data[i] != decoded[i] {
					errors++
				}
			}

			if errors > 0 {
				t.Errorf("%s: %d/%d errors (%.1f%%)",
					pattern.name, errors, len(data), float64(errors)/float64(len(data))*100)
			} else {
				fmt.Printf("%s: PASS\n", pattern.name)
			}
		})
	}
}
