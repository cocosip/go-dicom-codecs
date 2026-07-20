package extended

import (
	"bytes"
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg/baseline"
)

func TestEncodeUsesNativeBaselineSOF0ForEightBitSequential(t *testing.T) {
	pixels := make([]byte, 16*16*3)
	for i := range pixels {
		pixels[i] = byte(i)
	}

	encoded, err := Encode(pixels, 16, 16, 3, 8, 90)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if !bytes.Contains(encoded, []byte{0xff, 0xc0}) {
		t.Fatal("Encode() did not emit the JPEG Baseline SOF0 marker used by fo-dicom Native")
	}
	if bytes.Contains(encoded, []byte{0xff, 0xc1}) {
		t.Fatal("Encode() emitted an SOF1 marker instead of fo-dicom Native's baseline-compatible SOF0")
	}

	baselineEncoded, err := baseline.Encode(pixels, 16, 16, 3, 90)
	if err != nil {
		t.Fatalf("baseline.Encode() error = %v", err)
	}
	if !bytes.Equal(encoded, baselineEncoded) {
		t.Fatal("Encode() did not use the fo-dicom-compatible baseline sequential encoding configuration")
	}
}

// TestEncodeDecode8Bit tests 8-bit grayscale encoding/decoding
func TestEncodeDecode8Bit(t *testing.T) {
	width, height := 64, 64
	components := 1
	bitDepth := 8
	quality := 85

	// Create test data
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 3) % 256)
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, components, bitDepth, quality)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(encoded))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decoded, w, h, c, bd, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("Dimension mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	if c != components {
		t.Errorf("Component mismatch: got %d, want %d", c, components)
	}

	if bd != bitDepth {
		t.Errorf("BitDepth mismatch: got %d, want %d", bd, bitDepth)
	}

	// Verify data size
	if len(decoded) != len(pixelData) {
		t.Errorf("Data size mismatch: got %d, want %d", len(decoded), len(pixelData))
	}

	// Check lossy quality (should have some differences)
	maxDiff := 0
	totalDiff := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decoded[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
		}
		totalDiff += diff
	}

	avgDiff := float64(totalDiff) / float64(len(pixelData))
	t.Logf("Max pixel difference: %d", maxDiff)
	t.Logf("Avg pixel difference: %.2f", avgDiff)

	// For lossy compression, some difference is expected
	if avgDiff > 50 {
		t.Errorf("Average difference too large: %.2f (expected < 50)", avgDiff)
	}
}

// TestEncodeDecode12Bit tests 12-bit grayscale encoding/decoding
func TestEncodeDecode12Bit(t *testing.T) {
	width, height := 64, 64
	components := 1
	bitDepth := 12
	quality := 85

	// Create test data (12-bit values stored in 16-bit little-endian)
	pixelData := make([]byte, width*height*2)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := ((x + y*2) * 16) % 4096 // 12-bit value
			idx := (y*width + x) * 2
			pixelData[idx] = byte(val & 0xFF)          // Low byte
			pixelData[idx+1] = byte((val >> 8) & 0xFF) // High byte
		}
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, components, bitDepth, quality)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(encoded))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decoded, w, h, c, bd, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("Dimension mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	if c != components {
		t.Errorf("Component mismatch: got %d, want %d", c, components)
	}

	if bd != bitDepth {
		t.Errorf("BitDepth mismatch: got %d, want %d", bd, bitDepth)
	}

	// Verify data size
	if len(decoded) != len(pixelData) {
		t.Errorf("Data size mismatch: got %d, want %d", len(decoded), len(pixelData))
	}

	// Check lossy quality (12-bit version)
	maxDiff := 0
	totalDiff := 0
	numPixels := width * height
	for i := 0; i < numPixels; i++ {
		idx := i * 2
		origVal := int(pixelData[idx]) | (int(pixelData[idx+1]) << 8)
		decVal := int(decoded[idx]) | (int(decoded[idx+1]) << 8)

		diff := origVal - decVal
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
		}
		totalDiff += diff
	}

	avgDiff := float64(totalDiff) / float64(numPixels)
	t.Logf("Max pixel difference: %d", maxDiff)
	t.Logf("Avg pixel difference: %.2f", avgDiff)

	// For 12-bit lossy compression, allow larger differences
	if avgDiff > 800 {
		t.Errorf("Average difference too large: %.2f (expected < 800)", avgDiff)
	}
}

// TestEncodeDecodeRGB tests RGB encoding/decoding
func TestEncodeDecodeRGB(t *testing.T) {
	width, height := 32, 32
	components := 3
	bitDepth := 8
	quality := 85

	// Create RGB test data
	pixelData := make([]byte, width*height*components)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * components
			pixelData[offset+0] = byte(x * 8)       // R
			pixelData[offset+1] = byte(y * 8)       // G
			pixelData[offset+2] = byte((x + y) * 4) // B
		}
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, components, bitDepth, quality)
	if err != nil {
		t.Fatalf("RGB Encode failed: %v", err)
	}

	t.Logf("RGB Original size: %d bytes", len(pixelData))
	t.Logf("RGB Compressed size: %d bytes", len(encoded))
	t.Logf("RGB Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decoded, w, h, c, bd, err := Decode(encoded)
	if err != nil {
		t.Fatalf("RGB Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("RGB Dimension mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	if c != components {
		t.Errorf("RGB Component mismatch: got %d, want %d", c, components)
	}

	if bd != bitDepth {
		t.Errorf("RGB BitDepth mismatch: got %d, want %d", bd, bitDepth)
	}

	// Check lossy quality
	maxDiff := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decoded[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	t.Logf("RGB Max pixel difference: %d", maxDiff)

	// RGB has larger color space, allow more difference
	if maxDiff > 255 {
		t.Errorf("RGB max difference too large: %d", maxDiff)
	}

	t.Log("RGB lossy compression completed")
}

// TestEncodeInvalidParameters tests error handling for invalid parameters
func TestEncodeInvalidParameters(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		components int
		bitDepth   int
		quality    int
		wantErr    bool
	}{
		{"Invalid width", 0, 64, 1, 8, 85, true},
		{"Invalid height", 64, 0, 1, 8, 85, true},
		{"Invalid components", 64, 64, 2, 8, 85, true},
		{"Invalid bitDepth 16", 64, 64, 1, 16, 85, true},
		{"Invalid bitDepth 7", 64, 64, 1, 7, 85, true},
		{"Invalid quality low", 64, 64, 1, 8, 0, true},
		{"Invalid quality high", 64, 64, 1, 8, 101, true},
		{"Valid 8-bit", 64, 64, 1, 8, 85, false},
		{"Valid 12-bit", 64, 64, 1, 12, 85, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytesPerSample := 1
			if tt.bitDepth > 8 {
				bytesPerSample = 2
			}
			pixelData := make([]byte, tt.width*tt.height*tt.components*bytesPerSample)

			_, err := Encode(pixelData, tt.width, tt.height, tt.components, tt.bitDepth, tt.quality)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestQualityLevels tests different quality settings
func TestQualityLevels(t *testing.T) {
	width, height := 64, 64
	components := 1
	bitDepth := 8

	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 5) % 256)
	}

	qualities := []int{10, 50, 90}
	for _, quality := range qualities {
		encoded, err := Encode(pixelData, width, height, components, bitDepth, quality)
		if err != nil {
			t.Errorf("Quality %d: encode failed: %v", quality, err)
			continue
		}

		t.Logf("Quality %d: size = %d bytes", quality, len(encoded))
	}
}
