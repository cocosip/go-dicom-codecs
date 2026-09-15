package lossless

import (
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

// TestCodecInterface verifies the codec implements the interface
func TestCodecInterface(_ *testing.T) {
	var _ codec.Codec = (*Codec)(nil)
}

// TestCodecCreation tests codec creation
func TestCodecCreation(t *testing.T) {
	c := NewCodec()
	if c == nil {
		t.Fatal("NewCodec returned nil")
	}
}

// TestCodecName tests the codec name
func TestCodecName(t *testing.T) {
	c := NewCodec()
	name := c.Name()

	expected := "JPEG 2000 Lossless"
	if name != expected {
		t.Errorf("Name() = %s, want %s", name, expected)
	}
}

// TestCodecTransferSyntax tests the transfer syntax
func TestCodecTransferSyntax(t *testing.T) {
	c := NewCodec()
	ts := c.TransferSyntax()

	if ts == nil {
		t.Fatal("TransferSyntax() returned nil")
	}

	// The UID should be 1.2.840.10008.1.2.4.90
	uid := ts.UID().UID()
	expected := jpeg2000LosslessUID
	if uid != expected {
		t.Errorf("Transfer Syntax UID = %s, want %s", uid, expected)
	}
}

// TestDecodeNilInputs tests decode with nil inputs
func TestDecodeNilInputs(t *testing.T) {
	c := NewCodec()

	frameInfo := &codec.FrameInfo{
		Width:  64,
		Height: 64,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}

	tests := []struct {
		name string
		src  codec.FrameSource
		dst  codec.FrameSink
	}{
		{"Both nil", nil, nil},
		{"Src nil", nil, codecHelpers.NewTestPixelData(frameInfo)},
		{"Dst nil", codecHelpers.NewTestPixelData(frameInfo), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Decode(context.Background(), tt.src, tt.dst, nil)
			if err == nil {
				t.Error("Expected error for nil input, got nil")
			}
		})
	}
}

// TestDecodeEmptyData tests decode with empty data
func TestDecodeEmptyData(t *testing.T) {
	c := NewCodec()

	frameInfo := &codec.FrameInfo{
		Width:  64,
		Height: 64,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := c.Decode(context.Background(), src, dst, nil)
	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
}

// TestDecodeInvalidData tests decode with invalid JPEG 2000 data
func TestDecodeInvalidData(t *testing.T) {
	c := NewCodec()

	frameInfo := &codec.FrameInfo{
		Width:  64,
		Height: 64,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), []byte{0x00, 0x01, 0x02, 0x03}); err != nil {
		t.Fatalf("failed to add invalid frame: %v", err)
	}
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := c.Decode(context.Background(), src, dst, nil)
	if err == nil {
		t.Error("Expected error for invalid data, got nil")
	}
}

// TestEncodeNotImplemented tests that encoding returns error
func TestEncodeNotImplemented(t *testing.T) {
	c := NewCodec()

	// Create simple test data
	pixelData := make([]byte, 64*64)
	// Fill with simple pattern
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	frameInfo := &codec.FrameInfo{
		Width:  64,
		Height: 64,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("failed to add frame: %v", err)
	}
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := c.Encode(context.Background(), src, dst, nil)
	if err != nil {
		t.Errorf("Encoding failed: %v", err)
	}

	// Verify output
	dstData, _ := dst.Frame(context.Background(), 0)
	if len(dstData) == 0 {
		t.Error("Encoded data is empty")
	}
	if dst.FrameInfo().Width != src.FrameInfo().Width {
		t.Errorf("Width mismatch: got %d, want %d", dst.FrameInfo().Width, src.FrameInfo().Width)
	}
	if dst.FrameInfo().Height != src.FrameInfo().Height {
		t.Errorf("Height mismatch: got %d, want %d", dst.FrameInfo().Height, src.FrameInfo().Height)
	}
}

// TestEncodeNilInputs tests encode with nil inputs
func TestEncodeNilInputs(t *testing.T) {
	c := NewCodec()

	frameInfo := &codec.FrameInfo{
		Width:  64,
		Height: 64,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}
	dstPixel := codecHelpers.NewTestPixelData(frameInfo)
	if err := dstPixel.AddFrame(context.Background(), []byte{1}); err != nil {
		t.Fatalf("failed to add frame: %v", err)
	}

	tests := []struct {
		name string
		src  codec.FrameSource
		dst  codec.FrameSink
	}{
		{"Both nil", nil, nil},
		{"Src nil", nil, codecHelpers.NewTestPixelData(frameInfo)},
		{"Dst nil", dstPixel, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Encode(context.Background(), tt.src, tt.dst, nil)
			if err == nil {
				t.Error("Expected error for nil input, got nil")
			}
		})
	}
}

// TestEncodeEmptyData tests encode with empty data
func TestEncodeEmptyData(t *testing.T) {
	c := NewCodec()

	frameInfo := &codec.FrameInfo{
		Width:  64,
		Height: 64,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	dst := codecHelpers.NewTestPixelData(frameInfo)

	if err := src.AddFrame(context.Background(), []byte{}); err != nil {
		t.Fatalf("failed to add frame to src: %v", err)
	}
	err := c.Encode(context.Background(), src, dst, nil)
	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
}
