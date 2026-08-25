package cleanup

// Optimized VLC Decoder using pre-generated lookup tables
// 100% aligned with ISO/IEC 15444-15:2019 and OpenJPH
// Reference: OpenJPH ojph_block_common.cpp:154-189

// VLCDecoderOptimized implements optimized VLC decoding using pre-generated tables
// This decoder uses the VLCDecodeTbl0 and VLCDecodeTbl1 tables generated in Stage 3.1
type VLCDecoderOptimized struct {
	data      []byte
	pos       int    // Current byte position (forward reading)
	bitBuffer uint32 // Bit buffer for bit-level reading
	bitCount  int    // Number of valid bits in buffer
	lastByte  uint8  // Last byte read (for bit-unstuffing detection)
}

// NewVLCDecoderOptimized creates a new optimized VLC decoder
func NewVLCDecoderOptimized(data []byte) *VLCDecoderOptimized {
	v := &VLCDecoderOptimized{
		data:     data,
		pos:      0,
		lastByte: 0xFF, // Initialize consistent with encoder
	}

	// Generate tables if not already generated
	// In production, tables should be pre-generated at package init
	if VLCDecodeTbl0[0].CwdLen == 0 && VLCTbl0[0].CwdLen != 0 {
		_ = GenerateVLCTables()
	}

	// Skip initial 4-bit padding (encoder starts with 4 bits set to 1111)
	_, _ = v.readBits(4)

	return v
}

// readBits reads n bits from the VLC stream
// Implements bit-unstuffing per ISO/IEC 15444-15:2019 Clause F.4
//
// Bit-stuffing rule: When last byte > 0x8F and current byte == 0x7F,
// the MSB of current byte is a stuffed bit and should be ignored
func (v *VLCDecoderOptimized) readBits(n int) (uint32, bool) {
	// Refill bit buffer if needed
	for v.bitCount < n && v.pos < len(v.data) {
		b := v.data[v.pos]
		v.pos++

		// Check for bit-unstuffing condition
		if v.lastByte > 0x8F && b == 0x7F {
			// MSB is stuffed bit, use only lower 7 bits
			v.bitBuffer |= uint32(b&0x7F) << v.bitCount
			v.bitCount += 7
		} else {
			// Normal byte, use all 8 bits
			v.bitBuffer |= uint32(b) << v.bitCount
			v.bitCount += 8
		}

		v.lastByte = b
	}

	if v.bitCount < n {
		return 0, false // Insufficient bits
	}

	// Extract n bits (LSB first)
	mask := uint32((1 << n) - 1)
	result := v.bitBuffer & mask
	v.bitBuffer >>= n
	v.bitCount -= n

	return result, true
}

// DecodeQuadWithContext decodes a quad using context-aware VLC
//
// Parameters:
//   - context: Context value (0-7 for initial row as index, 0-3 actual values; 0-7 for non-initial row)
//   - isFirstRow: True for initial quad row
//
// Returns:
//   - rho: Significance pattern (4 bits)
//   - uOff: U offset flag (1 bit)
//   - ek: E_k value (4 bits)
//   - e1: E_1 value (4 bits)
//   - found: True if valid codeword was decoded
func (v *VLCDecoderOptimized) DecodeQuadWithContext(context uint8, isFirstRow bool) (rho, uOff, ek, e1 uint8, found bool) {
	// Progressive decode: read bits one at a time until we find a match
	// This matches the original decoder's behavior exactly
	var bits uint32

	for length := 1; length <= 7; length++ {
		b, ok := v.readBits(1)
		if !ok {
			return 0, 0, 0, 0, false
		}

		// Accumulate bits (LSB first)
		bits |= (b << (length - 1))

		// Check lookup table for this length
		// Index: [context (3 bits) << 7 | codeword (7 bits)]
		index := (uint16(context) << 7) | uint16(bits&0x7F)

		// Lookup in pre-generated table
		var entry VLCDecoderEntry
		if isFirstRow {
			entry = VLCDecodeTbl0[index]
		} else {
			entry = VLCDecodeTbl1[index]
		}

		// Check if this is a valid entry with matching length
		if entry.CwdLen == uint8(length) {
			return entry.Rho, entry.UOff, entry.EK, entry.E1, true
		}
	}

	// No match found after trying all possible lengths
	return 0, 0, 0, 0, false
}

// HasMoreData returns true if there are more bits to decode
func (v *VLCDecoderOptimized) HasMoreData() bool {
	return v.pos < len(v.data) || v.bitCount > 0
}

// GetBitPosition returns current bit position for debugging
func (v *VLCDecoderOptimized) GetBitPosition() (bytePos int, bitPos int) {
	return v.pos, v.bitCount
}

// Reset resets the decoder to initial state with new data
func (v *VLCDecoderOptimized) Reset(data []byte) {
	v.data = data
	v.pos = 0
	v.bitBuffer = 0
	v.bitCount = 0
	v.lastByte = 0xFF

	// Skip initial 4-bit padding
	_, _ = v.readBits(4)
}

// ReadBits reads n bits from the VLC stream (public API for external use)
func (v *VLCDecoderOptimized) ReadBits(n int) (uint32, bool) {
	return v.readBits(n)
}
