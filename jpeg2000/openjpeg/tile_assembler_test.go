package openjpeg

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

// TestTileLayout tests tile layout calculations
func TestTileLayout(t *testing.T) {
	tests := []struct {
		name        string
		imageWidth  uint32
		imageHeight uint32
		tileWidth   uint32
		tileHeight  uint32
		wantTilesX  int
		wantTilesY  int
	}{
		{
			name:        "Single tile",
			imageWidth:  256,
			imageHeight: 256,
			tileWidth:   256,
			tileHeight:  256,
			wantTilesX:  1,
			wantTilesY:  1,
		},
		{
			name:        "2x2 tiles",
			imageWidth:  512,
			imageHeight: 512,
			tileWidth:   256,
			tileHeight:  256,
			wantTilesX:  2,
			wantTilesY:  2,
		},
		{
			name:        "3x2 tiles",
			imageWidth:  600,
			imageHeight: 400,
			tileWidth:   256,
			tileHeight:  256,
			wantTilesX:  3,
			wantTilesY:  2,
		},
		{
			name:        "Non-aligned tiles",
			imageWidth:  500,
			imageHeight: 300,
			tileWidth:   256,
			tileHeight:  256,
			wantTilesX:  2,
			wantTilesY:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			siz := &codestream.SIZSegment{
				Xsiz:   tt.imageWidth,
				Ysiz:   tt.imageHeight,
				XOsiz:  0,
				YOsiz:  0,
				XTsiz:  tt.tileWidth,
				YTsiz:  tt.tileHeight,
				XTOsiz: 0,
				YTOsiz: 0,
				Csiz:   1,
			}

			layout := NewTileLayout(siz)

			if layout.numTilesX != tt.wantTilesX {
				t.Errorf("NumTilesX: got %d, want %d", layout.numTilesX, tt.wantTilesX)
			}

			if layout.numTilesY != tt.wantTilesY {
				t.Errorf("NumTilesY: got %d, want %d", layout.numTilesY, tt.wantTilesY)
			}

			expectedCount := tt.wantTilesX * tt.wantTilesY
			if layout.GetTileCount() != expectedCount {
				t.Errorf("TileCount: got %d, want %d", layout.GetTileCount(), expectedCount)
			}
		})
	}
}

