package openjpeg

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

// TestPrecinctCODMarker tests that precinct sizes are correctly written to COD marker
func TestPrecinctCODMarker(t *testing.T) {
	tests := []struct {
		name           string
		precinctWidth  int
		precinctHeight int
		numLevels      int
		expectPrecinct bool
	}{
		{
			name:           "Default (no precincts)",
			precinctWidth:  0,
			precinctHeight: 0,
			numLevels:      3,
			expectPrecinct: false,
		},
		{
			name:           "128x128 precincts",
			precinctWidth:  128,
			precinctHeight: 128,
			numLevels:      3,
			expectPrecinct: true,
		},
		{
			name:           "256x256 precincts",
			precinctWidth:  256,
			precinctHeight: 256,
			numLevels:      5,
			expectPrecinct: true,
		},
		{
			name:           "512x512 precincts",
			precinctWidth:  512,
			precinctHeight: 512,
			numLevels:      2,
			expectPrecinct: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test parameters
			params := DefaultEncodeParams(256, 256, 1, 8, false)
			params.NumLevels = tt.numLevels
			params.PrecinctWidth = tt.precinctWidth
			params.PrecinctHeight = tt.precinctHeight

			// Create simple test data
			pixelData := make([]byte, 256*256)
			for i := range pixelData {
				pixelData[i] = byte(i % 256)
			}

			// Encode
			encoder := NewEncoder(params)
			encoded, err := encoder.Encode(pixelData)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Parse the codestream
			parser := codestream.NewParser(encoded)
			cs, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Check COD segment
			if cs.COD == nil {
				t.Fatal("COD segment not found")
			}

			// Check Scod bit 0 (precinct flag)
			hasPrecinctFlag := (cs.COD.Scod & 0x01) != 0
			if hasPrecinctFlag != tt.expectPrecinct {
				t.Errorf("Expected precinct flag=%v, got=%v", tt.expectPrecinct, hasPrecinctFlag)
			}

			// If precincts are enabled, check precinct sizes
			if tt.expectPrecinct {
				expectedNumSizes := tt.numLevels + 1
				if len(cs.COD.PrecinctSizes) != expectedNumSizes {
					t.Errorf("Expected %d precinct sizes, got %d", expectedNumSizes, len(cs.COD.PrecinctSizes))
				}

				// Check that precinct sizes are reasonable
				for i, ps := range cs.COD.PrecinctSizes {
					t.Logf("Resolution level %d: PPx=%d (2^%d=%d), PPy=%d (2^%d=%d)",
						i, ps.PPx, ps.PPx, 1<<ps.PPx, ps.PPy, ps.PPy, 1<<ps.PPy)

					// PPx and PPy should be in valid range [0, 15]
					if ps.PPx > 15 || ps.PPy > 15 {
						t.Errorf("Resolution level %d: invalid precinct size exponents PPx=%d, PPy=%d",
							i, ps.PPx, ps.PPy)
					}
				}
			} else {
				// No precincts - sizes should be empty or default
				if len(cs.COD.PrecinctSizes) > 0 {
					t.Errorf("Expected no precinct sizes, got %d", len(cs.COD.PrecinctSizes))
				}
			}
		})
	}
}

func TestPrecinctScalingAcrossResolutions(t *testing.T) {
	params := DefaultEncodeParams(128, 128, 1, 8, false)
	params.NumLevels = 2 // numResolutions = 3
	params.PrecinctWidth = 64
	params.PrecinctHeight = 32

	pixelData := make([]byte, 128*128)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	parser := codestream.NewParser(encoded)
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if cs.COD == nil {
		t.Fatal("COD segment not found")
	}

	if len(cs.COD.PrecinctSizes) != params.NumLevels+1 {
		t.Fatalf("Expected %d precinct sizes, got %d", params.NumLevels+1, len(cs.COD.PrecinctSizes))
	}

	basePPx := floorLog2(params.PrecinctWidth)
	basePPy := floorLog2(params.PrecinctHeight)
	for res := 0; res <= params.NumLevels; res++ {
		shift := params.NumLevels - res
		expX := basePPx - shift
		expY := basePPy - shift
		if expX < 0 {
			expX = 0
		}
		if expY < 0 {
			expY = 0
		}
		got := cs.COD.PrecinctSizes[res]
		if int(got.PPx) != expX || int(got.PPy) != expY {
			t.Fatalf("Resolution %d: PPx/PPy = %d/%d, want %d/%d",
				res, got.PPx, got.PPy, expX, expY)
		}
	}
}

// TestPrecinctRoundtrip tests basic encode/decode roundtrip with precincts
func TestPrecinctRoundtrip(t *testing.T) {
	tests := []struct {
		name           string
		width          int
		height         int
		precinctWidth  int
		precinctHeight int
		numLevels      int
	}{
		{
			name:           "64x64 image with 32x32 precincts",
			width:          64,
			height:         64,
			precinctWidth:  32,
			precinctHeight: 32,
			numLevels:      2,
		},
		{
			name:           "128x128 image with 64x64 precincts",
			width:          128,
			height:         128,
			precinctWidth:  64,
			precinctHeight: 64,
			numLevels:      3,
		},
		{
			name:           "256x256 image with 128x128 precincts",
			width:          256,
			height:         256,
			precinctWidth:  128,
			precinctHeight: 128,
			numLevels:      3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test parameters
			params := DefaultEncodeParams(tt.width, tt.height, 1, 8, false)
			params.NumLevels = tt.numLevels
			params.PrecinctWidth = tt.precinctWidth
			params.PrecinctHeight = tt.precinctHeight

			// Create test data with gradient pattern
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

			t.Logf("Encoded size: %d bytes (compression ratio: %.2f:1)",
				len(encoded), float64(len(pixelData))/float64(len(encoded)))

			// Decode
			decoder := NewDecoder()
			if err := decoder.Decode(encoded); err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify dimensions
			if decoder.Width() != tt.width || decoder.Height() != tt.height {
				t.Errorf("Dimensions mismatch: expected %dx%d, got %dx%d",
					tt.width, tt.height, decoder.Width(), decoder.Height())
			}

			// Verify pixel data (lossless should be perfect)
			decodedBytes := decoder.GetPixelData()
			errorCount := 0
			maxError := 0

			for i := range pixelData {
				diff := int(pixelData[i]) - int(decodedBytes[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > 0 {
					errorCount++
					if diff > maxError {
						maxError = diff
					}
				}
			}

			t.Logf("Errors: %d pixels, max error: %d", errorCount, maxError)

			// Lossless should have 0 errors
			if errorCount > 0 {
				t.Errorf("Expected 0 pixel errors for lossless, got %d (max error: %d)",
					errorCount, maxError)
			}
		})
	}
}

func floorLog2(value int) int {
	result := 0
	for value > 1 {
		value >>= 1
		result++
	}
	return result
}
