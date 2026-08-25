package mqc

// MQ Arithmetic Decoder for JPEG 2000
// Reference: ISO/IEC 15444-1:2019 Annex C
// Based on the MQ-coder (multiplication-free, table-driven arithmetic coder)

// MQDecoder implements the MQ arithmetic decoder
type MQDecoder struct {
	// Input data
	data    []byte
	bp      int // Current byte position (points to last read byte)
	dataLen int // Original data length (without sentinel)

	// State variables
	a   uint32 // Probability interval
	c   uint32 // Code register
	ct  int    // Bit counter
	eos int    // End of byte stream counter (OpenJPEG compatibility)

	// Contexts
	contexts []uint8 // Context states (one per context)
}

// NewMQDecoder creates a new MQ decoder
func NewMQDecoder(data []byte, numContexts int) *MQDecoder {
	// Append 0xFF 0xFF sentinel marker to end of data
	// This acts as an artificial marker to stop bytein routines
	// Reference: OpenJPEG opj_mqc_init_dec_common
	dataWithSentinel := make([]byte, len(data)+2)
	copy(dataWithSentinel, data)
	dataWithSentinel[len(data)] = 0xFF
	dataWithSentinel[len(data)+1] = 0xFF

	mqc := &MQDecoder{
		data:     dataWithSentinel,
		bp:       0,
		dataLen:  len(data),
		a:        0x8000,
		c:        0,
		ct:       0,
		contexts: make([]uint8, numContexts),
	}

	// Initialize all contexts to state 0
	for i := range mqc.contexts {
		mqc.contexts[i] = 0
	}

	// Initialize decoder
	mqc.init()

	return mqc
}

// NewRawDecoder creates a new RAW (bypass) decoder.
func NewRawDecoder(data []byte) *MQDecoder {
	dataWithSentinel := make([]byte, len(data)+2)
	copy(dataWithSentinel, data)
	dataWithSentinel[len(data)] = 0xFF
	dataWithSentinel[len(data)+1] = 0xFF

	mqc := &MQDecoder{
		data:    dataWithSentinel,
		bp:      0,
		dataLen: len(data),
		a:       0,
		c:       0,
		ct:      0,
	}

	return mqc
}

// SetData updates the decoder's data buffer while preserving contexts
// This is used for lossy TERMALL mode where contexts should be maintained across passes
func (mqc *MQDecoder) SetData(data []byte) {
	// Append sentinel marker
	dataWithSentinel := make([]byte, len(data)+2)
	copy(dataWithSentinel, data)
	dataWithSentinel[len(data)] = 0xFF
	dataWithSentinel[len(data)+1] = 0xFF

	// Update data and reset position
	mqc.data = dataWithSentinel
	mqc.bp = 0
	mqc.dataLen = len(data)
	mqc.eos = 0

	// Reset decoder state registers but keep contexts
	mqc.a = 0x8000
	mqc.c = 0
	mqc.ct = 0

	// Reinitialize decoder with new data
	mqc.init()
}

// NewMQDecoderWithContexts creates a new MQ decoder with inherited contexts from previous decoder
// This is the correct approach for TERMALL mode: each pass gets a fresh decoder initialization
// but contexts can be preserved (lossy) or reset (lossless)
func NewMQDecoderWithContexts(data []byte, prevContexts []uint8) *MQDecoder {
	// Append sentinel marker
	dataWithSentinel := make([]byte, len(data)+2)
	copy(dataWithSentinel, data)
	dataWithSentinel[len(data)] = 0xFF
	dataWithSentinel[len(data)+1] = 0xFF

	mqc := &MQDecoder{
		data:     dataWithSentinel,
		bp:       0,
		dataLen:  len(data),
		a:        0x8000,
		c:        0,
		ct:       0,
		contexts: make([]uint8, len(prevContexts)),
	}

	// Copy contexts from previous decoder
	copy(mqc.contexts, prevContexts)

	// Initialize decoder
	mqc.init()

	return mqc
}

// GetContexts returns a copy of the current context states
// Used to preserve contexts across TERMALL pass boundaries
func (mqc *MQDecoder) GetContexts() []uint8 {
	contexts := make([]uint8, len(mqc.contexts))
	copy(contexts, mqc.contexts)
	return contexts
}

