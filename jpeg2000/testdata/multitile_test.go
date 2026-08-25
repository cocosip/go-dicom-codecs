package testdata

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

// TestGenerateMultiTileJ2K tests multi-tile JPEG 2000 generation
func TestGenerateMultiTileJ2K(t *testing.T) {
	tests := []struct {
		name          string
		width         int
		height        int
		tileWidth     int
		tileHeight    int
		bitDepth      int
		numLevels     int
		components    int
		expectedTiles int
	}{
		{
			name:          "2x2 tiles grayscale",
			width:         512,
			height:        512,
			tileWidth:     256,
			tileHeight:    256,
			bitDepth:      8,
			numLevels:     0,
			components:    1,
			expectedTiles: 4,
		},
		{
			name:          "3x2 tiles grayscale",
			width:         600,
			height:        400,
			tileWidth:     256,
			tileHeight:    256,
			bitDepth:      8,
			numLevels:     0,
			components:    1,
			expectedTiles: 6,
		},
		{
			name:          "2x2 tiles RGB",
			width:         512,
			height:        512,
			tileWidth:     256,
			tileHeight:    256,
			bitDepth:      8,
			numLevels:     0,
			components:    3,
			expectedTiles: 4,
		},
		{
			name:          "Single tile",
			width:         256,
			height:        256,
			tileWidth:     256,
			tileHeight:    256,
			bitDepth:      8,
			numLevels:     0,
			components:    1,
			expectedTiles: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := GenerateMultiTileJ2K(
				tt.width, tt.height,
				tt.tileWidth, tt.tileHeight,
				tt.bitDepth, tt.numLevels, tt.components,
			)

			if len(data) == 0 {
				t.Fatal("Generated empty data")
			}

			// Parse the codestream
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
				t.Errorf("Width: got %d, want %d", cs.SIZ.Xsiz, tt.width)
			}

			if int(cs.SIZ.Ysiz) != tt.height {
				t.Errorf("Height: got %d, want %d", cs.SIZ.Ysiz, tt.height)
			}

			if int(cs.SIZ.XTsiz) != tt.tileWidth {
				t.Errorf("Tile width: got %d, want %d", cs.SIZ.XTsiz, tt.tileWidth)
			}

			if int(cs.SIZ.YTsiz) != tt.tileHeight {
				t.Errorf("Tile height: got %d, want %d", cs.SIZ.YTsiz, tt.tileHeight)
			}

			if int(cs.SIZ.Csiz) != tt.components {
				t.Errorf("Components: got %d, want %d", cs.SIZ.Csiz, tt.components)
			}

			// Verify tiles
			if len(cs.Tiles) != tt.expectedTiles {
				t.Errorf("Number of tiles: got %d, want %d", len(cs.Tiles), tt.expectedTiles)
			}

			// Verify COD segment
			if cs.COD == nil {
				t.Fatal("Missing COD segment")
			}

			// Verify MCT for RGB
			if tt.components == 3 {
				if cs.COD.MultipleComponentTransform != 1 {
					t.Errorf("MCT: got %d, want 1 for RGB", cs.COD.MultipleComponentTransform)
				}
			} else {
				if cs.COD.MultipleComponentTransform != 0 {
					t.Errorf("MCT: got %d, want 0 for grayscale", cs.COD.MultipleComponentTransform)
				}
			}
		})
	}
}

// TestGenerate2x2TileJ2K tests the convenience function
func TestGenerate2x2TileJ2K(t *testing.T) {
	data := Generate2x2TileJ2K()

	if len(data) == 0 {
		t.Fatal("Generated empty data")
	}

	parser := codestream.NewParser(data)
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(cs.Tiles) != 4 {
		t.Errorf("Expected 4 tiles, got %d", len(cs.Tiles))
	}

	if int(cs.SIZ.Xsiz) != 512 || int(cs.SIZ.Ysiz) != 512 {
		t.Errorf("Expected 512x512 image, got %dx%d", cs.SIZ.Xsiz, cs.SIZ.Ysiz)
	}
}

// TestGenerate3x2TileJ2K tests the 3x2 convenience function
func TestGenerate3x2TileJ2K(t *testing.T) {
	data := Generate3x2TileJ2K()

	if len(data) == 0 {
		t.Fatal("Generated empty data")
	}

	parser := codestream.NewParser(data)
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(cs.Tiles) != 6 {
		t.Errorf("Expected 6 tiles, got %d", len(cs.Tiles))
	}

	if int(cs.SIZ.Xsiz) != 600 || int(cs.SIZ.Ysiz) != 400 {
		t.Errorf("Expected 600x400 image, got %dx%d", cs.SIZ.Xsiz, cs.SIZ.Ysiz)
	}
}

// TestGenerate2x2TileRGBJ2K tests RGB multi-tile generation
func TestGenerate2x2TileRGBJ2K(t *testing.T) {
	data := Generate2x2TileRGBJ2K()

	if len(data) == 0 {
		t.Fatal("Generated empty data")
	}

	parser := codestream.NewParser(data)
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(cs.Tiles) != 4 {
		t.Errorf("Expected 4 tiles, got %d", len(cs.Tiles))
	}

	if int(cs.SIZ.Csiz) != 3 {
		t.Errorf("Expected 3 components, got %d", cs.SIZ.Csiz)
	}
}

// TestTileIndexing tests that tiles are indexed correctly
func TestTileIndexing(t *testing.T) {
	data := Generate2x2TileJ2K()

	parser := codestream.NewParser(data)
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Verify we have 4 tiles
	if len(cs.Tiles) != 4 {
		t.Fatalf("Expected 4 tiles, got %d", len(cs.Tiles))
	}

	// Each tile should have an SOT segment
	for i, tile := range cs.Tiles {
		if tile.SOT == nil {
			t.Errorf("Tile %d: missing SOT segment", i)
		} else if int(tile.SOT.Isot) != i {
			t.Errorf("Tile %d: SOT index is %d", i, tile.SOT.Isot)
		}
	}
}

// TestMultiTileWithLevels tests multi-tile with wavelet decomposition
func TestMultiTileWithLevels(t *testing.T) {
	tests := []struct {
		name   string
		levels int
	}{
		{"0 levels", 0},
		{"1 level", 1},
		{"2 levels", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := GenerateMultiTileJ2K(512, 512, 256, 256, 8, tt.levels, 1)

			parser := codestream.NewParser(data)
			cs, err := parser.Parse()
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			if int(cs.COD.NumberOfDecompositionLevels) != tt.levels {
				t.Errorf("Decomposition levels: got %d, want %d",
					cs.COD.NumberOfDecompositionLevels, tt.levels)
			}
		})
	}
}
