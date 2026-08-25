package t2

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestPacketParserCreation tests creating a packet parser
func TestPacketParserCreation(t *testing.T) {
	tests := []struct {
		name             string
		data             []byte
		numCodeBlocksX   int
		numCodeBlocksY   int
	}{
		{"Empty data", []byte{}, 1, 1},
		{"Small data", []byte{0xFF, 0x00, 0x12}, 2, 2},
		{"Large grid", make([]byte, 100), 8, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewPacketParser(tt.data, tt.numCodeBlocksX, tt.numCodeBlocksY)

			if parser == nil {
				t.Fatal("NewPacketParser returned nil")
			}

			if parser.pos != 0 {
				t.Errorf("Initial pos = %d, want 0", parser.pos)
			}

			if parser.bitPos != 0 {
				t.Errorf("Initial bitPos = %d, want 0", parser.bitPos)
			}

			if parser.inclTagTree == nil {
				t.Error("inclTagTree is nil")
			}

			if parser.zbpTagTree == nil {
				t.Error("zbpTagTree is nil")
			}
		})
	}
}

// TestPacketParserReadBit tests bit reading
func TestPacketParserReadBit(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected []int // Expected bits to read
	}{
		{
			"Single byte 0xFF",
			[]byte{0xFF},
			[]int{1, 1, 1, 1, 1, 1, 1, 1},
		},
		{
			"Single byte 0x00",
			[]byte{0x00},
			[]int{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			"Pattern 0xAA (10101010)",
			[]byte{0xAA},
			[]int{1, 0, 1, 0, 1, 0, 1, 0},
		},
		{
			"Pattern 0x55 (01010101)",
			[]byte{0x55},
			[]int{0, 1, 0, 1, 0, 1, 0, 1},
		},
		{
			"Two bytes",
			[]byte{0xF0, 0x0F},
			[]int{1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewPacketParser(tt.data, 1, 1)

			for i, expectedBit := range tt.expected {
				bit, err := parser.readBit()
				if err != nil {
					t.Fatalf("readBit() at position %d failed: %v", i, err)
				}

				if bit != expectedBit {
					t.Errorf("Bit %d = %d, want %d", i, bit, expectedBit)
				}
			}

			// Reading past end should return EOF
			_, err := parser.readBit()
			if err == nil {
				t.Error("Expected EOF when reading past end")
			}
		})
	}
}

// TestPacketParserReadBitsActive tests multi-bit reading
func TestPacketParserReadBitsActive(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		numBits  int
		expected int
	}{
		{"4 bits from 0xF0", []byte{0xF0}, 4, 0x0F}, // 1111
		{"8 bits from 0xAA", []byte{0xAA}, 8, 0xAA},
		{"3 bits from 0xE0", []byte{0xE0}, 3, 0x07}, // 111
		{"16 bits", []byte{0x12, 0x34}, 16, 0x1234},
		{"12 bits", []byte{0xAB, 0xC0}, 12, 0xABC}, // 101010111100
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewPacketParser(tt.data, 1, 1)

			result, err := parser.readBitsActive(tt.numBits)
			if err != nil {
				t.Fatalf("readBitsActive(%d) failed: %v", tt.numBits, err)
			}

			if result != tt.expected {
				t.Errorf("readBitsActive(%d) = 0x%X, want 0x%X", tt.numBits, result, tt.expected)
			}
		})
	}
}

// TestPacketParserReadBitsActiveErrors tests error cases
func TestPacketParserReadBitsActiveErrors(t *testing.T) {
	parser := NewPacketParser([]byte{0xFF}, 1, 1)

	tests := []struct {
		name    string
		numBits int
	}{
		{"Zero bits", 0},
		{"Negative bits", -1},
		{"Too many bits", 33},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.readBitsActive(tt.numBits)
			if err == nil {
				t.Errorf("readBitsActive(%d) should return error", tt.numBits)
			}
		})
	}
}

// TestPacketParserParseEmptyPacket tests parsing empty packet
func TestPacketParserParseEmptyPacket(t *testing.T) {
	// Packet with presence bit = 0 (empty packet)
	data := []byte{0x00}
	parser := NewPacketParser(data, 2, 2)

	packet, err := parser.ParsePacket()
	if err != nil {
		t.Fatalf("ParsePacket() failed: %v", err)
	}

	if packet == nil {
		t.Fatal("ParsePacket() returned nil packet")
	}

	if packet.HeaderPresent {
		t.Error("Empty packet should have HeaderPresent = false")
	}
}

