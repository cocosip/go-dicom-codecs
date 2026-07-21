package lossless14sv1

import (
	"bytes"
	"testing"
)

func TestEncodeUsesOptimizedHuffmanTables(t *testing.T) {
	pixels := make([]byte, 64*64)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			pixels[y*64+x] = byte(x + y)
		}
	}

	encoded, err := Encode(pixels, 64, 64, 1, 8)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	standardDHT := []byte{0xff, 0xc4, 0x00, 0x1f, 0x00, 0x00, 0x02, 0x03, 0x01, 0x01}
	if bytes.Contains(encoded, standardDHT) {
		t.Fatal("Encode() used the fixed lossless Huffman table instead of optimized coding")
	}
}

func TestEncodeWritesNativeJFIFAPP0(t *testing.T) {
	encoded, err := Encode(make([]byte, 8*8), 8, 8, 1, 8)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	want := []byte{
		0xff, 0xd8,
		0xff, 0xe0, 0x00, 0x10,
		'J', 'F', 'I', 'F', 0x00,
		0x01, 0x01, 0x00,
		0x00, 0x01, 0x00, 0x01,
		0x00, 0x00,
	}
	if len(encoded) < len(want) {
		t.Fatalf("JPEG header is too short: got %d bytes, want at least %d", len(encoded), len(want))
	}
	if !bytes.Equal(encoded[:len(want)], want) {
		t.Fatalf("JPEG header = %x, want %x", encoded[:len(want)], want)
	}
}

func TestLosslessDifferenceDoesNotWrapAcrossSampleRange(t *testing.T) {
	if got := losslessDifference(255, 0); got != 255 {
		t.Errorf("losslessDifference(255, 0) = %d, want 255", got)
	}
	if got := losslessDifference(0, 255); got != -255 {
		t.Errorf("losslessDifference(0, 255) = %d, want -255", got)
	}
}

func TestRGBEncodeUsesOneSharedHuffmanTable(t *testing.T) {
	pixels := make([]byte, 16*16*3)
	for i := range pixels {
		pixels[i] = byte(i)
	}

	encoded, err := Encode(pixels, 16, 16, 3, 8)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got := bytes.Count(encoded, []byte{0xff, 0xc4}); got != 1 {
		t.Fatalf("DHT marker count = %d, want 1 for fo-dicom Native RGB lossless", got)
	}
}

func TestEncodeDecodeGrayscale8bit(t *testing.T) {
	// Create test image (8-bit grayscale)
	width, height := 64, 64
	bitDepth := 8
	pixelData := make([]byte, width*height)

	// Create a gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(jpegData))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, components, bits, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	if components != 1 {
		t.Errorf("Components mismatch: got %d, want 1", components)
	}

	if bits != bitDepth {
		t.Errorf("Bit depth mismatch: got %d, want %d", bits, bitDepth)
	}

	// Verify perfect reconstruction (lossless)
	if len(decodedData) != len(pixelData) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != decodedData[i] {
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

func TestEncodeDecodeRGB8bit(t *testing.T) {
	// Create test image (8-bit RGB)
	width, height := 64, 64
	bitDepth := 8
	pixelData := make([]byte, width*height*3)

	// Create a color gradient
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 3
			pixelData[offset+0] = byte(x * 4)       // R
			pixelData[offset+1] = byte(y * 4)       // G
			pixelData[offset+2] = byte((x + y) * 2) // B
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 3, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(jpegData))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, components, bits, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	if components != 3 {
		t.Errorf("Components mismatch: got %d, want 3", components)
	}

	if bits != bitDepth {
		t.Errorf("Bit depth mismatch: got %d, want %d", bits, bitDepth)
	}

	// Verify perfect reconstruction (lossless)
	if len(decodedData) != len(pixelData) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != decodedData[i] {
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

func TestEncodeDecode12bit(t *testing.T) {
	// Create test image (12-bit grayscale)
	width, height := 32, 32
	bitDepth := 12
	pixelData := make([]byte, width*height*2) // 2 bytes per sample

	// Create a 12-bit gradient (0-4095)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := ((x + y) * 64) % 4096
			offset := (y*width + x) * 2
			pixelData[offset] = byte(val & 0xFF)
			pixelData[offset+1] = byte((val >> 8) & 0xFF)
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(jpegData))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, _, bits, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	if bits != bitDepth {
		t.Errorf("Bit depth mismatch: got %d, want %d", bits, bitDepth)
	}

	// Verify perfect reconstruction
	if len(decodedData) != len(pixelData) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != decodedData[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Byte %d mismatch: got %d, want %d", i, decodedData[i], pixelData[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total errors: %d (lossless should have 0 errors)", errors)
	} else {
		t.Logf("Perfect reconstruction: all bytes match")
	}
}

