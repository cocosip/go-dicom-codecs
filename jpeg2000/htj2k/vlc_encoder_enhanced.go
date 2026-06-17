package htj2k

// Enhanced VLC encoder functionality for HTJ2K block encoding
// This file provides additional methods for VLC encoder integration
// Reference: OpenJPH ojph_block_encoder.cpp

import (
	"fmt"
	"math/bits"
)

// VLCReverseWriter implements reverse (backward) bit writing for VLC segment
// In HTJ2K, the VLC segment is written from back to front and then reversed
// This matches OpenJPH's rev_struct behavior for encoding
type VLCReverseWriter struct {
	buf      []byte // Buffer written forwards, to be reversed
	pos      int    // Current position
	tmp      uint8  // Bit accumulator
	bits     int    // Number of bits in accumulator
	lastByte uint8  // Last byte written (for unstuffing check)
	isVLC    bool   // True for VLC segment, affects unstuffing rules
}

// NewVLCReverseWriter creates a new reverse VLC writer
func NewVLCReverseWriter() *VLCReverseWriter {
	return &VLCReverseWriter{
		buf:   make([]byte, 0, 4096),
		bits:  4,   // Start with 4 bits (0xF padding)
		tmp:   0xF, // Initialize with 0xF
		isVLC: true,
	}
}

// WriteBits writes bits to the VLC stream in reverse order
// Bits are accumulated LSB-first and flushed in reverse order
func (w *VLCReverseWriter) WriteBits(value uint32, numBits int) error {
	if numBits < 0 || numBits > 32 {
		return fmt.Errorf("invalid bit count: %d", numBits)
	}

	for numBits > 0 {
		// Take LSB from value
		bit := uint8(value & 1)
		value >>= 1
		numBits--

		// Add to accumulator
		w.tmp |= (bit << w.bits)
		w.bits++

		// Check for VLC unstuffing condition
		// If last byte > 0x8F and accumulator = 0x7F, insert stuffing bit
		if w.isVLC && w.lastByte > 0x8F && w.tmp == 0x7F {
			w.bits++ // Insert stuffing bit (automatically 0)
		}

		// Flush byte when full
		if w.bits >= 8 {
			w.buf = append(w.buf, w.tmp)
			w.pos++
			w.lastByte = w.tmp
			w.tmp = 0
			w.bits = 0
		}
	}

	return nil
}

// Flush finalizes the VLC stream and returns reversed bytes
// Returns the VLC segment ready for inclusion in codeblock
func (w *VLCReverseWriter) Flush() []byte {
	// Flush any remaining bits (pad with 1s)
	if w.bits > 0 {
		// Pad to byte boundary with 1s
		for w.bits < 8 {
			w.tmp |= (1 << w.bits)
			w.bits++
		}
		w.buf = append(w.buf, w.tmp)
	}

	// Add trailing 0xFF byte for decoder safety
	w.buf = append(w.buf, 0xFF)

	// Skip first initialization byte and return rest AS-IS (don't reverse)
	// OpenJPH writes VLC forward and decoder reads backward
	if len(w.buf) <= 1 {
		return []byte{}
	}

	result := make([]byte, len(w.buf)-1)
	copy(result, w.buf[1:])
	return result
}

// Reset resets the writer for a new block
func (w *VLCReverseWriter) Reset() {
	w.buf = w.buf[:0]
	w.pos = 0
	w.tmp = 0xF
	w.bits = 4
	w.lastByte = 0
}

// GetLength returns current written length in bytes
func (w *VLCReverseWriter) GetLength() int {
	length := len(w.buf)
	if w.bits > 0 {
		length++ // Count partial byte
	}
	return length
}

// EncodeQuadVLC encodes a single quad using VLC tables
// This is the main entry point for quad encoding
//
// Parameters:
//
//	qx, qy: Quad coordinates
//	rho: Significance pattern (4 bits)
//	ek, e1: EMB patterns
//	uOff: U-offset flag (1 if u != 0)
//	context: VLC context (0-7)
//	isFirstRow: True for initial row (y=0)
//	encoder: VLCEncoder to use
//
// Returns: Number of bits written, or error
func EncodeQuadVLC(_, _ int, rho, ek, e1, uOff, context uint8,
	isFirstRow bool, encoder *VLCEncoder) (int, error) {

	// Use encoder's context-aware encoding
	return encoder.EncodeCxtVLCWithLen(context, rho, uOff, ek, e1, isFirstRow)
}

