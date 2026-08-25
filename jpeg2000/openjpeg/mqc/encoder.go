package mqc

// MQEncoder implements the MQ arithmetic encoder
// Reference: ISO/IEC 15444-1:2019 Annex C
type MQEncoder struct {
	// Output buffer (index 0 is dummy byte)
	buffer []byte
	start  int
	bp     int

	// State variables
	a   uint32 // Probability interval
	c   uint32 // Code register
	ct  int    // Bit counter

	// Contexts
	contexts []uint8 // Context states (one per context)
}

const bypassCtInit = 0xDEADBEEF

// NewMQEncoder creates a new MQ encoder
func NewMQEncoder(numContexts int) *MQEncoder {
	mqe := &MQEncoder{
		buffer:    make([]byte, 1, 1024),
		start:     1,
		bp:        0,
		a:         0x8000,
		c:         0,
		ct:        12,
		contexts:  make([]uint8, numContexts),
	}

	// Initialize all contexts to state 0
	for i := range mqe.contexts {
		mqe.contexts[i] = 0
	}

	return mqe
}

// Encode encodes a single bit using the specified context
func (mqe *MQEncoder) Encode(bit int, contextID int) {
	cx := &mqe.contexts[contextID]
	state := *cx & 0x7F  // Lower 7 bits = state
	mps := int(*cx >> 7) // Bit 7 = MPS (More Probable Symbol)

	// Get Qe value for this state
	qe := qeTable[state]

	if bit == mps {
		// Encoding MPS (Most Probable Symbol)
		mqe.a -= qe
		if (mqe.a & 0x8000) == 0 {
			// Renormalization needed
			// Conditional exchange (from ISO/IEC 15444-1)
			if mqe.a < qe {
				mqe.a = qe
			} else {
				mqe.c += qe
			}
			// Update state (MPS path)
			*cx = nmpsTable[state] | (uint8(mps) << 7)
			mqe.renorme()
		} else {
			// No renormalization
			mqe.c += qe
		}
	} else {
		// Encoding LPS (Least Probable Symbol)
		mqe.a -= qe
		if mqe.a < qe {
			mqe.c += qe
		} else {
			mqe.a = qe
		}
		// Update state (LPS path)
		newState := nlpsTable[state]
		newMPS := mps
		if switchTable[state] == 1 {
			newMPS = 1 - mps
		}
		*cx = newState | (uint8(newMPS) << 7)
		mqe.renorme()
	}
}

// renorme renormalizes the encoder (probability interval doubling)
func (mqe *MQEncoder) renorme() {
	for mqe.a < 0x8000 {
		mqe.a <<= 1
		mqe.c <<= 1
		mqe.ct--
		if mqe.ct == 0 {
			mqe.byteout()
		}
	}
}

// byteout outputs a byte to the stream
func (mqe *MQEncoder) byteout() {
	if mqe.bp >= len(mqe.buffer) {
		mqe.ensureIndex(mqe.bp)
	}

	if mqe.buffer[mqe.bp] == 0xFF {
		mqe.bp++
		mqe.ensureIndex(mqe.bp)
		mqe.buffer[mqe.bp] = byte(mqe.c >> 20)
		mqe.c &= 0xFFFFF
		mqe.ct = 7
		return
	}

	if (mqe.c & 0x8000000) == 0 {
		mqe.bp++
		mqe.ensureIndex(mqe.bp)
		mqe.buffer[mqe.bp] = byte(mqe.c >> 19)
		mqe.c &= 0x7FFFF
		mqe.ct = 8
		return
	}

	mqe.buffer[mqe.bp]++
	if mqe.buffer[mqe.bp] == 0xFF {
		mqe.c &= 0x7FFFFFF
		mqe.bp++
		mqe.ensureIndex(mqe.bp)
		mqe.buffer[mqe.bp] = byte(mqe.c >> 20)
		mqe.c &= 0xFFFFF
		mqe.ct = 7
		return
	}

	mqe.bp++
	mqe.ensureIndex(mqe.bp)
	mqe.buffer[mqe.bp] = byte(mqe.c >> 19)
	mqe.c &= 0x7FFFF
	mqe.ct = 8
}

// Flush finalizes encoding and returns the encoded data
func (mqe *MQEncoder) Flush() []byte {
	// setbits: fill remaining bits with 1's for flushing
	// Mirror OpenJPEG's opj_mqc_setbits()
	tempC := mqe.c + mqe.a
	mqe.c |= 0xFFFF
	if mqe.c >= tempC {
		mqe.c -= 0x8000
	}

	// Output final bytes
	mqe.c <<= uint(mqe.ct)
	mqe.byteout()
	mqe.c <<= uint(mqe.ct)
	mqe.byteout()

	// It is forbidden that a coding pass ends with 0xFF
	if mqe.buffer[mqe.bp] != 0xFF {
		mqe.bp++
	}

	if mqe.bp < mqe.start {
		return []byte{}
	}
	return mqe.buffer[mqe.start:mqe.bp]
}

// GetBuffer returns the current output buffer (for layered encoding)
func (mqe *MQEncoder) GetBuffer() []byte {
	if mqe.bp < mqe.start {
		return []byte{}
	}
	return mqe.buffer[mqe.start:mqe.bp]
}

// NumBytes returns the current number of bytes in the output buffer
// This is used for rate tracking in multi-layer encoding (following OpenJPEG)
func (mqe *MQEncoder) NumBytes() int {
	if mqe.bp < mqe.start {
		return 0
	}
	return mqe.bp - mqe.start
}