// TestPacketParserParsePresentPacket tests parsing present packet
func TestPacketParserParsePresentPacket(t *testing.T) {
	// Packet with presence bit = 1
	data := []byte{0x80, 0x00, 0x00, 0x00} // 1 followed by zeros
	parser := NewPacketParser(data, 2, 2)

	packet, err := parser.ParsePacket()
	if err != nil {
		t.Fatalf("ParsePacket() failed: %v", err)
	}

	if packet == nil {
		t.Fatal("ParsePacket() returned nil packet")
	}

	if !packet.HeaderPresent {
		t.Error("Packet should have HeaderPresent = true")
	}
}

// TestPacketParserReset tests parser reset
func TestPacketParserReset(t *testing.T) {
	data := []byte{0xFF, 0xAA, 0x55}
	parser := NewPacketParser(data, 2, 2)

	// Read some bits
	_, _ = parser.readBit()
	_, _ = parser.readBit()
	_, _ = parser.readBit()

	if parser.pos == 0 && parser.bitPos == 0 {
		t.Error("Parser position should have advanced")
	}

	// Reset
	parser.Reset()

	if parser.pos != 0 {
		t.Errorf("After Reset() pos = %d, want 0", parser.pos)
	}

	if parser.bitPos != 0 {
		t.Errorf("After Reset() bitPos = %d, want 0", parser.bitPos)
	}

	if parser.currentPacket != nil {
		t.Error("After Reset() currentPacket should be nil")
	}
}

// TestPacketParserPosition tests position tracking
func TestPacketParserPosition(t *testing.T) {
	data := make([]byte, 100)
	parser := NewPacketParser(data, 1, 1)

	if parser.Position() != 0 {
		t.Errorf("Initial Position() = %d, want 0", parser.Position())
	}

	if parser.Remaining() != 100 {
		t.Errorf("Initial Remaining() = %d, want 100", parser.Remaining())
	}

	// Read 8 bits (1 byte)
	for i := 0; i < 8; i++ {
		_, _ = parser.readBit()
	}

	if parser.Position() != 1 {
		t.Errorf("After 8 bits Position() = %d, want 1", parser.Position())
	}

	if parser.Remaining() != 99 {
		t.Errorf("After 8 bits Remaining() = %d, want 99", parser.Remaining())
	}
}

// TestPacketParserEOF tests EOF handling
func TestPacketParserEOF(t *testing.T) {
	data := []byte{0xFF}
	parser := NewPacketParser(data, 1, 1)

	// Read all 8 bits
	for i := 0; i < 8; i++ {
		_, err := parser.readBit()
		if err != nil {
			t.Fatalf("readBit() %d failed: %v", i, err)
		}
	}

	// Next read should fail
	_, err := parser.readBit()
	if err == nil {
		t.Error("Expected EOF error when reading past end")
	}
}

// TestPacketParserComplexBitPattern tests complex bit patterns
func TestPacketParserComplexBitPattern(t *testing.T) {
	// Create a packet with known bit pattern
	buf := bytes.Buffer{}
	buf.WriteByte(0xA5) // 10100101
	buf.WriteByte(0x5A) // 01011010
	buf.WriteByte(0xFF) // 11111111

	parser := NewPacketParser(buf.Bytes(), 1, 1)

	// Read first 4 bits: 1010 = 10
	val1, err := parser.readBitsActive(4)
	if err != nil {
		t.Fatalf("readBitsActive(4) failed: %v", err)
	}
	if val1 != 0x0A {
		t.Errorf("First 4 bits = 0x%X, want 0x0A", val1)
	}

	// Read next 4 bits: 0101 = 5
	val2, err := parser.readBitsActive(4)
	if err != nil {
		t.Fatalf("readBitsActive(4) failed: %v", err)
	}
	if val2 != 0x05 {
		t.Errorf("Next 4 bits = 0x%X, want 0x05", val2)
	}

	// Read next 8 bits: 01011010 = 0x5A
	val3, err := parser.readBitsActive(8)
	if err != nil {
		t.Fatalf("readBitsActive(8) failed: %v", err)
	}
	if val3 != 0x5A {
		t.Errorf("Next 8 bits = 0x%X, want 0x5A", val3)
	}
}

