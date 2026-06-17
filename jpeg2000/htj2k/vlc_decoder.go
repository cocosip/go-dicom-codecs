package htj2k

// VLCDecoder implements VLC decoding based on OpenJPEG's approach
// Reference: OpenJPEG t1_ht_generate_luts.c
type VLCDecoder struct {
	data      []byte
	pos       int    // Current byte position (grows backward)
	bitBuffer uint32 // Bit buffer
	bitCount  int    // Number of valid bits in buffer
	lastByte  uint8  // Last byte read (for bit-unstuffing detection)
	reader    BitReader
	reverse   *reverseBitReader

	// Lookup tables for fast decoding (1024 entries each)
	// Key: (cQ << 7) | codeword
	tbl0 [1024]uint16 // For initial quad rows
	tbl1 [1024]uint16 // For non-initial quad rows
}

// NewVLCDecoder creates a new VLC decoder
func NewVLCDecoder(data []byte) *VLCDecoder {
	v := &VLCDecoder{
		data:     data,
		pos:      0,    // Start from beginning (no reversal)
		lastByte: 0xFF, // Initialize with encoder's vlcLast (first byte was skipped)
	}
	v.buildLookupTables()

	// Skip initial 4-bit padding
	_, _ = v.readBits(4)

	return v
}

// buildLookupTables builds the lookup tables from VLCTbl0 and VLCTbl1
// This follows the OpenJPEG approach in vlc_init_tables()
func (v *VLCDecoder) buildLookupTables() {
	// Build lookup table for tbl0
	for i := 0; i < 1024; i++ {
		cwd := i & 0x7F
		cQ := i >> 7
		bestLen := uint8(0)
		var packed uint16
		for j := range VLCTbl0 {
			entry := &VLCTbl0[j]
			if int(entry.CQ) != cQ {
				continue
			}
			mask := (1 << entry.CwdLen) - 1
			if int(entry.Cwd) == (cwd & mask) {
				if entry.CwdLen > bestLen {
					bestLen = entry.CwdLen
					packed = (uint16(entry.EK) << 12) |
						(uint16(entry.E1) << 8) |
						(uint16(entry.Rho) << 4) |
						(uint16(entry.UOff) << 3) |
						uint16(entry.CwdLen)
				}
			}
		}
		v.tbl0[i] = packed
	}

	// Build lookup table for tbl1
	for i := 0; i < 1024; i++ {
		cwd := i & 0x7F
		cQ := i >> 7
		bestLen := uint8(0)
		var packed uint16
		for j := range VLCTbl1 {
			entry := &VLCTbl1[j]
			if int(entry.CQ) != cQ {
				continue
			}
			mask := (1 << entry.CwdLen) - 1
			if int(entry.Cwd) == (cwd & mask) {
				if entry.CwdLen > bestLen {
					bestLen = entry.CwdLen
					packed = (uint16(entry.EK) << 12) |
						(uint16(entry.E1) << 8) |
						(uint16(entry.Rho) << 4) |
						(uint16(entry.UOff) << 3) |
						uint16(entry.CwdLen)
				}
			}
		}
		v.tbl1[i] = packed
	}
}

