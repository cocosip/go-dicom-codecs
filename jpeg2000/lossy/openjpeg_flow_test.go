package lossy

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func TestOpenJPEGLossyDefaultRateBuildsFoDicomLayerRates(t *testing.T) {
	frameInfo := &imagetypes.FrameInfo{
		Width:               852,
		Height:              1100,
		SamplesPerPixel:     1,
		BitsAllocated:       8,
		BitsStored:          8,
		PixelRepresentation: 0,
	}
	params := NewLossyParameters()
	params.Validate()
	encParams := jpeg2000.DefaultEncodeParams(
		int(frameInfo.Width),
		int(frameInfo.Height),
		int(frameInfo.SamplesPerPixel),
		int(frameInfo.BitsStored),
		false,
	)

	NewCodec().configureBasicEncodeParams(encParams, frameInfo, params, 0)

	wantRates := []float64{1280, 640, 320, 160, 80, 40, 20}
	if encParams.Lossless {
		t.Fatalf("lossy OpenJPEG flow must use irreversible 9/7, got Lossless=true")
	}
	if !encParams.UsePCRDOpt {
		t.Fatalf("lossy OpenJPEG flow must enable rate-distortion allocation")
	}
	if encParams.NumLayers != len(wantRates) {
		t.Fatalf("NumLayers = %d, want %d", encParams.NumLayers, len(wantRates))
	}
	if len(encParams.LayerRates) != len(wantRates) {
		t.Fatalf("LayerRates length = %d, want %d", len(encParams.LayerRates), len(wantRates))
	}
	for i, want := range wantRates {
		if got := encParams.LayerRates[i]; got != want {
			t.Fatalf("LayerRates[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestOpenJPEGLossyDefaultRateScalesFinalLayerByStoredBits(t *testing.T) {
	frameInfo := &imagetypes.FrameInfo{
		Width:               852,
		Height:              1100,
		SamplesPerPixel:     1,
		BitsAllocated:       16,
		BitsStored:          12,
		PixelRepresentation: 0,
	}
	params := NewLossyParameters()
	params.Validate()
	encParams := jpeg2000.DefaultEncodeParams(
		int(frameInfo.Width),
		int(frameInfo.Height),
		int(frameInfo.SamplesPerPixel),
		int(frameInfo.BitsStored),
		false,
	)

	NewCodec().configureBasicEncodeParams(encParams, frameInfo, params, 0)

	wantRates := []float64{1280, 640, 320, 160, 80, 40, 15}
	if encParams.NumLayers != len(wantRates) {
		t.Fatalf("NumLayers = %d, want %d", encParams.NumLayers, len(wantRates))
	}
	for i, want := range wantRates {
		if got := encParams.LayerRates[i]; got != want {
			t.Fatalf("LayerRates[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestTargetRatioDoesNotUseDefaultOpenJPEGLayerRates(t *testing.T) {
	frameInfo := &imagetypes.FrameInfo{
		Width:               64,
		Height:              64,
		SamplesPerPixel:     1,
		BitsAllocated:       8,
		BitsStored:          8,
		PixelRepresentation: 0,
	}
	params := NewLossyParameters().WithTargetRatio(5).WithNumLayers(3)
	encParams := jpeg2000.DefaultEncodeParams(
		int(frameInfo.Width),
		int(frameInfo.Height),
		int(frameInfo.SamplesPerPixel),
		int(frameInfo.BitsStored),
		false,
	)
	codec := NewCodecWithRate(80)

	codec.configureBasicEncodeParams(encParams, frameInfo, params, codec.initializeBaseQuality(params, params.TargetRatio))
	codec.configureTargetRatio(encParams, params, params.TargetRatio)

	if len(encParams.LayerRates) != 0 {
		t.Fatalf("LayerRates = %v, want none when explicit TargetRatio drives PCRD", encParams.LayerRates)
	}
	if !encParams.UsePCRDOpt || encParams.TargetRatio != 5 {
		t.Fatalf("target-ratio PCRD not configured: UsePCRDOpt=%v TargetRatio=%v", encParams.UsePCRDOpt, encParams.TargetRatio)
	}
	if encParams.NumLayers != 3 {
		t.Fatalf("NumLayers = %d, want explicit TargetRatio layers", encParams.NumLayers)
	}
}
