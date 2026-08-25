package t2

import (
	"bytes"
	"testing"
)

// TestPacketHeaderParserEmpty tests parsing empty packets
func TestPacketHeaderParserEmpty(t *testing.T) {
	// Create a parser for 2x2 code-blocks
	data := []byte{0x00} // Empty packet (bit 0 = 0)
	parser := NewPacketHeaderParser(data, 2, 2)

	packet, err := parser.ParseHeader()
	if err != nil {
		t.Fatalf("ParseHeader failed: %v", err)
	}

	if packet.HeaderPresent {
		t.Error("Expected empty packet, but HeaderPresent=true")
	}

	if len(packet.CodeBlockIncls) != 0 {
		t.Errorf("Expected 0 code-block inclusions, got %d", len(packet.CodeBlockIncls))
	}
}

// TestPacketHeaderParserDecodeNumPasses tests the variable-length encoding for number of passes
func TestPacketHeaderParserDecodeNumPasses(t *testing.T) {
	tests := []struct {
		name     string
		bits     []byte
		expected int
	}{
		{
			name:     "1 pass (0)",
			bits:     []byte{0x00}, // Bit 0 = 0
			expected: 1,
		},
		{
			name:     "2 passes (10)",
			bits:     []byte{0x80}, // Bits 10 = 0x80 (10000000)
			expected: 2,
		},
		{
			name:     "3 passes (110 + 00)",
			bits:     []byte{0xC0}, // Bits 11000 = 0xC0 (11000000) → 110 + 00 = 3+0 = 3
			expected: 3,
		},
		{
			name:     "5 passes (1110)",
			bits:     []byte{0xE0}, // Bits 1110 = 0xE0 (11100000) → 11+10 (val=2) = 3+2 = 5
			expected: 5,
		},
		{
			name:     "6 passes (1111 + 00000)",
			bits:     []byte{0xF0, 0x00}, // Bits 11110000 0 → 1111+00000 (9 bits total) = 6+0 = 6
			expected: 6,
		},
		{
			name:     "10 passes (1111 + 00100)",
			bits:     []byte{0xF2, 0x00}, // Bits 11110010 0 → 1111+00100 (9 bits total) = 6+4 = 10
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewPacketHeaderParser(tt.bits, 1, 1)

			numPasses, err := parser.decodeNumPasses()
			if err != nil {
				t.Fatalf("decodeNumPasses failed: %v", err)
			}

			if numPasses != tt.expected {
				t.Errorf("Expected %d passes, got %d", tt.expected, numPasses)
			}
		})
	}
}

// TestPacketHeaderParserDecodeDataLength tests data length decoding
func TestPacketHeaderParserDecodeDataLength(t *testing.T) {
	tests := []struct {
		name     string
		expected int
	}{
		{
			name:     "Length 10",
			expected: 10,
		},
		{
			name:     "Length 100",
			expected: 100,
		},
		{
			name:     "Length 254",
			expected: 254,
		},
		{
			name:     "Length 255",
			expected: 255,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bw := newBioWriter()
			numlenbits := 3
			increment := (floorLog2(tt.expected) + 1) - (numlenbits + floorLog2(1))
			if increment < 0 {
				increment = 0
			}
			encodeCommaCode(bw, increment)
			numlenbits += increment
			bw.writeBits(tt.expected, numlenbits)
			data := bw.flush()

			parser := NewPacketHeaderParser(data, 1, 1)
			cbState := parser.codeBlockStates[0]
			cbState.NumLenBits = 3

			length, _, err := parser.decodeDataLength(1, cbState)
			if err != nil {
				t.Fatalf("decodeDataLength failed: %v", err)
			}

			if length != tt.expected {
				t.Errorf("Expected length %d, got %d", tt.expected, length)
			}
		})
	}
}

// TestPacketHeaderParserReadBit tests bit reading
func TestPacketHeaderParserReadBit(t *testing.T) {
	// Test data: 0b10110011 = 0xB3
	data := []byte{0xB3}
	parser := NewPacketHeaderParser(data, 1, 1)

	expected := []int{1, 0, 1, 1, 0, 0, 1, 1}

	for i, expectedBit := range expected {
		bit, err := parser.readBit()
		if err != nil {
			t.Fatalf("readBit failed at position %d: %v", i, err)
		}

		if bit != expectedBit {
			t.Errorf("Bit %d: expected %d, got %d", i, expectedBit, bit)
		}
	}

	// Reading beyond data should return EOF
	_, err := parser.readBit()
	if err == nil {
		t.Error("Expected EOF error when reading beyond data")
	}
}

