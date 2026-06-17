package htj2k

import (
	"fmt"
	"math/bits"
)

type ojphMELWriter struct {
	buf           []byte
	tmp           int
	remainingBits int
	run           int
	k             int
	threshold     int
}

func newOJPHMELWriter() *ojphMELWriter {
	return &ojphMELWriter{
		buf:           make([]byte, 0, 192),
		remainingBits: 8,
		threshold:     1,
	}
}

func (m *ojphMELWriter) encode(bit bool) {
	if !bit {
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

	m.emitBit(0)
	for t := MelE[m.k]; t > 0; {
		t--
		m.emitBit((m.run >> t) & 1)
	}
	m.run = 0
	if m.k > 0 {
		m.k--
	}
	m.threshold = 1 << MelE[m.k]
}

func (m *ojphMELWriter) emitBit(v int) {
	m.tmp = (m.tmp << 1) | (v & 1)
	m.remainingBits--
	if m.remainingBits == 0 {
		m.buf = append(m.buf, byte(m.tmp))
		if m.tmp == 0xFF {
			m.remainingBits = 7
		} else {
			m.remainingBits = 8
		}
		m.tmp = 0
	}
}

type ojphVLCWriter struct {
	buf               []byte
	usedBits          int
	tmp               int
	lastGreaterThan8F bool
}

func newOJPHVLCWriter() *ojphVLCWriter {
	return &ojphVLCWriter{
		buf:               []byte{0xFF},
		usedBits:          4,
		tmp:               0xF,
		lastGreaterThan8F: true,
	}
}

func (v *ojphVLCWriter) encode(cwd, cwdLen int) {
	for cwdLen > 0 {
		availBits := 8
		if v.lastGreaterThan8F {
			availBits--
		}
		availBits -= v.usedBits

		t := minInt(availBits, cwdLen)
		v.tmp |= (cwd & ((1 << t) - 1)) << v.usedBits
		v.usedBits += t
		availBits -= t
		cwdLen -= t
		cwd >>= t

		if availBits == 0 {
			if v.lastGreaterThan8F && v.tmp != 0x7F {
				v.lastGreaterThan8F = false
				continue
			}
			v.buf = append(v.buf, byte(v.tmp))
			v.lastGreaterThan8F = v.tmp > 0x8F
			v.tmp = 0
			v.usedBits = 0
		}
	}
}

func (v *ojphVLCWriter) bytes() []byte {
	out := make([]byte, 0, len(v.buf))
	for i := len(v.buf) - 1; i >= 1; i-- {
		out = append(out, v.buf[i])
	}
	out = append(out, v.buf[0])
	return out
}

type ojphMSWriter struct {
	buf      []byte
	maxBits  int
	usedBits int
	tmp      uint32
}

func newOJPHMSWriter() *ojphMSWriter {
	return &ojphMSWriter{
		buf:     make([]byte, 0, 2048),
		maxBits: 8,
	}
}

func (m *ojphMSWriter) encode(cwd uint32, cwdLen int) {
	for cwdLen > 0 {
		t := minInt(m.maxBits-m.usedBits, cwdLen)
		m.tmp |= (cwd & ((1 << t) - 1)) << m.usedBits
		m.usedBits += t
		cwd >>= t
		cwdLen -= t
		if m.usedBits >= m.maxBits {
			b := byte(m.tmp)
			m.buf = append(m.buf, b)
			if b == 0xFF {
				m.maxBits = 7
			} else {
				m.maxBits = 8
			}
			m.tmp = 0
			m.usedBits = 0
		}
	}
}

func (m *ojphMSWriter) terminate() {
	if m.usedBits != 0 {
		t := m.maxBits - m.usedBits
		m.tmp |= uint32((0xFF & ((1 << t) - 1)) << m.usedBits)
		m.usedBits += t
		if byte(m.tmp) != 0xFF {
			m.buf = append(m.buf, byte(m.tmp))
		}
	} else if m.maxBits == 7 && len(m.buf) > 0 {
		m.buf = m.buf[:len(m.buf)-1]
	}
}

type ojphUVLCEntry struct {
	pre, preLen int
	suf, sufLen int
	ext, extLen int
}

func ojphUVLC(code int) ojphUVLCEntry {
	switch {
	case code <= 0:
		return ojphUVLCEntry{}
	case code == 1:
		return ojphUVLCEntry{pre: 1, preLen: 1}
	case code == 2:
		return ojphUVLCEntry{pre: 2, preLen: 2}
	case code <= 4:
		return ojphUVLCEntry{pre: 4, preLen: 3, suf: code - 3, sufLen: 1}
	case code <= 32:
		return ojphUVLCEntry{pre: 0, preLen: 3, suf: code - 5, sufLen: 5}
	default:
		return ojphUVLCEntry{
			pre:    0,
			preLen: 3,
			suf:    28 + ((code - 33) % 4),
			sufLen: 5,
			ext:    (code - 33) / 4,
			extLen: 4,
		}
	}
}

func (h *HTEncoder) encodeOpenJPHCleanup(data []int32) ([]byte, error) {
	if h.kmax <= 0 || h.kmax >= 31 {
		return nil, fmt.Errorf("invalid HTJ2K Kmax: %d", h.kmax)
	}

	cb := make([]uint32, len(data))
	shift := uint(31 - h.kmax)
	var maxVal uint32
	for i, v := range data {
		sign := uint32(0)
		mag := v
		if mag < 0 {
			sign = 0x80000000
			mag = -mag
		}
		val := uint32(mag) << shift
		cb[i] = sign | val
		maxVal |= val
	}
	if maxVal < (uint32(1) << shift) {
		return nil, nil
	}

	missingMSBs := h.kmax - 1
	p := uint(30 - missingMSBs)
	mel := newOJPHMELWriter()
	vlc := newOJPHVLCWriter()
	ms := newOJPHMSWriter()

	eVal := make([]uint8, (h.width+1)/2+2)
	cxVal := make([]uint8, (h.width+1)/2+2)

	h.encodeOJPHInitialRows(cb, p, mel, vlc, ms, eVal, cxVal)
	h.encodeOJPHSubsequentRows(cb, p, mel, vlc, ms, eVal, cxVal)

	melData, vlcData := terminateOJPHMELVLC(mel, vlc)
	ms.terminate()

	result := make([]byte, 0, len(ms.buf)+len(melData)+len(vlcData))
	result = append(result, ms.buf...)
	result = append(result, melData...)
	result = append(result, vlcData...)
	if len(melData)+len(vlcData) == 0 {
		return nil, fmt.Errorf("HTJ2K cleanup suffix is empty")
	}
	writeScupLocator(result, len(melData)+len(vlcData))
	return result, nil
}

func (h *HTEncoder) encodeOJPHInitialRows(cb []uint32, p uint, mel *ojphMELWriter, vlc *ojphVLCWriter, ms *ojphMSWriter, eVal, cxVal []uint8) {
	lep := 0
	lcxp := 0
	eVal[lep] = 0
	cxVal[lcxp] = 0
	cq0 := 0

	for x := 0; x < h.width; x += 4 {
		var eQMax [2]int
		var eQ [8]int
		var rho [2]int
		var s [8]uint32

		h.prepareOJPHInitialQuad(cb, p, x, 0, &rho[0], &eQMax[0], eQ[:], s[:])
		uq0 := maxInt(eQMax[0], 1)
		u0 := uq0 - 1
		eps0 := ojphEPS(eQ[:4], eQMax[0], u0)
		eVal[lep] = uint8(maxInt(int(eVal[lep]), eQ[1]))
		lep++
		eVal[lep] = uint8(eQ[3])
		cxVal[lcxp] |= uint8((rho[0] & 2) >> 1)
		lcxp++
		cxVal[lcxp] = uint8((rho[0] & 8) >> 3)

		tuple0 := ojphEncodeTuple(true, cq0, rho[0], eps0)
		vlc.encode(tuple0>>8, (tuple0>>4)&7)
		if cq0 == 0 {
			mel.encode(rho[0] != 0)
		}
		ojphEncodeMagSgn(ms, rho[0], uq0, tuple0, s[:4])

		u1 := 0
		if x+2 < h.width {
			h.prepareOJPHInitialQuad(cb, p, x+2, 4, &rho[1], &eQMax[1], eQ[:], s[:])
			cq1 := (rho[0] >> 1) | (rho[0] & 1)
			uq1 := maxInt(eQMax[1], 1)
			u1 = uq1 - 1
			eps1 := ojphEPS(eQ[4:8], eQMax[1], u1)
			eVal[lep] = uint8(maxInt(int(eVal[lep]), eQ[5]))
			lep++
			eVal[lep] = uint8(eQ[7])
			cxVal[lcxp] |= uint8((rho[1] & 2) >> 1)
			lcxp++
			cxVal[lcxp] = uint8((rho[1] & 8) >> 3)

			tuple1 := ojphEncodeTuple(true, cq1, rho[1], eps1)
			vlc.encode(tuple1>>8, (tuple1>>4)&7)
			if cq1 == 0 {
				mel.encode(rho[1] != 0)
			}
			ojphEncodeMagSgn(ms, rho[1], uq1, tuple1, s[4:8])
		}

		if u0 > 0 && u1 > 0 {
			mel.encode(minInt(u0, u1) > 2)
		}
		ojphEncodeInitialUVLC(vlc, u0, u1)
		cq0 = (rho[1] >> 1) | (rho[1] & 1)
	}
	eVal[lep+1] = 0
}

func (h *HTEncoder) encodeOJPHSubsequentRows(cb []uint32, p uint, mel *ojphMELWriter, vlc *ojphVLCWriter, ms *ojphMSWriter, eVal, cxVal []uint8) {
	for y := 2; y < h.height; y += 2 {
		lep := 0
		maxE := maxInt(int(eVal[lep]), int(eVal[lep+1])) - 1
		eVal[lep] = 0
		lcxp := 0
		cq0 := int(cxVal[lcxp]) + (int(cxVal[lcxp+1]) << 2)
		cxVal[lcxp] = 0

		for x := 0; x < h.width; x += 4 {
			var eQMax [2]int
			var eQ [8]int
			var rho [2]int
			var s [8]uint32

			h.prepareOJPHQuad(cb, p, x, y, 0, &rho[0], &eQMax[0], eQ[:], s[:])
			kappa := 1
			if rho[0]&(rho[0]-1) != 0 {
				kappa = maxInt(1, maxE)
			}
			uq0 := maxInt(eQMax[0], kappa)
			u0 := uq0 - kappa
			eps0 := ojphEPS(eQ[:4], eQMax[0], u0)
			eVal[lep] = uint8(maxInt(int(eVal[lep]), eQ[1]))
			lep++
			maxE = maxInt(int(eVal[lep]), int(eVal[lep+1])) - 1
			eVal[lep] = uint8(eQ[3])
			cxVal[lcxp] |= uint8((rho[0] & 2) >> 1)
			lcxp++
			cq1 := int(cxVal[lcxp]) + (int(cxVal[lcxp+1]) << 2)
			cxVal[lcxp] = uint8((rho[0] & 8) >> 3)

			tuple0 := ojphEncodeTuple(false, cq0, rho[0], eps0)
			vlc.encode(tuple0>>8, (tuple0>>4)&7)
			if cq0 == 0 {
				mel.encode(rho[0] != 0)
			}
			ojphEncodeMagSgn(ms, rho[0], uq0, tuple0, s[:4])

			u1 := 0
			if x+2 < h.width {
				h.prepareOJPHQuad(cb, p, x+2, y, 4, &rho[1], &eQMax[1], eQ[:], s[:])
				kappa = 1
				if rho[1]&(rho[1]-1) != 0 {
					kappa = maxInt(1, maxE)
				}
				cq1 |= ((rho[0] & 4) >> 1) | ((rho[0] & 8) >> 2)
				uq1 := maxInt(eQMax[1], kappa)
				u1 = uq1 - kappa
				eps1 := ojphEPS(eQ[4:8], eQMax[1], u1)
				eVal[lep] = uint8(maxInt(int(eVal[lep]), eQ[5]))
				lep++
				maxE = maxInt(int(eVal[lep]), int(eVal[lep+1])) - 1
				eVal[lep] = uint8(eQ[7])
				cxVal[lcxp] |= uint8((rho[1] & 2) >> 1)
				lcxp++
				cq0 = int(cxVal[lcxp]) + (int(cxVal[lcxp+1]) << 2)
				cxVal[lcxp] = uint8((rho[1] & 8) >> 3)

				tuple1 := ojphEncodeTuple(false, cq1, rho[1], eps1)
				vlc.encode(tuple1>>8, (tuple1>>4)&7)
				if cq1 == 0 {
					mel.encode(rho[1] != 0)
				}
				ojphEncodeMagSgn(ms, rho[1], uq1, tuple1, s[4:8])
			}

			ojphEncodeNonInitialUVLC(vlc, u0, u1)
			cq0 |= ((rho[1] & 4) >> 1) | ((rho[1] & 8) >> 2)
		}
	}
}

func (h *HTEncoder) prepareOJPHInitialQuad(cb []uint32, p uint, x, offset int, rho, eQMax *int, eQ []int, s []uint32) {
	h.prepareOJPHSample(cb, p, x, 0, offset, rho, eQMax, eQ, s)
	h.prepareOJPHSample(cb, p, x, 1, offset+1, rho, eQMax, eQ, s)
	h.prepareOJPHSample(cb, p, x+1, 0, offset+2, rho, eQMax, eQ, s)
	h.prepareOJPHSample(cb, p, x+1, 1, offset+3, rho, eQMax, eQ, s)
}

func (h *HTEncoder) prepareOJPHQuad(cb []uint32, p uint, x, y, offset int, rho, eQMax *int, eQ []int, s []uint32) {
	h.prepareOJPHSample(cb, p, x, y, offset, rho, eQMax, eQ, s)
	h.prepareOJPHSample(cb, p, x, y+1, offset+1, rho, eQMax, eQ, s)
	h.prepareOJPHSample(cb, p, x+1, y, offset+2, rho, eQMax, eQ, s)
	h.prepareOJPHSample(cb, p, x+1, y+1, offset+3, rho, eQMax, eQ, s)
}

func (h *HTEncoder) prepareOJPHSample(cb []uint32, p uint, x, y, idx int, rho, eQMax *int, eQ []int, s []uint32) {
	if x >= h.width || y >= h.height {
		return
	}
	t := cb[y*h.width+x]
	val := (t + t) >> p
	val &= ^uint32(1)
	if val == 0 {
		return
	}
	*rho += 1 << uint(idx%4)
	val--
	eQ[idx] = bits.Len32(val)
	if eQ[idx] > *eQMax {
		*eQMax = eQ[idx]
	}
	val--
	s[idx] = val + (t >> 31)
}

func ojphEPS(eQ []int, eQMax int, u int) int {
	if u <= 0 {
		return 0
	}
	eps := 0
	for i, v := range eQ {
		if v == eQMax {
			eps |= 1 << uint(i)
		}
	}
	return eps
}

func ojphEncodeTuple(initial bool, cq, rho, eps int) int {
	if rho == 0 && cq == 0 {
		return 0
	}
	if initial {
		entry := ojphEncoderVLCTable0[(cq<<8)|(rho<<4)|eps]
		return int(entry)
	}
	entry := ojphEncoderVLCTable1[(cq<<8)|(rho<<4)|eps]
	return int(entry)
}

var ojphEncoderVLCTable0 [2048]uint16
var ojphEncoderVLCTable1 [2048]uint16

func init() {
	initOJPHEncoderVLCTable(VLCTbl0, &ojphEncoderVLCTable0)
	initOJPHEncoderVLCTable(VLCTbl1, &ojphEncoderVLCTable1)
}

func initOJPHEncoderVLCTable(src []VLCEntry, dst *[2048]uint16) {
	var best *VLCEntry
	for i := 0; i < 2048; i++ {
		cq := i >> 8
		rho := (i >> 4) & 0xF
		eps := i & 0xF
		if (eps&rho) != eps || (rho == 0 && cq == 0) {
			dst[i] = 0
			continue
		}
		best = nil
		if eps != 0 {
			bestEK := -1
			for j := range src {
				entry := &src[j]
				if int(entry.CQ) == cq && int(entry.Rho) == rho && entry.UOff == 1 && (eps&int(entry.EK)) == int(entry.E1) {
					ones := bits.OnesCount8(entry.EK)
					if ones >= bestEK {
						best = entry
						bestEK = ones
					}
				}
			}
		} else {
			for j := range src {
				entry := &src[j]
				if int(entry.CQ) == cq && int(entry.Rho) == rho && entry.UOff == 0 {
					best = entry
					break
				}
			}
		}
		if best != nil {
			dst[i] = uint16(best.Cwd)<<8 | uint16(best.CwdLen)<<4 | uint16(best.EK)
		}
	}
}

func ojphEncodeMagSgn(ms *ojphMSWriter, rho, uq, tuple int, s []uint32) {
	for i := 0; i < 4; i++ {
		if rho&(1<<uint(i)) == 0 {
			continue
		}
		m := uq - ((tuple >> uint(i)) & 1)
		if m < 0 {
			m = 0
		}
		ms.encode(s[i]&((1<<uint(m))-1), m)
	}
}

func ojphEncodeInitialUVLC(vlc *ojphVLCWriter, u0, u1 int) {
	if u0 > 2 && u1 > 2 {
		c0 := ojphUVLC(u0 - 2)
		c1 := ojphUVLC(u1 - 2)
		vlc.encode(c0.pre, c0.preLen)
		vlc.encode(c1.pre, c1.preLen)
		vlc.encode(c0.suf, c0.sufLen)
		vlc.encode(c1.suf, c1.sufLen)
		return
	}
	if u0 > 2 && u1 > 0 {
		c0 := ojphUVLC(u0)
		vlc.encode(c0.pre, c0.preLen)
		vlc.encode(u1-1, 1)
		vlc.encode(c0.suf, c0.sufLen)
		return
	}
	c0 := ojphUVLC(u0)
	c1 := ojphUVLC(u1)
	vlc.encode(c0.pre, c0.preLen)
	vlc.encode(c1.pre, c1.preLen)
	vlc.encode(c0.suf, c0.sufLen)
	vlc.encode(c1.suf, c1.sufLen)
}

func ojphEncodeNonInitialUVLC(vlc *ojphVLCWriter, u0, u1 int) {
	c0 := ojphUVLC(u0)
	c1 := ojphUVLC(u1)
	vlc.encode(c0.pre, c0.preLen)
	vlc.encode(c1.pre, c1.preLen)
	vlc.encode(c0.suf, c0.sufLen)
	vlc.encode(c1.suf, c1.sufLen)
}

func terminateOJPHMELVLC(mel *ojphMELWriter, vlc *ojphVLCWriter) ([]byte, []byte) {
	if mel.run > 0 {
		mel.emitBit(1)
	}
	mel.tmp <<= mel.remainingBits
	melMask := (0xFF << mel.remainingBits) & 0xFF
	vlcMask := 0
	if vlc.usedBits > 0 {
		vlcMask = 0xFF >> (8 - vlc.usedBits)
	}
	if (melMask | vlcMask) == 0 {
		return mel.buf, vlc.bytes()
	}
	fuse := mel.tmp | vlc.tmp
	if (((fuse^mel.tmp)&melMask)|((fuse^vlc.tmp)&vlcMask)) == 0 && fuse != 0xFF && len(vlc.buf) > 1 {
		mel.buf = append(mel.buf, byte(fuse))
	} else {
		mel.buf = append(mel.buf, byte(mel.tmp))
		vlc.buf = append(vlc.buf, byte(vlc.tmp))
	}
	return mel.buf, vlc.bytes()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
