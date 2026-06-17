package htj2k

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
)

// HTBlockDecoder implements complete HTJ2K block decoding
// with proper context computation and VLC decoding
type HTBlockDecoder struct {
	width  int
	height int
	numQX  int // Number of quads in X direction
	numQY  int // Number of quads in Y direction

	// Component decoders
	mel    *MELDecoderSpec
	magsgn *MagSgnDecoder
	vlc    vlcQuadDecoder

	// Decoded coefficients
	data []int32
}

// NewHTBlockDecoder creates a new HTJ2K block decoder
func NewHTBlockDecoder(width, height int) *HTBlockDecoder {
	numQX := (width + 1) / 2 // Ceiling division
	numQY := (height + 1) / 2

	return &HTBlockDecoder{
		width:  width,
		height: height,
		numQX:  numQX,
		numQY:  numQY,
		data:   make([]int32, width*height),
	}
}

// DecodeBlock decodes an HTJ2K codeblock
// Returns the decoded coefficient data
func (h *HTBlockDecoder) DecodeBlock(codeblock []byte) ([]int32, error) {
	// Clear data array to ensure clean state
	for i := range h.data {
		h.data[i] = 0
	}

	// Parse codeblock into three segments
	if err := h.parseSegments(codeblock); err != nil {
		return nil, err
	}

	if h.mel == nil || h.vlc == nil || h.magsgn == nil {
		return h.data, nil
	}

	qp := NewQuadPairDecoderWithVLC(h.vlc, h.numQX, h.numQY)
	qp.SetMELDecoder(h.mel)
	pairs, err := qp.DecodeAllQuadPairs(h.numQY)
	if err != nil {
		if errors.Is(err, ErrInsufficientData) {
			return h.data, nil
		}
		return h.data, err
	}

	type quadInfo struct {
		rho      uint8
		uf       uint8
		uq       uint32
		e1       uint8
		ek       uint8
		sigCount int
	}

	quads := make([]quadInfo, h.numQX*h.numQY)
	pairsPerRow := (h.numQX + 1) / 2
	pairIdx := 0
	for qy := 0; qy < h.numQY; qy++ {
		for g := 0; g < pairsPerRow; g++ {
			pair := pairs[pairIdx]
			pairIdx++

			qx1 := 2 * g
			if qx1 < h.numQX {
				rho, uf, uq, e1, ek := GetQuadInfo(pair, 0)
				quads[qy*h.numQX+qx1] = quadInfo{
					rho:      rho,
					uf:       uf,
					uq:       uq,
					e1:       e1,
					ek:       ek,
					sigCount: bits.OnesCount8(rho),
				}
			}

			qx2 := qx1 + 1
			if pair.HasSecondQuad && qx2 < h.numQX {
				rho, uf, uq, e1, ek := GetQuadInfo(pair, 1)
				quads[qy*h.numQX+qx2] = quadInfo{
					rho:      rho,
					uf:       uf,
					uq:       uq,
					e1:       e1,
					ek:       ek,
					sigCount: bits.OnesCount8(rho),
				}
			}
		}
	}

	predictor := NewExponentPredictorComputer(h.numQX, h.numQY)
	for qy := 0; qy < h.numQY; qy++ {
		for qx := 0; qx < h.numQX; qx++ {
			info := quads[qy*h.numQX+qx]

			if info.rho == 0 {
				// All-zero quad: set exponent=0 and continue
				predictor.SetQuadExponents(qx, qy, 0, info.sigCount)
				continue
			}

			Kq := predictor.ComputePredictor(qx, qy)
			Uq := Kq + int(info.uq)
			if Uq < 0 {
				Uq = 0
			}

			maxE := 0
			sx := qx * 2
			sy := qy * 2
			positions := [][2]int{
				{sx, sy}, {sx, sy + 1},
				{sx + 1, sy}, {sx + 1, sy + 1},
			}

			for i, pos := range positions {
				if (info.rho>>i)&1 == 0 {
					continue
				}
				ekBit := int((info.ek >> i) & 1)
				e1Bit := uint32((info.e1 >> i) & 1)
				mn := Uq - ekBit
				if mn < 0 {
					mn = 0
				}

				mag, sign, ok := h.magsgn.DecodeMagSgn(mn)
				if !ok {
					mag = 0
					sign = 0
				}

				if e1Bit != 0 && mn < 32 {
					mag |= 1 << mn
				}

				if mag > 0 {
					exp := MagnitudeExponent(mag)
					if exp > maxE {
						maxE = exp
					}
				}

				coeff := int32(mag)
				if sign != 0 {
					coeff = -coeff
				}

				px, py := pos[0], pos[1]
				if px < h.width && py < h.height {
					h.data[py*h.width+px] = coeff
				}
			}

			predictor.SetQuadExponents(qx, qy, maxE, info.sigCount)
		}
	}

	return h.data, nil
}

