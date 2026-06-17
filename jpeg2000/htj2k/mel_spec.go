package htj2k

// MEL (Adaptive Run-Length Coding) - Based on ISO/IEC 15444-15:2019
// Reference: Clause 7.3.3 - MEL symbol decoding procedure

// MelE is the exponent table for MEL decoding
// Table 2 from ISO/IEC 15444-15:2019
var MelE = [13]int{
	0, // k=0
	0, // k=1
	0, // k=2
	1, // k=3
	1, // k=4
	1, // k=5
	2, // k=6
	2, // k=7
	2, // k=8
	3, // k=9
	3, // k=10
	4, // k=11
	5, // k=12
}

// MELDecoderSpec implements the MEL decoder based on ISO/IEC 15444-15 specification
type MELDecoderSpec struct {
	decoder *MELDecoder
}

// NewMELDecoderSpec creates a new spec-compliant MEL decoder
func NewMELDecoderSpec(data []byte) *MELDecoderSpec {
	mel := &MELDecoderSpec{decoder: NewMELDecoder(data)}
	mel.initMELDecoder()
	return mel
}

// initMELDecoder initializes the MEL decoder state
// Procedure: initMELDecoder from ISO/IEC 15444-15:2019
func (m *MELDecoderSpec) initMELDecoder() {
	// State is initialized by NewMELDecoder.
}

// DecodeMELSym decodes the next MEL symbol
// Returns: (symbol, hasMore)
// symbol: 0 = continue run (all-zero context), 1 = end run (has significant samples)
// Procedure: decodeMELSym from ISO/IEC 15444-15:2019 Clause 7.3.3
func (m *MELDecoderSpec) DecodeMELSym() (int, bool) {
	return m.decoder.DecodeBit()
}
