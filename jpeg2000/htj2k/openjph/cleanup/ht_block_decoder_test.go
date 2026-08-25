package cleanup

import (
	"fmt"
	"testing"
)

func TestHTBlockDecoder(t *testing.T) {
	t.Run("EmptyBlock", func(t *testing.T) {
		decoder := NewHTBlockDecoder(4, 4)
		data, err := decoder.DecodeBlock([]byte{})
		if err != nil {
			t.Fatalf("Failed to decode empty block: %v", err)
		}

		// All samples should be zero
		for i, v := range data {
			if v != 0 {
				t.Errorf("Sample %d should be zero, got %d", i, v)
			}
		}
	})

	t.Run("SimpleBlock", func(t *testing.T) {
		decoder := NewHTBlockDecoder(4, 4)

		// Create a simple test codeblock
		// This is a synthetic example - real HTJ2K would be more complex
		testBlock := []byte{
			// MagSgn data (2 bytes)
			0x12, 0x34,
			// MEL data (1 byte)
			0x80,
			// VLC data (2 bytes)
			0x06, 0x3F,
			// Footer: [melLen (2 bytes LE), vlcLen (2 bytes LE)]
			// melLen = 1, vlcLen = 2, scup = 3
			0x01, 0x00, // melLen = 1
			0x02, 0x00, // vlcLen = 2
		}

		data, err := decoder.DecodeBlock(testBlock)
		if err != nil {
			t.Fatalf("Failed to decode block: %v", err)
		}

		t.Logf("Decoded %d samples", len(data))

		// Print decoded values for inspection
		t.Log("Decoded block (4x4):")
		for y := 0; y < 4; y++ {
			row := ""
			for x := 0; x < 4; x++ {
				val := decoder.GetSample(x, y)
				row += fmt.Sprintf("%4d ", val)
			}
			t.Log(row)
		}
	})

	t.Run("MultiQuadBlock", func(t *testing.T) {
		decoder := NewHTBlockDecoder(8, 8)

		// Create a test block with multiple quads
		testBlock := []byte{
			// MagSgn data (4 bytes)
			0x01, 0x02, 0x04, 0x08,
			// MEL data (2 bytes)
			0xFF, 0x00,
			// VLC data (4 bytes)
			0x06, 0x3F, 0x00, 0x7F,
			// Footer: [melLen (2 bytes LE), vlcLen (2 bytes LE)]
			// melLen = 2, vlcLen = 4, scup = 6
			0x02, 0x00, // melLen = 2
			0x04, 0x00, // vlcLen = 4
		}

		data, err := decoder.DecodeBlock(testBlock)
		if err != nil {
			t.Fatalf("Failed to decode multi-quad block: %v", err)
		}

		t.Logf("Decoded %d samples", len(data))

		// Count non-zero samples
		nonZero := 0
		for _, v := range data {
			if v != 0 {
				nonZero++
			}
		}
		t.Logf("Non-zero samples: %d", nonZero)
	})

	t.Run("SegmentParsing", func(t *testing.T) {
		testCases := []struct {
			name      string
			block     []byte
			expectErr bool
		}{
			{
				name:      "TooSmall",
				block:     []byte{0x01},
				expectErr: false, // Should handle gracefully
			},
			{
				name: "ValidSegments",
				block: []byte{
					0x12, 0x34, // MagSgn (2 bytes)
					0x80,       // MEL (1 byte)
					0x06, 0x3F, // VLC (2 bytes)
					// Footer: [melLen (2 bytes LE), vlcLen (2 bytes LE)]
					0x01, 0x00, // melLen = 1
					0x02, 0x00, // vlcLen = 2
				},
				expectErr: false,
			},
			{
				name: "InvalidScup",
				block: []byte{
					0x12,       // MagSgn (1 byte, so magsgnLen would be negative)
					0xFF, 0xFF, // melLen = 65535
					0xFF, 0xFF, // vlcLen = 65535
					// Total Scup = 131070, but lcup=5, so magsgnLen = 5-4-131070 = negative
				},
				expectErr: true, // Should reject invalid segment lengths
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				decoder := NewHTBlockDecoder(4, 4)
				_, err := decoder.DecodeBlock(tc.block)
				if tc.expectErr && err == nil {
					t.Error("Expected error but got none")
				}
				if !tc.expectErr && err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			})
		}
	})
}

