package t2

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
)

type countingPacketBitWriter struct {
	writeBitsCalls int
}

func (w *countingPacketBitWriter) writeBit(int) {}

func (w *countingPacketBitWriter) writeBits(int, int) {
	w.writeBitsCalls++
}

func TestEncodeCodeBlockLengthsUsesOpenJPEGPassTermFlags(t *testing.T) {
	cb := &PrecinctCodeBlock{
		NumLenBits: 3,
		Passes: []t1.PassData{
			{Len: 3, Terminated: true},
			{Len: 4, Terminated: false},
		},
	}
	bw := &countingPacketBitWriter{}

	encodeCodeBlockLengths(bw, cb, 7, 0, 2, false, []int{3, 4})

	if bw.writeBitsCalls != 2 {
		t.Fatalf("OpenJPEG writes one length segment at pass.term and one at layer end, got %d", bw.writeBitsCalls)
	}
}
