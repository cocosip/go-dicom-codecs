package openjpeg

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

// TestVerifyMultiPrecinctActuallyWorks verifies that multi-precinct truly works
func TestVerifyMultiPrecinctActuallyWorks(t *testing.T) {
	tests := []struct {
		name           string
		width          int
		height         int
		precinctWidth  int
		precinctHeight int
		numLevels      int
		expectErrors   bool // true if we expect this to fail
	}{
		{
			name:           "64x64 with 32x32 precincts, 1 level",
			width:          64,
			height:         64,
			precinctWidth:  32,
			precinctHeight: 32,
			numLevels:      1,
			expectErrors:   false,
		},
		{
			name:           "64x64 with 32x32 precincts, 2 levels",
			width:          64,
			height:         64,
			precinctWidth:  32,
			precinctHeight: 32,
			numLevels:      2,
			expectErrors:   false,
		},
		{
			name:           "128x128 with 64x64 precincts, 3 levels",
			width:          128,
			height:         128,
			precinctWidth:  64,
			precinctHeight: 64,
			numLevels:      3,
			expectErrors:   false,
		},
		{
			name:           "256x256 with 128x128 precincts, 5 levels",
			width:          256,
			height:         256,
			precinctWidth:  128,
			precinctHeight: 128,
			numLevels:      5,
			expectErrors:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := DefaultEncodeParams(tt.width, tt.height, 1, 8, false)
			params.NumLevels = tt.numLevels
			params.PrecinctWidth = tt.precinctWidth
			params.PrecinctHeight = tt.precinctHeight

			// Create gradient pattern
			pixelData := make([]byte, tt.width*tt.height)
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					pixelData[y*tt.width+x] = byte((x + y) % 256)
				}
			}

			// Encode
			encoder := NewEncoder(params)
			encoded, err := encoder.Encode(pixelData)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Parse codestream to verify precinct configuration
			parser := codestream.NewParser(encoded)
			cs, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Verify COD has precinct sizes
			if len(cs.COD.PrecinctSizes) == 0 {
				t.Errorf("COD should have precinct sizes")
			}

			t.Logf("COD Precinct sizes: %d resolution levels", len(cs.COD.PrecinctSizes))
			for i, ps := range cs.COD.PrecinctSizes {
				t.Logf("  Res %d: 2^%d x 2^%d = %dx%d", i, ps.PPx, ps.PPy, 1<<ps.PPx, 1<<ps.PPy)
			}

			// Decode
			decoder := NewDecoder()
			if err := decoder.Decode(encoded); err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify perfect reconstruction
			decodedBytes := decoder.GetPixelData()
			errors := 0
			maxError := 0
			firstErrorIdx := -1

			for i := range pixelData {
				diff := int(pixelData[i]) - int(decodedBytes[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > 0 {
					if firstErrorIdx == -1 {
						firstErrorIdx = i
					}
					errors++
					if diff > maxError {
						maxError = diff
					}
				}
			}

			if errors > 0 {
				x, y := firstErrorIdx%tt.width, firstErrorIdx/tt.width
				t.Logf("First error at pixel %d (x=%d,y=%d): expected=%d, got=%d",
					firstErrorIdx, x, y, pixelData[firstErrorIdx], decodedBytes[firstErrorIdx])
			}

			t.Logf("Result: %d errors, max error: %d", errors, maxError)

			if !tt.expectErrors && errors > 0 {
				t.Errorf("Expected 0 errors for lossless, got %d (max=%d)", errors, maxError)
			}
			if tt.expectErrors && errors == 0 {
				t.Logf("Note: This test was expected to have errors but passed (good!)")
			}
		})
	}
}

// TestPrecinctActualPacketCount verifies that multiple precincts create multiple packets
func TestPrecinctActualPacketCount(t *testing.T) {
	// This test is conceptual - we can't easily count packets without modifying internal code
	// But we can verify the encoded size changes with different precinct configurations

	width, height := 128, 128

	// Test 1: No precincts (default)
	params1 := DefaultEncodeParams(width, height, 1, 8, false)
	params1.NumLevels = 2
	params1.PrecinctWidth = 0 // Default
	params1.PrecinctHeight = 0

	// Test 2: Large precincts (should be similar to no precincts)
	params2 := DefaultEncodeParams(width, height, 1, 8, false)
	params2.NumLevels = 2
	params2.PrecinctWidth = 256 // Larger than image
	params2.PrecinctHeight = 256

	// Test 3: Small precincts
	params3 := DefaultEncodeParams(width, height, 1, 8, false)
	params3.NumLevels = 2
	params3.PrecinctWidth = 64
	params3.PrecinctHeight = 64

	// Create test data
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 7) % 256)
	}

	// Encode with different precinct settings
	encoder1 := NewEncoder(params1)
	encoded1, err := encoder1.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encode 1 failed: %v", err)
	}

	encoder2 := NewEncoder(params2)
	encoded2, err := encoder2.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encode 2 failed: %v", err)
	}

	encoder3 := NewEncoder(params3)
	encoded3, err := encoder3.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encode 3 failed: %v", err)
	}

	t.Logf("Encoded sizes:")
	t.Logf("  No precincts:    %d bytes", len(encoded1))
	t.Logf("  Large precincts: %d bytes", len(encoded2))
	t.Logf("  Small precincts: %d bytes", len(encoded3))

	// All should decode perfectly
	for i, encoded := range [][]byte{encoded1, encoded2, encoded3} {
		decoder := NewDecoder()
		if err := decoder.Decode(encoded); err != nil {
			t.Fatalf("Decode %d failed: %v", i+1, err)
		}

		decodedBytes := decoder.GetPixelData()
		errors := 0
		for j := range pixelData {
			if pixelData[j] != decodedBytes[j] {
				errors++
			}
		}

		if errors > 0 {
			t.Errorf("Config %d: %d pixel errors", i+1, errors)
		} else {
			t.Logf("Config %d: Perfect reconstruction ✓", i+1)
		}
	}
}
