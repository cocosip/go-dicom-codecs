package testdata

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

// TestGenerateRGBJ2K tests RGB JPEG 2000 generation
func TestGenerateRGBJ2K(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		bitDepth  int
		numLevels int
	}{
		{"16x16 0-level", 16, 16, 8, 0},
		{"32x32 1-level", 32, 32, 8, 1},
		{"64x64 2-level", 64, 64, 8, 2},
		{"16x16 12-bit", 16, 16, 12, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := GenerateRGBJ2K(tt.width, tt.height, tt.bitDepth, tt.numLevels)

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

			if int(cs.SIZ.Csiz) != 3 {
				t.Errorf("Components: got %d, want 3", cs.SIZ.Csiz)
			}

			// Verify all 3 components have same bit depth
			for i := 0; i < 3; i++ {
				compBitDepth := cs.SIZ.Components[i].BitDepth()
				if compBitDepth != tt.bitDepth {
					t.Errorf("Component %d bit depth: got %d, want %d",
						i, compBitDepth, tt.bitDepth)
				}
			}

			// Verify COD segment
			if cs.COD == nil {
				t.Fatal("Missing COD segment")
			}

			if int(cs.COD.NumberOfDecompositionLevels) != tt.numLevels {
				t.Errorf("Decomposition levels: got %d, want %d",
					cs.COD.NumberOfDecompositionLevels, tt.numLevels)
			}

			// Verify MCT flag (should be 1 for RGB)
			if cs.COD.MultipleComponentTransform != 1 {
				t.Errorf("MCT: got %d, want 1 (enabled)",
					cs.COD.MultipleComponentTransform)
			}

			// Verify QCD segment
			if cs.QCD == nil {
				t.Fatal("Missing QCD segment")
			}
		})
	}
}

// TestGenerateRGBTestImage tests RGB test image generation
func TestGenerateRGBTestImage(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"4x4", 4, 4},
		{"8x8", 8, 8},
		{"16x16", 16, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := GenerateRGBTestImage(tt.width, tt.height)

			expectedLen := tt.width * tt.height * 3
			if len(data) != expectedLen {
				t.Fatalf("Data length: got %d, want %d", len(data), expectedLen)
			}

			// Verify all values are in [0, 255]
			for i, val := range data {
				if val < 0 || val > 255 {
					t.Errorf("Index %d: value %d out of range [0, 255]", i, val)
				}
			}

			// Verify corner pixels have expected pattern
			// Top-left should have low R (0), low G (0)
			if data[0] != 0 { // R
				t.Errorf("Top-left R: got %d, want 0", data[0])
			}
			if data[1] != 0 { // G
				t.Errorf("Top-left G: got %d, want 0", data[1])
			}

			// Top-right should have high R (255)
			topRightIdx := (tt.width - 1) * 3
			if data[topRightIdx] != 255 {
				t.Errorf("Top-right R: got %d, want 255", data[topRightIdx])
			}

			// Bottom-right should have high R (255), high G (255)
			bottomRightIdx := ((tt.height-1)*tt.width + (tt.width - 1)) * 3
			if data[bottomRightIdx] != 255 {
				t.Errorf("Bottom-right R: got %d, want 255", data[bottomRightIdx])
			}
			if data[bottomRightIdx+1] != 255 {
				t.Errorf("Bottom-right G: got %d, want 255", data[bottomRightIdx+1])
			}
		})
	}
}

// TestGenerateRGBComponents tests separate component generation
func TestGenerateRGBComponents(t *testing.T) {
	width, height := 8, 8

	r, g, b := GenerateRGBComponents(width, height)

	if len(r) != width*height {
		t.Errorf("R length: got %d, want %d", len(r), width*height)
	}
	if len(g) != width*height {
		t.Errorf("G length: got %d, want %d", len(g), width*height)
	}
	if len(b) != width*height {
		t.Errorf("B length: got %d, want %d", len(b), width*height)
	}

	// Verify gradients
	// R: horizontal gradient (left=0, right=255)
	if r[0] != 0 {
		t.Errorf("R[0,0]: got %d, want 0", r[0])
	}
	if r[width-1] != 255 {
		t.Errorf("R[0,%d]: got %d, want 255", width-1, r[width-1])
	}

	// G: vertical gradient (top=0, bottom=255)
	if g[0] != 0 {
		t.Errorf("G[0,0]: got %d, want 0", g[0])
	}
	if g[(height-1)*width] != 255 {
		t.Errorf("G[%d,0]: got %d, want 255", height-1, g[(height-1)*width])
	}

	// B: inverse horizontal gradient (left=255, right=0)
	if b[0] != 255 {
		t.Errorf("B[0,0]: got %d, want 255", b[0])
	}
	if b[width-1] != 0 {
		t.Errorf("B[0,%d]: got %d, want 0", width-1, b[width-1])
	}
}

