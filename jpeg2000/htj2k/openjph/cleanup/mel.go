package cleanup

// MEL (Magnitude Exponent Likelihood) encoder/decoder.
// Implements the 13-state adaptive run-length coder from ISO/IEC 15444-15:2019
// (aligned with OpenJPH mel_encode/mel_decode behavior).

// MELEncoder implements the MEL encoder for HTJ2K block coding.
// Byte stuffing: if the emitted byte is 0xFF, the next byte carries only 7 bits.
type MELEncoder struct {
	buf           []byte
	tmp           uint8
	remainingBits int
	run           int
	k             int
	threshold     int
}

// NewMELEncoder creates a new MEL encoder.
func NewMELEncoder() *MELEncoder {
	return &MELEncoder{
		buf:           make([]byte, 0, 192), // MEL is ~1 bit/quad; pre-allocate enough.
		remainingBits: 8,
		threshold:     1,
	}
}

// EncodeBit encodes one MEL symbol (0 = continue run, 1 = terminate run).
func (m *MELEncoder) EncodeBit(bit int) {
	if bit == 0 {
		m.run++
		if m.run >= m.threshold {
			m.emitBit(1)
			m.run = 0
			if m.k < 12 {
				m.k++
			}
			m.threshold = 1 << MelE[m.k]
		}
		return
	}

	// bit == 1: emit 0 + run bits (length MelE[k]).
	m.emitBit(0)
	t := MelE[m.k]
	for t > 0 {
		t--
		m.emitBit(uint8((m.run >> t) & 1))
	}
	m.run = 0
	if m.k > 0 {
		m.k--
	}
	m.threshold = 1 << MelE[m.k]
}

// emitBit writes a single bit to the output buffer with byte stuffing.
func (m *MELEncoder) emitBit(b uint8) {
	m.tmp = (m.tmp << 1) | (b & 1)
	m.remainingBits--
	if m.remainingBits == 0 {
		m.buf = append(m.buf, m.tmp)
		if m.tmp == 0xFF {
			m.remainingBits = 7
		} else {
			m.remainingBits = 8
		}
		m.tmp = 0
	}
}

// Flush finalizes the MEL stream and returns the encoded bytes.
func (m *MELEncoder) Flush() []byte {
	if m.run > 0 {
		m.EncodeBit(1)
	}
	if m.remainingBits != 8 {
		m.tmp <<= m.remainingBits
		m.buf = append(m.buf, m.tmp)
		m.tmp = 0
		m.remainingBits = 8
	}
	return m.buf
}

// FlushForFusion finalizes the MEL stream and returns data for MEL/VLC fusion
// Returns: data bytes, last byte, number of used bits in last byte
// Reference: OpenJPH ojph_block_encoder.cpp lines 420-446
func (m *MELEncoder) FlushForFusion() ([]byte, uint8, int) {
	if m.run > 0 {
		m.EncodeBit(1)
	}

	usedBits := 0
	lastByte := uint8(0)

	if m.remainingBits != 8 {
		// There are pending bits
		usedBits = 8 - m.remainingBits
		lastByte = m.tmp << m.remainingBits
		m.buf = append(m.buf, lastByte)
		m.tmp = 0
		m.remainingBits = 8
	}

	if len(m.buf) > 0 {
		lastByte = m.buf[len(m.buf)-1]
		// If last byte was just added, use usedBits from above
		// Otherwise, it's a full byte
		if usedBits == 0 {
			usedBits = 8
		}
	}

	return m.buf, lastByte, usedBits
}

// GetBytes returns current encoded bytes (without final padding).
func (m *MELEncoder) GetBytes() []byte {
	return m.buf
}

// Length returns number of bytes already emitted (without padding).
func (m *MELEncoder) Length() int {
	return len(m.buf)
}

// Reset resets the encoder for encoding a new block
func (m *MELEncoder) Reset() {
	m.buf = m.buf[:0]
	m.tmp = 0
	m.remainingBits = 8
	m.run = 0
	m.k = 0
	m.threshold = 1
}

// MELDecoder mirrors OpenJPH mel_decode behavior and expands runs into symbols.
type MELDecoder struct {
	data []byte
	pos  int

	// Bit read state with un-stuffing (last 0xFF => next byte 7 bits).
	lastByte uint8
	bits     uint8
	buf      uint8

	// MEL state.
	k int

	// Pending symbols expanded from last decoded run.
	pendingZeros int
	pendingOne   bool
}

// NewMELDecoder creates a new MEL decoder.
func NewMELDecoder(data []byte) *MELDecoder {
	return &MELDecoder{
		data:     data,
		lastByte: 0x00,
	}
}

// readBit reads one bit applying the un-stuffing rule.
func (m *MELDecoder) readBit() (uint8, bool) {
	if m.bits == 0 {
		if m.pos >= len(m.data) {
			return 0, false
		}
		b := m.data[m.pos]
		m.pos++

		if m.lastByte == 0xFF {
			m.buf = (b & 0x7F) << 1
			m.bits = 7
		} else {
			m.buf = b
			m.bits = 8
		}
		m.lastByte = b
	}

	bit := (m.buf >> 7) & 1
	m.buf <<= 1
	m.bits--
	return bit, true
}

// DecodeBit decodes one MEL symbol (0 or 1).
func (m *MELDecoder) DecodeBit() (int, bool) {
	if m.pendingZeros > 0 {
		m.pendingZeros--
		return 0, true
	}
	if m.pendingOne {
		m.pendingOne = false
		return 1, true
	}

	lead, ok := m.readBit()
	if !ok {
		return 0, false
	}

	eval := MelE[m.k]
	if lead == 1 {
		m.pendingZeros = 1 << eval
		if m.k < 12 {
			m.k++
		}
	} else {
		runVal := 0
		for i := 0; i < eval; i++ {
			b, ok := m.readBit()
			if !ok {
				return 0, false
			}
			runVal = (runVal << 1) | int(b)
		}
		m.pendingZeros = runVal
		m.pendingOne = true
		if m.k > 0 {
			m.k--
		}
	}

	return m.DecodeBit()
}
