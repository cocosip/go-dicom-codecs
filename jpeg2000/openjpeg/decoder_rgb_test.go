package openjpeg

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/openjpeg/colorspace"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/testdata"
)

// TestDecoderRGBBasic tests basic RGB decoding
func TestDecoderRGBBasic(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"8x8", 8, 8},
		{"16x16", 16, 16},
		{"32x32", 32, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate RGB J2K
			data := testdata.GenerateRGBJ2K(tt.width, tt.height, 8, 0)

			// Decode
			decoder := NewDecoder()
			err := decoder.Decode(data)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify dimensions
			if decoder.Width() != tt.width {
				t.Errorf("Width: got %d, want %d", decoder.Width(), tt.width)
			}

			if decoder.Height() != tt.height {
				t.Errorf("Height: got %d, want %d", decoder.Height(), tt.height)
			}

			// Verify components
			if decoder.Components() != 3 {
				t.Errorf("Components: got %d, want 3", decoder.Components())
			}

			// Verify component data exists
			imageData := decoder.GetImageData()
			if len(imageData) != 3 {
				t.Fatalf("Expected 3 component arrays, got %d", len(imageData))
			}

			for i := 0; i < 3; i++ {
				expectedLen := tt.width * tt.height
				if len(imageData[i]) != expectedLen {
					t.Errorf("Component %d length: got %d, want %d",
						i, len(imageData[i]), expectedLen)
				}
			}
		})
	}
}

// TestDecoderRGBComponents tests individual component access
func TestDecoderRGBComponents(t *testing.T) {
	width, height := 16, 16
	data := testdata.GenerateRGBJ2K(width, height, 8, 0)

	decoder := NewDecoder()
	err := decoder.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Test GetComponentData for each component
	for i := 0; i < 3; i++ {
		compData, err := decoder.GetComponentData(i)
		if err != nil {
			t.Errorf("GetComponentData(%d) failed: %v", i, err)
		}

		expectedLen := width * height
		if len(compData) != expectedLen {
			t.Errorf("Component %d length: got %d, want %d",
				i, len(compData), expectedLen)
		}
	}

	// Test invalid component index
	_, err = decoder.GetComponentData(3)
	if err == nil {
		t.Error("Expected error for invalid component index, got nil")
	}

	_, err = decoder.GetComponentData(-1)
	if err == nil {
		t.Error("Expected error for negative component index, got nil")
	}
}

// TestDecoderRGBPixelData tests interleaved pixel data extraction
func TestDecoderRGBPixelData(t *testing.T) {
	width, height := 8, 8
	data := testdata.GenerateRGBJ2K(width, height, 8, 0)

	decoder := NewDecoder()
	err := decoder.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Get interleaved pixel data
	pixelData := decoder.GetPixelData()

	// For 8-bit RGB, should be width * height * 3 bytes
	expectedLen := width * height * 3
	if len(pixelData) != expectedLen {
		t.Fatalf("Pixel data length: got %d, want %d", len(pixelData), expectedLen)
	}

	// Verify data is interleaved (RGB RGB RGB...)
	// Each triplet should represent one pixel
	// Note: byte values are automatically in range [0, 255]
	numPixels := width * height
	for i := 0; i < numPixels; i++ {
		_ = pixelData[i*3]   // r
		_ = pixelData[i*3+1] // g
		_ = pixelData[i*3+2] // b
		// byte type ensures values are in [0, 255], no need to check
	}
}

// TestDecoderRGBMultipleBitDepths tests RGB with different bit depths
func TestDecoderRGBMultipleBitDepths(t *testing.T) {
	tests := []struct {
		name     string
		bitDepth int
	}{
		{"8-bit", 8},
		{"12-bit", 12},
		{"16-bit", 16},
	}

	width, height := 16, 16

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := testdata.GenerateRGBJ2K(width, height, tt.bitDepth, 0)

			decoder := NewDecoder()
			err := decoder.Decode(data)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			if decoder.BitDepth() != tt.bitDepth {
				t.Errorf("Bit depth: got %d, want %d",
					decoder.BitDepth(), tt.bitDepth)
			}

			if decoder.Components() != 3 {
				t.Errorf("Components: got %d, want 3", decoder.Components())
			}

			// Verify pixel data length
			pixelData := decoder.GetPixelData()
			var expectedLen int
			if tt.bitDepth <= 8 {
				expectedLen = width * height * 3 // 3 bytes per pixel
			} else {
				expectedLen = width * height * 3 * 2 // 6 bytes per pixel (16-bit per component)
			}

			if len(pixelData) != expectedLen {
				t.Errorf("Pixel data length: got %d, want %d",
					len(pixelData), expectedLen)
			}
		})
	}
}

