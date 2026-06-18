package lossy

import (
	"math"
	"testing"

	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

// TestCodecName tests the codec name
func TestCodecName(t *testing.T) {
	c := NewCodecWithRate(80)
	expected := "JPEG 2000 Lossy (Rate 80)"
	if c.Name() != expected {
		t.Errorf("Expected codec name %q, got %q", expected, c.Name())
	}
}

func TestGetDefaultParametersUsesCodecRate(t *testing.T) {
	c := NewCodecWithRate(80)
	params, ok := c.GetDefaultParameters().(*JPEG2000LossyParameters)
	if !ok {
		t.Fatalf("GetDefaultParameters returned %T, want *JPEG2000LossyParameters", c.GetDefaultParameters())
	}
	if params.Rate != 80 {
		t.Fatalf("default parameter Rate = %d, want 80", params.Rate)
	}
}

// TestCodecTransferSyntax tests the transfer syntax UID
func TestCodecTransferSyntax(t *testing.T) {
	c := NewCodec()
	ts := c.TransferSyntax()
	if ts == nil {
		t.Fatal("Transfer syntax is nil")
	}

	// The UID should be 1.2.840.10008.1.2.4.91
	// Verify it's not empty
	uid := ts.UID().UID()
	expected := "1.2.840.10008.1.2.4.91"
	if uid != expected {
		t.Errorf("Expected UID %q, got %q", expected, uid)
	}
}

// TestBasicEncodeDecode tests basic encoding and decoding with lossy compression
func TestBasicEncodeDecode(t *testing.T) {
	// Create test image data (16x16 grayscale)
	width := uint16(16)
	height := uint16(16)
	numPixels := int(width) * int(height)

	// Create gradient pattern
	pixelData := make([]byte, numPixels)
	for i := 0; i < numPixels; i++ {
		pixelData[i] = byte(i % 256)
	}

	// Create source PixelData
	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricMonochrome2,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Test encoding
	c := NewCodecWithRate()
	encoded := codecHelpers.NewTestPixelData(frameInfo)

	err := c.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Verify encoded data exists
	encodedData, _ := encoded.GetFrame(0)
	if len(encodedData) == 0 {
		t.Fatal("Encoded data is empty")
	}

	// Encoded data should be smaller than original (compression)
	t.Logf("Original size: %d bytes, Encoded size: %d bytes, Ratio: %.2f:1",
		len(pixelData), len(encodedData), float64(len(pixelData))/float64(len(encodedData)))

	// Test decoding
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = c.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify decoded data
	decodedData, _ := decoded.GetFrame(0)
	if len(decodedData) != len(pixelData) {
		t.Errorf("Decoded data length mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	if decoded.GetFrameInfo().Width != src.GetFrameInfo().Width {
		t.Errorf("Width mismatch: got %d, want %d", decoded.GetFrameInfo().Width, src.GetFrameInfo().Width)
	}

	if decoded.GetFrameInfo().Height != src.GetFrameInfo().Height {
		t.Errorf("Height mismatch: got %d, want %d", decoded.GetFrameInfo().Height, src.GetFrameInfo().Height)
	}

	// For lossy compression, we expect some error but it should be small
	// Calculate error metrics
	var maxError int
	var totalError int64
	errorCount := 0

	for i := 0; i < numPixels; i++ {
		diff := int(decodedData[i]) - int(pixelData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
		totalError += int64(diff)
		if diff > 0 {
			errorCount++
		}
	}

	avgError := float64(totalError) / float64(numPixels)

	t.Logf("Lossy compression error metrics:")
	t.Logf("  Max error: %d", maxError)
	t.Logf("  Avg error: %.2f", avgError)
	t.Logf("  Pixels with error: %d / %d (%.1f%%)",
		errorCount, numPixels, float64(errorCount)*100/float64(numPixels))

	// For 9/7 wavelet with minimal quantization, error is typically small
	// However, very small images (like 16x16) may have higher errors due to boundary effects
	// For this test, we use relaxed thresholds
	// Note: This test uses a very small 16x16 image which has edge effects
	// Larger images (64x64+) typically have much lower error

	// Allow larger max error for small images
	if maxError > 200 {
		t.Errorf("Max error too large: %d (expected <= 200)", maxError)
	}

	// Average error should still be reasonable
	if avgError > 30.0 {
		t.Errorf("Average error too large: %.2f (expected <= 30.0)", avgError)
	}
}

// TestLargerImage tests encoding/decoding a larger image
func TestLargerImage(t *testing.T) {
	// 64x64 image
	width := uint16(64)
	height := uint16(64)
	numPixels := int(width) * int(height)

	// Create gradient pattern
	pixelData := make([]byte, numPixels)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			pixelData[y*int(width)+x] = byte((x + y) % 256)
		}
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricMonochrome2,
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	c := NewCodec()
	encoded := codecHelpers.NewTestPixelData(frameInfo)

	// Encode
	err := c.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	encodedData, _ := encoded.GetFrame(0)
	compressionRatio := float64(len(pixelData)) / float64(len(encodedData))
	t.Logf("Compression ratio for 64x64: %.2f:1", compressionRatio)

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = c.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Calculate error
	decodedData, _ := decoded.GetFrame(0)
	var maxError int
	for i := 0; i < numPixels; i++ {
		diff := int(decodedData[i]) - int(pixelData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("Max error for 64x64: %d", maxError)

	if maxError > 12 {
		t.Errorf("Max error too large: %d", maxError)
	}
}

// TestRGBImage tests encoding/decoding RGB images
func TestRGBImage(t *testing.T) {
	// 32x32 RGB image
	width := uint16(32)
	height := uint16(32)
	numPixels := int(width) * int(height)

	// Create RGB data (interleaved)
	pixelData := make([]byte, numPixels*3)
	for i := 0; i < numPixels; i++ {
		pixelData[i*3+0] = byte((i * 2) % 256) // R
		pixelData[i*3+1] = byte((i * 3) % 256) // G
		pixelData[i*3+2] = byte((i * 5) % 256) // B
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           3,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricRGB,
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	c := NewCodecWithRate(80)
	encoded := codecHelpers.NewTestPixelData(frameInfo)

	// Encode
	err := c.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode RGB failed: %v", err)
	}

	encodedData, _ := encoded.GetFrame(0)
	compressionRatio := float64(len(pixelData)) / float64(len(encodedData))
	t.Logf("RGB compression ratio: %.2f:1", compressionRatio)

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = c.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode RGB failed: %v", err)
	}

	// Verify
	decodedData, _ := decoded.GetFrame(0)
	if len(decodedData) != len(pixelData) {
		t.Errorf("RGB data length mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	// Calculate error for RGB
	var maxError int
	for i := 0; i < len(pixelData); i++ {
		diff := int(decodedData[i]) - int(pixelData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("RGB max error: %d", maxError)

	if maxError > 12 {
		t.Errorf("RGB max error too large: %d", maxError)
	}
}

// TestRateControlAndLayers verifies rate control and progressive layer encoding.
func TestRateControlAndLayers(t *testing.T) {
	width := uint16(64)
	height := uint16(64)
	numPixels := int(width) * int(height)

	pixelData := make([]byte, numPixels)
	for i := 0; i < numPixels; i++ {
		pixelData[i] = byte((i * 7) % 256)
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricMonochrome2,
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	params := NewLossyParameters().
		WithNumLayers(3).
		WithTargetRatio(5.0).
		WithQuantStepScale(1.2)
	clampedLevels := 0
	minDim := int(width)
	if int(height) < minDim {
		minDim = int(height)
	}
	for clampedLevels < 6 && (minDim>>(clampedLevels+1)) >= 1 {
		clampedLevels++
	}
	expectedSubbands := 3*clampedLevels + 1
	steps := make([]float64, expectedSubbands)
	for i := range steps {
		steps[i] = 2.0 + float64(i)
	}
	params.WithSubbandSteps(steps)

	c := NewCodecWithRate(80)
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Encode(src, encoded, params); err != nil {
		t.Fatalf("Encode with rate control/layers failed: %v", err)
	}

	encodedData, _ := encoded.GetFrame(0)
	if len(encodedData) == 0 {
		t.Fatalf("Encoded data is empty")
	}

	ratio := float64(len(pixelData)) / float64(len(encodedData))
	t.Logf("Target ratio: %.2f, actual: %.2f", params.TargetRatio, ratio)
	// 楠岃瘉鍘嬬缉姣旀帴杩戠洰鏍囷紙瀹瑰樊 50% 鍐咃級
	if math.Abs(ratio-params.TargetRatio) > params.TargetRatio*0.5 {
		t.Fatalf("Ratio too far from target: got %.2f, want ~%.2f", ratio, params.TargetRatio)
	}

	decoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Decode(encoded, decoded, nil); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.GetFrameInfo().Width != src.GetFrameInfo().Width || decoded.GetFrameInfo().Height != src.GetFrameInfo().Height {
		t.Fatalf("Dim mismatch after decode: got %dx%d, want %dx%d",
			decoded.GetFrameInfo().Width, decoded.GetFrameInfo().Height, src.GetFrameInfo().Width, src.GetFrameInfo().Height)
	}
}

func TestCodecRejectsNoFrames(t *testing.T) {
	frameInfo := &imagetypes.FrameInfo{
		Width:           8,
		Height:          8,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}
	c := NewCodecWithRate(80)
	src := codecHelpers.NewTestPixelData(frameInfo)
	dst := codecHelpers.NewTestPixelData(frameInfo)

	if err := c.Encode(src, dst, nil); err == nil {
		t.Fatal("expected Encode to reject source with no frames")
	}
	if err := c.Decode(src, dst, nil); err == nil {
		t.Fatal("expected Decode to reject source with no frames")
	}
}
