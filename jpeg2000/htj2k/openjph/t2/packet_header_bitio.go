package t2

import "bytes"

// bioWriter implements OpenJPEG-style bit I/O with 0xFF bit-stuffing.
// When the last written byte is 0xFF, the next byte only has 7 usable bits.
type bioWriter struct {
	buf bytes.Buffer
	out uint16
	ct  int
}

func newBioWriter() *bioWriter {
	return &bioWriter{ct: 8}
}

func (bw *bioWriter) writeBit(bit int) {
	if bw.ct == 0 {
		bw.byteOut()
	}
	bw.ct--
	if bit != 0 {
		bw.out |= 1 << bw.ct
	}
}

// WriteBit satisfies the BitWriter interface used by tag-tree encoding.
func (bw *bioWriter) WriteBit(bit int) error {
	bw.writeBit(bit)
	return nil
}

func (bw *bioWriter) writeBits(value, n int) {
	for i := n - 1; i >= 0; i-- {
		bw.writeBit((value >> i) & 1)
	}
}

func (bw *bioWriter) flush() []byte {
	bw.byteOut()
	if bw.ct == 7 {
		bw.byteOut()
	}
	return bw.buf.Bytes()
}

func (bw *bioWriter) byteOut() {
	bw.out = (bw.out << 8) & 0xffff
	if bw.out == 0xff00 {
		bw.ct = 7
	} else {
		bw.ct = 8
	}
	bw.buf.WriteByte(byte(bw.out >> 8))
}

// bioReader implements OpenJPEG-style bit I/O with 0xFF bit-stuffing.
type bioReader struct {
	data []byte
	pos  int
	buf  uint16
	ct   int
}

func newBioReader(data []byte) *bioReader {
	return &bioReader{data: data, ct: 0}
}

func (br *bioReader) readBit() (int, error) {
	if br.ct == 0 {
		if err := br.byteIn(); err != nil {
			return 0, err
		}
	}
	br.ct--
	return int((br.buf >> br.ct) & 1), nil
}

func (br *bioReader) readBits(n int) (int, error) {
	if n <= 0 || n > 32 {
		return 0, errInvalidBitCount
	}
	v := 0
	for i := 0; i < n; i++ {
		bit, err := br.readBit()
		if err != nil {
			return 0, err
		}
		v = (v << 1) | bit
	}
	return v, nil
}

func (br *bioReader) alignToByte() error {
	if br.ct == 0 {
		return nil
	}
	// If last byte was 0xFF, consume the stuffed bit as in opj_bio_inalign.
	if (br.buf & 0xff) == 0xff {
		if err := br.byteIn(); err != nil {
			return err
		}
	}
	br.ct = 0
	return nil
}

func (br *bioReader) bytesRead() int {
	return br.pos
}

func (br *bioReader) byteIn() error {
	if br.pos >= len(br.data) {
		return errEndOfData
	}
	br.buf = (br.buf << 8) & 0xffff
	if br.buf == 0xff00 {
		br.ct = 7
	} else {
		br.ct = 8
	}
	br.buf |= uint16(br.data[br.pos])
	br.pos++
	return nil
}

var errEndOfData = ioEOF{}

type ioEOF struct{}

func (ioEOF) Error() string { return "end of data" }

var errInvalidBitCount = invalidBitCount{}

type invalidBitCount struct{}

func (invalidBitCount) Error() string { return "invalid number of bits" }

func floorLog2(n int) int {
	if n <= 1 {
		return 0
	}
	r := 0
	for n > 1 {
		n >>= 1
		r++
	}
	return r
}