func TestEncodeDecode16bit(t *testing.T) {
	// Create test image (16-bit grayscale) - testing the extended Huffman tables
	width, height := 32, 32
	bitDepth := 16
	pixelData := make([]byte, width*height*2) // 2 bytes per sample

	// Create a 16-bit gradient with values spanning the full range
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Use values that require category 16 (high values)
			val := ((x+y)*1024 + 590) % 65536
			offset := (y*width + x) * 2
			pixelData[offset] = byte(val & 0xFF)
			pixelData[offset+1] = byte((val >> 8) & 0xFF)
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(jpegData))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, _, bits, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	if bits != bitDepth {
		t.Errorf("Bit depth mismatch: got %d, want %d", bits, bitDepth)
	}

	// Verify perfect reconstruction
	if len(decodedData) != len(pixelData) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	errors := 0
	maxError := 0
	for i := 0; i < len(pixelData)/2; i++ {
		origLow := pixelData[i*2]
		origHigh := pixelData[i*2+1]
		decodedLow := decodedData[i*2]
		decodedHigh := decodedData[i*2+1]

		origVal := int(origLow) | (int(origHigh) << 8)
		decodedVal := int(decodedLow) | (int(decodedHigh) << 8)

		if origVal != decodedVal {
			errors++
			diff := origVal - decodedVal
			if diff < 0 {
				diff = -diff
			}
			if diff > maxError {
				maxError = diff
			}
			if errors <= 5 {
				t.Errorf("Pixel %d mismatch: got %d, want %d", i, decodedVal, origVal)
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors: %d (lossless should have 0 errors), max error: %d", errors, maxError)
	} else {
		t.Logf("Perfect reconstruction: all %d pixels match", width*height)
	}
}

func TestEncodeInvalidParameters(t *testing.T) {
	pixelData := make([]byte, 64*64)

	tests := []struct {
		name       string
		width      int
		height     int
		components int
		bitDepth   int
		wantErr    bool
	}{
		{"Invalid width", 0, 64, 1, 8, true},
		{"Invalid height", 64, 0, 1, 8, true},
		{"Invalid components", 64, 64, 2, 8, true},
		{"Invalid bit depth low", 64, 64, 1, 1, true},
		{"Invalid bit depth high", 64, 64, 1, 17, true},
		{"Valid", 64, 64, 1, 8, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encode(pixelData, tt.width, tt.height, tt.components, tt.bitDepth)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSigned16BitRoundTrip(t *testing.T) {
	width, height := 8, 4
	pixelData := make([]byte, width*height*2)

	values := []int16{-2000, -1000, -10, 0, 10, 1000, 2000, 30000}
	for i := 0; i < width*height; i++ {
		v := values[i%len(values)]
		u := uint16(v)
		off := i * 2
		pixelData[off] = byte(u & 0xFF)
		pixelData[off+1] = byte(u >> 8)
	}

	jpegData, err := Encode(pixelData, width, height, 1, 16)
	if err != nil {
		t.Fatalf("Encode signed 16-bit failed: %v", err)
	}

	decoded, w, h, comps, bits, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode signed 16-bit failed: %v", err)
	}
	if w != width || h != height || comps != 1 || bits != 16 {
		t.Fatalf("Metadata mismatch w=%d h=%d comps=%d bits=%d", w, h, comps, bits)
	}
	if len(decoded) != len(pixelData) {
		t.Fatalf("Decoded length mismatch: got %d want %d", len(decoded), len(pixelData))
	}
	for i := range pixelData {
		if decoded[i] != pixelData[i] {
			t.Fatalf("Pixel %d mismatch got=%d want=%d", i, decoded[i], pixelData[i])
		}
	}
}

func BenchmarkEncode8bitGrayscale(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encode(pixelData, width, height, 1, 8)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecode8bitGrayscale(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	jpegData, err := Encode(pixelData, width, height, 1, 8)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _, err := Decode(jpegData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode12bit(b *testing.B) {
	width, height := 256, 256
	pixelData := make([]byte, width*height*2)
	for i := 0; i < len(pixelData); i += 2 {
		val := (i / 2) % 4096
		pixelData[i] = byte(val & 0xFF)
		pixelData[i+1] = byte((val >> 8) & 0xFF)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encode(pixelData, width, height, 1, 12)
		if err != nil {
			b.Fatal(err)
		}
	}
}