// init initializes the decoder
func (mqc *MQDecoder) init() {
	// Implements ISO 15444-1 C.3.5 Initialization of the decoder (INITDEC)
	if mqc.dataLen == 0 {
		mqc.c = 0xFF << 16
	} else {
		mqc.c = uint32(mqc.data[0]) << 16
	}
	mqc.bytein()
	mqc.c <<= 7
	mqc.ct -= 7
	mqc.a = 0x8000
}

// RawInit reinitializes the decoder for RAW (bypass) decoding.
func (mqc *MQDecoder) RawInit(data []byte) {
	dataWithSentinel := make([]byte, len(data)+2)
	copy(dataWithSentinel, data)
	dataWithSentinel[len(data)] = 0xFF
	dataWithSentinel[len(data)+1] = 0xFF

	mqc.data = dataWithSentinel
	mqc.bp = 0
	mqc.dataLen = len(data)
	mqc.eos = 0
	mqc.a = 0
	mqc.c = 0
	mqc.ct = 0
}

// Decode decodes a single bit using the specified context
//
// Performance notes:
// - Hot path function - called millions of times during decoding
// - Table-driven (no branches in probability lookup)
// - Multiplication-free arithmetic (shifts and adds only)
// - Context state maintained in single byte (compact)
// - Typical performance: ~10-20ns per bit on modern CPUs
// - Optimization: inline candidate, branch prediction important
func (mqc *MQDecoder) Decode(contextID int) int {
	cx := &mqc.contexts[contextID]
	state := *cx & 0x7F  // Lower 7 bits = state
	mps := int(*cx >> 7) // Bit 7 = MPS (More Probable Symbol)

	// Get Qe value for this state
	qe := qeTable[state]

	// Calculate sub-interval
	mqc.a -= qe

	var d int // Decoded bit

	// Check if coded bit is LPS or MPS
	if (mqc.c >> 16) < qe {
		// LPS exchange (ISO/IEC 15444-1 C.3.2)
		if mqc.a < qe {
			// Exchange: MPS becomes the larger interval
			mqc.a = qe
			d = mps
			*cx = nmpsTable[state] | (uint8(mps) << 7)
		} else {
			// No exchange: decode LPS
			mqc.a = qe
			d = 1 - mps
			newState := nlpsTable[state]
			newMPS := mps
			if switchTable[state] == 1 {
				newMPS = 1 - mps
			}
			*cx = newState | (uint8(newMPS) << 7)
		}
		mqc.renormd()
	} else {
		// MPS path
		mqc.c -= qe << 16

		if (mqc.a & 0x8000) != 0 {
			d = mps
			return d
		}

		if mqc.a < qe {
			d = 1 - mps
			newState := nlpsTable[state]
			newMPS := mps
			if switchTable[state] == 1 {
				newMPS = 1 - mps
			}
			*cx = newState | (uint8(newMPS) << 7)
		} else {
			d = mps
			*cx = nmpsTable[state] | (uint8(mps) << 7)
		}
		mqc.renormd()
	}

	return d
}

// renormd renormalizes the decoder (probability interval doubling)
func (mqc *MQDecoder) renormd() {
	for mqc.a < 0x8000 {
		if mqc.ct == 0 {
			mqc.bytein()
		}
		mqc.a <<= 1
		mqc.c <<= 1
		mqc.ct--
	}
}

// bytein reads the next byte from input stream
// Reference: OpenJPEG opj_mqc_bytein_macro
// Note: Data has 0xFF 0xFF sentinel appended, so we're guaranteed to have data
//
// OpenJPEG semantics: bp points to last read byte, reads *(bp+1) as next value
// Our semantics: pos points to next unread byte, lastByte holds last read byte
// Mapping: OpenJPEG *bp corresponds to our lastByte, *(bp+1) to data[pos]
func (mqc *MQDecoder) bytein() {
	// OpenJPEG bytein macro
	next := mqc.data[mqc.bp+1]
	if mqc.data[mqc.bp] == 0xFF {
		if next > 0x8F {
			mqc.c += 0xFF00
			mqc.ct = 8
			mqc.eos++
		} else {
			mqc.bp++
			mqc.c += uint32(next) << 9
			mqc.ct = 7
		}
	} else {
		mqc.bp++
		mqc.c += uint32(next) << 8
		mqc.ct = 8
	}
}

