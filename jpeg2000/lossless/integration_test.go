package lossless

import (
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

// TestCodecRegistration verifies the codec is registered in the global registry
func TestCodecRegistration(t *testing.T) {
	registry := codec.GlobalRegistry()

	// Get the codec from registry
	retrievedCodec, exists := registry.Lookup(transfer.JPEG2000Lossless)
	if !exists {
		t.Fatal("JPEG 2000 Lossless codec not found in global registry")
	}

	// Verify it's the correct codec
	expectedName := NewCodec().Name()
	if retrievedCodec.Name() != expectedName {
		t.Errorf("Expected codec name '%s', got '%s'", expectedName, retrievedCodec.Name())
	}

	// Verify transfer syntax
	ts := retrievedCodec.TransferSyntax()
	if ts == nil {
		t.Fatal("Transfer syntax is nil")
	}

	expectedUID := jpeg2000LosslessUID
	if ts.UID().UID() != expectedUID {
		t.Errorf("Expected UID %s, got %s", expectedUID, ts.UID().UID())
	}
}

// TestCodecInterfaceCompliance verifies the codec implements all required methods
func TestCodecInterfaceCompliance(t *testing.T) {
	c := NewCodec()

	// Test Name method
	name := c.Name()
	if name == "" {
		t.Error("Name() returned empty string")
	}

	// Test TransferSyntax method
	ts := c.TransferSyntax()
	if ts == nil {
		t.Error("TransferSyntax() returned nil")
	}

	// Test Encode method exists and works
	frameInfo := &codec.FrameInfo{
		Width:  1,
		Height: 1,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), []byte{1, 2, 3}); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}
	dst := codecHelpers.NewTestPixelData(frameInfo)
	err := c.Encode(context.Background(
	// Encoding should work now
	), src, dst, nil)

	if err != nil {
		t.Errorf("Encode failed: %v", err)
	}
	dstData, _ := dst.Frame(context.Background(), 0)
	if len(dstData) == 0 {
		t.Error("Encoded data is empty")
	}

	// Test Decode method exists
	emptyDst := codecHelpers.NewTestPixelData(frameInfo)
	err = c.Decode(context.Background(), codecHelpers.NewTestPixelData(frameInfo), emptyDst, nil)
	// We expect an error (empty data)
	if err == nil {
		t.Error("Decode should return error for empty data")
	}
}

// TestCodecMetadata verifies codec metadata is correct
func TestCodecMetadata(t *testing.T) {
	c := NewCodec()

	tests := []struct {
		name     string
		getValue func() interface{}
		expected interface{}
	}{
		{
			name:     "Codec Name",
			getValue: func() interface{} { return c.Name() },
			expected: c.Name(),
		},
		{
			name:     "Transfer Syntax UID",
			getValue: func() interface{} { return c.TransferSyntax().UID().UID() },
			expected: jpeg2000LosslessUID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.getValue()
			if got != tt.expected {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

// TestDecodeErrorHandling tests various error conditions for decode
func TestDecodeErrorHandling(t *testing.T) {
	c := NewCodec()

	frameInfo := &codec.FrameInfo{
		Width:  64,
		Height: 64,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}

	srcWithData := codecHelpers.NewTestPixelData(frameInfo)
	if err := srcWithData.AddFrame(context.Background(), []byte{1}); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	srcWithInvalidData := codecHelpers.NewTestPixelData(frameInfo)
	if err := srcWithInvalidData.AddFrame(context.Background(), []byte{0x00, 0x01, 0x02}); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	tests := []struct {
		name          string
		src           codec.FrameSource
		dst           codec.FrameSink
		expectError   bool
		errorContains string
	}{
		{
			name:          "Nil source",
			src:           nil,
			dst:           codecHelpers.NewTestPixelData(frameInfo),
			expectError:   true,
			errorContains: "cannot be nil",
		},
		{
			name:          "Nil destination",
			src:           srcWithData,
			dst:           nil,
			expectError:   true,
			errorContains: "cannot be nil",
		},
		{
			name:          "Empty data",
			src:           codecHelpers.NewTestPixelData(frameInfo),
			dst:           codecHelpers.NewTestPixelData(frameInfo),
			expectError:   true,
			errorContains: "empty",
		},
		{
			name:          "Invalid JPEG 2000 data",
			src:           srcWithInvalidData,
			dst:           codecHelpers.NewTestPixelData(frameInfo),
			expectError:   true,
			errorContains: "decode failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Decode(context.Background(), tt.src, tt.dst, nil)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && err != nil && tt.errorContains != "" {
				// Simple substring check
				errMsg := err.Error()
				found := false
				// Check if error message contains the expected substring
				for i := 0; i <= len(errMsg)-len(tt.errorContains); i++ {
					if errMsg[i:i+len(tt.errorContains)] == tt.errorContains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Error message '%s' does not contain '%s'", errMsg, tt.errorContains)
				}
			}
		})
	}
}

// TestEncodeErrorHandling tests various error conditions for encode
func TestEncodeErrorHandling(t *testing.T) {
	c := NewCodec()

	frameInfo := &codec.FrameInfo{
		Width:  64,
		Height: 64,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}

	srcWithData := codecHelpers.NewTestPixelData(frameInfo)
	if err := srcWithData.AddFrame(context.Background(), []byte{1}); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	frameInfoSmall := &codec.FrameInfo{
		Width:  8,
		Height: 8,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}
	srcValid := codecHelpers.NewTestPixelData(frameInfoSmall)
	if err := srcValid.AddFrame(context.Background(), make([]byte, 64)); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	tests := []struct {
		name        string
		src         codec.FrameSource
		dst         codec.FrameSink
		expectError bool
	}{
		{
			name:        "Nil source",
			src:         nil,
			dst:         codecHelpers.NewTestPixelData(frameInfo),
			expectError: true,
		},
		{
			name:        "Nil destination",
			src:         srcWithData,
			dst:         nil,
			expectError: true,
		},
		{
			name:        "Empty data",
			src:         codecHelpers.NewTestPixelData(frameInfo),
			dst:         codecHelpers.NewTestPixelData(frameInfo),
			expectError: true,
		},
		{
			name:        "Valid data (encoding works)",
			src:         srcValid,
			dst:         codecHelpers.NewTestPixelData(frameInfoSmall),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Encode(context.Background(), tt.src, tt.dst, nil)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