// TestPacketParserTagTreeIntegration tests tag tree integration
func TestPacketParserTagTreeIntegration(t *testing.T) {
	data := make([]byte, 100)
	// Fill with pattern
	for i := range data {
		data[i] = byte(i)
	}

	parser := NewPacketParser(data, 4, 4)

	if parser.inclTagTree == nil {
		t.Error("Inclusion tag tree not initialized")
	}

	if parser.zbpTagTree == nil {
		t.Error("Zero bit-plane tag tree not initialized")
	}

	// Verify tag trees have correct dimensions
	if parser.inclTagTree.Width() != 4 {
		t.Errorf("inclTagTree width = %d, want 4", parser.inclTagTree.Width())
	}

	if parser.inclTagTree.Height() != 4 {
		t.Errorf("inclTagTree height = %d, want 4", parser.inclTagTree.Height())
	}
}

// TestPacketParserReadTagTreeValue tests tag tree value reading
func TestPacketParserReadTagTreeValue(t *testing.T) {
	// Create data with known pattern
	buf := bytes.Buffer{}
	_ = binary.Write(&buf, binary.BigEndian, uint16(0x5678)) // Some data

	parser := NewPacketParser(buf.Bytes(), 2, 2)
	tree := parser.inclTagTree

	// Read a tag tree value
	value, err := parser.readTagTreeValue(tree, 0, 0, 10)
	if err != nil {
		t.Fatalf("readTagTreeValue() failed: %v", err)
	}

	// Value should be between 0 and 15 (4-bit encoding)
	if value < 0 || value > 15 {
		t.Errorf("readTagTreeValue() = %d, should be in range [0, 15]", value)
	}

	// Reading again should return cached value
	value2, err := parser.readTagTreeValue(tree, 0, 0, 10)
	if err != nil {
		t.Fatalf("readTagTreeValue() second call failed: %v", err)
	}

	if value2 != value {
		t.Errorf("Second readTagTreeValue() = %d, want %d (cached)", value2, value)
	}
}

// TestPacketParserAlignToByte tests byte alignment
func TestPacketParserAlignToByte(t *testing.T) {
	data := []byte{0xFF, 0xAA, 0x55}
	parser := NewPacketParser(data, 1, 1)

	// Read 3 bits
	for i := 0; i < 3; i++ {
		_, _ = parser.readBit()
	}

	if parser.bitPos != 3 {
		t.Errorf("bitPos = %d, want 3", parser.bitPos)
	}

	// Align to byte
	parser.alignToByteActive()

	if parser.bitPos != 0 {
		t.Errorf("After align bitPos = %d, want 0", parser.bitPos)
	}

	if parser.pos != 1 {
		t.Errorf("After align pos = %d, want 1", parser.pos)
	}

	// Aligning when already aligned should do nothing
	initialPos := parser.pos
	parser.alignToByteActive()
	if parser.pos != initialPos {
		t.Error("Aligning when already aligned should not change position")
	}
}

// TestPacketParserMultiplePackets tests parsing a single packet with data
func TestPacketParserMultiplePackets(t *testing.T) {
	// Create data for a present packet
	buf := bytes.Buffer{}

	// Packet: present (presence = 1, followed by some data)
	buf.WriteByte(0x80) // 10000000 - first bit = 1 (present)
	buf.Write(make([]byte, 10))

	data := buf.Bytes()
	parser := NewPacketParser(data, 1, 1)

	// Parse packet (present)
	packet, err := parser.ParsePacket()
	if err != nil {
		t.Fatalf("ParsePacket() failed: %v", err)
	}
	if !packet.HeaderPresent {
		t.Error("Packet should be present")
	}

	if len(packet.Body) == 0 {
		t.Error("Packet body should not be empty")
	}
}

// TestPacketParserStressTest tests parser with large data
func TestPacketParserStressTest(t *testing.T) {
	// Create large data
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i & 0xFF)
	}

	parser := NewPacketParser(data, 8, 8)

	// Read many bits
	for i := 0; i < 1000; i++ {
		_, err := parser.readBit()
		if err != nil {
			t.Fatalf("readBit() %d failed: %v", i, err)
		}
	}

	// Verify position advanced correctly
	// 1000 bits = 125 bytes
	expectedPos := 125
	if parser.Position() != expectedPos {
		t.Errorf("After 1000 bits Position() = %d, want %d", parser.Position(), expectedPos)
	}
}