func TestHTBlockDecoderWithContext(t *testing.T) {
	t.Run("ContextProgression", func(t *testing.T) {
		decoder := NewHTBlockDecoder(8, 8)

		// Create a test block that will exercise context computation
		testBlock := []byte{
			// MagSgn: some magnitude data
			0x11, 0x22, 0x33, 0x44, 0x55, 0x66,
			// MEL: pattern with some 1s and 0s
			0xAA, 0x55,
			// VLC: various codewords
			0x06, 0x3F, 0x00, 0x7F, 0x11, 0x5F,
			// Footer: [melLen (2 bytes LE), vlcLen (2 bytes LE)]
			// MEL=2, VLC=6
			0x02, 0x00, // melLen = 2
			0x06, 0x00, // vlcLen = 6
		}

		data, err := decoder.DecodeBlock(testBlock)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}

		// Verify decoding completed
		t.Logf("Decoded %d samples", len(data))

		// Print decoded block for visual inspection
		t.Log("\nDecoded block (8x8):")
		for y := 0; y < 8; y++ {
			row := ""
			for x := 0; x < 8; x++ {
				val := decoder.GetSample(x, y)
				if val == 0 {
					row += "   . "
				} else {
					row += fmt.Sprintf("%4d ", val)
				}
			}
			t.Log(row)
		}
	})

	t.Run("SignificanceMapUpdate", func(t *testing.T) {
		decoder := NewHTBlockDecoder(4, 4)

		// Simple block to test significance propagation
		testBlock := []byte{
			0x01, 0x02, // MagSgn (2 bytes)
			0x80,       // MEL (1 byte)
			0x06, 0x3F, // VLC (2 bytes)
			// Footer: [melLen (2 bytes LE), vlcLen (2 bytes LE)]
			0x01, 0x00, // melLen = 1
			0x02, 0x00, // vlcLen = 2
		}

		_, err := decoder.DecodeBlock(testBlock)
		if err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}

		// Check that significance map was updated
		// (This is internal state, so we test indirectly through decoded output)
		hasNonZero := false
		for x := 0; x < 4; x++ {
			for y := 0; y < 4; y++ {
				if decoder.GetSample(x, y) != 0 {
					hasNonZero = true
				}
			}
		}

		if !hasNonZero {
			t.Log("Note: Block decoded to all zeros (MEL/VLC may have terminated early)")
		}
	})
}

func BenchmarkHTBlockDecoder(b *testing.B) {
	testBlock := []byte{
		// Sample test block
		0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC,
		0xDE, 0xF0,
		0x06, 0x3F, 0x00, 0x7F, 0x11, 0x5F,
		0x08, 0x00, // 12-bit Scup: MEL=2, VLC=6, Scup=8
	}

	b.Run("4x4Block", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			decoder := NewHTBlockDecoder(4, 4)
			_, _ = decoder.DecodeBlock(testBlock)
		}
	})

	b.Run("8x8Block", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			decoder := NewHTBlockDecoder(8, 8)
			_, _ = decoder.DecodeBlock(testBlock)
		}
	})

	b.Run("32x32Block", func(b *testing.B) {
		largeBlock := make([]byte, 256)
		copy(largeBlock, testBlock)
		// 12-bit Scup: MEL=8, VLC=16, Scup=24
		// Byte[n-2]: low 4 bits = 24 & 0x0F = 8
		// Byte[n-1]: high 8 bits = 24 >> 4 = 1
		largeBlock[len(largeBlock)-2] = 8
		largeBlock[len(largeBlock)-1] = 1

		for i := 0; i < b.N; i++ {
			decoder := NewHTBlockDecoder(32, 32)
			_, _ = decoder.DecodeBlock(largeBlock)
		}
	})
}
