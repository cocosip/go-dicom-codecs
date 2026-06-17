package htj2k

// Reverse VLC decoding for HTJ2K VLC segments that grow backward.
// Matches OpenJPH's rev_init/rev_fetch/rev_advance implementation:
// - Scup is packed into the last 12 bits of the cleanup suffix
// - VLC reading starts from the upper nibble of the penultimate byte
// - Bytes are read from end to start and consumed LSB-first from tmp

type reverseBitReader struct {
	data     []byte
	pos      int
	tmp      uint64
	num      int // number of valid bits in tmp
	unstuff  bool
	initDone bool
}

func (r *reverseBitReader) init() bool {
	if r.initDone {
		return true
	}
	r.initDone = true
	if len(r.data) < 2 {
		return false
	}

	r.pos = len(r.data) - 2
	d := r.data[r.pos]
	r.pos--

	r.tmp = uint64(d >> 4)
	r.num = 4
	if (r.tmp & 0x7) == 0x7 {
		r.num--
	}
	r.unstuff = (d | 0x0F) > 0x8F

	r.readChunk()
	return true
}

func (r *reverseBitReader) readChunk() {
	if r.num > 32 {
		return
	}
	var val uint32
	shift := 24
	for i := 0; i < 4 && r.pos >= 0; i++ {
		val |= uint32(r.data[r.pos]) << uint(shift)
		r.pos--
		shift -= 8
	}

	tmp := val >> 24
	bits := 8
	if r.unstuff && ((val>>24)&0x7F) == 0x7F {
		bits = 7
	}
	unstuff := (val >> 24) > 0x8F

	tmp |= ((val >> 16) & 0xFF) << uint(bits)
	if unstuff && ((val>>16)&0x7F) == 0x7F {
		bits += 7
	} else {
		bits += 8
	}
	unstuff = ((val >> 16) & 0xFF) > 0x8F

	tmp |= ((val >> 8) & 0xFF) << uint(bits)
	if unstuff && ((val>>8)&0x7F) == 0x7F {
		bits += 7
	} else {
		bits += 8
	}
	unstuff = ((val >> 8) & 0xFF) > 0x8F

	tmp |= (val & 0xFF) << uint(bits)
	if unstuff && (val&0x7F) == 0x7F {
		bits += 7
	} else {
		bits += 8
	}
	r.unstuff = (val & 0xFF) > 0x8F

	r.tmp |= uint64(tmp) << uint(r.num)
	r.num += bits
}

func (r *reverseBitReader) readMore(minBits int) bool {
	if !r.init() {
		return false
	}
	for r.num < minBits {
		if r.pos < 0 {
			break
		}
		r.readChunk()
	}
	return r.num >= minBits
}

func (r *reverseBitReader) readBits(n int) (uint32, bool) {
	if n == 0 {
		return 0, true
	}
	if !r.readMore(n) {
		return 0, false
	}
	mask := uint64((1 << uint(n)) - 1)
	val := uint32(r.tmp & mask)
	r.tmp >>= uint(n)
	r.num -= n
	return val, true
}

func (r *reverseBitReader) ReadBit() (uint8, error) {
	bit, ok := r.readBits(1)
	if !ok {
		return 0, ErrInsufficientData
	}
	return uint8(bit), nil
}

func (r *reverseBitReader) ReadBitsLE(n int) (uint32, error) {
	val, ok := r.readBits(n)
	if !ok {
		return 0, ErrInsufficientData
	}
	return val, nil
}
