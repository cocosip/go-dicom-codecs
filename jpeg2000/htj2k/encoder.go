package htj2k

import "fmt"

// HTEncoder implements the HTJ2K High-Throughput block encoder
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019
//
// HTJ2K replaces the standard EBCOT Tier-1 encoder with a high-throughput
// block coder that processes samples in 2x2 quads using three entropy coding tools:
// 1. MagSgn - Magnitude and sign bits
// 2. MEL - Adaptive run-length coding for quad significance
// 3. VLC - Variable-length coding for sample patterns with context and U-VLC
// HTEncoder encodes HTJ2K blocks using MagSgn, MEL, VLC and UVLC segments.
type HTEncoder struct {
	// Block dimensions
	width  int
	height int

	// Input data
	data []int32 // Wavelet coefficients

	// Encoding state
	roishift int
	kmax     int

	// Dimensions in quads
	qw int // width in quads
	qh int // height in quads
}

// NewHTEncoder creates a new HT block encoder
func NewHTEncoder(width, height int) *HTEncoder {
	qw := (width + 1) / 2
	qh := (height + 1) / 2

	enc := &HTEncoder{
		width:  width,
		height: height,
		qw:     qw,
		qh:     qh,
	}

	return enc
}

// SetKMax accepts the JPEG 2000 band Kmax value when supplied by the outer
// encoder. OpenJPH derives HT cleanup precision from this value.
func (h *HTEncoder) SetKMax(kmax int) {
	h.kmax = kmax
}

// Encode encodes a code-block using HTJ2K HT cleanup pass
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019
func (h *HTEncoder) Encode(data []int32, _ int, roishift int) ([]byte, error) {
	if len(data) != h.width*h.height {
		return nil, fmt.Errorf("data size mismatch: expected %d, got %d",
			h.width*h.height, len(data))
	}

	h.data = data
	h.roishift = roishift

	if h.kmax <= 0 {
		return nil, fmt.Errorf("HTJ2K OpenJPH cleanup encoding requires Kmax coding context")
	}
	return h.encodeOpenJPHCleanup(data)
}

// QuadInfo holds encoding information for a single quad
// Reference: ITU-T T.814 Annex C
type QuadInfo struct {
	Qx, Qy      int      // Quad position
	Samples     [4]int32 // Sample values [TL, BL, TR, BR]
	Significant [4]bool  // Significance flags
	Rho         uint8    // Significance pattern (0-15)
	EQ          [4]int   // Exponent for each sample
	MaxE        int      // Maximum exponent in quad
	Eps         uint8    // Exponent mask (4 bits)
	SigCount    int      // Number of significant samples
	MelBit      int      // MEL bit (0=all zero, 1=has significant)
}

func writeScupLocator(block []byte, scup int) {
	if len(block) < 2 {
		return
	}
	block[len(block)-1] = byte(scup >> 4)
	block[len(block)-2] = (block[len(block)-2] & 0xF0) | byte(scup&0x0F)
}
