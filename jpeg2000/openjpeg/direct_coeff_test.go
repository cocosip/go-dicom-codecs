package openjpeg

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/openjpeg/wavelet"
)

// TestDirectCoefficientComparison directly compares coefficients
func TestDirectCoefficientComparison192x192(t *testing.T) {
	width, height := 192, 192
	numLevels := 1

	// Create original data
	original := make([]int32, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			original[y*width+x] = int32((x + y) % 256)
		}
	}

	// Encoder path: DC shift + forward wavelet
	encoderCoeffs := make([]int32, width*height)
	copy(encoderCoeffs, original)
	for i := range encoderCoeffs {
		encoderCoeffs[i] -= 128
	}
	wavelet.ForwardMultilevel(encoderCoeffs, width, height, numLevels)

	// Full encode/decode
	componentData := make([][]int32, 1)
	componentData[0] = original

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = numLevels
	encoder := NewEncoder(params)

	encoded, err := encoder.EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	decoder := NewDecoder()
	err = decoder.Decode(encoded)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	// Now we need to access decoder's internal coefficients
	// Since we can't access them directly, let's decode and apply the INVERSE process
	decoded, err := decoder.GetComponentData(0)
	if err != nil {
		t.Fatalf("Failed to get decoded data: %v", err)
	}

	// Reverse the decoder process: subtract DC shift and apply forward wavelet
	// This should give us the decoder's coefficient array
	decoderCoeffs := make([]int32, width*height)
	copy(decoderCoeffs, decoded)
	for i := range decoderCoeffs {
		decoderCoeffs[i] -= 128
	}
	wavelet.ForwardMultilevel(decoderCoeffs, width, height, numLevels)

	// Compare coefficients
	mismatchCount := 0
	firstMismatch := -1
	for i := 0; i < len(encoderCoeffs); i++ {
		if encoderCoeffs[i] != decoderCoeffs[i] {
			if firstMismatch < 0 {
				firstMismatch = i
			}
			mismatchCount++
			if mismatchCount <= 10 {
				x := i % width
				y := i / width
				t.Logf("Coeff mismatch at (%d,%d): encoder=%d decoder=%d diff=%d",
					x, y, encoderCoeffs[i], decoderCoeffs[i], decoderCoeffs[i]-encoderCoeffs[i])
			}
		}
	}

	if mismatchCount > 0 {
		t.Errorf("Found %d coefficient mismatches out of %d", mismatchCount, len(encoderCoeffs))
	} else {
		t.Log("✓ All coefficients match")
	}

	// Also check final pixels
	pixelMismatches := 0
	for i := 0; i < len(original); i++ {
		if decoded[i] != original[i] {
			pixelMismatches++
		}
	}
	if pixelMismatches > 0 {
		t.Logf("Final pixel mismatches: %d", pixelMismatches)
	}
}
