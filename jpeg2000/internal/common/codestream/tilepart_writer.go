package codestream

import (
	"fmt"
	"io"
	"math"
)

// TilePart contains already-planned tile-part header and packet bytes.
// It owns no progression, packet, or codec-family policy.
type TilePart struct {
	TileIndex int
	PartIndex int
	PartCount int
	Header    []byte
	Data      []byte
}

// WriteTilePart writes a bounded SOT/header/SOD/data structure.
func WriteTilePart(dst io.Writer, part TilePart) error {
	if part.TileIndex < 0 || part.TileIndex > math.MaxUint16 {
		return fmt.Errorf("tile index %d is outside uint16 range", part.TileIndex)
	}
	if part.PartIndex < 0 || part.PartIndex > math.MaxUint8 {
		return fmt.Errorf("tile-part index %d is outside uint8 range", part.PartIndex)
	}
	if part.PartCount < 0 || part.PartCount > math.MaxUint8 {
		return fmt.Errorf("tile-part count %d is outside uint8 range", part.PartCount)
	}
	length := uint64(14) + uint64(len(part.Header)) + uint64(len(part.Data))
	if length > math.MaxUint32 {
		return fmt.Errorf("tile-part length %d is outside uint32 range", length)
	}

	writer := newBoundedWriter(dst, length)
	if err := writer.writeUint16(MarkerSOT); err != nil {
		return err
	}
	if err := writer.writeUint16(10); err != nil {
		return err
	}
	if err := writer.writeUint16(uint16(part.TileIndex)); err != nil {
		return err
	}
	if err := writer.writeUint32(uint32(length)); err != nil {
		return err
	}
	if err := writer.writeUint8(uint8(part.PartIndex)); err != nil {
		return err
	}
	if err := writer.writeUint8(uint8(part.PartCount)); err != nil {
		return err
	}
	if err := writer.write(part.Header); err != nil {
		return err
	}
	if err := writer.writeUint16(MarkerSOD); err != nil {
		return err
	}
	return writer.write(part.Data)
}