// TestPacketHeaderParserReadBits tests multi-bit reading
func TestPacketHeaderParserReadBits(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		nbits    int
		expected int
	}{
		{
			name:     "Read 4 bits from 0xF0",
			data:     []byte{0xF0},
			nbits:    4,
			expected: 0xF, // 1111
		},
		{
			name:     "Read 8 bits from 0xAB",
			data:     []byte{0xAB},
			nbits:    8,
			expected: 0xAB,
		},
		{
			name:     "Read 3 bits from 0xE0",
			data:     []byte{0xE0},
			nbits:    3,
			expected: 0x7, // 111
		},
		{
			name:     "Read 5 bits from 0xA8",
			data:     []byte{0xA8},
			nbits:    5,
			expected: 0x15, // 10101
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewPacketHeaderParser(tt.data, 1, 1)

			value, err := parser.readBits(tt.nbits)
			if err != nil {
				t.Fatalf("readBits failed: %v", err)
			}

			if value != tt.expected {
				t.Errorf("Expected value 0x%X, got 0x%X", tt.expected, value)
			}
		})
	}
}

// TestPacketHeaderParserAlignToByte tests byte alignment
func TestPacketHeaderParserAlignToByte(t *testing.T) {
	data := []byte{0xAB, 0xCD}
	parser := NewPacketHeaderParser(data, 1, 1)

	// Read 3 bits
	_, _ = parser.readBits(3)

	// Align to byte
	if err := parser.alignToByte(); err != nil {
		t.Fatalf("alignToByte failed: %v", err)
	}

	if parser.Position() != 1 {
		t.Errorf("Expected pos=1 after alignment, got %d", parser.Position())
	}
}

// TestPacketHeaderParserSimplePacket tests parsing a simple packet with one included code-block
func TestPacketHeaderParserSimplePacket(t *testing.T) {
	// Create test data for a simple packet:
	// - Non-empty packet (bit 1)
	// - 1 code-block included
	// - Inclusion tag tree value 0 (included in layer 0)
	// - Zero bit-planes = 0
	// - Num passes = 1
	// - Data length = 10

	buf := &bytes.Buffer{}

	// Bit 0: packet not empty (1)
	// Bits 1+: inclusion tag tree for CB[0,0]
	//   - Inclusion value 0 → bit sequence: 1
	// Zero bit-planes for CB[0,0]
	//   - Value 0 → bit sequence: 1
	// Num passes: 1 pass → bit sequence: 0
	// Data length: 10 bytes → 8 bits: 00001010

	// Combined bit sequence:
	// 1 (not empty) + 1 (incl=0) + 1 (zbp=0) + 0 (1 pass) + 00001010 (length=10)
	// = 1110 00001010
	// Padded to bytes: 11100000 1010xxxx → 0xE0, 0xA0

	buf.WriteByte(0xEA)
	buf.WriteByte(0x80)

	parser := NewPacketHeaderParser(buf.Bytes(), 1, 1) // 1x1 grid (single code-block)

	packet, err := parser.ParseHeader()
	if err != nil {
		t.Fatalf("ParseHeader failed: %v", err)
	}

	if !packet.HeaderPresent {
		t.Error("Expected non-empty packet")
	}

	if len(packet.CodeBlockIncls) != 1 {
		t.Fatalf("Expected 1 code-block inclusion, got %d", len(packet.CodeBlockIncls))
	}

	cbIncl := packet.CodeBlockIncls[0]
	if !cbIncl.Included {
		t.Error("Expected code-block to be included")
	}

	if !cbIncl.FirstInclusion {
		t.Error("Expected first inclusion")
	}

	if cbIncl.NumPasses != 1 {
		t.Errorf("Expected 1 pass, got %d", cbIncl.NumPasses)
	}

	if cbIncl.DataLength != 10 {
		t.Errorf("Expected data length 10, got %d", cbIncl.DataLength)
	}
}

