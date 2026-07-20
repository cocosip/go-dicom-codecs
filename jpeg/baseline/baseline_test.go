// Package baseline contains tests for JPEG Baseline codec.
package baseline

import (
	"bytes"
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg/standard"
)

func TestDefaultQualityMatchesFoDicom(t *testing.T) {
	if got := NewBaselineParameters().Quality; got != 90 {
		t.Errorf("NewBaselineParameters().Quality = %d, want 90", got)
	}

	parameters := &JPEGBaselineParameters{Quality: 0}
	if err := parameters.Validate(); err != nil {
		t.Fatalf("JPEGBaselineParameters.Validate() error = %v", err)
	}
	if got := parameters.Quality; got != 90 {
		t.Errorf("JPEGBaselineParameters{Quality: 0}.Validate() quality = %d, want 90", got)
	}

	if got := NewBaselineCodec(0).GetDefaultParameters().GetParameter("quality"); got != 90 {
		t.Errorf("NewBaselineCodec(0) default quality = %v, want 90", got)
	}

	if got := NewBaselineCodec(75).GetDefaultParameters().GetParameter("quality"); got != 75 {
		t.Errorf("NewBaselineCodec(75) default quality = %v, want 75", got)
	}
}

func TestRGBSamplingMatchesFoDicom444(t *testing.T) {
	const width, height = 16, 16
	pixelData := make([]byte, width*height*3)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 3
			if (x+y)%2 == 0 {
				pixelData[offset] = 255
				pixelData[offset+2] = 255
			} else {
				pixelData[offset+1] = 255
			}
		}
	}

	jpegData, err := Encode(pixelData, width, height, 3, 90)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	sofOffset := bytes.Index(jpegData, []byte{0xff, 0xc0})
	if sofOffset < 0 || len(jpegData) < sofOffset+19 {
		t.Fatal("encoded JPEG is missing a complete SOF0 segment")
	}

	for component := 0; component < 3; component++ {
		samplingFactor := jpegData[sofOffset+11+component*3]
		if samplingFactor != 0x11 {
			t.Errorf("component %d sampling factor = 0x%02x, want 0x11 (4:4:4)", component+1, samplingFactor)
		}
	}
}

func TestQuantizationTablesMatchFoDicomQuality90(t *testing.T) {
	pixelData := make([]byte, 8*8)
	jpegData, err := Encode(pixelData, 8, 8, 1, 90)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	firstDQT := bytes.Index(jpegData, []byte{0xff, 0xdb})
	if firstDQT < 0 || len(jpegData) < firstDQT+69 {
		t.Fatal("encoded JPEG is missing a complete luminance DQT segment")
	}

	want := []byte{
		0x00, 0x03, 0x02, 0x02, 0x03, 0x02, 0x02, 0x03,
		0x03, 0x03, 0x03, 0x04, 0x03, 0x03, 0x04, 0x05,
		0x08, 0x05, 0x05, 0x04, 0x04, 0x05, 0x0a, 0x07,
		0x07, 0x06, 0x08, 0x0c, 0x0a, 0x0c, 0x0c, 0x0b,
		0x0a, 0x0b, 0x0b, 0x0d, 0x0e, 0x12, 0x10, 0x0d,
		0x0e, 0x11, 0x0e, 0x0b, 0x0b, 0x10, 0x16, 0x10,
		0x11, 0x13, 0x14, 0x15, 0x15, 0x15, 0x0c, 0x0f,
		0x17, 0x18, 0x16, 0x14, 0x18, 0x12, 0x14, 0x15,
	}

	got := jpegData[firstDQT+4 : firstDQT+4+len(want)]
	if !bytes.Equal(got, want) {
		t.Errorf("luminance DQT = %x, want %x", got, want)
	}
}

func TestQuantizeBlockRoundsNegativeCoefficientsSymmetrically(t *testing.T) {
	encoder := &Encoder{}
	encoder.qtables[0] = standard.ScaleQuantTable(standard.DefaultLuminanceQuantTable, 90)

	coef := encoder.quantizeBlock(bytes.Repeat([]byte{127}, 8*8), 0, 0, 8, 0)
	if coef[0] != -3 {
		t.Errorf("quantized DC coefficient = %d, want -3", coef[0])
	}
}

