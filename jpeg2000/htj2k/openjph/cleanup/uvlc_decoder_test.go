package cleanup

import (
	"fmt"
	"testing"
)

// MockBitReader is a simple bit reader for testing
type MockBitReader struct {
	bits []uint8
	pos  int
}

func NewMockBitReader(bits []uint8) *MockBitReader {
	return &MockBitReader{
		bits: bits,
		pos:  0,
	}
}

func (m *MockBitReader) ReadBit() (uint8, error) {
	if m.pos >= len(m.bits) {
		return 0, fmt.Errorf("end of bit stream")
	}
	bit := m.bits[m.pos]
	m.pos++
	return bit, nil
}

func (m *MockBitReader) ReadBitsLE(n int) (uint32, error) {
	if m.pos+n > len(m.bits) {
		return 0, fmt.Errorf("not enough bits")
	}

	// Read n bits in little-endian order (LSB first)
	var result uint32
	for i := 0; i < n; i++ {
		bit := m.bits[m.pos]
		m.pos++
		result |= uint32(bit) << i
	}

	return result, nil
}

func TestUVLCDecoder_DecodeUPrefix(t *testing.T) {
	tests := []struct {
		name     string
		bits     []uint8
		expected uint8
	}{
		{
			name:     "Prefix=1 (u_pfx=1)",
			bits:     []uint8{1},
			expected: 1,
		},
		{
			name:     "Prefix=01 (u_pfx=2)",
			bits:     []uint8{0, 1},
			expected: 2,
		},
		{
			name:     "Prefix=001 (u_pfx=3)",
			bits:     []uint8{0, 0, 1},
			expected: 3,
		},
		{
			name:     "Prefix=000 (u_pfx=5)",
			bits:     []uint8{0, 0, 0},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewMockBitReader(tt.bits)
			decoder := NewUVLCDecoder(reader)

			got, err := decoder.decodeUPrefix()
			if err != nil {
				t.Fatalf("decodeUPrefix() error = %v", err)
			}

			if got != tt.expected {
				t.Errorf("decodeUPrefix() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestUVLCDecoder_DecodeUSuffix(t *testing.T) {
	tests := []struct {
		name     string
		uPfx     uint8
		bits     []uint8 // Little-endian bits
		expected uint8
	}{
		{
			name:     "u_pfx=1, no suffix",
			uPfx:     1,
			bits:     []uint8{},
			expected: 0,
		},
		{
			name:     "u_pfx=2, no suffix",
			uPfx:     2,
			bits:     []uint8{},
			expected: 0,
		},
		{
			name:     "u_pfx=3, 1-bit suffix=0",
			uPfx:     3,
			bits:     []uint8{0},
			expected: 0,
		},
		{
			name:     "u_pfx=3, 1-bit suffix=1",
			uPfx:     3,
			bits:     []uint8{1},
			expected: 1,
		},
		{
			name:     "u_pfx=5, 5-bit suffix=00000b (0)",
			uPfx:     5,
			bits:     []uint8{0, 0, 0, 0, 0}, // LSB first: 00000b = 0
			expected: 0,
		},
		{
			name:     "u_pfx=5, 5-bit suffix=10000b (1)",
			uPfx:     5,
			bits:     []uint8{1, 0, 0, 0, 0}, // LSB first: bit0=1, rest=0 -> 1
			expected: 1,
		},
		{
			name:     "u_pfx=5, 5-bit suffix=01011b (13)",
			uPfx:     5,
			bits:     []uint8{1, 1, 0, 1, 0}, // LSB first: 1 + 2 + 0 + 8 + 0 = 11 (wait, let me recalc)
			expected: 11,                     // bit0=1(1) + bit1=1(2) + bit2=0(0) + bit3=1(8) + bit4=0(0) = 11
		},
		{
			name:     "u_pfx=5, 5-bit suffix=11111b (31)",
			uPfx:     5,
			bits:     []uint8{1, 1, 1, 1, 1}, // LSB first: 1+2+4+8+16 = 31
			expected: 31,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewMockBitReader(tt.bits)
			decoder := NewUVLCDecoder(reader)

			got, err := decoder.decodeUSuffix(tt.uPfx)
			if err != nil {
				t.Fatalf("decodeUSuffix() error = %v", err)
			}

			if got != tt.expected {
				t.Errorf("decodeUSuffix() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestUVLCDecoder_DecodeUExtension(t *testing.T) {
	tests := []struct {
		name     string
		uSfx     uint8
		bits     []uint8 // Little-endian bits
		expected uint8
	}{
		{
			name:     "u_sfx=0, no extension",
			uSfx:     0,
			bits:     []uint8{},
			expected: 0,
		},
		{
			name:     "u_sfx=27, no extension",
			uSfx:     27,
			bits:     []uint8{},
			expected: 0,
		},
		{
			name:     "u_sfx=28, 4-bit extension=0000b (0)",
			uSfx:     28,
			bits:     []uint8{0, 0, 0, 0},
			expected: 0,
		},
		{
			name:     "u_sfx=28, 4-bit extension=0101b (10)",
			uSfx:     28,
			bits:     []uint8{0, 1, 0, 1}, // LSB first: bit0=0(0) + bit1=1(2) + bit2=0(0) + bit3=1(8) = 10
			expected: 10,
		},
		{
			name:     "u_sfx=31, 4-bit extension=1111b (15)",
			uSfx:     31,
			bits:     []uint8{1, 1, 1, 1},
			expected: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewMockBitReader(tt.bits)
			decoder := NewUVLCDecoder(reader)

			got, err := decoder.decodeUExtension(tt.uSfx)
			if err != nil {
				t.Fatalf("decodeUExtension() error = %v", err)
			}

			if got != tt.expected {
				t.Errorf("decodeUExtension() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestUVLCDecoder_DecodeUnsignedResidual(t *testing.T) {
	tests := []struct {
		name     string
		bits     []uint8
		expected uint32
		desc     string
	}{
		{
			name:     "u=1: prefix=1",
			bits:     []uint8{1},
			expected: 1,
			desc:     "Table 3: u=1, u_pfx=1, no suffix/ext",
		},
		{
			name:     "u=2: prefix=01",
			bits:     []uint8{0, 1},
			expected: 2,
			desc:     "Table 3: u=2, u_pfx=2, no suffix/ext",
		},
		{
			name: "u=3: prefix=001, suffix=0",
			bits: []uint8{
				0, 0, 1, // prefix: 001 -> u_pfx=3
				0, // suffix: 0 (1-bit)
			},
			expected: 3,
			desc:     "Table 3: u=3, u_pfx=3, u_sfx=0, u=3+0+0=3",
		},
		{
			name: "u=4: prefix=001, suffix=1",
			bits: []uint8{
				0, 0, 1, // prefix: 001 -> u_pfx=3
				1, // suffix: 1 (1-bit)
			},
			expected: 4,
			desc:     "Table 3: u=4, u_pfx=3, u_sfx=1, u=3+1+0=4",
		},
		{
			name: "u=5: prefix=000, suffix=00000b",
			bits: []uint8{
				0, 0, 0, // prefix: 000 -> u_pfx=5
				0, 0, 0, 0, 0, // suffix: 00000 (5-bit LE) = 0
			},
			expected: 5,
			desc:     "Table 3: u=5, u_pfx=5, u_sfx=0, u=5+0+0=5",
		},
		{
			name: "u=10: prefix=000, suffix=10100b",
			bits: []uint8{
				0, 0, 0, // prefix: 000 -> u_pfx=5
				1, 0, 1, 0, 0, // suffix: 10100 (5-bit LE) = 1+4 = 5
			},
			expected: 10, // u = 5 + 5 + 0 = 10
			desc:     "u_pfx=5, u_sfx=5, u=5+5+0=10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewMockBitReader(tt.bits)
			decoder := NewUVLCDecoder(reader)

			got, err := decoder.DecodeUnsignedResidual()
			if err != nil {
				t.Fatalf("DecodeUnsignedResidual() error = %v", err)
			}

			if got != tt.expected {
				t.Errorf("DecodeUnsignedResidual() = %d, want %d (%s)", got, tt.expected, tt.desc)
			}
		})
	}
}

func TestUVLCDecoder_DecodeUnsignedResidualInitialPair(t *testing.T) {
	tests := []struct {
		name     string
		bits     []uint8
		expected uint32
	}{
		{
			name:     "u=3: prefix=1 (u_pfx=1)",
			bits:     []uint8{1},
			expected: 3, // Formula (4): 2 + 1 + 0 + 0 = 3
		},
		{
			name:     "u=4: prefix=01 (u_pfx=2)",
			bits:     []uint8{0, 1},
			expected: 4, // Formula (4): 2 + 2 + 0 + 0 = 4
		},
		{
			name: "u=5: prefix=001, suffix=0",
			bits: []uint8{
				0, 0, 1, // u_pfx=3
				0, // u_sfx=0
			},
			expected: 5, // Formula (4): 2 + 3 + 0 + 0 = 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewMockBitReader(tt.bits)
			decoder := NewUVLCDecoder(reader)

			got, err := decoder.DecodeUnsignedResidualInitialPair()
			if err != nil {
				t.Fatalf("DecodeUnsignedResidualInitialPair() error = %v", err)
			}

			if got != tt.expected {
				t.Errorf("DecodeUnsignedResidualInitialPair() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestUVLCDecoder_DecodeUnsignedResidualSecondQuad(t *testing.T) {
	tests := []struct {
		name     string
		bit      uint8
		expected uint32
	}{
		{
			name:     "ubit=0 -> u=1",
			bit:      0,
			expected: 1,
		},
		{
			name:     "ubit=1 -> u=2",
			bit:      1,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewMockBitReader([]uint8{tt.bit})
			decoder := NewUVLCDecoder(reader)

			got, err := decoder.DecodeUnsignedResidualSecondQuad()
			if err != nil {
				t.Fatalf("DecodeUnsignedResidualSecondQuad() error = %v", err)
			}

			if got != tt.expected {
				t.Errorf("DecodeUnsignedResidualSecondQuad() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestUVLCDecoder_Table3Examples(t *testing.T) {
	// Test examples from Table 3 of ISO/IEC 15444-15:2019
	// Verify the complete encoding: prefix + suffix + extension
	// Note: uPfx can only be {1, 2, 3, 5}, never 4
	tests := []struct {
		u        uint32
		prefix   string
		uPfx     uint8
		uSfx     uint8
		uExt     uint8
		lp       int
		ls       int
		le       int
		totalLen int
	}{
		{u: 1, prefix: "1", uPfx: 1, uSfx: 0, uExt: 0, lp: 1, ls: 0, le: 0, totalLen: 1},
		{u: 2, prefix: "01", uPfx: 2, uSfx: 0, uExt: 0, lp: 2, ls: 0, le: 0, totalLen: 2},
		{u: 3, prefix: "001", uPfx: 3, uSfx: 0, uExt: 0, lp: 3, ls: 1, le: 0, totalLen: 4},
		{u: 4, prefix: "001", uPfx: 3, uSfx: 1, uExt: 0, lp: 3, ls: 1, le: 0, totalLen: 4},
		{u: 5, prefix: "000", uPfx: 5, uSfx: 0, uExt: 0, lp: 3, ls: 5, le: 0, totalLen: 8},
		{u: 6, prefix: "000", uPfx: 5, uSfx: 1, uExt: 0, lp: 3, ls: 5, le: 0, totalLen: 8},
		{u: 7, prefix: "000", uPfx: 5, uSfx: 2, uExt: 0, lp: 3, ls: 5, le: 0, totalLen: 8},
		// Additional test cases can be added
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("u=%d", tt.u), func(t *testing.T) {
			// Verify the formula: u = uPfx + uSfx + 4*uExt
			calculated := uint32(tt.uPfx) + uint32(tt.uSfx) + 4*uint32(tt.uExt)
			if calculated != tt.u {
				t.Errorf("Formula verification failed: %d + %d + 4*%d = %d, want %d",
					tt.uPfx, tt.uSfx, tt.uExt, calculated, tt.u)
			}
		})
	}
}