// TestGenerateSolidColorRGB tests solid color generation
func TestGenerateSolidColorRGB(t *testing.T) {
	tests := []struct {
		name        string
		red         int32
		green       int32
		blue        int32
		description string
	}{
		{"Red", 255, 0, 0, "Pure red"},
		{"Green", 0, 255, 0, "Pure green"},
		{"Blue", 0, 0, 255, "Pure blue"},
		{"White", 255, 255, 255, "White"},
		{"Black", 0, 0, 0, "Black"},
		{"Gray", 128, 128, 128, "Gray"},
	}

	width, height := 8, 8

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := GenerateSolidColorRGB(width, height, tt.red, tt.green, tt.blue)

			expectedLen := width * height * 3
			if len(data) != expectedLen {
				t.Fatalf("Data length: got %d, want %d", len(data), expectedLen)
			}

			// Verify all pixels have the same color
			for i := 0; i < width*height; i++ {
				r := data[i*3]
				g := data[i*3+1]
				b := data[i*3+2]

				if r != tt.red {
					t.Errorf("Pixel %d R: got %d, want %d", i, r, tt.red)
				}
				if g != tt.green {
					t.Errorf("Pixel %d G: got %d, want %d", i, g, tt.green)
				}
				if b != tt.blue {
					t.Errorf("Pixel %d B: got %d, want %d", i, b, tt.blue)
				}
			}
		})
	}
}

// TestGenerateColorBarsRGB tests color bars generation
func TestGenerateColorBarsRGB(t *testing.T) {
	width, height := 70, 10 // 70 = 7 bars * 10 pixels each

	data := GenerateColorBarsRGB(width, height)

	expectedLen := width * height * 3
	if len(data) != expectedLen {
		t.Fatalf("Data length: got %d, want %d", len(data), expectedLen)
	}

	// Verify color bars
	barWidth := 10

	// Check first pixel of each bar
	expectedColors := [][3]int32{
		{255, 255, 255}, // White
		{255, 255, 0},   // Yellow
		{0, 255, 255},   // Cyan
		{0, 255, 0},     // Green
		{255, 0, 255},   // Magenta
		{255, 0, 0},     // Red
		{0, 0, 255},     // Blue
	}

	for bar := 0; bar < 7; bar++ {
		x := bar * barWidth
		idx := x * 3

		r := data[idx]
		g := data[idx+1]
		b := data[idx+2]

		if r != expectedColors[bar][0] {
			t.Errorf("Bar %d R: got %d, want %d", bar, r, expectedColors[bar][0])
		}
		if g != expectedColors[bar][1] {
			t.Errorf("Bar %d G: got %d, want %d", bar, g, expectedColors[bar][1])
		}
		if b != expectedColors[bar][2] {
			t.Errorf("Bar %d B: got %d, want %d", bar, b, expectedColors[bar][2])
		}
	}

	// Verify bars are uniform (all pixels in a bar have same color)
	for bar := 0; bar < 7; bar++ {
		x0 := bar * barWidth
		x1 := x0 + barWidth

		// Check all pixels in this bar
		for x := x0; x < x1 && x < width; x++ {
			idx := x * 3
			r := data[idx]
			g := data[idx+1]
			b := data[idx+2]

			if r != expectedColors[bar][0] || g != expectedColors[bar][1] || b != expectedColors[bar][2] {
				t.Errorf("Bar %d, pixel %d: got RGB(%d,%d,%d), want RGB(%d,%d,%d)",
					bar, x, r, g, b,
					expectedColors[bar][0], expectedColors[bar][1], expectedColors[bar][2])
			}
		}
	}
}

// TestRGBCodestreamStructure tests the structure of generated RGB codestream
func TestRGBCodestreamStructure(t *testing.T) {
	data := GenerateRGBJ2K(32, 32, 8, 1)

	parser := codestream.NewParser(data)
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Verify marker sequence
	if cs.SIZ == nil {
		t.Error("Missing SIZ")
	}
	if cs.COD == nil {
		t.Error("Missing COD")
	}
	if cs.QCD == nil {
		t.Error("Missing QCD")
	}
	if len(cs.Tiles) == 0 {
		t.Error("No tiles")
	}

	// Verify component configuration
	if len(cs.SIZ.Components) != 3 {
		t.Fatalf("Expected 3 components, got %d", len(cs.SIZ.Components))
	}

	// All components should have same sampling (1x1)
	for i := 0; i < 3; i++ {
		comp := cs.SIZ.Components[i]
		if comp.XRsiz != 1 || comp.YRsiz != 1 {
			t.Errorf("Component %d sampling: got %dx%d, want 1x1",
				i, comp.XRsiz, comp.YRsiz)
		}
	}
}
