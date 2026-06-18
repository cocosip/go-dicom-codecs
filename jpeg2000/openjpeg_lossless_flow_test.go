package jpeg2000

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
)

func TestOpenJPEGLosslessMultiLayerCODDoesNotEnableTERMALL(t *testing.T) {
	params := DefaultEncodeParams(64, 64, 1, 16, false)
	params.Lossless = true
	params.NumLayers = 8
	params.TargetRatio = 20

	var buf bytes.Buffer
	enc := NewEncoder(params)
	if err := enc.writeCOD(&buf); err != nil {
		t.Fatalf("writeCOD failed: %v", err)
	}

	data := buf.Bytes()
	const codeBlockStyleOffset = 12
	if len(data) <= codeBlockStyleOffset {
		t.Fatalf("COD segment too short: %d bytes", len(data))
	}
	if got := data[codeBlockStyleOffset]; got != 0x00 {
		t.Fatalf("classic OpenJPEG/fo-dicom lossless uses cblksty=0x00, got 0x%02X", got)
	}
}

func TestOpenJPEGVersionCOMMatchesFoDicomCodec(t *testing.T) {
	params := DefaultEncodeParams(8, 8, 1, 8, false)
	var buf bytes.Buffer

	if err := NewEncoder(params).writeVersionCOM(&buf); err != nil {
		t.Fatalf("writeVersionCOM failed: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 6 {
		t.Fatalf("COM marker too short: %d", len(data))
	}
	if marker := binary.BigEndian.Uint16(data[0:2]); marker != 0xff64 {
		t.Fatalf("marker = 0x%04X, want COM", marker)
	}
	if rcom := binary.BigEndian.Uint16(data[4:6]); rcom != 1 {
		t.Fatalf("Rcom = %d, want 1", rcom)
	}
	if got, want := string(data[6:]), "Created by OpenJPEG version 2.5.4"; got != want {
		t.Fatalf("COM = %q, want %q", got, want)
	}
}

func TestOpenJPEGLayerBudgetSubtractsMainHeaderBytes(t *testing.T) {
	params := DefaultEncodeParams(852, 1100, 1, 8, false)
	params.NumLayers = 8
	params.LayerRates = []float64{1280, 640, 320, 160, 80, 40, 20, 0}

	enc := NewEncoder(params)
	enc.openJPEGMainHeaderBytes = 119
	budgets := enc.openJPEGLayerBudgets(1_000_000)

	if got, want := budgets[0], 614.0; got != want {
		t.Fatalf("first layer budget = %.0f, want %.0f", got, want)
	}
}

func TestOpenJPEGLossyQCDMatchesFoDicomDefaultSteps(t *testing.T) {
	params := DefaultEncodeParams(852, 1100, 1, 8, false)
	params.Lossless = false
	params.NumLayers = 7
	params.LayerRates = []float64{1280, 640, 320, 160, 80, 40, 20}

	var buf bytes.Buffer
	if err := NewEncoder(params).writeQCD(&buf); err != nil {
		t.Fatalf("writeQCD failed: %v", err)
	}

	got := buf.Bytes()
	want := []byte{
		0xff, 0x5c, 0x00, 0x23, 0x42,
		0x77, 0x20, 0x76, 0xf0, 0x76, 0xf0, 0x76, 0xc0,
		0x6f, 0x00, 0x6f, 0x00, 0x6e, 0xe0, 0x67, 0x50,
		0x67, 0x50, 0x67, 0x68, 0x50, 0x05, 0x50, 0x05,
		0x50, 0x47, 0x57, 0xd3, 0x57, 0xd3, 0x57, 0x62,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("QCD = %x, want %x", got, want)
	}
}

func TestOpenJPEGLossyQuantizationRoundsAfterT1FractionalScaling(t *testing.T) {
	params := DefaultEncodeParams(1, 1, 1, 8, false)
	params.Lossless = false
	enc := NewEncoder(params)
	out := make([]int32, 1)

	enc.quantizeSubbandFloat([]float32{0.49}, out, 0, 0, 1, 1, 1, 1.0)

	if got, want := out[0], int32(31); got != want {
		t.Fatalf("quantized coefficient = %d, want %d like OpenJPEG lrintf((coeff/step)*64)", got, want)
	}
}

func TestOpenJPEGLossyDistortionWeightUsesRuntimeStepAndDWTNorm(t *testing.T) {
	params := DefaultEncodeParams(64, 64, 1, 8, false)
	params.Lossless = false
	params.NumLevels = 5
	params.LayerRates = []float64{1280, 640, 320, 160, 80, 40, 20}
	enc := NewEncoder(params)
	blockEnc := enc.newCodeBlockEncoder(64, 64, 4, 1, 6)
	t1Enc, ok := blockEnc.(*t1.Encoder)
	if !ok {
		t.Fatalf("block encoder = %T, want *t1.Encoder", blockEnc)
	}
	data := make([]int32, 64*64)
	data[0] = 1 << t1NMSEDecFracBits
	passes, _, err := t1Enc.EncodeLayered(data, 1, 0, nil, 0x00)
	if err != nil {
		t.Fatalf("EncodeLayered failed: %v", err)
	}
	if len(passes) != 1 {
		t.Fatalf("passes = %d, want 1", len(passes))
	}

	quantParams := CalculateOpenJPEGQuantizationParams(params.NumLevels, params.BitDepth)
	steps := OpenJPEGRuntimeQuantizationSteps(quantParams.EncodedSteps, params.NumLevels, params.BitDepth)
	step := steps[subbandIndexForResolutionBand(params.NumLevels, 4, 1)]
	want := 8192.0 * openJPEGDistortionWeight(false, params.NumLevels-4, 1, step)

	if got := passes[0].Distortion; got != want {
		t.Fatalf("distortion = %.12g, want OpenJPEG weighted MSE %.12g", got, want)
	}
}
