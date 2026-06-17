package htj2k

import (
	"fmt"
	"math/bits"
)

type ojphMELReader struct {
	data    []byte
	pos     int
	size    int
	tmp     uint64
	bitCnt  int
	unstuff bool
	k       int
	numRuns int
	runs    uint64
	inited  bool
	bitBuf  []uint8
}

func newOJPHMELReader(data []byte) *ojphMELReader {
	return &ojphMELReader{data: data, size: len(data) - 1}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (m *ojphMELReader) getRun() int {
	if !m.inited {
		m.init()
	}
	if m.numRuns == 0 {
		m.decodeMore()
	}
	if m.numRuns == 0 {
		return 1 << 30
	}
	run := int(m.runs & 0x7F)
	m.runs >>= 7
	m.numRuns--
	return run
}

func (m *ojphMELReader) init() {
	m.inited = true
}

func (m *ojphMELReader) decodeMore() {
	for m.numRuns < 8 {
		eval := MelE[m.k]
		run := 0
		lead := m.readBit()
		if lead == 1 {
			run = (1 << eval) - 1
			if m.k < 12 {
				m.k++
			}
			run <<= 1
		} else {
			if eval > 0 {
				for i := 0; i < eval; i++ {
					run = (run << 1) | int(m.readBit())
				}
			}
			if m.k > 0 {
				m.k--
			}
			run = (run << 1) + 1
		}
		shift := uint(m.numRuns * 7)
		m.runs &^= uint64(0x3F) << shift
		m.runs |= uint64(run) << shift
		m.numRuns++
	}
}

func (m *ojphMELReader) readBit() uint8 {
	for len(m.bitBuf) == 0 {
		if m.size <= 0 {
			return 1
		}
		d := byte(0xFF)
		if m.pos < len(m.data) {
			d = m.data[m.pos]
			m.pos++
			if m.size == 1 {
				d |= 0x0F
			}
			m.size--
		}
		validBits := 8
		if m.unstuff {
			validBits = 7
		}
		for i := validBits - 1; i >= 0; i-- {
			m.bitBuf = append(m.bitBuf, (d>>uint(i))&1)
		}
		m.unstuff = d == 0xFF
	}
	b := m.bitBuf[0]
	m.bitBuf = m.bitBuf[1:]
	return b
}

type ojphMSReader struct {
	dec *MagSgnDecoder
}

func newOJPHMSReader(data []byte) *ojphMSReader {
	return &ojphMSReader{dec: NewMagSgnDecoder(data)}
}

func (m *ojphMSReader) fetch(n int) uint32 {
	v, _ := m.dec.readBits(n)
	return uint32(v)
}

func decodeOpenJPHCleanup(codeblock []byte, width, height, kmax, missingMSBs int) ([]int32, error) {
	if len(codeblock) == 0 {
		return make([]int32, width*height), nil
	}
	if kmax <= 0 {
		return nil, fmt.Errorf("missing HTJ2K Kmax")
	}
	if missingMSBs < 0 {
		return nil, fmt.Errorf("invalid HTJ2K missing MSBs: %d", missingMSBs)
	}
	if missingMSBs >= 30 {
		return nil, fmt.Errorf("unsupported HTJ2K missing MSBs: %d", missingMSBs)
	}

	magsgnData, cleanupData, err := parseStandardSegments(codeblock)
	if err != nil {
		return nil, err
	}

	p := uint(30 - missingMSBs)
	sstr := ((width + 2) + 7) &^ 7
	scratch := make([]uint16, sstr*((height+1)/2+1)+8)

	mel := newOJPHMELReader(cleanupData)
	vlc := NewVLCDecoder(cleanupData)
	reverse := &reverseBitReader{data: cleanupData}
	vlc.reader = reverse
	vlc.reverse = reverse
	run := mel.getRun()

	cq := 0
	for x, sp := 0, 0; x < width; sp += 4 {
		vlcVal := vlc.readerPeek()
		t0 := VLCLookupTable0[cq+int(vlcVal&0x7F)]
		if cq == 0 {
			run -= 2
			if run != -1 {
				t0 = 0
			}
			if run < 0 {
				run = mel.getRun()
			}
		}
		scratch[sp] = uint16(t0)
		x += 2
		cq = int(((t0 & 0x10) << 3) | ((t0 & 0xE0) << 2))
		vlc.readerAdvance(int(t0 & 0x7))

		t1 := VLCLookupEntry(0)
		vlcVal = vlc.readerPeek()
		t1 = VLCLookupTable0[cq+int(vlcVal&0x7F)]
		if cq == 0 && x < width {
			run -= 2
			if run != -1 {
				t1 = 0
			}
			if run < 0 {
				run = mel.getRun()
			}
		}
		if x >= width {
			t1 = 0
		}
		scratch[sp+2] = uint16(t1)
		x += 2
		cq = int(((t1 & 0x10) << 3) | ((t1 & 0xE0) << 2))
		vlc.readerAdvance(int(t1 & 0x7))

		uvlcMode := int((t0&0x8)<<3) | int((t1&0x8)<<4)
		if uvlcMode == 0xC0 {
			run -= 2
			if run == -1 {
				uvlcMode += 0x40
			}
			if run < 0 {
				run = mel.getRun()
			}
		}
		u0, u1 := decodeOJPHUVLC(true, uvlcMode, vlc)
		scratch[sp+1] = uint16(1 + u0)
		scratch[sp+3] = uint16(1 + u1)
	}
	initialSentinel := ((width + 3) / 4) * 4
	scratch[initialSentinel+0] = 0
	scratch[initialSentinel+1] = 0

	for y := 2; y < height; y += 2 {
		cq := 0
		sp := (y >> 1) * sstr
		prev := sp - sstr
		for x := 0; x < width; sp += 4 {
			cq |= int((scratch[sp-sstr]&0xA0)<<2) | int((scratch[sp-sstr+2]&0x20)<<4)
			vlcVal := vlc.readerPeek()
			t0 := VLCLookupTable1[cq+int(vlcVal&0x7F)]
			if cq == 0 {
				run -= 2
				if run != -1 {
					t0 = 0
				}
				if run < 0 {
					run = mel.getRun()
				}
			}
			scratch[sp] = uint16(t0)
			x += 2
			cq = int((t0&0x40)<<2) | int((t0&0x80)<<1)
			cq |= int(scratch[sp-sstr] & 0x80)
			cq |= int((scratch[sp-sstr+2]&0xA0)<<2) | int((scratch[sp-sstr+4]&0x20)<<4)
			vlc.readerAdvance(int(t0 & 0x7))

			vlcVal = vlc.readerPeek()
			t1 := VLCLookupTable1[cq+int(vlcVal&0x7F)]
			if cq == 0 && x < width {
				run -= 2
				if run != -1 {
					t1 = 0
				}
				if run < 0 {
					run = mel.getRun()
				}
			}
			if x >= width {
				t1 = 0
			}
			scratch[sp+2] = uint16(t1)
			x += 2
			cq = int((t1&0x40)<<2) | int((t1&0x80)<<1)
			cq |= int(scratch[sp-sstr+2] & 0x80)
			vlc.readerAdvance(int(t1 & 0x7))

			uvlcMode := int((t0&0x8)<<3) | int((t1&0x8)<<4)
			u0, u1 := decodeOJPHUVLC(false, uvlcMode, vlc)
			scratch[sp+1] = uint16(u0)
			scratch[sp+3] = uint16(u1)
			_ = prev
		}
		scratch[sp] = 0
		scratch[sp+1] = 0
	}

	cb, err := decodeOJPHScratchMagSgn(magsgnData, scratch, width, height, sstr, p, missingMSBs)
	if err != nil {
		return nil, err
	}

	out := make([]int32, width*height)
	shift := uint(31 - kmax)
	for i, v := range cb {
		mag := int32((v & 0x7FFFFFFF) >> shift)
		if (v & 0x80000000) != 0 {
			mag = -mag
		}
		out[i] = mag
	}
	return out, nil
}

func decodeOJPHUVLC(initial bool, mode int, vlc *VLCDecoder) (int, int) {
	vlcVal := vlc.readerPeek()
	tableIndex := mode + int(vlcVal&0x3F)
	var entry UVLCDecodeEntry
	if initial {
		entry = UVLCTbl0[tableIndex]
	} else {
		entry = UVLCTbl1[tableIndex]
	}
	vlc.readerAdvance(entry.TotalPrefixLen())
	vlcVal = vlc.readerPeek()
	totalSuffix := entry.TotalSuffixLen()
	tmp := int(vlcVal & uint32((1<<uint(totalSuffix))-1))
	vlc.readerAdvance(totalSuffix)
	u0SuffixLen := entry.U0SuffixLen()
	u0 := entry.U0Prefix() + (tmp & ((1 << uint(u0SuffixLen)) - 1))
	u1 := entry.U1Prefix() + (tmp >> uint(u0SuffixLen))
	return u0, u1
}

func decodeOJPHScratchMagSgn(magsgnData []byte, scratch []uint16, width, height, sstr int, p uint, missingMSBs int) ([]uint32, error) {
	mmsbp2 := missingMSBs + 2
	ms := newOJPHMSReader(magsgnData)
	out := make([]uint32, width*height)
	vnScratch := make([]uint32, width+4)

	prevVN := uint32(0)
	sp := 0
	vp := 0
	for x := 0; x < width; sp += 2 {
		inf := uint32(scratch[sp])
		uq := int(scratch[sp+1])
		if uq > mmsbp2 {
			return nil, fmt.Errorf("HTJ2K U_q=%d exceeds missing_msbs+2=%d", uq, mmsbp2)
		}
		v0 := decodeOJPHSampleMS(ms, inf, uq, 0, p)
		out[x] = v0.val
		v1 := decodeOJPHSampleMS(ms, inf, uq, 1, p)
		if height > 1 {
			out[width+x] = v1.val
		}
		vnScratch[vp] = prevVN | v1.vn
		prevVN = 0
		x++
		vp++
		if x >= width {
			vp++
			break
		}
		v2 := decodeOJPHSampleMS(ms, inf, uq, 2, p)
		out[x] = v2.val
		v3 := decodeOJPHSampleMS(ms, inf, uq, 3, p)
		if height > 1 {
			out[width+x] = v3.val
		}
		prevVN = v3.vn
		x++
	}
	vnScratch[vp] = prevVN

	for y := 2; y < height; y += 2 {
		sp := (y >> 1) * sstr
		vp := 0
		prevVN = 0
		for x := 0; x < width; sp += 2 {
			inf := uint32(scratch[sp])
			uq := uint32(scratch[sp+1])
			gamma := inf & 0xF0
			gamma &= gamma - 0x10
			emax := uint32(bits.Len32((vnScratch[vp]|vnScratch[vp+1])|2) - 1)
			kappa := uint32(1)
			if gamma != 0 {
				kappa = emax
			}
			Uq := int(uq + kappa)
			if Uq > mmsbp2 {
				return nil, fmt.Errorf("HTJ2K U_q=%d exceeds missing_msbs+2=%d", Uq, mmsbp2)
			}
			v0 := decodeOJPHSampleMS(ms, inf, Uq, 0, p)
			out[y*width+x] = v0.val
			v1 := decodeOJPHSampleMS(ms, inf, Uq, 1, p)
			if y+1 < height {
				out[(y+1)*width+x] = v1.val
			}
			vnScratch[vp] = prevVN | v1.vn
			prevVN = 0
			x++
			vp++
			if x >= width {
				vp++
				break
			}
			v2 := decodeOJPHSampleMS(ms, inf, Uq, 2, p)
			out[y*width+x] = v2.val
			v3 := decodeOJPHSampleMS(ms, inf, Uq, 3, p)
			if y+1 < height {
				out[(y+1)*width+x] = v3.val
			}
			prevVN = v3.vn
			x++
		}
		vnScratch[vp] = prevVN
	}

	return out, nil
}

type ojphCleanupTrace struct {
	VLCVal   uint32
	T0       VLCLookupEntry
	T1       VLCLookupEntry
	UVLCMode int
	U0       int
	U1       int
}

func traceOpenJPHCleanupFirstPair(codeblock []byte) (ojphCleanupTrace, error) {
	_, cleanupData, err := parseStandardSegments(codeblock)
	if err != nil {
		return ojphCleanupTrace{}, err
	}
	mel := newOJPHMELReader(cleanupData)
	vlc := NewVLCDecoder(cleanupData)
	reverse := &reverseBitReader{data: cleanupData}
	vlc.reader = reverse
	vlc.reverse = reverse
	run := mel.getRun()
	vlcVal := vlc.readerPeek()
	t0 := VLCLookupTable0[int(vlcVal&0x7F)]
	run -= 2
	if run != -1 {
		t0 = 0
	}
	if run < 0 {
		run = mel.getRun()
	}
	cq := int(((t0 & 0x10) << 3) | ((t0 & 0xE0) << 2))
	vlc.readerAdvance(int(t0 & 0x7))
	t1 := VLCLookupTable0[cq+int(vlc.readerPeek()&0x7F)]
	if cq == 0 {
		run -= 2
		if run != -1 {
			t1 = 0
		}
	}
	vlc.readerAdvance(int(t1 & 0x7))
	uvlcMode := int((t0&0x8)<<3) | int((t1&0x8)<<4)
	if uvlcMode == 0xC0 {
		run -= 2
		if run == -1 {
			uvlcMode += 0x40
		}
		if run < 0 {
			run = mel.getRun()
		}
	}
	u0, u1 := decodeOJPHUVLC(true, uvlcMode, vlc)
	return ojphCleanupTrace{
		VLCVal:   vlcVal,
		T0:       t0,
		T1:       t1,
		UVLCMode: uvlcMode,
		U0:       u0,
		U1:       u1,
	}, nil
}

type ojphDecodedSample struct {
	val uint32
	vn  uint32
}

func decodeOJPHSampleMS(ms *ojphMSReader, inf uint32, uq int, bit int, p uint) ojphDecodedSample {
	if inf&(1<<uint(4+bit)) == 0 {
		return ojphDecodedSample{}
	}
	mn := uq - int((inf>>uint(12+bit))&1)
	msVal := ms.fetch(mn)
	val := msVal << 31
	vn := msVal & ((1 << uint(mn)) - 1)
	vn |= ((inf >> uint(8+bit)) & 1) << uint(mn)
	vn |= 1
	val |= (vn + 2) << (p - 1)
	return ojphDecodedSample{val: val, vn: vn}
}