// readBits reads n bits from the bit stream (forward, LSB first)
// Implements bit-unstuffing for JPEG-style bit-stuffing
func (v *VLCDecoder) readBits(n int) (uint32, bool) {
	if v.reader != nil {
		bits, err := v.reader.ReadBitsLE(n)
		return bits, err == nil
	}

	// Ensure we have enough bits in buffer (read forward)
	for v.bitCount < n && v.pos < len(v.data) {
		b := v.data[v.pos]
		v.pos++

		// Check for bit-unstuffing: if lastByte > 0x8F and b == 0x7F
		// Then the MSB of b is a stuffed bit and should be ignored
		if v.lastByte > 0x8F && b == 0x7F {
			// Only use lower 7 bits (MSB is stuffed)
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
		return 0, false // Not enough bits
	}

	// Extract n bits
	mask := uint32((1 << n) - 1)
	result := v.bitBuffer & mask
	v.bitBuffer >>= n
	v.bitCount -= n

	return result, true
}

// ReadBit implements BitReader interface for U-VLC integration
func (v *VLCDecoder) ReadBit() (uint8, error) {
	bit, ok := v.readBits(1)
	if !ok {
		return 0, ErrInsufficientData
	}
	return uint8(bit), nil
}

// ReadBitsLE implements BitReader interface for U-VLC integration
func (v *VLCDecoder) ReadBitsLE(n int) (uint32, error) {
	bits, ok := v.readBits(n)
	if !ok {
		return 0, ErrInsufficientData
	}
	return bits, nil
}

// DecodeInitialRow decodes VLC for initial quad row
// Returns: (rho, u_off, e_k, e_1, found)
func (v *VLCDecoder) DecodeInitialRow(context uint8) (uint8, uint8, uint8, uint8, bool) {
	// Progressive decode to select exact-length match
	var bits uint32
	for length := 1; length <= 7; length++ {
		b, ok := v.readBits(1)
		if !ok {
			return 0, 0, 0, 0, false
		}
		bits |= (b << (length - 1))
		// search entries with this length
		for _, entry := range VLCTbl0 {
			if entry.CQ != context || int(entry.CwdLen) != length {
				continue
			}
			mask := (1 << entry.CwdLen) - 1
			if int(entry.Cwd) == int(bits&uint32(mask)) {
				uOff := entry.UOff
				rho := entry.Rho
				e1 := entry.E1
				eK := entry.EK
				return rho, uOff, eK, e1, true
			}
		}
	}
	return 0, 0, 0, 0, false
}

// DecodeNonInitialRow decodes VLC for non-initial quad row
func (v *VLCDecoder) DecodeNonInitialRow(context uint8) (uint8, uint8, uint8, uint8, bool) {
	var bits uint32
	for length := 1; length <= 7; length++ {
		b, ok := v.readBits(1)
		if !ok {
			return 0, 0, 0, 0, false
		}
		bits |= (b << (length - 1))
		for _, entry := range VLCTbl1 {
			if entry.CQ != context || int(entry.CwdLen) != length {
				continue
			}
			mask := (1 << entry.CwdLen) - 1
			if int(entry.Cwd) == int(bits&uint32(mask)) {
				uOff := entry.UOff
				rho := entry.Rho
				e1 := entry.E1
				eK := entry.EK
				return rho, uOff, eK, e1, true
			}
		}
	}
	return 0, 0, 0, 0, false
}

// HasMore returns true if there are more bits to decode
func (v *VLCDecoder) HasMore() bool {
	return v.bitCount > 0 || v.pos > 0
}

// DecodeUVLCWithTable 尝试使用 UVLC 表驱动解码（单 quad 视角）。
// isInitialRow 表示是否初始行对，melBit 为当前 quad 的 MEL 事件（0/1，未用时可传 0）。
// 返回 (u, ok)；ok=false 表示未能解码，应回退其它路径。
func (v *VLCDecoder) DecodeUVLCWithTable(isInitialRow bool, melBit int) (uint32, bool) {
	decoder := NewUVLCDecoder(v)
	u, ok := decoder.DecodeWithTable(1, isInitialRow, melBit)
	return u, ok
}

// DecodeUVLC 使用逐步解析方式解码 U-VLC（无初始行对偏置）。
func (v *VLCDecoder) DecodeUVLC() (uint32, error) {
	decoder := NewUVLCDecoder(v)
	return decoder.DecodeUnsignedResidual()
}

// DecodeQuad decodes a quad (simplified compatibility method)
// Returns: (significance_pattern, magnitudes, found)
func (v *VLCDecoder) DecodeQuad() (uint8, []uint32, bool) {
	// Simplified implementation - read basic pattern
	// In a full implementation, this would use context and proper VLC decoding

	if !v.HasMore() {
		return 0, nil, false
	}

	// Read significance pattern (4 bits for 2x2 quad)
	sig, ok := v.readBits(4)
	if !ok {
		return 0, nil, false
	}

	// Decode magnitudes for significant samples
	mags := make([]uint32, 0, 4)
	for i := 0; i < 4; i++ {
		if (sig & (1 << i)) != 0 {
			// Sample is significant - decode magnitude
			mag, ok := v.readBits(4) // Simplified - use 4 bits
			if !ok {
				return uint8(sig), mags, true
			}
			mags = append(mags, mag)
		}
	}

	return uint8(sig), mags, true
}

// DecodeQuadWithContext decodes a quad using context-based VLC decoding
// This is the proper HTJ2K implementation using context computation
// Returns: (rho, u_off, e_k, e_1, found)
func (v *VLCDecoder) DecodeQuadWithContext(context uint8, isFirstRow bool) (uint8, uint8, uint8, uint8, bool) {
	if isFirstRow {
		return v.DecodeInitialRow(context)
	}
	return v.DecodeNonInitialRow(context)
}
