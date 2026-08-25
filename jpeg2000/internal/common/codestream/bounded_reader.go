package codestream

import (
	"encoding/binary"
	"io"
)

type boundedReader struct {
	data   []byte
	offset int
}

func newBoundedReader(data []byte) *boundedReader {
	return &boundedReader{data: data}
}

func (r *boundedReader) readUint8() (uint8, error) {
	if len(r.data)-r.offset < 1 {
		return 0, io.ErrUnexpectedEOF
	}
	value := r.data[r.offset]
	r.offset++
	return value, nil
}

func (r *boundedReader) readUint16() (uint16, error) {
	if len(r.data)-r.offset < 2 {
		return 0, io.ErrUnexpectedEOF
	}
	value := binary.BigEndian.Uint16(r.data[r.offset : r.offset+2])
	r.offset += 2
	return value, nil
}

func (r *boundedReader) readUint32() (uint32, error) {
	if len(r.data)-r.offset < 4 {
		return 0, io.ErrUnexpectedEOF
	}
	value := binary.BigEndian.Uint32(r.data[r.offset : r.offset+4])
	r.offset += 4
	return value, nil
}

func (r *boundedReader) read(dst []byte) error {
	if len(r.data)-r.offset < len(dst) {
		return io.ErrUnexpectedEOF
	}
	copy(dst, r.data[r.offset:r.offset+len(dst)])
	r.offset += len(dst)
	return nil
}
