package lossless14sv1

import (
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

func TestLosslessSV1CodecInterface(t *testing.T) {
	// Create codec
	sv1Codec := NewLosslessSV1Codec()

	// Verify interface implementation
	var _ codec.Codec = sv1Codec

	// Test Name
	name := sv1Codec.Name()
	if name == "" {
		t.Error("Codec name should not be empty")
	}
	t.Logf("Codec name: %s", name)

	// Test TransferSyntax
	ts := sv1Codec.TransferSyntax()
	if ts == nil {
		t.Fatal("Transfer syntax should not be nil")
	}
	if ts.UID().UID() != transfer.JPEGLosslessSV1.UID().UID() {
		t.Errorf("Transfer syntax UID mismatch: got %s, want %s",
			ts.UID().UID(), transfer.JPEGLosslessSV1.UID().UID())
	}
}

func TestLosslessSV1CodecEncodeDecode(t *testing.T) {
	// Create test pixel data (64x64 grayscale)
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y*2) % 256)
		}
	}

	// Create source PixelData using test helper
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Create destination PixelData for encoded data
	encoded := codecHelpers.NewTestPixelData(frameInfo)

	// Create codec
	sv1Codec := NewLosslessSV1Codec()

	// Encode
	err := sv1Codec.Encode(context.Background(), src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Get encoded frame data
	encodedFrame, err := encoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get encoded frame: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(encodedFrame))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encodedFrame)))

	// Verify encoded data is not empty
	if len(encodedFrame) == 0 {
		t.Fatal("Encoded data is empty")
	}

	// Create destination PixelData for decoded data
	decoded := codecHelpers.NewTestPixelData(frameInfo)

	// Decode
	err = sv1Codec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Get decoded frame data
	decodedFrame, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get decoded frame: %v", err)
	}

	// Verify perfect reconstruction (lossless)
	if len(decodedFrame) != len(pixelData) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(decodedFrame), len(pixelData))
	}

	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if decodedFrame[i] != pixelData[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Pixel %d mismatch: got %d, want %d", i, decodedFrame[i], pixelData[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors: %d (lossless should have 0 errors)", errors)
	} else {
		t.Logf("Perfect lossless reconstruction: all %d pixels match", len(pixelData))
	}
}

func TestLosslessSV1CodecRGB(t *testing.T) {
	// Create RGB test data (32x32)
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

	// Create source PixelData using test helper
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: uint16(components), BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Create destination PixelData for encoded data
	encoded := codecHelpers.NewTestPixelData(frameInfo)

	// Create codec
	sv1Codec := NewLosslessSV1Codec()

	// Encode
	err := sv1Codec.Encode(context.Background(), src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Get encoded frame data
	encodedFrame, err := encoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get encoded frame: %v", err)
	}

	t.Logf("RGB Original size: %d bytes", len(pixelData))
	t.Logf("RGB Compressed size: %d bytes", len(encodedFrame))
	t.Logf("RGB Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encodedFrame)))

	// Create destination PixelData for decoded data
	decoded := codecHelpers.NewTestPixelData(frameInfo)

	// Decode
	err = sv1Codec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Get decoded frame data
	decodedFrame, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get decoded frame: %v", err)
	}

	// Verify perfect reconstruction (lossless)
	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if decodedFrame[i] != pixelData[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Pixel %d mismatch: got %d, want %d", i, decodedFrame[i], pixelData[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("RGB Total pixel errors: %d (lossless should have 0 errors)", errors)
	} else {
		t.Logf("Perfect RGB lossless reconstruction: all %d pixels match", len(pixelData))
	}
}

func TestLosslessSV1CodecRegistry(t *testing.T) {
	// Register codec
	RegisterLosslessSV1Codec()

	// Get from global registry
	registry := codec.GlobalRegistry()
	retrievedCodec, exists := registry.Lookup(transfer.JPEGLosslessSV1)
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

	// Create source PixelData using test helper
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

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

	// Verify lossless reconstruction
	decodedFrame, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get decoded frame: %v", err)
	}

	for i := 0; i < len(pixelData); i++ {
		if decodedFrame[i] != pixelData[i] {
			t.Errorf("Pixel %d mismatch with registry codec", i)
			break
		}
	}

	t.Logf("Registry codec test passed")
}

func TestLosslessSV116Bit(t *testing.T) {
	// Test 16-bit data with extended Huffman tables
	// Extended DC Huffman tables support categories 0-16 (卤65535 range)

	// Test 16-bit data
	width, height := 32, 32
	pixelData := make([]byte, width*height*2) // 2 bytes per pixel

	// Create 12-bit test data (stored in 16-bit)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := ((x + y*2) * 16) % 4096 // 12-bit value
			offset := (y*width + x) * 2
			pixelData[offset] = byte(val & 0xFF)          // Low byte
			pixelData[offset+1] = byte((val >> 8) & 0xFF) // High byte
		}
	}

	// Create source PixelData using test helper
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 16, BitsStored: 12, HighBit: 11, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	encoded := codecHelpers.NewTestPixelData(frameInfo)

	sv1Codec := NewLosslessSV1Codec()

	// Encode
	err := sv1Codec.Encode(context.Background(), src, encoded, nil)
	if err != nil {
		t.Fatalf("16-bit encode failed: %v", err)
	}

	// Get encoded frame data
	encodedFrame, err := encoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get encoded frame: %v", err)
	}

	t.Logf("16-bit Original size: %d bytes", len(pixelData))
	t.Logf("16-bit Compressed size: %d bytes", len(encodedFrame))

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = sv1Codec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		t.Fatalf("16-bit decode failed: %v", err)
	}

	// Get decoded frame data
	decodedFrame, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Failed to get decoded frame: %v", err)
	}

	// Verify lossless
	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if decodedFrame[i] != pixelData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("16-bit: %d pixel errors", errors)
	} else {
		t.Logf("16-bit perfect reconstruction")
	}
}
