package openjpeg

import (
	"fmt"
	"strings"
	"testing"
)

// TestEncoderDecoderRoundTrip tests complete encode-decode round trip
func TestEncoderDecoderRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		bitDepth  int
		numLevels int
		pattern   string // patternGradient, patternSolid, patternChecker
	}{
		{"8x8 0-level gradient", 8, 8, 8, 0, patternGradient},
		{"16x16 0-level gradient", 16, 16, 8, 0, patternGradient},
		{"32x32 0-level gradient", 32, 32, 8, 0, patternGradient},
		{"64x64 0-level gradient", 64, 64, 8, 0, patternGradient},
		{"8x8 0-level solid", 8, 8, 8, 0, patternSolid},
		{"16x16 0-level checker", 16, 16, 8, 0, patternChecker},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data
			numPixels := tt.width * tt.height
			componentData := make([][]int32, 1)
			componentData[0] = make([]int32, numPixels)

			// Fill with test pattern
			switch tt.pattern {
			case patternGradient:
				for y := 0; y < tt.height; y++ {
					for x := 0; x < tt.width; x++ {
						idx := y*tt.width + x
						componentData[0][idx] = int32((x + y) % 256)
					}
				}
			case patternSolid:
				for i := range componentData[0] {
					componentData[0][i] = 128
				}
			case patternChecker:
				for y := 0; y < tt.height; y++ {
					for x := 0; x < tt.width; x++ {
						idx := y*tt.width + x
						if (x/4+y/4)%2 == 0 {
							componentData[0][idx] = 0
						} else {
							componentData[0][idx] = 255
						}
					}
				}
			}

			// Encode
			params := DefaultEncodeParams(tt.width, tt.height, 1, tt.bitDepth, false)
			params.NumLevels = tt.numLevels
			encoder := NewEncoder(params)

			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			t.Logf("Original: %d bytes, Encoded: %d bytes, Ratio: %.2f:1",
				numPixels, len(encoded), float64(numPixels)/float64(len(encoded)))

			// Decode
			decoder := NewDecoder()
			err = decoder.Decode(encoded)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			// Verify dimensions
			if decoder.width != tt.width {
				t.Errorf("Width mismatch: got %d, want %d", decoder.width, tt.width)
			}
			if decoder.height != tt.height {
				t.Errorf("Height mismatch: got %d, want %d", decoder.height, tt.height)
			}

			// Get decoded data
			decoded, err := decoder.GetComponentData(0)
			if err != nil {
				t.Fatalf("Failed to get decoded data: %v", err)
			}
			if decoded == nil {
				t.Fatal("Decoded data is nil")
			}

			// Verify decoded data matches original
			if len(decoded) != len(componentData[0]) {
				t.Fatalf("Length mismatch: got %d, want %d", len(decoded), len(componentData[0]))
			}

			// For lossless, should be exact match
			mismatchCount := 0
			maxError := int32(0)
			for i := range componentData[0] {
				diff := decoded[i] - componentData[0][i]
				if diff < 0 {
					diff = -diff
				}
				if diff > 0 {
					mismatchCount++
					if diff > maxError {
						maxError = diff
					}
					if mismatchCount <= 5 {
						t.Errorf("Pixel %d (x=%d, y=%d): got %d, want %d (diff=%d)",
							i, i%tt.width, i/tt.width, decoded[i], componentData[0][i], diff)
					}
				}
			}

			if mismatchCount > 0 {
				t.Errorf("Total mismatches: %d / %d pixels (%.2f%%), max error: %d",
					mismatchCount, len(componentData[0]),
					100.0*float64(mismatchCount)/float64(len(componentData[0])),
					maxError)
			} else {
				t.Log("鉁?Perfect reconstruction (lossless)")
			}
		})
	}
}

