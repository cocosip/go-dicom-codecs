package standard

import (
	"errors"
	"testing"
)

var errWriteFailed = errors.New("write failed")

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errWriteFailed }

func TestHuffmanEncoderReportsWriteError(t *testing.T) {
	enc := NewHuffmanEncoder(errorWriter{})

	var err error
	for i := 0; i < 1<<17 && err == nil; i++ {
		err = enc.WriteBits(0xFF, 8)
	}
	if !errors.Is(err, errWriteFailed) {
		t.Fatalf("WriteBits error = %v, want %v", err, errWriteFailed)
	}
	if err := enc.Flush(); !errors.Is(err, errWriteFailed) {
		t.Fatalf("Flush error = %v, want %v", err, errWriteFailed)
	}
}
