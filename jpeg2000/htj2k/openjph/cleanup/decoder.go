package cleanup

import (
	"fmt"
)

// Decoder is the OpenJPH-compatible HTJ2K block decoder used by the JPEG 2000 pipeline.
type Decoder struct {
	// Block dimensions
	width  int
	height int

	// Decoded data
	data []int32

	// Decoding state
	maxBitplane   int
	bandNumbps    int
	zeroBitplanes int
	irreversible  bool

	// Dimensions in quads
	qw int
	qh int
}

// NewDecoder creates a new HT decoder.
func NewDecoder(width, height int) *Decoder {
	qw := (width + 1) / 2
	qh := (height + 1) / 2

	return &Decoder{
		width:  width,
		height: height,
		qw:     qw,
		qh:     qh,
		data:   make([]int32, width*height),
	}
}

// Decode decodes a HTJ2K code-block.
// params: codeblock - encoded bytes, numPasses - pass count (unused in HT path)
// returns: decoded int32 coefficients and error
func (h *Decoder) Decode(codeblock []byte, _ int) ([]int32, error) {
	if len(codeblock) == 0 {
		return h.data, nil
	}

	if h.bandNumbps <= 0 {
		return nil, fmt.Errorf("HTJ2K OpenJPH cleanup decoding requires band precision context")
	}
	decoded, err := decodeOpenJPHCleanupWithMode(
		codeblock, h.width, h.height, h.bandNumbps, h.zeroBitplanes, h.irreversible,
	)
	if err != nil {
		return nil, fmt.Errorf("decode OpenJPH cleanup pass: %w", err)
	}
	h.data = decoded
	return h.data, nil
}

func parseStandardSegments(codeblock []byte) (magsgnData, cleanupData []byte, err error) {
	lcup := len(codeblock)
	scup := int(codeblock[lcup-1])<<4 | int(codeblock[lcup-2]&0x0F)
	if scup < 2 || scup > lcup || scup > 4079 {
		return nil, nil, fmt.Errorf("invalid HTJ2K Scup locator: scup=%d lcup=%d", scup, lcup)
	}

	magsgnLen := lcup - scup
	return codeblock[:magsgnLen], codeblock[magsgnLen:], nil
}

// GetData returns decoded data.
func (h *Decoder) GetData() []int32 {
	return h.data
}

// DecodeWithBitplane implements BlockDecoder interface.
func (h *Decoder) DecodeWithBitplane(data []byte, numPasses int, maxBitplane int, _ int) error {
	h.maxBitplane = maxBitplane
	_, err := h.Decode(data, numPasses)
	return err
}

// DecodeLayered implements BlockDecoder interface.
func (h *Decoder) DecodeLayered(data []byte, passLengths []int, maxBitplane int, _ int) error {
	h.maxBitplane = maxBitplane
	numPasses := len(passLengths)
	if numPasses == 0 {
		numPasses = 1
	}
	_, err := h.Decode(data, numPasses)
	return err
}

// SetCodingContext receives packet/QCD coding state for HTJ2K cleanup decoding.
func (h *Decoder) SetCodingContext(bandNumbps int, zeroBitplanes int) {
	h.bandNumbps = bandNumbps
	h.zeroBitplanes = zeroBitplanes
}

// SetIrreversible selects OpenJPH's irreversible code-block transfer, which
// preserves the full reconstructed magnitude for float dequantization.
func (h *Decoder) SetIrreversible(irreversible bool) {
	h.irreversible = irreversible
}

// Reset resets decoder.
func (h *Decoder) Reset() {
	for i := range h.data {
		h.data[i] = 0
	}
}
