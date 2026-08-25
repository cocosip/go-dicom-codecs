package codestream

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"testing"
)

func TestBoundedReaderRejectsTruncatedUint32WithoutAdvancing(t *testing.T) {
	reader := newBoundedReader([]byte{0x01, 0x02, 0x03})
	if _, err := reader.readUint32(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("readUint32 error = %v, want io.ErrUnexpectedEOF", err)
	}
	if reader.offset != 0 {
		t.Fatalf("offset = %d, want 0 after rejected read", reader.offset)
	}
}

func TestWriteTilePartExactBytes(t *testing.T) {
	var got bytes.Buffer
	err := WriteTilePart(&got, TilePart{
		TileIndex: 3,
		PartIndex: 1,
		PartCount: 2,
		Header:    []byte{0xff, 0x5e, 0x00, 0x04},
		Data:      []byte{0x11, 0x22, 0xff, 0x33},
	})
	if err != nil {
		t.Fatalf("WriteTilePart: %v", err)
	}
	want, err := hex.DecodeString("ff90000a0003000000160102ff5e0004ff931122ff33")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("tile-part bytes = %x, want %x", got.Bytes(), want)
	}
}

func TestWriteTilePartRejectsOutOfRangeFields(t *testing.T) {
	tests := []TilePart{
		{TileIndex: 1 << 16},
		{PartIndex: 1 << 8},
		{PartCount: 1 << 8},
	}
	for _, part := range tests {
		var dst bytes.Buffer
		if err := WriteTilePart(&dst, part); err == nil {
			t.Fatalf("WriteTilePart(%+v) succeeded", part)
		}
		if dst.Len() != 0 {
			t.Fatalf("WriteTilePart(%+v) wrote %d bytes before validation", part, dst.Len())
		}
	}
}

func TestWriteTilePartPropagatesShortWrite(t *testing.T) {
	err := WriteTilePart(shortWriter{}, TilePart{Data: []byte{1, 2, 3}})
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("WriteTilePart error = %v, want io.ErrShortWrite", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	return len(data) - 1, nil
}
