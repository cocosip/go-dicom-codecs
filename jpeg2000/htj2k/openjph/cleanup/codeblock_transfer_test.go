package cleanup

import (
	"bytes"
	"reflect"
	"testing"
)

func TestOpenJPHIrreversibleCodeblockTransferDoesNotShiftQuantizedMagnitude(t *testing.T) {
	quantized := []int32{-569105408, -142276352, 0, 142276352, 564659264}
	want := []uint32{0xa1ebdc00, 0x887af700, 0, 0x087af700, 0x21a80440}

	got, maxValue := openJPHCodeblockWords(quantized, 22, true)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("code-block words = %08x, want %08x", got, want)
	}
	if maxValue != 0x29fbff40 {
		t.Fatalf("OR-reduced magnitude = %08x, want 29fbff40", maxValue)
	}
}

func TestOpenJPHCleanupEncoderSelectsIrreversibleCodeblockTransfer(t *testing.T) {
	reversibleData := make([]int32, 16)
	irreversibleData := make([]int32, 16)
	for i := range reversibleData {
		reversibleData[i] = 1
		irreversibleData[i] = 1 << 9
	}

	reversible := NewEncoder(4, 4)
	reversible.SetKMax(22)
	want, err := reversible.Encode(reversibleData, 1, 0)
	if err != nil {
		t.Fatalf("encode reversible reference: %v", err)
	}

	irreversible := NewEncoder(4, 4)
	irreversible.SetKMax(22)
	irreversible.SetIrreversible(true)
	got, err := irreversible.Encode(irreversibleData, 1, 0)
	if err != nil {
		t.Fatalf("encode irreversible block: %v", err)
	}

	if len(got) == 0 {
		t.Fatal("irreversible cleanup block is empty")
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("irreversible cleanup = % X, want reversible-equivalent % X", got, want)
	}
}

func TestOpenJPHCleanupDecoderPreservesIrreversibleBinCenterMagnitude(t *testing.T) {
	encoder := NewEncoder(1, 1)
	encoder.SetKMax(22)
	encoder.SetIrreversible(true)
	encoded, err := encoder.Encode([]int32{512}, 1, 0)
	if err != nil {
		t.Fatalf("encode irreversible cleanup block: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("irreversible cleanup block is empty")
	}

	decoder := NewDecoder(1, 1)
	decoder.SetCodingContext(22, 21)
	decoder.SetIrreversible(true)
	got, err := decoder.Decode(encoded, 1)
	if err != nil {
		t.Fatalf("decode irreversible cleanup block: %v", err)
	}

	// OpenJPH reconstructs the center of the encoded bin: (1 + 2) << 8.
	// The irreversible tx_from_cb32 path consumes this full magnitude.
	if got[0] != 768 {
		t.Fatalf("irreversible cleanup magnitude = %d, want 768", got[0])
	}
}
