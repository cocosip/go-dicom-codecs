package lossless

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func TestNewLosslessParametersMatchesOpenJPEGDefaults(t *testing.T) {
	params := NewLosslessParameters()

	if params.Rate != 20 {
		t.Fatalf("Rate = %d, want 20", params.Rate)
	}
	if !params.AppendLosslessLayer {
		t.Fatal("AppendLosslessLayer = false, want true")
	}
	if params.NumLayers != 1 {
		t.Fatalf("NumLayers = %d, want 1 before rate-derived expansion", params.NumLayers)
	}
}

func TestConfigureLosslessEncodeParamsExpandsOpenJPEGLayers(t *testing.T) {
	codec := NewCodec()
	params := NewLosslessParameters()
	frameInfo := &imagetypes.FrameInfo{
		Width:           852,
		Height:          1100,
		BitsAllocated:   8,
		BitsStored:      8,
		SamplesPerPixel: 1,
	}

	encParams := codec.configureLosslessEncodeParams(frameInfo, params)

	if encParams.TargetRatio != 20 {
		t.Fatalf("TargetRatio = %v, want 20", encParams.TargetRatio)
	}
	if !encParams.UsePCRDOpt {
		t.Fatal("UsePCRDOpt = false, want true")
	}
	if !encParams.AppendLosslessLayer {
		t.Fatal("AppendLosslessLayer = false, want true")
	}
	if encParams.NumLayers != 8 {
		t.Fatalf("NumLayers = %d, want 8", encParams.NumLayers)
	}
	wantLayerRates := []float64{1280, 640, 320, 160, 80, 40, 20, 0}
	if len(encParams.LayerRates) != len(wantLayerRates) {
		t.Fatalf("LayerRates len = %d, want %d (%v)", len(encParams.LayerRates), len(wantLayerRates), encParams.LayerRates)
	}
	for i := range wantLayerRates {
		if encParams.LayerRates[i] != wantLayerRates[i] {
			t.Fatalf("LayerRates[%d] = %v, want %v (all %v)", i, encParams.LayerRates[i], wantLayerRates[i], encParams.LayerRates)
		}
	}
}

func TestConfigureLosslessEncodeParamsScalesFinalRateForStoredBits(t *testing.T) {
	codec := NewCodec()
	params := NewLosslessParameters()
	frameInfo := &imagetypes.FrameInfo{
		Width:           288,
		Height:          288,
		BitsAllocated:   16,
		BitsStored:      12,
		SamplesPerPixel: 1,
	}

	encParams := codec.configureLosslessEncodeParams(frameInfo, params)

	if encParams.TargetRatio != 15 {
		t.Fatalf("TargetRatio = %v, want 15", encParams.TargetRatio)
	}
	wantLayerRates := []float64{1280, 640, 320, 160, 80, 40, 15, 0}
	if len(encParams.LayerRates) != len(wantLayerRates) {
		t.Fatalf("LayerRates len = %d, want %d (%v)", len(encParams.LayerRates), len(wantLayerRates), encParams.LayerRates)
	}
	for i := range wantLayerRates {
		if encParams.LayerRates[i] != wantLayerRates[i] {
			t.Fatalf("LayerRates[%d] = %v, want %v (all %v)", i, encParams.LayerRates[i], wantLayerRates[i], encParams.LayerRates)
		}
	}
}