// ComputeQuadEMB computes EMB patterns (ek, e1) for a quad
// Reference: ITU-T T.814 Annex C
//
// EMB (Exponent and Mantissa Bits) patterns indicate which samples
// in the quad have exponent bits (ek) and mantissa MSB bits (e1)
//
// Parameters:
//
//	samples: 4 sample values in quad [TL, BL, TR, BR]
//	rho: Significance pattern (determines which samples to check)
//
// Returns: ek (4 bits), e1 (4 bits)
func ComputeQuadEMB(samples [4]int32, rho uint8) (ek, e1 uint8) {
	for i := 0; i < 4; i++ {
		if (rho>>i)&1 == 0 {
			continue // Sample not significant
		}

		mag := uint32(samples[i])
		if samples[i] < 0 {
			mag = uint32(-samples[i])
		}

		if mag == 0 {
			continue
		}

		// Find magnitude exponent (position of MSB, 0-indexed)
		// For mag=7 (0b111), MSB is at position 2
		// bits.Len32(7) = 3, so exp = 3-1 = 2
		exp := bits.Len32(mag) - 1

		// ek bit indicates if exponent > 0
		if exp > 0 {
			ek |= (1 << i)
		}

		// e1 bit indicates if bit at position (exp-1) is set
		// For mag=7 (0b111), exp=2, check bit 1: (7>>1)&1 = 1
		if exp > 0 && ((mag>>(exp-1))&1) != 0 {
			e1 |= (1 << i)
		}
	}

	return ek, e1
}

// ComputeQuadRho computes significance pattern for a quad
// Returns 4-bit pattern where bit i indicates if sample i is non-zero
//
// Sample order matches OpenJPH bit order: [0]=TL, [1]=BL, [2]=TR, [3]=BR
func ComputeQuadRho(samples [4]int32) uint8 {
	rho := uint8(0)
	for i := 0; i < 4; i++ {
		if samples[i] != 0 {
			rho |= (1 << i)
		}
	}
	return rho
}

// ExtractQuadSamples extracts 4 samples for a quad from coefficient array.
// Returns samples in OpenJPH bit order: [TL, BL, TR, BR].
func ExtractQuadSamples(data []int32, width, qx, qy int) [4]int32 {
	samples := [4]int32{0, 0, 0, 0}

	// Top-left (TL)
	x0 := qx * 2
	y0 := qy * 2
	if x0 < width && y0*width+x0 < len(data) {
		samples[0] = data[y0*width+x0]
	}

	// Bottom-left (BL)
	y1 := y0 + 1
	if x0 < width && y1*width+x0 < len(data) {
		samples[1] = data[y1*width+x0]
	}

	// Top-right (TR)
	x1 := x0 + 1
	if x1 < width && y0*width+x1 < len(data) {
		samples[2] = data[y0*width+x1]
	}

	// Bottom-right (BR)
	if x1 < width && y1*width+x1 < len(data) {
		samples[3] = data[y1*width+x1]
	}

	return samples
}

// EncodeQuadPair encodes a pair of horizontally adjacent quads
// This is the basic unit of HTJ2K encoding
//
// Parameters:
//
//	qx: X-coordinate of first quad (must be even)
//	qy: Y-coordinate
//	data: Coefficient array
//	width: Block width
//	context: Context computer
//	vlcEnc: VLC encoder
//	melEnc: MEL encoder
//	msEnc: MagSgn encoder
//	expPred: Exponent predictor (optional, can be nil)
//	uvlcEnc: UVLC encoder (optional, can be nil)
//
// Returns: error if any
func EncodeQuadPair(qx, qy int, data []int32, width int,
	context *ContextComputer, vlcEnc *VLCEncoder,
	melEnc *MELEncoder, msEnc *MagSgnEncoder,
	expPred *ExponentPredictorComputer, uvlcEnc *UVLCEncoder) error {

	qw := (width + 1) / 2
	isFirstRow := (qy == 0)
	hasSecondQuad := (qx+1 < qw)

	s1, err := encodeQuadStats(qx, qy, data, width, isFirstRow, context, vlcEnc, melEnc, expPred)
	if err != nil {
		return err
	}
	var s2 qpQuadStats
	if hasSecondQuad {
		s2, err = encodeQuadStats(qx+1, qy, data, width, isFirstRow, context, vlcEnc, melEnc, expPred)
		if err != nil {
			return err
		}
	}
	if err := encodeQPUVLCPair(qx, qy, isFirstRow, hasSecondQuad, s1, s2, melEnc, uvlcEnc); err != nil {
		return err
	}
	encodeQPMagSgn(s1, msEnc)
	if hasSecondQuad {
		encodeQPMagSgn(s2, msEnc)
	}
	return nil
}

type qpQuadStats struct {
	samples  [4]int32
	rho      uint8
	sigCount int
	maxE     int
	eps0     uint8
	uOff     uint8
	tableEK  uint8
	Uq       int
	uq       int
}

