package openjph

import (
	"bytes"
	"math"
	"reflect"
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k/openjph/cleanup"
)

func TestOpenJPHIrreversibleSubbandToCodeblockVectors(t *testing.T) {
	state := openJPHIrreversibleSubbandState(0xb718, 1, 0)
	if state.kmax != 22 {
		t.Fatalf("Kmax = %d, want 22", state.kmax)
	}
	if bits := math.Float32bits(state.delta); bits != 0x30718000 {
		t.Fatalf("delta bits = %08x, want 30718000", bits)
	}
	if bits := math.Float32bits(state.deltaInv); bits != 0x4e87af70 {
		t.Fatalf("delta_inv bits = %08x, want 4e87af70", bits)
	}

	coefficients := []float32{-0.5, -0.125, 0, 0.125, 0.49609375}
	wantQuantized := []int32{-569105408, -142276352, 0, 142276352, 564659264}
	if got := quantizeOpenJPHIrreversible(coefficients, state.deltaInv); !reflect.DeepEqual(got, wantQuantized) {
		t.Fatalf("quantized coefficients = %v, want %v", got, wantQuantized)
	}
	want := []uint32{0xa1ebdc00, 0x887af700, 0, 0x087af700, 0x21a80440}
	if got := openJPHIrreversibleCodeblockWords(coefficients, state.deltaInv); !reflect.DeepEqual(got, want) {
		t.Fatalf("code-block words = %08x, want %08x", got, want)
	}
}

func TestOpenJPHIrreversibleQuantizationMatchesNativeSIMDRounding(t *testing.T) {
	coefficient := math.Float32frombits(0x38e37023)
	deltaInv := math.Float32frombits(0x4e0951f0)
	want := []int32{62464}

	if got := quantizeOpenJPHIrreversible([]float32{coefficient}, deltaInv); !reflect.DeepEqual(got, want) {
		t.Fatalf("quantized accepted-range coefficient = %v, want %v", got, want)
	}
}

func TestOpenJPHIrreversibleQuantizationUsesPerSubbandQCDState(t *testing.T) {
	coefficients := []float32{
		-0.5, 0.5, -0.5, 0.5,
		0.5, -0.5, 0.5, -0.5,
		-0.5, 0.5, -0.5, 0.5,
		0.5, -0.5, 0.5, -0.5,
	}
	encodedSteps := []uint16{0xb718, 0xb6ea, 0xb6ea, 0xb6bc}
	want := []int32{
		-569105408, 569105408, -287981056, 287981056,
		569105408, -569105408, 287981056, -287981056,
		-287981056, 287981056, -145746512, 145746512,
		287981056, -287981056, 145746512, -145746512,
	}

	got := applyOpenJPHIrreversibleQuantization(coefficients, 4, 4, 0, 0, 1, encodedSteps, 1)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("quantized subbands = %v, want %v", got, want)
	}
}

func TestApplyIrreversibleWaveletTransformQuantizesWithoutDecomposition(t *testing.T) {
	encoder := &Encoder{params: &EncodeParams{
		BitDepth:  16,
		NumLevels: 0,
		Lossless:  false,
	}}
	input := [][]float32{{-0.5, 0.5}}
	want := [][]int32{{-1073741824, 1073741824}}

	got := encoder.applyIrreversibleWaveletTransform(input, 2, 1, 0, 0)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("irreversible coefficients = %v, want %v", got, want)
	}
}

func TestLossyEncoderConfiguresIrreversibleCleanupTransfer(t *testing.T) {
	reversibleData := make([]int32, 16)
	irreversibleData := make([]int32, 16)
	for i := range reversibleData {
		reversibleData[i] = 1
		irreversibleData[i] = 1 << 9
	}

	reference := cleanup.NewEncoder(4, 4)
	reference.SetKMax(22)
	want, err := reference.Encode(reversibleData, 1, 0)
	if err != nil {
		t.Fatalf("encode reversible reference: %v", err)
	}

	encoder := &Encoder{params: &EncodeParams{Lossless: false}}
	got, err := encoder.newCodeBlockEncoder(4, 4, 22).Encode(irreversibleData, 1, 0)
	if err != nil {
		t.Fatalf("encode lossy cleanup block: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("lossy cleanup = % X, want irreversible transfer bytes % X", got, want)
	}
}
