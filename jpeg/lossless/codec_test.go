package lossless

import (
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

func TestLosslessCodecInterface(t *testing.T) {
	// Create codec
	losslessCodec := NewLosslessCodec(4) // Use predictor 4

	// Verify interface implementation
	var _ codec.Codec = losslessCodec

	// Test Name
	name := losslessCodec.Name()
	if name == "" {
		t.Error("Codec name should not be empty")
	}
	t.Logf("Codec name: %s", name)

	// Test TransferSyntax
	ts := losslessCodec.TransferSyntax()
	if ts == nil {
		t.Fatal("Transfer syntax should not be nil")
	}
	if ts.UID().UID() != transfer.JPEGLossless.UID().UID() {
		t.Errorf("Transfer syntax UID mismatch: got %s, want %s",
			ts.UID().UID(), transfer.JPEGLossless.UID().UID())
	}
}

func TestLosslessCodecEncodeDecode(t *testing.T) {
	// Create test pixel data
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y*2) % 256)
		}
	}

	// Create frame info
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	// Create source PixelData using helper
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Create codec with predictor 4
	losslessCodec := NewLosslessCodec(4)

	// Encode
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := losslessCodec.Encode(context.Background(), src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	encodedData, err := encoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get encoded frame: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(encodedData))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encodedData)))

	// Verify encoded data is not empty
	if len(encodedData) == 0 {
		t.Fatal("Encoded data is empty")
	}

	// Verify encoded data is smaller than original (should be compressed)
	if len(encodedData) >= len(pixelData) {
		t.Logf("Warning: Encoded data (%d bytes) is not smaller than original (%d bytes)",
			len(encodedData), len(pixelData))
	}

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = losslessCodec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Get decoded frame data
	decodedData, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get decoded frame: %v", err)
	}

	// Verify frame info
	decodedFrameInfo := decoded.FrameInfo()
	if decodedFrameInfo.Width != frameInfo.Width || decodedFrameInfo.Height != frameInfo.Height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d",
			decodedFrameInfo.Width, decodedFrameInfo.Height, frameInfo.Width, frameInfo.Height)
	}

	// Verify samples per pixel
	if decodedFrameInfo.SamplesPerPixel != frameInfo.SamplesPerPixel {
		t.Errorf("Samples per pixel mismatch: got %d, want %d",
			decodedFrameInfo.SamplesPerPixel, frameInfo.SamplesPerPixel)
	}

	// Verify perfect reconstruction (lossless)
	if len(decodedData) != len(pixelData) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if decodedData[i] != pixelData[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Pixel %d mismatch: got %d, want %d", i, decodedData[i], pixelData[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors: %d (lossless should have 0 errors)", errors)
	} else {
		t.Logf("Perfect reconstruction: all %d pixels match", len(pixelData))
	}
}

func TestLosslessCodecRGB(t *testing.T) {
	// Create RGB test data
	width, height := 32, 32
	components := 3
	pixelData := make([]byte, width*height*components)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * components
			pixelData[offset+0] = byte(x * 8)       // R
			pixelData[offset+1] = byte(y * 8)       // G
			pixelData[offset+2] = byte((x + y) * 4) // B
		}
	}

	// Create frame info
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: uint16(components), BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
	}

	// Create source PixelData using helper
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Create codec with predictor 4
	losslessCodec := NewLosslessCodec(4)

	// Encode
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := losslessCodec.Encode(context.Background(), src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	encodedData, err := encoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get encoded frame: %v", err)
	}

	t.Logf("RGB Original size: %d bytes", len(pixelData))
	t.Logf("RGB Compressed size: %d bytes", len(encodedData))

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = losslessCodec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decodedData, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get decoded frame: %v", err)
	}

	// Verify perfect reconstruction
	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if decodedData[i] != pixelData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("RGB Total pixel errors: %d", errors)
	} else {
		t.Logf("Perfect RGB reconstruction")
	}
}

func TestLosslessCodecWithParameters(t *testing.T) {
	// Create test data
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Create frame info
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	// Create source PixelData using helper
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Create codec with default predictor
	losslessCodec := NewLosslessCodec(0) // Auto-select

	// Create parameters and set predictor to 5
	params := NewLosslessParameters().WithPredictor(5)

	// Encode with parameters
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := losslessCodec.Encode(context.Background(), src, encoded, params)
	if err != nil {
		t.Fatalf("Encode with parameters failed: %v", err)
	}

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = losslessCodec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decodedData, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get decoded frame: %v", err)
	}

	// Verify perfect reconstruction
	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if decodedData[i] != pixelData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors with parameters: %d", errors)
	} else {
		t.Logf("Perfect reconstruction with predictor from parameters")
	}
}

func TestCodecRegistry(t *testing.T) {
	// Register codec
	RegisterLosslessCodec(4)

	// Get from global registry
	registry := codec.GlobalRegistry()
	retrievedCodec, exists := registry.Lookup(transfer.JPEGLossless)
	if !exists {
		t.Fatal("Codec not found in registry")
	}

	if retrievedCodec == nil {
		t.Fatal("Retrieved codec is nil")
	}

	// Verify it's the correct codec
	name := retrievedCodec.Name()
	t.Logf("Retrieved codec name: %s", name)

	// Test with the retrieved codec
	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Create frame info
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	// Create source PixelData using helper
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := retrievedCodec.Encode(context.Background(), src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode with retrieved codec failed: %v", err)
	}

	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = retrievedCodec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode with retrieved codec failed: %v", err)
	}

	// Verify reconstruction
	decodedData, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get decoded frame: %v", err)
	}

	for i := 0; i < len(pixelData); i++ {
		if decodedData[i] != pixelData[i] {
			t.Errorf("Pixel %d mismatch with registry codec", i)
			break
		}
	}

	t.Logf("Registry codec test passed")
}
