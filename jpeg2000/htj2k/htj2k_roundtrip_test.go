package htj2k

import (
	"testing"

	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

// TestHTJ2KLosslessRoundTrip tests HTJ2K lossless encoding and decoding
func TestHTJ2KLosslessRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		width  uint16
		height uint16
		large  bool
	}{
		{"64x64", 64, 64, false},
		{"128x128", 128, 128, false},
		{"256x256", 256, 256, false},
		{"512x512", 512, 512, true},
		{"1024x1024", 1024, 1024, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.large && testing.Short() {
				t.Skip("Skipping large image test in short mode")
			}
			// Create test image (grayscale gradient)
			size := int(tt.width) * int(tt.height)
			testData := make([]byte, size)
			for i := 0; i < size; i++ {
				testData[i] = byte(i % 256)
			}

			// Create source PixelData
			frameInfo := &imagetypes.FrameInfo{
				Width:                     tt.width,
				Height:                    tt.height,
				BitsAllocated:             8,
				BitsStored:                8,
				HighBit:                   7,
				SamplesPerPixel:           1,
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: photometricMonochrome2,
			}
			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(testData); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}

			// Create HTJ2K lossless codec
			htCodec := NewLosslessCodec()

			// Encode
			encoded := codecHelpers.NewTestPixelData(frameInfo)
			err := htCodec.Encode(src, encoded, nil)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			encodedData, _ := encoded.GetFrame(0)
			t.Logf("Original size: %d bytes", len(testData))
			t.Logf("Encoded size: %d bytes", len(encodedData))
			t.Logf("Compression ratio: %.2f:1", float64(len(testData))/float64(len(encodedData)))

			// Decode
			decoded := codecHelpers.NewTestPixelData(frameInfo)
			err = htCodec.Decode(encoded, decoded, nil)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			decodedData, _ := decoded.GetFrame(0)
			// Verify perfect reconstruction (lossless)
			if len(decodedData) != len(testData) {
				t.Fatalf("Decoded data size mismatch: got %d, want %d", len(decodedData), len(testData))
			}

			errors := 0
			maxError := 0
			for i := 0; i < len(testData); i++ {
				diff := int(testData[i]) - int(decodedData[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > 0 {
					errors++
					if diff > maxError {
						maxError = diff
					}
				}
			}

			t.Logf("Pixel errors: %d/%d", errors, len(testData))
			t.Logf("Max error: %d", maxError)

			// For lossless, we expect perfect reconstruction
			if errors > 0 {
				t.Errorf("Lossless mode should have 0 errors, got %d errors with max error %d", errors, maxError)
			}
		})
	}
}

// TestHTJ2KLosslessRPCLRoundTrip tests HTJ2K lossless RPCL encoding
func TestHTJ2KLosslessRPCLRoundTrip(t *testing.T) {
	// Create test image
	width := uint16(64)
	height := uint16(64)
	size := int(width) * int(height)
	testData := make([]byte, size)
	for i := 0; i < size; i++ {
		testData[i] = byte(i % 256)
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricMonochrome2,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(testData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Create HTJ2K lossless RPCL codec
	htCodec := NewLosslessRPCLCodec()

	// Encode
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := htCodec.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	encodedData, _ := encoded.GetFrame(0)
	t.Logf("RPCL Compression ratio: %.2f:1", float64(len(testData))/float64(len(encodedData)))

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = htCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decodedData, _ := decoded.GetFrame(0)
	// Verify perfect reconstruction
	errors := 0
	for i := 0; i < len(testData); i++ {
		if testData[i] != decodedData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("Lossless RPCL mode should have 0 errors, got %d", errors)
	}
}

// TestHTJ2KLossyRoundTrip tests HTJ2K lossy encoding and decoding
func TestHTJ2KLossyRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		quality int
		width   uint16
		height  uint16
	}{
		{"64x64_Q50", 50, 64, 64},
		{"64x64_Q80", 80, 64, 64},
		{"128x128_Q70", 70, 128, 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test image
			size := int(tt.width) * int(tt.height)
			testData := make([]byte, size)
			for i := 0; i < size; i++ {
				testData[i] = byte(i % 256)
			}

			frameInfo := &imagetypes.FrameInfo{
				Width:                     tt.width,
				Height:                    tt.height,
				BitsAllocated:             8,
				BitsStored:                8,
				HighBit:                   7,
				SamplesPerPixel:           1,
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: photometricMonochrome2,
			}
			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(testData); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}

			// Create HTJ2K lossy codec
			htCodec := NewCodec(tt.quality)

			// Encode
			encoded := codecHelpers.NewTestPixelData(frameInfo)
			err := htCodec.Encode(src, encoded, nil)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			encodedData, _ := encoded.GetFrame(0)
			t.Logf("Quality %d - Compression ratio: %.2f:1", tt.quality, float64(len(testData))/float64(len(encodedData)))

			// Decode
			decoded := codecHelpers.NewTestPixelData(frameInfo)
			err = htCodec.Decode(encoded, decoded, nil)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			decodedData, _ := decoded.GetFrame(0)
			// Calculate error metrics
			var sumSquaredError int64
			maxError := 0
			for i := 0; i < len(testData); i++ {
				diff := int(testData[i]) - int(decodedData[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > maxError {
					maxError = diff
				}
				sumSquaredError += int64(diff * diff)
			}

			mse := float64(sumSquaredError) / float64(len(testData))
			psnr := 10 * (20 - (0.5 * float64(mse)))

			t.Logf("Max error: %d", maxError)
			t.Logf("MSE: %.2f", mse)
			t.Logf("PSNR: %.2f dB", psnr)

			// For lossy, check reasonable error bounds
			if maxError > 50 {
				t.Errorf("Max error too high: %d (expected < 50)", maxError)
			}
		})
	}
}