// TestTileBounds tests tile boundary calculations
func TestTileBounds(t *testing.T) {
	// Create 2x2 tile layout: 512x512 image with 256x256 tiles
	siz := &codestream.SIZSegment{
		Xsiz:   512,
		Ysiz:   512,
		XOsiz:  0,
		YOsiz:  0,
		XTsiz:  256,
		YTsiz:  256,
		XTOsiz: 0,
		YTOsiz: 0,
		Csiz:   1,
	}

	layout := NewTileLayout(siz)

	tests := []struct {
		name           string
		tileIdx        int
		wantX0, wantY0 int
		wantX1, wantY1 int
		wantWidth      int
		wantHeight     int
	}{
		{"Tile 0 (top-left)", 0, 0, 0, 256, 256, 256, 256},
		{"Tile 1 (top-right)", 1, 256, 0, 512, 256, 256, 256},
		{"Tile 2 (bottom-left)", 2, 0, 256, 256, 512, 256, 256},
		{"Tile 3 (bottom-right)", 3, 256, 256, 512, 512, 256, 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x0, y0, x1, y1 := layout.GetTileBounds(tt.tileIdx)

			if x0 != tt.wantX0 || y0 != tt.wantY0 || x1 != tt.wantX1 || y1 != tt.wantY1 {
				t.Errorf("Bounds: got (%d,%d,%d,%d), want (%d,%d,%d,%d)",
					x0, y0, x1, y1, tt.wantX0, tt.wantY0, tt.wantX1, tt.wantY1)
			}

			width, height := layout.GetTileSize(tt.tileIdx)
			if width != tt.wantWidth || height != tt.wantHeight {
				t.Errorf("Size: got %dx%d, want %dx%d",
					width, height, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

// TestTileBoundsNonAligned tests tiles with non-aligned image dimensions
func TestTileBoundsNonAligned(t *testing.T) {
	// 500x300 image with 256x256 tiles
	// Should create 2x2 tile grid, but edge tiles are smaller
	siz := &codestream.SIZSegment{
		Xsiz:   500,
		Ysiz:   300,
		XOsiz:  0,
		YOsiz:  0,
		XTsiz:  256,
		YTsiz:  256,
		XTOsiz: 0,
		YTOsiz: 0,
		Csiz:   1,
	}

	layout := NewTileLayout(siz)

	// Tile 1 (right edge): should be 244 pixels wide (500 - 256)
	x0, y0, x1, y1 := layout.GetTileBounds(1)
	if x0 != 256 || y0 != 0 || x1 != 500 || y1 != 256 {
		t.Errorf("Tile 1 bounds: got (%d,%d,%d,%d), want (256,0,500,256)",
			x0, y0, x1, y1)
	}

	width, height := layout.GetTileSize(1)
	if width != 244 || height != 256 {
		t.Errorf("Tile 1 size: got %dx%d, want 244x256", width, height)
	}

	// Tile 2 (bottom edge): should be 44 pixels high (300 - 256)
	x0, y0, x1, y1 = layout.GetTileBounds(2)
	if x0 != 0 || y0 != 256 || x1 != 256 || y1 != 300 {
		t.Errorf("Tile 2 bounds: got (%d,%d,%d,%d), want (0,256,256,300)",
			x0, y0, x1, y1)
	}

	width, height = layout.GetTileSize(2)
	if width != 256 || height != 44 {
		t.Errorf("Tile 2 size: got %dx%d, want 256x44", width, height)
	}

	// Tile 3 (bottom-right corner): should be 244x44
	x0, y0, x1, y1 = layout.GetTileBounds(3)
	if x0 != 256 || y0 != 256 || x1 != 500 || y1 != 300 {
		t.Errorf("Tile 3 bounds: got (%d,%d,%d,%d), want (256,256,500,300)",
			x0, y0, x1, y1)
	}

	width, height = layout.GetTileSize(3)
	if width != 244 || height != 44 {
		t.Errorf("Tile 3 size: got %dx%d, want 244x44", width, height)
	}
}

// TestTileAssemblerSingleTile tests assembling a single tile
func TestTileAssemblerSingleTile(t *testing.T) {
	siz := &codestream.SIZSegment{
		Xsiz:   256,
		Ysiz:   256,
		XOsiz:  0,
		YOsiz:  0,
		XTsiz:  256,
		YTsiz:  256,
		XTOsiz: 0,
		YTOsiz: 0,
		Csiz:   1,
		Components: []codestream.ComponentSize{
			{Ssiz: 7}, // 8-bit unsigned
		},
	}

	assembler := NewTileAssembler(siz)

	// Create tile data
	tileData := make([][]int32, 1)
	tileData[0] = make([]int32, 256*256)
	for i := range tileData[0] {
		tileData[0][i] = int32(i % 256)
	}

	// Assemble tile
	err := assembler.AssembleTile(0, tileData)
	if err != nil {
		t.Fatalf("AssembleTile failed: %v", err)
	}

	// Verify image data
	imageData := assembler.GetImageData()
	if len(imageData) != 1 {
		t.Fatalf("Expected 1 component, got %d", len(imageData))
	}

	if len(imageData[0]) != 256*256 {
		t.Fatalf("Expected 65536 pixels, got %d", len(imageData[0]))
	}

	// Verify data matches
	for i := range imageData[0] {
		if imageData[0][i] != tileData[0][i] {
			t.Errorf("Pixel %d: got %d, want %d", i, imageData[0][i], tileData[0][i])
			break
		}
	}
}

// TestTileAssemblerMultipleTiles tests assembling a 2x2 tile grid
func TestTileAssemblerMultipleTiles(t *testing.T) {
	siz := &codestream.SIZSegment{
		Xsiz:   512,
		Ysiz:   512,
		XOsiz:  0,
		YOsiz:  0,
		XTsiz:  256,
		YTsiz:  256,
		XTOsiz: 0,
		YTOsiz: 0,
		Csiz:   1,
		Components: []codestream.ComponentSize{
			{Ssiz: 7},
		},
	}

	assembler := NewTileAssembler(siz)

	// Create and assemble 4 tiles
	for tileIdx := 0; tileIdx < 4; tileIdx++ {
		tileData := make([][]int32, 1)
		tileData[0] = make([]int32, 256*256)

		// Fill with tile-specific value
		fillValue := int32((tileIdx + 1) * 10)
		for i := range tileData[0] {
			tileData[0][i] = fillValue
		}

		err := assembler.AssembleTile(tileIdx, tileData)
		if err != nil {
			t.Fatalf("AssembleTile(%d) failed: %v", tileIdx, err)
		}
	}

	// Verify assembled image
	imageData := assembler.GetImageData()

	// Check specific pixels from each tile
	tests := []struct {
		x, y int
		want int32
	}{
		{0, 0, 10},     // Tile 0 (top-left)
		{300, 0, 20},   // Tile 1 (top-right)
		{0, 300, 30},   // Tile 2 (bottom-left)
		{300, 300, 40}, // Tile 3 (bottom-right)
	}

	for _, tt := range tests {
		idx := tt.y*512 + tt.x
		if imageData[0][idx] != tt.want {
			t.Errorf("Pixel (%d,%d): got %d, want %d",
				tt.x, tt.y, imageData[0][idx], tt.want)
		}
	}
}

// TestTileAssemblerMultiComponent tests multi-component tile assembly
func TestTileAssemblerMultiComponent(t *testing.T) {
	// RGB image: 512x512, 3 components, 2x2 tiles
	siz := &codestream.SIZSegment{
		Xsiz:   512,
		Ysiz:   512,
		XOsiz:  0,
		YOsiz:  0,
		XTsiz:  256,
		YTsiz:  256,
		XTOsiz: 0,
		YTOsiz: 0,
		Csiz:   3,
		Components: []codestream.ComponentSize{
			{Ssiz: 7}, // R
			{Ssiz: 7}, // G
			{Ssiz: 7}, // B
		},
	}

	assembler := NewTileAssembler(siz)

	// Assemble 4 tiles with different colors
	colors := [][]int32{
		{255, 0, 0},     // Tile 0: Red
		{0, 255, 0},     // Tile 1: Green
		{0, 0, 255},     // Tile 2: Blue
		{255, 255, 255}, // Tile 3: White
	}

	for tileIdx := 0; tileIdx < 4; tileIdx++ {
		tileData := make([][]int32, 3)
		for c := 0; c < 3; c++ {
			tileData[c] = make([]int32, 256*256)
			for i := range tileData[c] {
				tileData[c][i] = colors[tileIdx][c]
			}
		}

		err := assembler.AssembleTile(tileIdx, tileData)
		if err != nil {
			t.Fatalf("AssembleTile(%d) failed: %v", tileIdx, err)
		}
	}

	// Verify colors
	imageData := assembler.GetImageData()

	// Check tile 0 (red)
	idx := 100*512 + 100
	if imageData[0][idx] != 255 || imageData[1][idx] != 0 || imageData[2][idx] != 0 {
		t.Errorf("Tile 0 color: got RGB(%d,%d,%d), want (255,0,0)",
			imageData[0][idx], imageData[1][idx], imageData[2][idx])
	}

	// Check tile 3 (white)
	idx = 400*512 + 400
	if imageData[0][idx] != 255 || imageData[1][idx] != 255 || imageData[2][idx] != 255 {
		t.Errorf("Tile 3 color: got RGB(%d,%d,%d), want (255,255,255)",
			imageData[0][idx], imageData[1][idx], imageData[2][idx])
	}
}

// TestTileAssemblerErrors tests error conditions
func TestTileAssemblerErrors(t *testing.T) {
	siz := &codestream.SIZSegment{
		Xsiz:   512,
		Ysiz:   512,
		XOsiz:  0,
		YOsiz:  0,
		XTsiz:  256,
		YTsiz:  256,
		XTOsiz: 0,
		YTOsiz: 0,
		Csiz:   1,
		Components: []codestream.ComponentSize{
			{Ssiz: 7},
		},
	}

	assembler := NewTileAssembler(siz)

	t.Run("Invalid tile index", func(t *testing.T) {
		tileData := make([][]int32, 1)
		tileData[0] = make([]int32, 256*256)

		err := assembler.AssembleTile(-1, tileData)
		if err == nil {
			t.Error("Expected error for negative tile index")
		}

		err = assembler.AssembleTile(4, tileData)
		if err == nil {
			t.Error("Expected error for out-of-range tile index")
		}
	})

	t.Run("Wrong component count", func(t *testing.T) {
		tileData := make([][]int32, 2) // 2 components instead of 1
		tileData[0] = make([]int32, 256*256)
		tileData[1] = make([]int32, 256*256)

		err := assembler.AssembleTile(0, tileData)
		if err == nil {
			t.Error("Expected error for wrong component count")
		}
	})

	t.Run("Wrong tile data size", func(t *testing.T) {
		tileData := make([][]int32, 1)
		tileData[0] = make([]int32, 100) // Wrong size

		err := assembler.AssembleTile(0, tileData)
		if err == nil {
			t.Error("Expected error for wrong tile data size")
		}
	})
}