func encodeQuadStats(qxi, qy int, data []int32, width int, isFirstRow bool,
	context *ContextComputer, vlcEnc *VLCEncoder, melEnc *MELEncoder, expPred *ExponentPredictorComputer) (qpQuadStats, error) {

	var s qpQuadStats
	s.samples = ExtractQuadSamples(data, width, qxi, qy)
	s.rho = ComputeQuadRho(s.samples)
	for i := 0; i < 4; i++ {
		if (s.rho>>i)&1 != 0 {
			s.sigCount++
			val := s.samples[i]
			mag := uint32(val)
			if val < 0 {
				mag = uint32(-val)
			}
			eQ := 0
			if mag > 0 {
				eQ = bits.Len32(mag)
			}
			if eQ > s.maxE {
				s.maxE = eQ
			}
		}
	}
	Kq := 0
	if expPred != nil {
		Kq = expPred.ComputePredictor(qxi, qy)
	}
	computeQPEps0AndUOff(&s, Kq)
	ctx := context.ComputeContext(qxi, qy, isFirstRow)
	if ctx == 0 {
		if s.rho == 0 {
			melEnc.EncodeBit(0)
		} else {
			melEnc.EncodeBit(1)
		}
	}
	shouldEncodeVLC := (ctx != 0) || (s.rho != 0)
	if shouldEncodeVLC {
		_, tableEK, err := vlcEnc.EncodeQuadVLCByEMB(ctx, s.rho, s.uOff, s.eps0, isFirstRow)
		if err != nil {
			return s, fmt.Errorf("VLC encode quad (%d,%d): %w", qxi, qy, err)
		}
		s.tableEK = tableEK
	}
	if s.rho != 0 {
		context.UpdateQuadSignificance(qxi, qy, s.rho)
	}
	if expPred != nil {
		expPred.SetQuadExponents(qxi, qy, s.maxE, s.sigCount)
	}
	return s, nil
}

func computeQPEps0AndUOff(s *qpQuadStats, Kq int) {
	if s.rho == 0 {
		return
	}
	s.Uq = s.maxE
	if Kq > s.Uq {
		s.Uq = Kq
	}
	s.uq = s.Uq - Kq
	if s.uq > 0 {
		for i := 0; i < 4; i++ {
			if (s.rho>>i)&1 != 0 {
				mag := uint32(s.samples[i])
				if s.samples[i] < 0 {
					mag = uint32(-s.samples[i])
				}
				eQ := 0
				if mag > 0 {
					eQ = bits.Len32(mag)
				}
				if eQ == s.maxE {
					s.eps0 |= (1 << i)
				}
			}
		}
		if s.eps0 > 0 {
			s.uOff = 1
		}
	}
}

func encodeQPUVLCPair(qx, qy int, isFirstRow, hasSecondQuad bool, s1, s2 qpQuadStats, melEnc *MELEncoder, uvlcEnc *UVLCEncoder) error {
	bothUOff := hasSecondQuad && s1.uOff == 1 && s2.uOff == 1
	if uvlcEnc != nil {
		uOff0 := s1.uOff
		uOff1 := uint8(0)
		if hasSecondQuad {
			uOff1 = s2.uOff
		}
		u0 := s1.uq
		u1 := 0
		if hasSecondQuad {
			u1 = s2.uq
		}
		melEvent := 0
		if isFirstRow && bothUOff {
			if !uvlcEnc.HasTableEntry(uOff0, uOff1, u0, u1, true, 0) {
				melEvent = 1
			}
			melEnc.EncodeBit(melEvent)
		}
		if err := uvlcEnc.EncodePair(uOff0, uOff1, u0, u1, isFirstRow, melEvent); err != nil {
			return fmt.Errorf("UVLC encode pair (%d,%d): %w", qx, qy, err)
		}
	} else if isFirstRow && bothUOff {
		melEnc.EncodeBit(0)
	}
	return nil
}

func encodeQPMagSgn(s qpQuadStats, msEnc *MagSgnEncoder) {
	if s.rho == 0 {
		return
	}
	for i := 0; i < 4; i++ {
		if (s.rho>>i)&1 != 0 {
			val := s.samples[i]
			mag := uint32(val)
			sign := 0
			if val < 0 {
				mag = uint32(-val)
				sign = 1
			}
			ekBit := int((s.tableEK >> i) & 1)
			mn := s.Uq - ekBit
			if mn < 0 {
				mn = 0
			}
			magLower := mag & ((1 << mn) - 1)
			msEnc.EncodeMagSgn(magLower, sign, mn)
		}
	}
}
