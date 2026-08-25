package codestream

import (
	"encoding/binary"
	"fmt"
	"io"
)

type boundedWriter struct {
	dst       io.Writer
	remaining uint64
}

func newBoundedWriter(dst io.Writer, limit uint64) *boundedWriter {
	return &boundedWriter{dst: dst, remaining: limit}
}

func (w *boundedWriter) write(data []byte) error {
	if uint64(len(data)) > w.remaining {
		return fmt.Errorf("codestream write of %d bytes exceeds remaining bound %d", len(data), w.remaining)
	}
	n, err := w.dst.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	w.remaining -= uint64(n)
	return nil
}

func (w *boundedWriter) writeUint8(value uint8) error {
	return w.write([]byte{value})
}

func (w *boundedWriter) writeUint16(value uint16) error {
	var data [2]byte
	binary.BigEndian.PutUint16(data[:], value)
	return w.write(data[:])
}

func (w *boundedWriter) writeUint32(value uint32) error {
	var data [4]byte
	binary.BigEndian.PutUint32(data[:], value)
	return w.write(data[:])
}
