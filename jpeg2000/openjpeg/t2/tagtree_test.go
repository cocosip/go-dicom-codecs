package t2

import (
	"fmt"
	"testing"
)

// mockBitReader implements BitReader interface for testing
type mockBitReader struct {
	bits []int
	idx  int
	t    *testing.T
}

func (m *mockBitReader) ReadBit() (int, error) {
	if m.idx >= len(m.bits) {
		return 0, fmt.Errorf("ran out of test bits")
	}
	bit := m.bits[m.idx]
	m.idx++
	return bit, nil
}

type bitCollector struct {
	bits []int
}

func (bc *bitCollector) WriteBit(bit int) error {
	bc.bits = append(bc.bits, bit)
	return nil
}

func encodeBits(width, height, leafX, leafY, value, threshold int) ([]int, error) {
	tree := NewTagTree(width, height)
	tree.SetValue(leafX, leafY, value)
	writer := &bitCollector{}
	if err := tree.Encode(writer, leafX, leafY, threshold); err != nil {
		return nil, err
	}
	return writer.bits, nil
}

// TestTagTreeDecoderBasic tests basic tag tree decoding
func TestTagTreeDecoderBasic(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		leafX     int
		leafY     int
		threshold int
		value     int
		expected  int
	}{
		{
			name:      "2x2 tree, decode value 0",
			width:     2,
			height:    2,
			leafX:     0,
			leafY:     0,
			threshold: 5,
			value:     0,
			expected:  0,
		},
		{
			name:      "2x2 tree, decode value 2",
			width:     2,
			height:    2,
			leafX:     1,
			leafY:     0,
			threshold: 5,
			value:     2,
			expected:  2,
		},
		{
			name:      "4x4 tree, decode value 3",
			width:     4,
			height:    4,
			leafX:     2,
			leafY:     1,
			threshold: 10,
			value:     3,
			expected:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bits, err := encodeBits(tt.width, tt.height, tt.leafX, tt.leafY, tt.value, tt.threshold)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			tree := NewTagTree(tt.width, tt.height)
			decoder := NewTagTreeDecoder(tree)

			bitReader := &mockBitReader{
				bits: bits,
				idx:  0,
				t:    t,
			}

			value, err := decoder.Decode(bitReader, tt.leafX, tt.leafY, tt.threshold)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			if value != tt.expected {
				t.Errorf("Expected value %d, got %d", tt.expected, value)
			}
		})
	}
}

// TestTagTreeDecoderInclusion tests code-block inclusion decoding
func TestTagTreeDecoderInclusion(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		height       int
		cbX          int
		cbY          int
		currentLayer int
		value        int
		wantIncluded bool
		wantLayer    int
	}{
		{
			name:         "Included in layer 0",
			width:        2,
			height:       2,
			cbX:          0,
			cbY:          0,
			currentLayer: 0,
			value:        0,
			wantIncluded: true,
			wantLayer:    0,
		},
		{
			name:         "Not included yet (value > currentLayer)",
			width:        2,
			height:       2,
			cbX:          1,
			cbY:          1,
			currentLayer: 0,
			value:        1,
			wantIncluded: false,
			wantLayer:    -1,
		},
		{
			name:         "Included in layer 2",
			width:        4,
			height:       4,
			cbX:          2,
			cbY:          2,
			currentLayer: 3,
			value:        2,
			wantIncluded: true,
			wantLayer:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bits, err := encodeBits(tt.width, tt.height, tt.cbX, tt.cbY, tt.value, tt.currentLayer+1)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			bitIdx := 0
			bitReader := func() (int, error) {
				if bitIdx >= len(bits) {
					t.Fatalf("Ran out of test bits")
				}
				bit := bits[bitIdx]
				bitIdx++
				return bit, nil
			}

			tree := NewTagTree(tt.width, tt.height)
			decoder := NewTagTreeDecoder(tree)

			included, layer, err := decoder.DecodeInclusion(tt.cbX, tt.cbY, tt.currentLayer, bitReader)
			if err != nil {
				t.Fatalf("DecodeInclusion failed: %v", err)
			}

			if included != tt.wantIncluded {
				t.Errorf("Expected included=%v, got %v", tt.wantIncluded, included)
			}
			if layer != tt.wantLayer {
				t.Errorf("Expected layer=%d, got %d", tt.wantLayer, layer)
			}
		})
	}
}

