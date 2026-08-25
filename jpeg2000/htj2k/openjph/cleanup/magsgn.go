package cleanup

// MagSgn (Magnitude-Sign) Encoder/Decoder for HTJ2K
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019
//
// The MagSgn segment stores magnitude and sign bits for significant samples
// It grows forward in the codeblock
// Bits are packed LSB-first with byte-stuffing after 0xFF.

// MagSgnEncoder implements the magnitude-sign encoder for HTJ2K
type MagSgnEncoder struct {
	buffer []byte // Output buffer (grows forward)

	// Bit-level output (LSB-first, with 0xFF stuffing)
	tmp      byte // Buffer for incomplete bytes
	usedBits int  // Number of bits in tmp
	maxBits  int  // Maximum bits allowed in tmp (8 or 7 after 0xFF)
}

// NewMagSgnEncoder creates a new MagSgn encoder
func NewMagSgnEncoder() *MagSgnEncoder {
	return &MagSgnEncoder{
		buffer:   make([]byte, 0, 2048),
		tmp:      0,
		usedBits: 0,
		maxBits:  8,
	}
}

// EncodeMagSgn encodes magnitude and sign bits for a sample
// mag: magnitude value (absolute value of coefficient)
// sign: sign bit (0 = positive, 1 = negative)
// numBits: number of magnitude bits to encode (sign is encoded as LSB)
func (m *MagSgnEncoder) EncodeMagSgn(mag uint32, sign int, numBits int) {
	value := (uint64(mag) << 1) | uint64(sign&1)
	m.writeBits(value, numBits+1)
}

// EncodeMagnitude encodes only magnitude bits (no sign)
func (m *MagSgnEncoder) EncodeMagnitude(mag uint32, numBits int) {
	m.writeBits(uint64(mag), numBits)
}

// EncodeSign encodes only sign bit
func (m *MagSgnEncoder) EncodeSign(sign int) {
	m.writeBits(uint64(sign&1), 1)
}

// writeBits writes bits LSB-first with 0xFF byte stuffing
func (m *MagSgnEncoder) writeBits(value uint64, numBits int) {
	for numBits > 0 {
		space := m.maxBits - m.usedBits
		if space > numBits {
			space = numBits
		}
		mask := uint64((1 << space) - 1)
		m.tmp |= byte((value & mask) << m.usedBits)
		m.usedBits += space
		value >>= space
		numBits -= space

		if m.usedBits == m.maxBits {
			m.buffer = append(m.buffer, m.tmp)
			if m.tmp == 0xFF {
				m.maxBits = 7
			} else {
				m.maxBits = 8
			}
			m.tmp = 0
			m.usedBits = 0
		}
	}
}

// Flush finalizes encoding and returns the buffer
func (m *MagSgnEncoder) Flush() []byte {
	// Terminate with 1s per HTJ2K byte-stuffing rules.
	if m.usedBits > 0 {
		remaining := m.maxBits - m.usedBits
		if remaining > 0 {
			m.tmp |= byte((0xFF & ((1 << remaining) - 1)) << m.usedBits)
			m.usedBits += remaining
		}
		m.buffer = append(m.buffer, m.tmp)
	}

	m.tmp = 0
	m.usedBits = 0
	m.maxBits = 8

	return m.buffer
}

// GetBytes returns the current buffer without flushing
func (m *MagSgnEncoder) GetBytes() []byte {
	return m.buffer
}

// Length returns the current length of encoded data
func (m *MagSgnEncoder) Length() int {
	return len(m.buffer)
}

// Reset resets the encoder for encoding a new block
func (m *MagSgnEncoder) Reset() {
	m.buffer = m.buffer[:0]
	m.tmp = 0
	m.usedBits = 0
	m.maxBits = 8
}

// MagSgnDecoder implements the magnitude-sign decoder for HTJ2K
type MagSgnDecoder struct {
	data []byte // Input data
	pos  int    // Current byte position

	// Bit-level input (LSB-first, with 0xFF unstuffing)
	bitBuffer uint64
	bitCount  int
	lastByte  byte
}

// NewMagSgnDecoder creates a new MagSgn decoder
func NewMagSgnDecoder(data []byte) *MagSgnDecoder {
	return &MagSgnDecoder{
		data:      data,
		pos:       0,
		bitBuffer: 0,
		bitCount:  0,
		lastByte:  0,
	}
}

// DecodeMagSgn decodes magnitude and sign bits
// numBits: number of magnitude bits to decode (sign is encoded as LSB)
// Returns: (magnitude, sign, hasMore)
func (m *MagSgnDecoder) DecodeMagSgn(numBits int) (uint32, int, bool) {
	value, ok := m.readBits(numBits + 1)
	if !ok {
		return 0, 0, false
	}
	sign := int(value & 1)
	mag := value >> 1
	return uint32(mag), sign, true
}

// DecodeMagnitude decodes only magnitude bits
func (m *MagSgnDecoder) DecodeMagnitude(numBits int) (uint32, bool) {
	value, ok := m.readBits(numBits)
	return uint32(value), ok
}

// DecodeSign decodes only sign bit
func (m *MagSgnDecoder) DecodeSign() (int, bool) {
	bit, ok := m.readBits(1)
	return int(bit), ok
}

// readBits reads bits LSB-first with 0xFF unstuffing
func (m *MagSgnDecoder) readBits(n int) (uint64, bool) {
	if n == 0 {
		return 0, true
	}
	ok := true
	for m.bitCount < n && m.pos < len(m.data) {
		b := m.data[m.pos]
		m.pos++

		if m.lastByte == 0xFF {
			m.bitBuffer |= uint64(b&0x7F) << m.bitCount
			m.bitCount += 7
		} else {
			m.bitBuffer |= uint64(b) << m.bitCount
			m.bitCount += 8
		}

		m.lastByte = b
	}

	if m.bitCount < n {
		if m.pos >= len(m.data) {
			ok = false
		}
		// When exhausted, MagSgn feeds 0xFF bytes (all-ones padding).
		for m.bitCount < n {
			b := byte(0xFF)
			if m.lastByte == 0xFF {
				m.bitBuffer |= uint64(b&0x7F) << m.bitCount
				m.bitCount += 7
			} else {
				m.bitBuffer |= uint64(b) << m.bitCount
				m.bitCount += 8
			}
			m.lastByte = b
		}
	}

	mask := uint64((1 << n) - 1)
	value := m.bitBuffer & mask
	m.bitBuffer >>= n
	m.bitCount -= n
	return value, ok
}

// HasMore returns true if more data is available
func (m *MagSgnDecoder) HasMore() bool {
	return m.pos < len(m.data) || m.bitCount > 0
}

// Reset resets the decoder to the beginning
func (m *MagSgnDecoder) Reset() {
	m.pos = 0
	m.bitBuffer = 0
	m.bitCount = 0
	m.lastByte = 0
}
