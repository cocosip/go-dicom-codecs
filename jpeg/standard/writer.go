package standard

import (
	"encoding/binary"
	"io"
)

// Writer provides utilities for writing JPEG data
type Writer struct {
	w   io.Writer
	buf [2]byte
}

// NewWriter creates a new JPEG writer
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// WriteByte writes a single byte
func (w *Writer) WriteByte(b byte) error {
	w.buf[0] = b
	_, err := w.w.Write(w.buf[:1])
	return err
}

// WriteUint16 writes a 16-bit big-endian value
func (w *Writer) WriteUint16(v uint16) error {
	binary.BigEndian.PutUint16(w.buf[:2], v)
	_, err := w.w.Write(w.buf[:2])
	return err
}

// WriteMarker writes a JPEG marker
func (w *Writer) WriteMarker(marker uint16) error {
	return w.WriteUint16(marker)
}

// WriteSegment writes a segment with length
// The length field is automatically calculated and includes itself (2 bytes)
func (w *Writer) WriteSegment(marker uint16, data []byte) error {
	if err := w.WriteMarker(marker); err != nil {
		return err
	}

	// Length includes the 2 bytes for the length field itself
	length := uint16(len(data) + 2)
	if err := w.WriteUint16(length); err != nil {
		return err
	}

	_, err := w.w.Write(data)
	return err
}

// WriteJFIFAPP0 writes the JFIF application marker emitted by fo-dicom Native.
func (w *Writer) WriteJFIFAPP0() error {
	return w.WriteSegment(MarkerAPP0, []byte{
		'J', 'F', 'I', 'F', 0x00,
		0x01, 0x01, 0x00,
		0x00, 0x01, 0x00, 0x01,
		0x00, 0x00,
	})
}

// Write writes raw bytes
func (w *Writer) Write(data []byte) (int, error) {
	return w.w.Write(data)
}

// WriteBytes is an alias for Write
func (w *Writer) WriteBytes(data []byte) error {
	_, err := w.w.Write(data)
	return err
}
