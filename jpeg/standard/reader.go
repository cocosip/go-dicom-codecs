package standard

import (
	"encoding/binary"
	"io"
)

// Reader provides utilities for reading JPEG data from an in-memory buffer.
type Reader struct {
	data []byte
	pos  int
}

// NewReader creates a new JPEG reader over data.
func NewReader(data []byte) *Reader {
	return &Reader{data: data}
}

// ReadByte reads a single byte.
func (r *Reader) ReadByte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

// ReadUint16 reads a 16-bit big-endian value.
func (r *Reader) ReadUint16() (uint16, error) {
	if r.pos+2 > len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.BigEndian.Uint16(r.data[r.pos:])
	r.pos += 2
	return v, nil
}

// ReadMarker reads the next JPEG marker.
// Returns the marker value (without the 0xFF prefix).
func (r *Reader) ReadMarker() (uint16, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != 0xFF {
		return 0, ErrInvalidMarker
	}

	// Skip any padding 0xFF bytes.
	for {
		b, err = r.ReadByte()
		if err != nil {
			return 0, err
		}
		if b != 0xFF {
			break
		}
	}

	// 0x00 is a stuffed byte (escaped 0xFF in data), not a marker.
	if b == 0x00 {
		return 0, ErrInvalidMarker
	}

	return uint16(0xFF00) | uint16(b), nil
}

// ReadSegment reads a segment with its length.
// Returns the segment data (without the length field).
func (r *Reader) ReadSegment() ([]byte, error) {
	length, err := r.ReadUint16()
	if err != nil {
		return nil, err
	}
	if length < 2 {
		return nil, ErrInvalidData
	}
	n := int(length) - 2
	if r.pos+n > len(r.data) {
		return nil, io.ErrUnexpectedEOF
	}
	segment := r.data[r.pos : r.pos+n]
	r.pos += n
	return segment, nil
}

// ReadFull reads exactly len(buf) bytes.
func (r *Reader) ReadFull(buf []byte) error {
	if r.pos+len(buf) > len(r.data) {
		return io.ErrUnexpectedEOF
	}
	copy(buf, r.data[r.pos:r.pos+len(buf)])
	r.pos += len(buf)
	return nil
}

// Skip skips n bytes.
func (r *Reader) Skip(n int) error {
	if n <= 0 {
		return nil
	}
	if r.pos+n > len(r.data) {
		return io.ErrUnexpectedEOF
	}
	r.pos += n
	return nil
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// ReadEntropySegment returns the entropy-coded bytes up to the next marker and
// leaves the reader positioned at that marker. Byte stuffing is preserved, so
// the Huffman decoder can remove it. Restart markers are dropped from the scan;
// segments that contain one are copied, the rest are returned as a view.
func (r *Reader) ReadEntropySegment() []byte {
	data := r.data
	start := r.pos
	i := start
	for i+1 < len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		next := data[i+1]
		if next == 0x00 {
			i += 2
			continue
		}
		if IsRST(uint16(0xFF00) | uint16(next)) {
			return r.copyEntropySegment(start, i)
		}
		r.pos = i
		return data[start:i]
	}
	r.pos = len(data)
	return data[start:]
}

// copyEntropySegment copies the scan bytes in [start, firstMarker), dropping
// any restart marker it meets.
func (r *Reader) copyEntropySegment(start, firstMarker int) []byte {
	data := r.data
	scan := make([]byte, 0, len(data)-start)
	scan = append(scan, data[start:firstMarker]...)
	i := firstMarker
	for i+1 < len(data) {
		marker := uint16(data[i])<<8 | uint16(data[i+1])
		if IsRST(marker) {
			i += 2
			continue
		}
		if data[i] == 0xFF && data[i+1] != 0x00 {
			break
		}
		scan = append(scan, data[i])
		i++
	}
	r.pos = i
	return scan
}

// Remaining returns the not-yet-read bytes.
func (r *Reader) Remaining() []byte {
	return r.data[r.pos:]
}