// FlushToOutput flushes the encoder to the output buffer
// This is used for pass termination in multi-layer encoding
func (mqe *MQEncoder) FlushToOutput() {
	// setbits: fill remaining bits with 1's for flushing
	// Mirror OpenJPEG's opj_mqc_setbits()
	tempC := mqe.c + mqe.a
	mqe.c |= 0xFFFF
	if mqe.c >= tempC {
		mqe.c -= 0x8000
	}

	// Output final bytes
	mqe.c <<= uint(mqe.ct)
	mqe.byteout()
	mqe.c <<= uint(mqe.ct)
	mqe.byteout()

	// It is forbidden that a coding pass ends with 0xFF
	if mqe.buffer[mqe.bp] != 0xFF {
		mqe.bp++
	}
}

// ErtermEnc performs predictable termination (PTERM) flush.
func (mqe *MQEncoder) ErtermEnc() {
	k := 11 - mqe.ct + 1
	for k > 0 {
		mqe.c <<= uint(mqe.ct)
		mqe.ct = 0
		mqe.byteout()
		k -= mqe.ct
	}
	if mqe.buffer[mqe.bp] != 0xFF {
		mqe.byteout()
	}
}

// BypassInitEnc initializes RAW (bypass) encoding.
func (mqe *MQEncoder) BypassInitEnc() {
	mqe.c = 0
	mqe.ct = bypassCtInit
}

// BypassEncode encodes a bit in RAW (bypass) mode.
func (mqe *MQEncoder) BypassEncode(bit int) {
	if mqe.ct == bypassCtInit {
		mqe.ct = 8
	}
	mqe.ct--
	mqe.c += uint32(bit) << uint(mqe.ct)
	if mqe.ct == 0 {
		if mqe.bp >= len(mqe.buffer) {
			mqe.ensureIndex(mqe.bp)
		}
		mqe.buffer[mqe.bp] = byte(mqe.c)
		mqe.ct = 8
		if mqe.buffer[mqe.bp] == 0xFF {
			mqe.ct = 7
		}
		mqe.bp++
		mqe.c = 0
	}
}

// BypassExtraBytes returns the extra bytes for a non-terminating RAW pass.
func (mqe *MQEncoder) BypassExtraBytes(erterm bool) int {
	if mqe.ct < 7 {
		return 1
	}
	if mqe.ct == 7 && (erterm || (mqe.bp > 0 && mqe.buffer[mqe.bp-1] != 0xFF)) {
		return 1
	}
	return 0
}

// BypassFlushEnc flushes RAW (bypass) encoding with optional ERTERM behavior.
func (mqe *MQEncoder) BypassFlushEnc(erterm bool) {
	if mqe.ct < 7 || (mqe.ct == 7 && (erterm || (mqe.bp > 0 && mqe.buffer[mqe.bp-1] != 0xFF))) {
		bitValue := 0
		for mqe.ct > 0 {
			mqe.ct--
			mqe.c += uint32(bitValue) << uint(mqe.ct)
			if bitValue == 0 {
				bitValue = 1
			} else {
				bitValue = 0
			}
		}
		if mqe.bp >= len(mqe.buffer) {
			mqe.ensureIndex(mqe.bp)
		}
		mqe.buffer[mqe.bp] = byte(mqe.c)
		mqe.bp++
	} else if mqe.ct == 7 && mqe.bp > 0 && mqe.buffer[mqe.bp-1] == 0xFF {
		if !erterm {
			mqe.bp--
		}
	} else if mqe.ct == 8 && !erterm && mqe.bp > 1 && mqe.buffer[mqe.bp-1] == 0x7F && mqe.buffer[mqe.bp-2] == 0xFF {
		mqe.bp -= 2
	}
}

// Reset resets the encoder state
func (mqe *MQEncoder) Reset() {
	mqe.buffer = make([]byte, 1, 1024)
	mqe.start = 1
	mqe.bp = 0
	mqe.a = 0x8000
	mqe.c = 0
	mqe.ct = 12
}

// SegmarkEnc emits the SEGSYM marker in MQ mode.
func (mqe *MQEncoder) SegmarkEnc() {
	for i := 1; i < 5; i++ {
		mqe.Encode(i%2, 18)
	}
}

// ResetContext resets a context to initial state
func (mqe *MQEncoder) ResetContext(contextID int) {
	mqe.contexts[contextID] = 0
}

// ResetContexts resets all contexts to initial state
func (mqe *MQEncoder) ResetContexts() {
	for i := range mqe.contexts {
		mqe.contexts[i] = 0
	}
}

// GetContextState returns the current state of a context
func (mqe *MQEncoder) GetContextState(contextID int) uint8 {
	return mqe.contexts[contextID]
}

// SetContextState sets the state of a context
func (mqe *MQEncoder) SetContextState(contextID int, state uint8) {
	mqe.contexts[contextID] = state
}

// RestartInitEnc reinitializes MQC state after a terminated pass.
// Mirrors OpenJPEG opj_mqc_restart_init_enc().
func (mqe *MQEncoder) RestartInitEnc() {
	mqe.a = 0x8000
	mqe.c = 0
	mqe.ct = 12
	if mqe.bp > mqe.start-1 {
		mqe.bp--
	}
	if mqe.bp >= 0 && mqe.bp < len(mqe.buffer) && mqe.buffer[mqe.bp] == 0xFF {
		mqe.ct = 13
	}
}

func (mqe *MQEncoder) ensureIndex(idx int) {
	if idx < len(mqe.buffer) {
		return
	}
	needed := idx + 1
	if needed <= cap(mqe.buffer) {
		mqe.buffer = mqe.buffer[:needed]
		return
	}
	newCap := cap(mqe.buffer) * 2
	if newCap < needed {
		newCap = needed
	}
	newBuf := make([]byte, needed, newCap)
	copy(newBuf, mqe.buffer)
	mqe.buffer = newBuf
}