// RawDecode decodes a single bit using RAW (bypass) decoding.
func (mqc *MQDecoder) RawDecode() int {
	if mqc.ct == 0 {
		if mqc.c == 0xFF {
			next := mqc.data[mqc.bp]
			if next > 0x8F {
				mqc.c = 0xFF
				mqc.ct = 8
			} else {
				mqc.c = uint32(next)
				mqc.bp++
				mqc.ct = 7
			}
		} else {
			mqc.c = uint32(mqc.data[mqc.bp])
			mqc.bp++
			mqc.ct = 8
		}
	}
	mqc.ct--
	return int((mqc.c >> uint(mqc.ct)) & 0x01)
}

// ResetContext resets a context to initial state
func (mqc *MQDecoder) ResetContext(contextID int) {
	mqc.contexts[contextID] = 0
}

// ResetContexts resets all contexts to initial state
func (mqc *MQDecoder) ResetContexts() {
	for i := range mqc.contexts {
		mqc.contexts[i] = 0
	}
}

// ReinitAfterTermination reinitializes the decoder state after a terminated pass
// This is used in TERMALL mode where each pass is independently terminated
// Simply resets the MQC state variables - the decoder will naturally continue
// reading from the current position in the bitstream
func (mqc *MQDecoder) ReinitAfterTermination() {
	// Reset MQC state variables to initial values
	// The probability interval and code register are reset
	mqc.a = 0x8000
	mqc.c = 0
	mqc.ct = 0
	// Note: We do NOT reset bp - decoder continues from current position
}

// GetContextState returns the current state of a context
func (mqc *MQDecoder) GetContextState(contextID int) uint8 {
	return mqc.contexts[contextID]
}

// SetContextState sets the state of a context
func (mqc *MQDecoder) SetContextState(contextID int, state uint8) {
	mqc.contexts[contextID] = state
}

// MQ-coder state tables
// Reference: ISO/IEC 15444-1:2019 Table C.2

// qeTable - Qe values for each state
var qeTable = [47]uint32{
	0x5601, 0x3401, 0x1801, 0x0AC1, 0x0521, 0x0221, 0x5601, 0x5401,
	0x4801, 0x3801, 0x3001, 0x2401, 0x1C01, 0x1601, 0x5601, 0x5401,
	0x5101, 0x4801, 0x3801, 0x3401, 0x3001, 0x2801, 0x2401, 0x2201,
	0x1C01, 0x1801, 0x1601, 0x1401, 0x1201, 0x1101, 0x0AC1, 0x09C1,
	0x08A1, 0x0521, 0x0441, 0x02A1, 0x0221, 0x0141, 0x0111, 0x0085,
	0x0049, 0x0025, 0x0015, 0x0009, 0x0005, 0x0001, 0x5601,
}

// nmpsTable - Next state for MPS
var nmpsTable = [47]uint8{
	1, 2, 3, 4, 5, 38, 7, 8,
	9, 10, 11, 12, 13, 29, 15, 16,
	17, 18, 19, 20, 21, 22, 23, 24,
	25, 26, 27, 28, 29, 30, 31, 32,
	33, 34, 35, 36, 37, 38, 39, 40,
	41, 42, 43, 44, 45, 45, 46,
}

// nlpsTable - Next state for LPS
var nlpsTable = [47]uint8{
	1, 6, 9, 12, 29, 33, 6, 14,
	14, 14, 17, 18, 20, 21, 14, 14,
	15, 16, 17, 18, 19, 19, 20, 21,
	22, 23, 24, 25, 26, 27, 28, 29,
	30, 31, 32, 33, 34, 35, 36, 37,
	38, 39, 40, 41, 42, 43, 46,
}

// switchTable - MPS/LPS switch indicator
var switchTable = [47]uint8{
	1, 0, 0, 0, 0, 0, 1, 0,
	0, 0, 0, 0, 0, 0, 1, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0,
}

// GetQeTable returns the Qe probability estimation table for validation
func GetQeTable() [47]uint32 {
	return qeTable
}

// GetNmpsTable returns the NMPS state transition table for validation
func GetNmpsTable() [47]uint8 {
	return nmpsTable
}

// GetNlpsTable returns the NLPS state transition table for validation
func GetNlpsTable() [47]uint8 {
	return nlpsTable
}

// GetSwitchTable returns the MPS/LPS switch table for validation
func GetSwitchTable() [47]uint8 {
	return switchTable
}
