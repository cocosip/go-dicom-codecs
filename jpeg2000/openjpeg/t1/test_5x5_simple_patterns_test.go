package t1

import (
	"fmt"
	"testing"
)

// Test5x5SimplePatterns - 娴嬭瘯5x5鐨勫悇绉嶇畝鍗曟ā寮?
func Test5x5SimplePatterns(t *testing.T) {
	testCases := []struct {
		name string
		gen  func(x, y int) int32
	}{
		{
			"all_minus128",
			func(_, _ int) int32 { return -128 },
		},
		{
			"two_values_minus128_minus127",
			func(x, y int) int32 {
				if (x+y)%2 == 0 {
					return -128
				}
				return -127
			},
		},
		{
			"horizontal_gradient",
			func(x, _ int) int32 { return int32(-128 + x) },
		},
		{
			"vertical_gradient",
			func(_, y int) int32 { return int32(-128 + y) },
		},
		{
			"full_gradient",
			func(x, y int) int32 {
				i := y*5 + x
				return int32(i%256) - 128
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			size := 5
			numPixels := size * size
			input := make([]int32, numPixels)

			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					idx := y*size + x
					input[idx] = tc.gen(x, y)
				}
			}

			// 鎵撳嵃pattern
			t.Log("\nInput pattern:")
			for y := 0; y < size; y++ {
				line := ""
				for x := 0; x < size; x++ {
					idx := y*size + x
					line += fmt.Sprintf("%5d ", input[idx])
				}
				t.Log(line)
			}

			maxBitplane := CalculateMaxBitplane(input)
			numPasses := (maxBitplane * 3) + 1

			encoder := NewT1Encoder(size, size, 0)
			encoded, err := encoder.Encode(input, numPasses, 0)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			decoder := NewT1Decoder(size, size, 0)
			err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			decoded := decoder.GetData()

			errorCount := 0
			for i := range input {
				if decoded[i] != input[i] {
					errorCount++
				}
			}

			if errorCount > 0 {
				errorRate := float64(errorCount) / float64(numPixels) * 100
				t.Errorf("%s: %d/%d errors (%.1f%%)", tc.name, errorCount, numPixels, errorRate)
			} else {
				t.Logf("%s: PASS (0 errors)", tc.name)
			}
		})
	}
}