// TestTagTreeDecoderZeroBitPlanes tests zero bit-planes decoding
func TestTagTreeDecoderZeroBitPlanes(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		cbX      int
		cbY      int
		value    int
		expected int
	}{
		{
			name:     "0 zero bit-planes",
			width:    2,
			height:   2,
			cbX:      0,
			cbY:      0,
			value:    0,
			expected: 0,
		},
		{
			name:     "3 zero bit-planes",
			width:    4,
			height:   4,
			cbX:      1,
			cbY:      2,
			value:    3,
			expected: 3,
		},
		{
			name:     "5 zero bit-planes",
			width:    4,
			height:   4,
			cbX:      3,
			cbY:      3,
			value:    5,
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bits, err := encodeBits(tt.width, tt.height, tt.cbX, tt.cbY, tt.value, 32)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			bitIdx := 0
			bitReader := func() (int, error) {
				if bitIdx >= len(bits) {
					t.Fatalf("Ran out of test bits")
				}
				bit := bits[bitIdx]
				bitIdx++
				return bit, nil
			}

			tree := NewTagTree(tt.width, tt.height)
			decoder := NewTagTreeDecoder(tree)

			zbp, err := decoder.DecodeZeroBitPlanes(tt.cbX, tt.cbY, bitReader)
			if err != nil {
				t.Fatalf("DecodeZeroBitPlanes failed: %v", err)
			}

			if zbp != tt.expected {
				t.Errorf("Expected %d zero bit-planes, got %d", tt.expected, zbp)
			}
		})
	}
}

// TestTagTreeDecoderReset tests tag tree state reset
func TestTagTreeDecoderReset(t *testing.T) {
	tree := NewTagTree(2, 2)
	decoder := NewTagTreeDecoder(tree)

	bits1, err := encodeBits(2, 2, 0, 0, 2, 5)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	bitReader1 := &mockBitReader{
		bits: bits1,
		idx:  0,
		t:    t,
	}

	value, err := decoder.Decode(bitReader1, 0, 0, 5)
	if err != nil || value != 2 {
		t.Fatalf("Initial decode failed: value=%d, err=%v", value, err)
	}

	// Reset
	decoder.Reset()

	bits2, err := encodeBits(2, 2, 0, 0, 0, 5)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	bitReader2 := &mockBitReader{
		bits: bits2,
		idx:  0,
		t:    t,
	}
	value, err = decoder.Decode(bitReader2, 0, 0, 5)
	if err != nil || value != 0 {
		t.Errorf("After reset, expected value 0, got %d (err=%v)", value, err)
	}
}

// TestTagTreeDecoderMultipleLeaves tests decoding multiple leaves
func TestTagTreeDecoderMultipleLeaves(t *testing.T) {
	tree := NewTagTree(4, 4)
	decoder := NewTagTreeDecoder(tree)

	// Prepare different bit sequences for different leaves
	leafData := map[int]int{
		0: 0,
		1: 1,
		4: 2,
		5: 3,
	}

	for leafIdx, expectedValue := range leafData {
		decoder.Reset()

		leafX := leafIdx % 4
		leafY := leafIdx / 4

		bits, err := encodeBits(4, 4, leafX, leafY, expectedValue, 10)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		bitReader := &mockBitReader{
			bits: bits,
			idx:  0,
			t:    t,
		}
		value, err := decoder.Decode(bitReader, leafX, leafY, 10)
		if err != nil {
			t.Fatalf("Decode failed for leaf %d: %v", leafIdx, err)
		}

		if value != expectedValue {
			t.Errorf("Leaf %d: expected value %d, got %d", leafIdx, expectedValue, value)
		}
	}
}

// TestTagTreeDecoderBoundaryConditions tests edge cases
func TestTagTreeDecoderBoundaryConditions(t *testing.T) {
	t.Run("Out of bounds coordinates", func(t *testing.T) {
		tree := NewTagTree(2, 2)
		decoder := NewTagTreeDecoder(tree)

		bitReader := &mockBitReader{
			bits: []int{},
			idx:  0,
			t:    t,
		}

		// Test out of bounds - should return error
		_, err := decoder.Decode(bitReader, -1, 0, 5)
		if err == nil {
			t.Error("Out of bounds x=-1 should return error")
		}

		_, err = decoder.Decode(bitReader, 0, -1, 5)
		if err == nil {
			t.Error("Out of bounds y=-1 should return error")
		}

		_, err = decoder.Decode(bitReader, 2, 0, 5)
		if err == nil {
			t.Error("Out of bounds x=2 should return error")
		}

		_, err = decoder.Decode(bitReader, 0, 2, 5)
		if err == nil {
			t.Error("Out of bounds y=2 should return error")
		}
	})

	t.Run("Already decoded value below threshold", func(t *testing.T) {
		tree := NewTagTree(2, 2)
		decoder := NewTagTreeDecoder(tree)

		bits1, err := encodeBits(2, 2, 0, 0, 1, 5)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		bitReader1 := &mockBitReader{
			bits: bits1,
			idx:  0,
			t:    t,
		}

		value, _ := decoder.Decode(bitReader1, 0, 0, 5)
		if value != 1 {
			t.Fatalf("Initial decode should give value 1, got %d", value)
		}

		// Second decode with same threshold - should return same value
		bitReader2 := &mockBitReader{
			bits: bits1,
			idx:  0,
			t:    t,
		}

		value, _ = decoder.Decode(bitReader2, 0, 0, 5)
		if value != 1 {
			t.Errorf("Cached decode should give value 1, got %d", value)
		}
	})
}