// TestEncoderDecoderRoundTripMultiComponent tests RGB encoding/decoding
func TestEncoderDecoderRoundTripMultiComponent(t *testing.T) {
	width, height := 16, 16
	numPixels := width * height

	// Create RGB test data
	componentData := make([][]int32, 3)
	for c := 0; c < 3; c++ {
		componentData[c] = make([]int32, numPixels)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				idx := y*width + x
				// Different pattern for each component
				componentData[c][idx] = int32((x + y + c*50) % 256)
			}
		}
	}

	// Encode
	params := DefaultEncodeParams(width, height, 3, 8, false)
	params.NumLevels = 0 // Start with 0 levels for debugging
	encoder := NewEncoder(params)

	encoded, err := encoder.EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	t.Logf("Original: %d bytes, Encoded: %d bytes",
		numPixels*3, len(encoded))

	// Decode
	decoder := NewDecoder()
	err = decoder.Decode(encoded)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	// Verify each component
	for c := 0; c < 3; c++ {
		decoded, err := decoder.GetComponentData(c)
		if err != nil {
			t.Fatalf("Component %d: failed to get decoded data: %v", c, err)
		}
		if decoded == nil {
			t.Fatalf("Component %d decoded data is nil", c)
		}

		// Count mismatches
		mismatchCount := 0
		for i := range componentData[c] {
			if decoded[i] != componentData[c][i] {
				if mismatchCount < 3 {
					t.Errorf("Component %d, pixel %d: got %d, want %d",
						c, i, decoded[i], componentData[c][i])
				}
				mismatchCount++
			}
		}

		if mismatchCount > 0 {
			t.Errorf("Component %d: %d / %d pixels mismatch",
				c, mismatchCount, len(componentData[c]))
		} else {
			t.Logf("Component %d: 鉁?Perfect reconstruction", c)
		}
	}
}

// TestEncoderDecoderRoundTripLarger tests with larger images
func TestEncoderDecoderRoundTripLarger(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping larger image test in short mode")
	}

	tests := []struct {
		name      string
		width     int
		height    int
		numLevels int
	}{
		{"64x64 1-level", 64, 64, 1},
		{"128x128 1-level", 128, 128, 1},
		{"128x128 2-level", 128, 128, 2},
		{"192x192 1-level", 192, 192, 1},
		{"256x256 1-level", 256, 256, 1},
		{"256x256 2-level", 256, 256, 2},
		{"512x512 1-level", 512, 512, 1},
		{"512x512 2-level", 512, 512, 2},
		{"512x512 3-level", 512, 512, 3},
		{"1024x1024 1-level", 1024, 1024, 1},
		{"1024x1024 2-level", 1024, 1024, 2},
		{"1024x1024 3-level", 1024, 1024, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			numPixels := tt.width * tt.height
			componentData := make([][]int32, 1)
			componentData[0] = make([]int32, numPixels)

			// Gradient pattern
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					idx := y*tt.width + x
					componentData[0][idx] = int32((x + y) % 256)
				}
			}

			// Encode
			params := DefaultEncodeParams(tt.width, tt.height, 1, 8, false)
			params.NumLevels = tt.numLevels
			encoder := NewEncoder(params)

			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			compressionRatio := float64(numPixels) / float64(len(encoded))
			t.Logf("Compression: %d 鈫?%d bytes (%.2f:1)",
				numPixels, len(encoded), compressionRatio)

			// Decode
			decoder := NewDecoder()
			err = decoder.Decode(encoded)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			// Verify
			decoded, err := decoder.GetComponentData(0)
			if err != nil {
				t.Fatalf("Failed to get decoded data: %v", err)
			}
			mismatchCount := 0
			var firstMismatches []string
			for i := 0; i < numPixels; i++ {
				if decoded[i] != componentData[0][i] {
					if mismatchCount < 20 {
						x := i % tt.width
						y := i / tt.width
						diff := decoded[i] - componentData[0][i]
						firstMismatches = append(firstMismatches,
							fmt.Sprintf("  (%d,%d): expected=%d got=%d diff=%d",
								x, y, componentData[0][i], decoded[i], diff))
					}
					mismatchCount++
				}
			}

			if mismatchCount > 0 {
				t.Logf("First mismatches:\n%s", strings.Join(firstMismatches, "\n"))
				t.Errorf("Found %d total mismatches", mismatchCount)
			} else {
				t.Log("鉁?Perfect reconstruction")
			}
		})
	}
}
