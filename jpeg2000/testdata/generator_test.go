package testdata

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

// TestGenerateSimpleJ2K tests the simple JPEG 2000 generator
func TestGenerateSimpleJ2K(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		bitDepth int
	}{
		{"8x8_8bit", 8, 8, 8},
		{"16x16_8bit", 16, 16, 8},
		{"32x32_12bit", 32, 32, 12},
		{"64x64_16bit", 64, 64, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate codestream
			data := GenerateSimpleJ2K(tt.width, tt.height, tt.bitDepth)

			if len(data) == 0 {
				t.Fatal("Generated codestream is empty")
			}

			// Verify it starts with SOC marker
			if len(data) < 2 {
				t.Fatal("Generated codestream too short")
			}

			if data[0] != 0xFF || data[1] != 0x4F {
				t.Errorf("Expected SOC marker (0xFF4F), got 0x%02X%02X", data[0], data[1])
			}

			// Verify it ends with EOC marker
			if data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
				t.Errorf("Expected EOC marker (0xFFD9), got 0x%02X%02X",
					data[len(data)-2], data[len(data)-1])
			}
		})
	}
}

// TestGeneratedCodestreamParsing tests that generated codestreams can be parsed
func TestGeneratedCodestreamParsing(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		bitDepth int
	}{
		{"Small_8bit", 8, 8, 8},
		{"Medium_12bit", 32, 32, 12},
		{"Large_16bit", 128, 128, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate codestream
			data := GenerateSimpleJ2K(tt.width, tt.height, tt.bitDepth)

			// Parse codestream
			parser := codestream.NewParser(data)
			cs, err := parser.Parse()
			if err != nil {
				t.Fatalf("Failed to parse generated codestream: %v", err)
			}

			// Verify SIZ segment
			if cs.SIZ == nil {
				t.Fatal("SIZ segment is nil")
			}

			// Check dimensions
			width := int(cs.SIZ.Xsiz - cs.SIZ.XOsiz)
			height := int(cs.SIZ.Ysiz - cs.SIZ.YOsiz)

			if width != tt.width {
				t.Errorf("Width mismatch: got %d, want %d", width, tt.width)
			}

			if height != tt.height {
				t.Errorf("Height mismatch: got %d, want %d", height, tt.height)
			}

			// Check number of components
			if cs.SIZ.Csiz != 1 {
				t.Errorf("Expected 1 component, got %d", cs.SIZ.Csiz)
			}

			// Check bit depth
			if len(cs.SIZ.Components) != 1 {
				t.Fatalf("Expected 1 component, got %d", len(cs.SIZ.Components))
			}

			gotBitDepth := cs.SIZ.Components[0].BitDepth()
			if gotBitDepth != tt.bitDepth {
				t.Errorf("Bit depth mismatch: got %d, want %d", gotBitDepth, tt.bitDepth)
			}

			// Verify COD segment
			if cs.COD == nil {
				t.Fatal("COD segment is nil")
			}

			// Check decomposition levels
			if cs.COD.NumberOfDecompositionLevels != 0 {
				t.Errorf("Expected 0 decomposition levels, got %d",
					cs.COD.NumberOfDecompositionLevels)
			}

			// Verify QCD segment
			if cs.QCD == nil {
				t.Fatal("QCD segment is nil")
			}

			// Verify at least one tile
			if len(cs.Tiles) == 0 {
				t.Fatal("No tiles found")
			}
		})
	}
}

// TestGeneratedCodestreamMarkers tests marker sequence in generated codestream
func TestGeneratedCodestreamMarkers(t *testing.T) {
	data := GenerateSimpleJ2K(16, 16, 8)

	// Expected marker sequence:
	// SOC, SIZ, COD, QCD, SOT, SOD, EOC
	expectedMarkers := []uint16{
		0xFF4F, // SOC
		0xFF51, // SIZ
		0xFF52, // COD
		0xFF5C, // QCD
		0xFF90, // SOT
		0xFF93, // SOD
		0xFFD9, // EOC
	}

	markerCount := 0
	for i := 0; i < len(data)-1; i++ {
		if data[i] == 0xFF && data[i+1] >= 0x4F {
			marker := uint16(data[i])<<8 | uint16(data[i+1])

			// Check if this is an expected marker
			found := false
			for _, expected := range expectedMarkers {
				if marker == expected {
					found = true
					break
				}
			}

			if found {
				markerCount++
			}
		}
	}

	if markerCount < 5 {
		t.Errorf("Expected at least 5 markers, found %d", markerCount)
	}
}
