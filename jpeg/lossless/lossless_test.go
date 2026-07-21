package lossless

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

	encoded, err := Encode(pixels, 64, 64, 1, 8, 1)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	standardDHT := []byte{0xff, 0xc4, 0x00, 0x1f, 0x00, 0x00, 0x02, 0x03, 0x01, 0x01}
	if bytes.Contains(encoded, standardDHT) {
		t.Fatal("Encode() used the fixed lossless Huffman table instead of optimized coding")
	}
}

func TestEncodeWritesNativeJFIFAPP0(t *testing.T) {
	encoded, err := Encode(make([]byte, 8*8), 8, 8, 1, 8, 1)
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

	encoded, err := Encode(pixels, 16, 16, 3, 8, 1)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got := bytes.Count(encoded, []byte{0xff, 0xc4}); got != 1 {
		t.Fatalf("DHT marker count = %d, want 1 for fo-dicom Native RGB lossless", got)
	}
}

func TestAllPredictors(t *testing.T) {
	// Test each predictor individually
	width, height := 64, 64
	bitDepth := 8
	pixelData := make([]byte, width*height)

	// Create a gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y*2) % 256)
		}
	}

	for predictor := 1; predictor <= 7; predictor++ {
		t.Run(PredictorName(predictor), func(t *testing.T) {
			// Encode
			jpegData, err := Encode(pixelData, width, height, 1, bitDepth, predictor)
			if err != nil {
				t.Fatalf("Encode with predictor %d failed: %v", predictor, err)
			}

			t.Logf("Predictor %d: Compressed size: %d bytes (%.2fx)",
				predictor, len(jpegData), float64(len(pixelData))/float64(len(jpegData)))

			// Decode
			decodedData, w, h, components, bits, err := Decode(jpegData)
			if err != nil {
				t.Fatalf("Decode with predictor %d failed: %v", predictor, err)
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
		})
	}
}

func TestAutoSelectPredictor(t *testing.T) {
	// Test automatic predictor selection (predictor = 0)
	width, height := 64, 64
	bitDepth := 8
	pixelData := make([]byte, width*height)

	// Create a test pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x*3 + y*5) % 256)
		}
	}

	// Encode with auto predictor selection
	jpegData, err := Encode(pixelData, width, height, 1, bitDepth, 0)
	if err != nil {
		t.Fatalf("Encode with auto predictor failed: %v", err)
	}

	t.Logf("Auto predictor: Compressed size: %d bytes", len(jpegData))

	// Decode
	decodedData, w, h, _, _, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	// Verify perfect reconstruction
	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != decodedData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors: %d (lossless should have 0 errors)", errors)
	} else {
		t.Logf("Perfect reconstruction with auto predictor")
	}
}

func TestRGBLossless(t *testing.T) {
	// Test RGB image with predictor 4 (Ra + Rb - Rc)
	width, height := 32, 32
	bitDepth := 8
	pixelData := make([]byte, width*height*3)

	// Create a color gradient
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 3
			pixelData[offset+0] = byte(x * 8)       // R
			pixelData[offset+1] = byte(y * 8)       // G
			pixelData[offset+2] = byte((x + y) * 4) // B
		}
	}

	// Encode with predictor 4
	jpegData, err := Encode(pixelData, width, height, 3, bitDepth, 4)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("RGB Original size: %d bytes", len(pixelData))
	t.Logf("RGB Compressed size: %d bytes", len(jpegData))
	t.Logf("RGB Compression ratio: %.2fx", float64(len(pixelData))/float64(len(jpegData)))

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

	// Verify perfect reconstruction
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
		t.Logf("Perfect RGB reconstruction: all %d pixels match", len(pixelData))
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
		predictor  int
		wantErr    bool
	}{
		{"Invalid width", 0, 64, 1, 8, 1, true},
		{"Invalid height", 64, 0, 1, 8, 1, true},
		{"Invalid components", 64, 64, 2, 8, 1, true},
		{"Invalid bit depth low", 64, 64, 1, 1, 1, true},
		{"Invalid bit depth high", 64, 64, 1, 17, 1, true},
		{"Invalid predictor low", 64, 64, 1, 8, -1, true},
		{"Invalid predictor high", 64, 64, 1, 8, 8, true},
		{"Valid predictor 1", 64, 64, 1, 8, 1, false},
		{"Valid predictor 7", 64, 64, 1, 8, 7, false},
		{"Valid auto predictor", 64, 64, 1, 8, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encode(pixelData, tt.width, tt.height, tt.components, tt.bitDepth, tt.predictor)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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
		_, err := Encode(pixelData, width, height, 1, 8, 1)
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

	jpegData, err := Encode(pixelData, width, height, 1, 8, 1)
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

func BenchmarkPredictors(b *testing.B) {
	width, height := 256, 256
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	for predictor := 1; predictor <= 7; predictor++ {
		b.Run(PredictorName(predictor), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := Encode(pixelData, width, height, 1, 8, predictor)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestHighBitDepth12Bit(t *testing.T) {
	width, height := 32, 32
	bitDepth := 12
	pixelData := make([]byte, width*height*2)

	// 12-bit ramp
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := ((x + y*3) * 37) % 4096
			off := (y*width + x) * 2
			pixelData[off] = byte(val & 0xFF)
			pixelData[off+1] = byte((val >> 8) & 0xFF)
		}
	}

	jpegData, err := Encode(pixelData, width, height, 1, bitDepth, 1)
	if err != nil {
		t.Fatalf("Encode 12-bit failed: %v", err)
	}

	decoded, w, h, comps, bits, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode 12-bit failed: %v", err)
	}

	if w != width || h != height || comps != 1 || bits != bitDepth {
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

func TestHighBitDepth16Bit(t *testing.T) {
	width, height := 32, 32
	bitDepth := 16
	pixelData := make([]byte, width*height*2)

	// Use large values to exercise category 16 codes
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := (y*1024 + x*17 + 50000) % 65536
			off := (y*width + x) * 2
			pixelData[off] = byte(val & 0xFF)
			pixelData[off+1] = byte((val >> 8) & 0xFF)
		}
	}

	jpegData, err := Encode(pixelData, width, height, 1, bitDepth, 1)
	if err != nil {
		t.Fatalf("Encode 16-bit failed: %v", err)
	}

	decoded, w, h, comps, bits, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode 16-bit failed: %v", err)
	}

	if w != width || h != height || comps != 1 || bits != bitDepth {
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

	jpegData, err := Encode(pixelData, width, height, 1, 16, 1)
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