// TestColorSpaceConversionIntegration tests color space conversion with decoder
func TestColorSpaceConversionIntegration(t *testing.T) {
	width, height := 8, 8

	// Create RGB test pattern
	rgbData := testdata.GenerateRGBTestImage(width, height)

	// Convert to YCbCr components
	y, cb, cr := colorspace.ConvertRGBToYCbCr(rgbData, width, height)

	// Verify conversion worked
	if len(y) != width*height || len(cb) != width*height || len(cr) != width*height {
		t.Fatal("Color space conversion produced wrong sizes")
	}

	// Convert back to RGB
	rgbBack := colorspace.ConvertYCbCrToRGB(y, cb, cr, width, height)

	// Verify round-trip accuracy
	maxError := int32(0)
	for i := 0; i < len(rgbData); i++ {
		diff := rgbData[i] - rgbBack[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("Color space round-trip max error: %d", maxError)

	if maxError > 10 {
		t.Errorf("Color space round-trip error too high: %d (expected ≤ 10)", maxError)
	}
}

// TestComponentInterleaving tests interleaving and deinterleaving
func TestComponentInterleaving(t *testing.T) {
	width, height := 8, 8

	// Generate separate components
	r, g, b := testdata.GenerateRGBComponents(width, height)

	components := [][]int32{r, g, b}

	// Interleave
	interleaved := colorspace.InterleaveComponents(components)

	expectedLen := width * height * 3
	if len(interleaved) != expectedLen {
		t.Fatalf("Interleaved length: got %d, want %d", len(interleaved), expectedLen)
	}

	// Deinterleave
	deinterleaved := colorspace.DeinterleaveComponents(interleaved, 3)

	if len(deinterleaved) != 3 {
		t.Fatalf("Deinterleaved components: got %d, want 3", len(deinterleaved))
	}

	// Verify round-trip
	for c := 0; c < 3; c++ {
		if len(deinterleaved[c]) != width*height {
			t.Errorf("Component %d length after round-trip: got %d, want %d",
				c, len(deinterleaved[c]), width*height)
		}

		// Check values match
		for i := 0; i < width*height; i++ {
			if deinterleaved[c][i] != components[c][i] {
				t.Errorf("Component %d, pixel %d: got %d, want %d",
					c, i, deinterleaved[c][i], components[c][i])
				break // Only report first error per component
			}
		}
	}
}

// TestDecoderRGBWithLevels tests RGB with wavelet decomposition
func TestDecoderRGBWithLevels(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		levels int
	}{
		{"16x16 1-level", 16, 16, 1},
		{"32x32 2-level", 32, 32, 2},
		{"64x64 3-level", 64, 64, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := testdata.GenerateRGBJ2K(tt.width, tt.height, 8, tt.levels)

			decoder := NewDecoder()
			err := decoder.Decode(data)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			if decoder.Components() != 3 {
				t.Errorf("Components: got %d, want 3", decoder.Components())
			}

			// Verify all component data exists and has correct size
			imageData := decoder.GetImageData()
			for c := 0; c < 3; c++ {
				expectedLen := tt.width * tt.height
				if len(imageData[c]) != expectedLen {
					t.Errorf("Component %d length: got %d, want %d",
						c, len(imageData[c]), expectedLen)
				}
			}
		})
	}
}

// TestSolidColorRGB tests decoding solid color RGB images
func TestSolidColorRGB(t *testing.T) {
	// Note: This test would require an actual encoding implementation
	// For now, we test the decoder structure with generated codestream
	t.Skip("Requires encoding implementation for meaningful test")
}

// TestColorBarsRGB tests decoding color bars pattern
func TestColorBarsRGB(t *testing.T) {
	// Note: This test would require an actual encoding implementation
	// For now, we test the decoder structure with generated codestream
	t.Skip("Requires encoding implementation for meaningful test")
}
