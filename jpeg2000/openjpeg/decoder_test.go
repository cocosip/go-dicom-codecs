package openjpeg

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/testdata"
)

// TestDecoderWithGeneratedData tests the decoder with generated test data
func TestDecoderWithGeneratedData(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		bitDepth int
	}{
		{"Small_8x8_8bit", 8, 8, 8},
		{"Medium_32x32_12bit", 32, 32, 12},
		{"Large_64x64_16bit", 64, 64, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate test codestream
			data := testdata.GenerateSimpleJ2K(tt.width, tt.height, tt.bitDepth)

			// Create decoder
			decoder := NewDecoder()

			// Decode
			err := decoder.Decode(data)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify dimensions
			if decoder.Width() != tt.width {
				t.Errorf("Width mismatch: got %d, want %d", decoder.Width(), tt.width)
			}

			if decoder.Height() != tt.height {
				t.Errorf("Height mismatch: got %d, want %d", decoder.Height(), tt.height)
			}

			// Verify components
			if decoder.Components() != 1 {
				t.Errorf("Expected 1 component, got %d", decoder.Components())
			}

			// Verify bit depth
			if decoder.BitDepth() != tt.bitDepth {
				t.Errorf("Bit depth mismatch: got %d, want %d", decoder.BitDepth(), tt.bitDepth)
			}

			// Get pixel data
			pixelData := decoder.GetPixelData()
			if pixelData == nil {
				t.Fatal("Pixel data is nil")
			}

			// Calculate expected size
			expectedSize := tt.width * tt.height
			if tt.bitDepth > 8 {
				expectedSize *= 2 // 16-bit data
			}

			if len(pixelData) != expectedSize {
				t.Errorf("Pixel data size mismatch: got %d, want %d",
					len(pixelData), expectedSize)
			}
		})
	}
}

// TestDecoderNilInput tests decoder with nil input
func TestDecoderNilInput(t *testing.T) {
	decoder := NewDecoder()
	err := decoder.Decode(nil)
	if err == nil {
		t.Error("Expected error for nil input, got nil")
	}
}

// TestDecoderEmptyInput tests decoder with empty input
func TestDecoderEmptyInput(t *testing.T) {
	decoder := NewDecoder()
	err := decoder.Decode([]byte{})
	if err == nil {
		t.Error("Expected error for empty input, got nil")
	}
}

// TestDecoderInvalidInput tests decoder with invalid input
func TestDecoderInvalidInput(t *testing.T) {
	decoder := NewDecoder()
	err := decoder.Decode([]byte{0x00, 0x01, 0x02})
	if err == nil {
		t.Error("Expected error for invalid input, got nil")
	}
}

// TestDecoderGetters tests decoder getter methods
func TestDecoderGetters(t *testing.T) {
	// Generate test data
	data := testdata.GenerateSimpleJ2K(16, 16, 8)

	decoder := NewDecoder()
	err := decoder.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Test all getter methods
	if decoder.Width() != 16 {
		t.Errorf("Width() = %d, want 16", decoder.Width())
	}

	if decoder.Height() != 16 {
		t.Errorf("Height() = %d, want 16", decoder.Height())
	}

	if decoder.Components() != 1 {
		t.Errorf("Components() = %d, want 1", decoder.Components())
	}

	if decoder.BitDepth() != 8 {
		t.Errorf("BitDepth() = %d, want 8", decoder.BitDepth())
	}

	if decoder.IsSigned() {
		t.Error("IsSigned() = true, want false")
	}

	imageData := decoder.GetImageData()
	if imageData == nil {
		t.Error("GetImageData() returned nil")
	}

	if len(imageData) != 1 {
		t.Errorf("GetImageData() length = %d, want 1", len(imageData))
	}

	pixelData := decoder.GetPixelData()
	if pixelData == nil {
		t.Error("GetPixelData() returned nil")
	}
}

// TestDecoderMultipleSizes tests decoder with various image sizes
func TestDecoderMultipleSizes(t *testing.T) {
	sizes := []struct {
		width  int
		height int
	}{
		{4, 4},
		{8, 8},
		{16, 16},
		{32, 32},
		{64, 64},
		{128, 128},
	}

	for _, size := range sizes {
		t.Run(sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			data := testdata.GenerateSimpleJ2K(size.width, size.height, 8)
			decoder := NewDecoder()

			err := decoder.Decode(data)
			if err != nil {
				t.Fatalf("Decode failed for %dx%d: %v", size.width, size.height, err)
			}

			if decoder.Width() != size.width || decoder.Height() != size.height {
				t.Errorf("Size mismatch: got %dx%d, want %dx%d",
					decoder.Width(), decoder.Height(), size.width, size.height)
			}
		})
	}
}

// sprintf is a helper function (go doesn't allow importing fmt in tests sometimes)
func sprintf(_ string, width, height int) string {
	// Simple integer to string conversion for test names
	return string(rune('0'+width/100)) + string(rune('0'+(width/10)%10)) + string(rune('0'+width%10)) +
		"x" +
		string(rune('0'+height/100)) + string(rune('0'+(height/10)%10)) + string(rune('0'+height%10))
}

// TestDecoderDifferentBitDepths tests decoder with various bit depths
func TestDecoderDifferentBitDepths(t *testing.T) {
	bitDepths := []int{8, 12, 16}

	for _, bitDepth := range bitDepths {
		t.Run(sprintf("BitDepth_%d", bitDepth, 0), func(t *testing.T) {
			data := testdata.GenerateSimpleJ2K(16, 16, bitDepth)
			decoder := NewDecoder()

			err := decoder.Decode(data)
			if err != nil {
				t.Fatalf("Decode failed for bit depth %d: %v", bitDepth, err)
			}

			if decoder.BitDepth() != bitDepth {
				t.Errorf("Bit depth mismatch: got %d, want %d",
					decoder.BitDepth(), bitDepth)
			}
		})
	}
}