// TestPacketHeaderParserMultipleLayers tests parsing across multiple layers
func TestPacketHeaderParserMultipleLayers(t *testing.T) {
	// Layer 0: Include CB[0,0]
	// Layer 1: Include CB[0,0] again (not first inclusion)

	// Layer 0 packet
	buf0 := &bytes.Buffer{}
	// 1 (not empty) + 1 (incl=0) + 1 (zbp=0) + 0 (1 pass) + 00001010 (len=10)
	// = 1110 00001010 → 0xE0, 0xA0
	buf0.WriteByte(0xEA)
	buf0.WriteByte(0x80)

	parser := NewPacketHeaderParser(buf0.Bytes(), 1, 1)
	parser.SetLayer(0)

	packet0, err := parser.ParseHeader()
	if err != nil {
		t.Fatalf("ParseHeader layer 0 failed: %v", err)
	}

	if len(packet0.CodeBlockIncls) != 1 {
		t.Fatalf("Layer 0: expected 1 CB, got %d", len(packet0.CodeBlockIncls))
	}

	if !packet0.CodeBlockIncls[0].FirstInclusion {
		t.Error("Layer 0: expected first inclusion")
	}

	// Layer 1 packet - CB already included, so no ZBP decoding
	buf1 := &bytes.Buffer{}
	// 1 (not empty) + 1 (incl still ≤1) + 0 (1 pass) + 00001010 (len=10)
	// = 110 00001010 → 0xC0, 0xA0
	buf1.WriteByte(0xD5)
	buf1.WriteByte(0x00)

	// Create new parser with same state (simulating multi-layer decode)
	// In real implementation, parser state would persist
	parser2 := NewPacketHeaderParser(buf1.Bytes(), 1, 1)
	parser2.SetLayer(1)
	// Mark CB[0,0] as already included
	parser2.codeBlockStates[0].Included = true
	parser2.codeBlockStates[0].FirstLayer = 0
	parser2.codeBlockStates[0].ZeroBitPlanes = 0
	parser2.codeBlockStates[0].NumLenBits = 4

	packet1, err := parser2.ParseHeader()
	if err != nil {
		t.Fatalf("ParseHeader layer 1 failed: %v", err)
	}

	if len(packet1.CodeBlockIncls) != 1 {
		t.Fatalf("Layer 1: expected 1 CB, got %d", len(packet1.CodeBlockIncls))
	}

	if packet1.CodeBlockIncls[0].FirstInclusion {
		t.Error("Layer 1: should not be first inclusion")
	}
}

// TestPacketHeaderParserReset tests parser reset functionality
func TestPacketHeaderParserReset(t *testing.T) {
	data := []byte{0xE0, 0xA0}
	parser := NewPacketHeaderParser(data, 1, 1)

	// Parse first time
	packet1, err := parser.ParseHeader()
	if err != nil {
		t.Fatalf("First parse failed: %v", err)
	}

	if len(packet1.CodeBlockIncls) != 1 {
		t.Error("Expected 1 CB inclusion")
	}

	// Reset
	parser.Reset()

	// Verify reset state
	if parser.Position() != 0 {
		t.Errorf("After reset, expected pos=0, got %d", parser.Position())
	}

	if parser.currentLayer != 0 {
		t.Errorf("After reset, expected currentLayer=0, got %d", parser.currentLayer)
	}

	// Verify CB states are reset
	for i, cbState := range parser.codeBlockStates {
		if cbState.Included {
			t.Errorf("CB[%d] should not be included after reset", i)
		}
		if cbState.FirstLayer != -1 {
			t.Errorf("CB[%d] FirstLayer should be -1, got %d", i, cbState.FirstLayer)
		}
	}
}

// TestPacketHeaderParserPosition tests position tracking
func TestPacketHeaderParserPosition(t *testing.T) {
	data := []byte{0x00, 0x11, 0x22, 0x33}
	parser := NewPacketHeaderParser(data, 1, 1)

	if parser.Position() != 0 {
		t.Errorf("Initial position should be 0, got %d", parser.Position())
	}

	// Read some bits
	_, _ = parser.readBits(16) // Read 2 bytes

	if parser.Position() != 2 {
		t.Errorf("After reading 16 bits, position should be 2, got %d", parser.Position())
	}
}
