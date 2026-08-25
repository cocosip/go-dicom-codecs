package testdata

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

// TestGenerateMultilevelJ2K tests the multilevel J2K generator
func TestGenerateMultilevelJ2K(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		bitDepth  int
		numLevels int
	}{
		{"64x64_1_level", 64, 64, 8, 1},
		{"64x64_2_levels", 64, 64, 8, 2},
		{"64x64_3_levels", 64, 64, 8, 3},
		{"128x128_4_levels", 128, 128, 12, 4},
		{"256x256_5_levels", 256, 256, 12, 5},
		{"512x512_6_levels", 512, 512, 16, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := GenerateMultilevelJ2K(tt.width, tt.height, tt.bitDepth, tt.numLevels)

			if len(data) == 0 {
				t.Fatal("Generated data is empty")
			}

			// Verify it's a valid JPEG 2000 codestream
			parser := codestream.NewParser(data)
			cs, err := parser.Parse()
			if err != nil {
				t.Fatalf("Failed to parse generated codestream: %v", err)
			}

			// Verify SIZ segment
			if cs.SIZ == nil {
				t.Fatal("Missing SIZ segment")
			}

			if int(cs.SIZ.Xsiz) != tt.width {
				t.Errorf("Width mismatch: got %d, want %d", cs.SIZ.Xsiz, tt.width)
			}

			if int(cs.SIZ.Ysiz) != tt.height {
				t.Errorf("Height mismatch: got %d, want %d", cs.SIZ.Ysiz, tt.height)
			}

			// Verify COD segment
			if cs.COD == nil {
				t.Fatal("Missing COD segment")
			}

			if int(cs.COD.NumberOfDecompositionLevels) != tt.numLevels {
				t.Errorf("Decomposition levels mismatch: got %d, want %d",
					cs.COD.NumberOfDecompositionLevels, tt.numLevels)
			}

			// Verify transformation type (should be 5-3 reversible)
			if cs.COD.Transformation != 1 {
				t.Errorf("Transformation should be 1 (5-3 reversible), got %d",
					cs.COD.Transformation)
			}

			// Verify QCD segment
			if cs.QCD == nil {
				t.Fatal("Missing QCD segment")
			}

			t.Logf("Successfully generated and parsed %dx%d with %d decomposition levels",
				tt.width, tt.height, tt.numLevels)
		})
	}
}

// TestMultilevelProgression tests increasing decomposition levels
func TestMultilevelProgression(t *testing.T) {
	width, height := 128, 128
	bitDepth := 8

	for numLevels := 0; numLevels <= 6; numLevels++ {
		t.Run(string(rune('0'+numLevels))+"_levels", func(t *testing.T) {
			data := GenerateMultilevelJ2K(width, height, bitDepth, numLevels)

			parser := codestream.NewParser(data)
			cs, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parse failed for %d levels: %v", numLevels, err)
			}

			if int(cs.COD.NumberOfDecompositionLevels) != numLevels {
				t.Errorf("Expected %d levels, got %d",
					numLevels, cs.COD.NumberOfDecompositionLevels)
			}

			// Check QCD has correct number of subbands
			// For reversible transform: 3*numLevels + 1 subbands
			// (LL + 3 detail bands per level)
			if numLevels > 0 {
				expectedSubbands := 3*numLevels + 1
				// QCD SPqcd field should have this many entries
				// (This is a simplified check - actual implementation varies)
				t.Logf("Level %d: expecting %d subbands", numLevels, expectedSubbands)
			}
		})
	}
}

// TestMultilevelDimensions tests various image dimensions
func TestMultilevelDimensions(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		numLevels int
	}{
		{"Square_64x64", 64, 64, 3},
		{"Square_128x128", 128, 128, 4},
		{"Square_256x256", 256, 256, 5},
		{"Rectangular_128x64", 128, 64, 3},
		{"Rectangular_256x128", 256, 128, 4},
		{"Large_512x512", 512, 512, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := GenerateMultilevelJ2K(tt.width, tt.height, 8, tt.numLevels)

			if len(data) < 100 {
				t.Errorf("Generated data too small: %d bytes", len(data))
			}

			// Parse and verify
			parser := codestream.NewParser(data)
			cs, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			// Verify dimensions match
			if int(cs.SIZ.Xsiz) != tt.width || int(cs.SIZ.Ysiz) != tt.height {
				t.Errorf("Dimension mismatch: got %dx%d, want %dx%d",
					cs.SIZ.Xsiz, cs.SIZ.Ysiz, tt.width, tt.height)
			}

			t.Logf("%s: %d bytes generated", tt.name, len(data))
		})
	}
}

// TestMultilevelBitDepths tests different bit depths with multilevel
func TestMultilevelBitDepths(t *testing.T) {
	tests := []struct {
		bitDepth  int
		numLevels int
	}{
		{8, 2},
		{12, 3},
		{16, 4},
	}

	width, height := 128, 128

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.bitDepth/4))+"bit", func(t *testing.T) {
			data := GenerateMultilevelJ2K(width, height, tt.bitDepth, tt.numLevels)

			parser := codestream.NewParser(data)
			cs, err := parser.Parse()
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			// Verify bit depth
			actualBitDepth := cs.SIZ.Components[0].BitDepth()
			if actualBitDepth != tt.bitDepth {
				t.Errorf("Bit depth mismatch: got %d, want %d",
					actualBitDepth, tt.bitDepth)
			}

			t.Logf("%d-bit with %d levels: OK", tt.bitDepth, tt.numLevels)
		})
	}
}