// TestHTJ2KRGBRoundTrip tests HTJ2K with RGB images
func TestHTJ2KRGBRoundTrip(t *testing.T) {
	width := uint16(64)
	height := uint16(64)
	size := int(width) * int(height) * 3 // RGB

	// Create RGB test image
	testData := make([]byte, size)
	for i := 0; i < int(width*height); i++ {
		testData[i*3+0] = byte(i % 256)       // R
		testData[i*3+1] = byte((i * 2) % 256) // G
		testData[i*3+2] = byte((i * 3) % 256) // B
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           3,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricRGB,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(testData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Test lossless
	t.Run("RGB_Lossless", func(t *testing.T) {
		htCodec := NewLosslessCodec()

		encoded := codecHelpers.NewTestPixelData(frameInfo)
		err := htCodec.Encode(src, encoded, nil)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		encodedData, _ := encoded.GetFrame(0)
		t.Logf("RGB Compression ratio: %.2f:1", float64(len(testData))/float64(len(encodedData)))

		decoded := codecHelpers.NewTestPixelData(frameInfo)
		err = htCodec.Decode(encoded, decoded, nil)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		decodedData, _ := decoded.GetFrame(0)
		// Verify perfect reconstruction
		errors := 0
		for i := 0; i < len(testData); i++ {
			if testData[i] != decodedData[i] {
				errors++
			}
		}

		t.Logf("RGB errors: %d/%d", errors, len(testData))
		if errors > 0 {
			t.Errorf("RGB lossless should have 0 errors, got %d", errors)
		}
	})
}

// TestHTJ2K12BitRoundTrip tests HTJ2K with 12-bit images
func TestHTJ2K12BitRoundTrip(t *testing.T) {
	width := uint16(64)
	height := uint16(64)
	size := int(width) * int(height) * 2 // 12-bit stored as uint16

	// Create 12-bit test image (stored as uint16)
	testData := make([]byte, size)
	for i := 0; i < int(width*height); i++ {
		val := uint16(i % 4096) // 12-bit range: 0-4095
		testData[i*2] = byte(val & 0xFF)
		testData[i*2+1] = byte((val >> 8) & 0xFF)
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             16,
		BitsStored:                12,
		HighBit:                   11,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricMonochrome2,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(testData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	htCodec := NewLosslessCodec()

	// Encode
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	err := htCodec.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	encodedData, _ := encoded.GetFrame(0)
	t.Logf("12-bit Compression ratio: %.2f:1", float64(len(testData))/float64(len(encodedData)))

	// Decode
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	err = htCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decodedData, _ := decoded.GetFrame(0)
	// Verify perfect reconstruction
	errors := 0
	maxError := 0
	for i := 0; i < int(width*height); i++ {
		origVal := uint16(testData[i*2]) | (uint16(testData[i*2+1]) << 8)
		decVal := uint16(decodedData[i*2]) | (uint16(decodedData[i*2+1]) << 8)

		diff := int(origVal) - int(decVal)
		if diff < 0 {
			diff = -diff
		}
		if diff > 0 {
			errors++
			if diff > maxError {
				maxError = diff
			}
		}
	}

	t.Logf("12-bit errors: %d/%d", errors, int(width*height))
	t.Logf("Max error: %d", maxError)

	if errors > 0 {
		t.Errorf("12-bit lossless should have 0 errors, got %d errors with max %d", errors, maxError)
	}
}
