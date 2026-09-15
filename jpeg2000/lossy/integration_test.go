package lossy

import (
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

// TestTypeSafeParametersIntegration tests that type-safe parameters work with codec
func TestTypeSafeParametersIntegration(t *testing.T) {
	// Create test data
	width := uint16(64)
	height := uint16(64)
	pixelData := make([]byte, int(width)*int(height))
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	frameInfo := &codec.FrameInfo{
		Width:  width,
		Height: height,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Test with type-safe parameters
	c := NewCodecWithRate(80)
	params := NewLossyParameters().WithRate(95).WithNumLevels(5)

	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := c.Encode(context.Background(), src, encoded, params)
	if err != nil {
		t.Fatalf("Encode with type-safe parameters failed: %v", err)
	}

	// Verify encoding worked
	encodedData, _ := encoded.Frame(context.Background(), 0)
	if len(encodedData) == 0 {
		t.Fatal("Encoded data is empty")
	}

	t.Logf("Encoded size: %d bytes", len(encodedData))
	t.Logf("Rate used: %d", params.Rate)
	t.Logf("Levels used: %d", params.NumLevels)

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = c.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify decoding
	decodedData, _ := decoded.Frame(context.Background(), 0)
	if len(decodedData) != len(pixelData) {
		t.Errorf("Decoded data length = %d, want %d", len(decodedData), len(pixelData))
	}
}

// TestBackwardCompatibility tests that old string-based parameters still work
func TestBackwardCompatibility(t *testing.T) {
	width := uint16(64)
	height := uint16(64)
	pixelData := make([]byte, int(width)*int(height))

	frameInfo := &codec.FrameInfo{
		Width:  width,
		Height: height,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	c := NewCodecWithRate(80)

	// Use string-based parameters (old way)
	params := NewLossyParameters()
	params.SetParameter("rate", 90)
	params.SetParameter("numLevels", 3)

	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := c.Encode(context.Background(), src, encoded, params)
	if err != nil {
		t.Fatalf("Encode with string-based parameters failed: %v", err)
	}

	// Verify the parameters were actually used
	usedRate := params.GetParameter("rate").(int)
	usedLevels := params.GetParameter("numLevels").(int)

	if usedRate != 90 {
		t.Errorf("Rate = %d, want 90", usedRate)
	}

	if usedLevels != 3 {
		t.Errorf("NumLevels = %d, want 3", usedLevels)
	}
}

// TestParametersMixedUsage tests mixing direct access and Get/Set methods
func TestParametersMixedUsage(t *testing.T) {
	params := NewLossyParameters()

	// Set via direct access
	params.Rate = 85

	// Read via GetParameter
	rate := params.GetParameter("rate")
	if rate != 85 {
		t.Errorf("GetParameter(rate) = %v, want 85", rate)
	}

	// Set via SetParameter
	params.SetParameter("numLevels", 4)

	// Read via direct access
	if params.NumLevels != 4 {
		t.Errorf("NumLevels = %d, want 4", params.NumLevels)
	}
}

// TestParametersImplementCodecContract verifies the v0.8 parameter ownership contract.
func TestParametersImplementCodecContract(t *testing.T) {
	parameters := codec.Parameters(NewLossyParameters().WithRate(95))
	if err := parameters.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	cloned, ok := parameters.Clone().(*JPEG2000LossyParameters)
	if !ok {
		t.Fatalf("Clone() type = %T, want *JPEG2000LossyParameters", cloned)
	}
	if cloned.Rate != 95 {
		t.Errorf("cloned Rate = %d, want 95", cloned.Rate)
	}
}

// TestNilParameters tests that nil parameters use defaults
func TestNilParameters(t *testing.T) {
	width := uint16(64)
	height := uint16(64)
	pixelData := make([]byte, int(width)*int(height))

	frameInfo := &codec.FrameInfo{
		Width:  width,
		Height: height,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	c := NewCodecWithRate(85) // Codec default rate

	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := c.Encode(context.Background(), src, encoded, nil) // nil parameters
	if err != nil {
		t.Fatalf("Encode with nil parameters failed: %v", err)
	}

	// Should use codec's default rate (85)
	encodedData, _ := encoded.Frame(context.Background(), 0)
	if len(encodedData) == 0 {
		t.Fatal("Encoded data is empty")
	}
}

// BenchmarkTypeSafeVsStringBased compares performance
func BenchmarkTypeSafeVsStringBased(b *testing.B) {
	b.Run("TypeSafe", func(b *testing.B) {
		params := NewLossyParameters()
		for i := 0; i < b.N; i++ {
			params.Rate = 95
			params.NumLevels = 5
			_ = params.Rate
			_ = params.NumLevels
		}
	})

	b.Run("StringBased", func(b *testing.B) {
		params := NewLossyParameters()
		for i := 0; i < b.N; i++ {
			params.SetParameter("rate", 95)
			params.SetParameter("numLevels", 5)
			_ = params.GetParameter("rate")
			_ = params.GetParameter("numLevels")
		}
	})
}
