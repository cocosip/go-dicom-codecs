package t2

import (
	"fmt"
	"io"
)

// PacketParser parses JPEG 2000 packets
// Reference: ISO/IEC 15444-1:2019 Annex B.10
type PacketParser struct {
	data   []byte
	pos    int
	bitPos int // Current bit position within byte (0-7)

	// Tag trees for packet header parsing
	inclTagTree  *TagTree // Inclusion tag tree
	zbpTagTree   *TagTree // Zero bit-plane tag tree

	// Current packet state
	currentPacket *Packet
}

// NewPacketParser creates a new packet parser
func NewPacketParser(data []byte, numCodeBlocksX, numCodeBlocksY int) *PacketParser {
	return &PacketParser{
		data:         data,
		pos:          0,
		bitPos:       0,
		inclTagTree:  NewTagTree(numCodeBlocksX, numCodeBlocksY),
		zbpTagTree:   NewTagTree(numCodeBlocksX, numCodeBlocksY),
	}
}

// ParsePacket parses a single packet from the data stream
func (pp *PacketParser) ParsePacket() (*Packet, error) {
	if pp.pos >= len(pp.data) {
		return nil, io.EOF
	}

	packet := &Packet{
		CodeBlockIncls: make([]CodeBlockIncl, 0),
	}
	pp.currentPacket = packet

	// Read packet presence bit
	present, err := pp.readBit()
	if err != nil {
		return nil, fmt.Errorf("failed to read packet presence: %w", err)
	}

	packet.HeaderPresent = present == 1

	if !packet.HeaderPresent {
		// Empty packet - no data
		return packet, nil
	}

	// Parse packet header
	pp.parsePacketHeader(packet)

	// Parse packet body
	pp.parsePacketBody(packet)

	return packet, nil
}

// parsePacketHeader parses the packet header
func (pp *PacketParser) parsePacketHeader(packet *Packet) {
	headerStart := pp.pos

	// For each code-block in the precinct
	// We need to read:
	// 1. Inclusion information (tag tree)
	// 2. Number of zero bit-planes (tag tree, only for first inclusion)
	// 3. Number of coding passes
	// 4. Length of code-block contribution

	// Note: In a real implementation, we would iterate over all code-blocks
	// in the precinct. For this MVP, we'll parse a simplified header.

	// Align to byte boundary
	if pp.bitPos != 0 {
		pp.pos++
		pp.bitPos = 0
	}

	packet.Header = pp.data[headerStart:pp.pos]
}

// parsePacketBody parses the packet body (compressed code-block data)
func (pp *PacketParser) parsePacketBody(packet *Packet) {
	// Read code-block contributions
	// Each contribution has:
	// - Length (already parsed from header)
	// - Compressed data

	// For MVP, we'll read until the next packet or end of data
	// In a real implementation, we would use the lengths from the header

	bodyStart := pp.pos

	// For now, read remaining data as body
	// This is simplified - real implementation would use header information
	if pp.pos < len(pp.data) {
		packet.Body = pp.data[bodyStart:]
		pp.pos = len(pp.data)
	}

}

// readBit reads a single bit from the data stream
func (pp *PacketParser) readBit() (int, error) {
	if pp.pos >= len(pp.data) {
		return 0, io.EOF
	}

	bit := int((pp.data[pp.pos] >> (7 - pp.bitPos)) & 1)
	pp.bitPos++

	if pp.bitPos >= 8 {
		pp.bitPos = 0
		pp.pos++
	}

	return bit, nil
}

// readBits reads multiple bits from the data stream
// Reserved for future packet header parsing implementation
// func (pp *PacketParser) readBits(n int) (int, error) {
// 	if n <= 0 || n > 32 {
// 		return 0, fmt.Errorf("invalid number of bits: %d", n)
// 	}

// 	result := 0
// 	for i := 0; i < n; i++ {
// 		bit, err := pp.readBit()
// 		if err != nil {
// 			return 0, err
// 		}
// 		result = (result << 1) | bit
// 	}

// 	return result, nil
// }

// readTagTree reads a value from a tag tree
// Reference: ISO/IEC 15444-1:2019 Annex B.10.2
// Reserved for future implementation
// func (pp *PacketParser) readTagTree(tree *TagTree, leafIndex int) (int, error) {
// 	// Tag tree decoding
// 	// This is a simplified implementation
// 	// Real implementation would traverse the tree properly

// 	// For MVP, return a default value
// 	// TODO: Implement full tag tree decoding
// 	return 0, nil
// }

// alignToByte aligns the bit position to the next byte boundary
// Reserved for future implementation
// func (pp *PacketParser) alignToByte() {
// 	if pp.bitPos != 0 {
// 		pp.pos++
// 		pp.bitPos = 0
// 	}
// }

// Reset resets the parser to parse from the beginning
func (pp *PacketParser) Reset() {
	pp.pos = 0
	pp.bitPos = 0
	pp.inclTagTree.Reset()
	pp.zbpTagTree.Reset()
	pp.currentPacket = nil
}

// Position returns the current byte position
func (pp *PacketParser) Position() int {
	return pp.pos
}

// Remaining returns the number of bytes remaining
func (pp *PacketParser) Remaining() int {
	return len(pp.data) - pp.pos
}

// readBitsActive reads multiple bits from the data stream
// Reserved for future packet header parsing
//
//nolint:unused // Reserved for future use
func (pp *PacketParser) readBitsActive(n int) (int, error) {
	if n <= 0 || n > 32 {
		return 0, fmt.Errorf("invalid number of bits: %d", n)
	}

	result := 0
	for i := 0; i < n; i++ {
		bit, err := pp.readBit()
		if err != nil {
			return 0, err
		}
		result = (result << 1) | bit
	}

	return result, nil
}

// readTagTreeValue reads a value from a tag tree
// Reference: ISO/IEC 15444-1:2019 Annex B.10.2
// Simplified implementation for MVP
//
//nolint:unused // Reserved for future use
func (pp *PacketParser) readTagTreeValue(tree *TagTree, leafX, leafY int, threshold int) (int, error) {
	// Tag tree decoding - simplified version
	// Check if value is already known
	value := tree.GetValue(leafX, leafY)
	if value >= 0 && value < threshold {
		return value, nil
	}

	// For MVP, read a simple encoded value
	// Real implementation would traverse tree levels
	val, err := pp.readBitsActive(4) // Simple 4-bit encoding for MVP
	if err != nil {
		return 0, err
	}

	tree.SetValue(leafX, leafY, val)
	return val, nil
}

// alignToByteActive aligns the bit position to the next byte boundary
//
//nolint:unused // Reserved for future use
func (pp *PacketParser) alignToByteActive() {
	if pp.bitPos != 0 {
		pp.pos++
		pp.bitPos = 0
	}
}