// parseSegments parses the codeblock into MagSgn and cleanup suffix segments.
func (h *HTBlockDecoder) parseSegments(codeblock []byte) error {
	if len(codeblock) < 4 {
		// Empty or too small - all zeros
		return nil
	}

	magsgnData, cleanupData, err := parseStandardSegments(codeblock)
	if err == nil {
		h.magsgn = NewMagSgnDecoder(magsgnData)
		h.mel = NewMELDecoderSpec(cleanupData)
		h.vlc = NewVLCReverseDecoder(cleanupData)
		return nil
	}

	if magsgnData, melData, vlcData, ok := parseLegacySegments(codeblock); ok {
		h.magsgn = NewMagSgnDecoder(magsgnData)
		h.mel = NewMELDecoderSpec(melData)    // Reads forward through MEL portion
		h.vlc = NewVLCDecoderForward(vlcData) // Reads forward through VLC portion
		return nil
	}

	return fmt.Errorf("invalid HTJ2K code-block layout: %w", err)
}

// NewVLCDecoderForward creates a forward VLC decoder that implements vlcQuadDecoder.
// Used when VLC segment is stored in forward byte order (encoder writes forward).
func NewVLCDecoderForward(data []byte) vlcQuadDecoder {
	return NewVLCDecoder(data)
}

func parseLegacySegments(codeblock []byte) (magsgnData, melData, vlcData []byte, ok bool) {
	lcup := len(codeblock)
	if lcup < 4 {
		return nil, nil, nil, false
	}
	melLen := int(binary.LittleEndian.Uint16(codeblock[lcup-4 : lcup-2]))
	vlcLen := int(binary.LittleEndian.Uint16(codeblock[lcup-2 : lcup]))
	scup := melLen + vlcLen
	magsgnLen := lcup - 4 - scup
	if magsgnLen < 0 {
		return nil, nil, nil, false
	}
	if melLen == 0 && vlcLen == 0 {
		return codeblock[:magsgnLen], nil, nil, true
	}
	if scup == 0 || scup > lcup-4 {
		return nil, nil, nil, false
	}
	return codeblock[:magsgnLen],
		codeblock[magsgnLen : magsgnLen+melLen],
		codeblock[magsgnLen+melLen : magsgnLen+melLen+vlcLen],
		true
}

// NewVLCReverseDecoder creates a reverse VLC decoder for standard HTJ2K cleanup suffixes.
func NewVLCReverseDecoder(data []byte) vlcQuadDecoder {
	decoder := NewVLCDecoder(data)
	reverse := &reverseBitReader{data: data}
	decoder.reader = reverse
	decoder.reverse = reverse
	return decoder
}

func (v *VLCDecoder) readerPeek() uint32 {
	if v.reverse == nil {
		return 0
	}
	_ = v.reverse.readMore(32)
	return uint32(v.reverse.tmp)
}

func (v *VLCDecoder) readerAdvance(n int) {
	if v.reverse == nil || n <= 0 {
		return
	}
	_ = v.reverse.readMore(n)
	if n > v.reverse.num {
		v.reverse.tmp = 0
		v.reverse.num = 0
		return
	}
	v.reverse.tmp >>= uint(n)
	v.reverse.num -= n
}

// GetData returns the decoded coefficient data
func (h *HTBlockDecoder) GetData() []int32 {
	return h.data
}

// GetSample returns the decoded coefficient at (x, y)
func (h *HTBlockDecoder) GetSample(x, y int) int32 {
	if x < 0 || x >= h.width || y < 0 || y >= h.height {
		return 0
	}
	return h.data[y*h.width+x]
}
