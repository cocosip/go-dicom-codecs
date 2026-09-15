package lossless

import (
	"bytes"
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

func TestCodecRGBUsesSampleInterleave(t *testing.T) {
	const (
		width  = 4
		height = 3
	)

	pixelData := make([]byte, width*height*3)
	for i := range pixelData {
		pixelData[i] = byte(i * 13)
	}
	frameInfo := &codec.FrameInfo{
		Width:  width,
		Height: height,

		SamplesPerPixel: 3, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := NewJPEGLSLosslessCodec().Encode(context.Background(), src, encoded, nil); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	frame, err := encoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetFrame failed: %v", err)
	}

	sos := bytes.Index(frame, []byte{0xFF, 0xDA})
	if sos < 0 || len(frame) < sos+14 {
		t.Fatalf("encoded frame does not contain a complete SOS segment")
	}
	if got := frame[sos+4]; got != 3 {
		t.Errorf("SOS component count = %d, want 3", got)
	}
	if got := frame[sos+12]; got != 2 {
		t.Errorf("SOS ILV = %d, want 2 (sample interleaved)", got)
	}
}

func TestDefaultEncodeOmitsLSEPresetParameters(t *testing.T) {
	encoded, err := Encode(make([]byte, 8*8), 8, 8, 1, 8)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if bytes.Contains(encoded, []byte{0xff, 0xf8}) {
		t.Fatal("Encode() emitted an LSE preset-parameter segment for standard defaults")
	}
}

func TestEncodeEndsAtEOI(t *testing.T) {
	encoded, err := Encode([]byte{0, 8, 16, 24, 32, 40}, 3, 2, 1, 8)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) < 2 || !bytes.Equal(encoded[len(encoded)-2:], []byte{0xff, 0xd9}) {
		t.Errorf("raw JPEG-LS output must end at EOI, got suffix %x", encoded[max(0, len(encoded)-4):])
	}
}

// TestCodecInterface tests the Codec interface implementation for JPEGLSLosslessCodec
func TestCodecInterface(t *testing.T) {
	c := NewJPEGLSLosslessCodec()

	// Test UID
	expectedUID := transfer.JPEGLSLossless.UID().UID()
	if c.TransferSyntax().UID().UID() != expectedUID {
		t.Errorf("UID mismatch: got %s, want %s", c.TransferSyntax().UID().UID(), expectedUID)
	}

	// Test Name
	name := c.Name()
	t.Logf("Codec name: %s", name)
	if name == "" {
		t.Error("Name should not be empty")
	}
}

// TestCodecEncodeDecode8Bit tests codec encode/decode with 8-bit data
func TestCodecEncodeDecode8Bit(t *testing.T) {
	c := NewJPEGLSLosslessCodec()

	width, height := 64, 64
	// Create test frame (grayscale)
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Create source PixelData
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
	if err := c.Encode(context.Background(), src, encoded, nil); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Decode(context.Background(), encoded, decoded, nil); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decodedInfo := decoded.FrameInfo()
	if int(decodedInfo.Width) != width || int(decodedInfo.Height) != height {
		t.Errorf("Dimension mismatch: got %dx%d, want %dx%d", decodedInfo.Width, decodedInfo.Height, width, height)
	}
	if decodedInfo.SamplesPerPixel != 1 {
		t.Errorf("Components mismatch: got %d, want %d", decodedInfo.SamplesPerPixel, 1)
	}

	decodedFrame, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetFrame failed: %v", err)
	}
	errors := 0
	for i := 0; i < len(decodedFrame); i++ {
		if decodedFrame[i] != pixelData[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Pixel %d mismatch: got %d, want %d", i, decodedFrame[i], pixelData[i])
			}
		}
	}
	if errors > 0 {
		t.Errorf("Total pixel errors: %d / %d", errors, len(pixelData))
	}
}

// TestCodecEncodeDecode12Bit tests codec with 12-bit data
func TestCodecEncodeDecode12Bit(t *testing.T) {
	c := NewJPEGLSLosslessCodec()

	width, height := 32, 32
	maxVal := 4095

	// Create 12-bit test data (stored in 16-bit little-endian)
	pixelData := make([]byte, width*height*2)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 2
			val := ((x + y*2) * 16) % (maxVal + 1)
			pixelData[idx] = byte(val & 0xFF)
			pixelData[idx+1] = byte((val >> 8) & 0xFF)
		}
	}

	// Create source PixelData
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
	if err := c.Encode(context.Background(), src, encoded, nil); err != nil {
		t.Fatalf("12-bit Encode failed: %v", err)
	}

	decoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Decode(context.Background(), encoded, decoded, nil); err != nil {
		t.Fatalf("12-bit Decode failed: %v", err)
	}

	decodedFrame, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetFrame failed: %v", err)
	}
	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if decodedFrame[i] != pixelData[i] {
			errors++
		}
	}
	if errors > 0 {
		t.Errorf("12-bit: Total pixel errors: %d / %d", errors, len(pixelData))
	}
}

// TestCodecRGB tests codec with RGB data
func TestCodecRGB(t *testing.T) {
	c := NewJPEGLSLosslessCodec()

	width, height := 32, 32
	components := 3

	// Create RGB test data
	pixelData := make([]byte, width*height*components)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * components
			pixelData[idx+0] = byte(x * 8)
			pixelData[idx+1] = byte(y * 8)
			pixelData[idx+2] = byte((x + y) * 4)
		}
	}

	// Create source PixelData
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: uint16(components), BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	encoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Encode(context.Background(), src, encoded, nil); err != nil {
		t.Fatalf("RGB Encode failed: %v", err)
	}

	decoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Decode(context.Background(), encoded, decoded, nil); err != nil {
		t.Fatalf("RGB Decode failed: %v", err)
	}

	decodedFrame, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetFrame failed: %v", err)
	}
	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if decodedFrame[i] != pixelData[i] {
			errors++
		}
	}
	if errors > 0 {
		t.Errorf("RGB: Total pixel errors: %d / %d", errors, len(pixelData))
	}
}

// TestCodecRegistry tests codec registration
func TestCodecRegistry(t *testing.T) {
	// Ensure codec registered
	RegisterJPEGLSLosslessCodec()

	// Get from global registry
	registry := codec.GlobalRegistry()
	retrievedCodec, exists := registry.Lookup(transfer.JPEGLSLossless)
	if !exists {
		t.Fatal("Codec not found in registry")
	}

	// Test encode/decode through registry codec
	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Create source PixelData
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
		t.Fatalf("Registry encode failed: %v", err)
	}

	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = retrievedCodec.Decode(context.Background(), encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Registry decode failed: %v", err)
	}

	decodedFrame, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetFrame failed: %v", err)
	}
	for i := 0; i < len(pixelData); i++ {
		if decodedFrame[i] != pixelData[i] {
			t.Errorf("Pixel %d mismatch in registry test", i)
			break
		}
	}
}