func TestEncodeDecodeGrayscale(t *testing.T) {
	// Create a simple test pattern (grayscale)
	width, height := 64, 64
	pixelData := make([]byte, width*height)

	// Create a gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 1, 85)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded size: %d bytes (compression ratio: %.2fx)",
		len(jpegData), float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, components, err := Decode(jpegData)
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

	// Verify data length
	if len(decodedData) != width*height {
		t.Errorf("Data length mismatch: got %d, want %d", len(decodedData), width*height)
	}

	// Check that decoded data is reasonably close to original (lossy compression)
	// We'll allow a generous error margin
	maxError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("Maximum pixel error: %d", maxError)

	// For lossy JPEG, we expect some error, but it shouldn't be too large
	if maxError > 50 {
		t.Errorf("Maximum error too large: %d (expected <= 50)", maxError)
	}
}

func TestEncodeDecodeRGB(t *testing.T) {
	// Create a simple test pattern (RGB)
	width, height := 64, 64
	pixelData := make([]byte, width*height*3)

	// Create a color gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 3
			pixelData[offset+0] = byte(x * 4)       // R
			pixelData[offset+1] = byte(y * 4)       // G
			pixelData[offset+2] = byte((x + y) * 2) // B
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 3, 85)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded size: %d bytes (compression ratio: %.2fx)",
		len(jpegData), float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, components, err := Decode(jpegData)
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

	// Verify data length
	if len(decodedData) != width*height*3 {
		t.Errorf("Data length mismatch: got %d, want %d", len(decodedData), width*height*3)
	}

	// Check that decoded data is reasonably close to original (lossy compression)
	maxError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("Maximum pixel error: %d", maxError)

	// For lossy JPEG with YCbCr conversion and 4:2:0 subsampling, we expect some error
	// The error can be larger due to chroma subsampling
	if maxError > 120 {
		t.Errorf("Maximum error too large: %d (expected <= 120)", maxError)
	}
}

func TestEncodeInvalidParameters(t *testing.T) {
	pixelData := make([]byte, 64*64)

	tests := []struct {
		name       string
		width      int
		height     int
		components int
		quality    int
		wantErr    bool
	}{
		{"Invalid width", 0, 64, 1, 85, true},
		{"Invalid height", 64, 0, 1, 85, true},
		{"Invalid components", 64, 64, 2, 85, true},
		{"Invalid quality low", 64, 64, 1, 0, true},
		{"Invalid quality high", 64, 64, 1, 101, true},
		{"Valid", 64, 64, 1, 85, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encode(pixelData, tt.width, tt.height, tt.components, tt.quality)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQualityLevels(t *testing.T) {
	width, height := 32, 32
	pixelData := make([]byte, width*height)

	// Create a test pattern
	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	qualities := []int{10, 50, 90}
	var prevSize int

	for _, quality := range qualities {
		jpegData, err := Encode(pixelData, width, height, 1, quality)
		if err != nil {
			t.Fatalf("Encode at quality %d failed: %v", quality, err)
		}

		t.Logf("Quality %d: size = %d bytes", quality, len(jpegData))

		// Higher quality should generally result in larger file sizes
		if prevSize > 0 && len(jpegData) < prevSize {
			t.Logf("Quality %d produced smaller file than previous quality (expected)", quality)
		}
		prevSize = len(jpegData)
	}
}

func BenchmarkEncodeGrayscale(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height)

	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encode(pixelData, width, height, 1, 85)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeGrayscale(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height)

	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	jpegData, err := Encode(pixelData, width, height, 1, 85)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, err := Decode(jpegData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeRGB(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height*3)

	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encode(pixelData, width, height, 3, 85)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeRGB(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height*3)

	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	jpegData, err := Encode(pixelData, width, height, 3, 85)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, err := Decode(jpegData)
		if err != nil {
			b.Fatal(err)
		}
	}
}
