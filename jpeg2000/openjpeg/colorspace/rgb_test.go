package colorspace

import (
	"math"
	"testing"
)

// TestRGBToYCbCr tests RGB to YCbCr conversion
func TestRGBToYCbCr(t *testing.T) {
	tests := []struct {
		name       string
		r, g, b    int32
		wantY      int32
		wantCb     int32
		wantCr     int32
		tolerance  int32
	}{
		{
			name:      "Black (0,0,0)",
			r:         0,
			g:         0,
			b:         0,
			wantY:     0,
			wantCb:    0,
			wantCr:    0,
			tolerance: 1,
		},
		{
			name:      "White (255,255,255)",
			r:         255,
			g:         255,
			b:         255,
			wantY:     255,
			wantCb:    0,
			wantCr:    0,
			tolerance: 1,
		},
		{
			name:      "Red (255,0,0)",
			r:         255,
			g:         0,
			b:         0,
			wantY:     76,
			wantCb:    -43,
			wantCr:    128,
			tolerance: 2,
		},
		{
			name:      "Green (0,255,0)",
			r:         0,
			g:         255,
			b:         0,
			wantY:     150,
			wantCb:    -84,
			wantCr:    -107,
			tolerance: 2,
		},
		{
			name:      "Blue (0,0,255)",
			r:         0,
			g:         0,
			b:         255,
			wantY:     29,
			wantCb:    128,
			wantCr:    -21,
			tolerance: 2,
		},
		{
			name:      "Gray (128,128,128)",
			r:         128,
			g:         128,
			b:         128,
			wantY:     128,
			wantCb:    0,
			wantCr:    0,
			tolerance: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			y, cb, cr := RGBToYCbCr(tt.r, tt.g, tt.b)

			if abs(y-tt.wantY) > tt.tolerance {
				t.Errorf("Y: got %d, want %d (±%d)", y, tt.wantY, tt.tolerance)
			}
			if abs(cb-tt.wantCb) > tt.tolerance {
				t.Errorf("Cb: got %d, want %d (±%d)", cb, tt.wantCb, tt.tolerance)
			}
			if abs(cr-tt.wantCr) > tt.tolerance {
				t.Errorf("Cr: got %d, want %d (±%d)", cr, tt.wantCr, tt.tolerance)
			}
		})
	}
}

// TestYCbCrToRGB tests YCbCr to RGB conversion
func TestYCbCrToRGB(t *testing.T) {
	tests := []struct {
		name       string
		y, cb, cr  int32
		wantR      int32
		wantG      int32
		wantB      int32
		tolerance  int32
	}{
		{
			name:      "Black YCbCr",
			y:         0,
			cb:        0,
			cr:        0,
			wantR:     0,
			wantG:     0,
			wantB:     0,
			tolerance: 1,
		},
		{
			name:      "White YCbCr",
			y:         255,
			cb:        0,
			cr:        0,
			wantR:     255,
			wantG:     255,
			wantB:     255,
			tolerance: 1,
		},
		{
			name:      "Mid Gray",
			y:         128,
			cb:        0,
			cr:        0,
			wantR:     128,
			wantG:     128,
			wantB:     128,
			tolerance: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b := YCbCrToRGB(tt.y, tt.cb, tt.cr)

			if abs(r-tt.wantR) > tt.tolerance {
				t.Errorf("R: got %d, want %d (±%d)", r, tt.wantR, tt.tolerance)
			}
			if abs(g-tt.wantG) > tt.tolerance {
				t.Errorf("G: got %d, want %d (±%d)", g, tt.wantG, tt.tolerance)
			}
			if abs(b-tt.wantB) > tt.tolerance {
				t.Errorf("B: got %d, want %d (±%d)", b, tt.wantB, tt.tolerance)
			}
		})
	}
}

// TestRoundTripConversion tests RGB → YCbCr → RGB round-trip
func TestRoundTripConversion(t *testing.T) {
	tests := []struct {
		name      string
		r, g, b   int32
		tolerance int32
	}{
		{"Black", 0, 0, 0, 1},
		{"White", 255, 255, 255, 1},
		{"Red", 255, 0, 0, 5},
		{"Green", 0, 255, 0, 5},
		{"Blue", 0, 0, 255, 5},
		{"Cyan", 0, 255, 255, 5},
		{"Magenta", 255, 0, 255, 5},
		{"Yellow", 255, 255, 0, 5},
		{"Mid Gray", 128, 128, 128, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// RGB → YCbCr → RGB
			y, cb, cr := RGBToYCbCr(tt.r, tt.g, tt.b)
			r2, g2, b2 := YCbCrToRGB(y, cb, cr)

			if abs(r2-tt.r) > tt.tolerance {
				t.Errorf("R round-trip: %d → %d (diff=%d, tolerance=%d)",
					tt.r, r2, abs(r2-tt.r), tt.tolerance)
			}
			if abs(g2-tt.g) > tt.tolerance {
				t.Errorf("G round-trip: %d → %d (diff=%d, tolerance=%d)",
					tt.g, g2, abs(g2-tt.g), tt.tolerance)
			}
			if abs(b2-tt.b) > tt.tolerance {
				t.Errorf("B round-trip: %d → %d (diff=%d, tolerance=%d)",
					tt.b, b2, abs(b2-tt.b), tt.tolerance)
			}
		})
	}
}

// TestConvertRGBToYCbCr tests image conversion from RGB to YCbCr
func TestConvertRGBToYCbCr(t *testing.T) {
	// Create a 2x2 RGB image
	// [Red, Green]
	// [Blue, White]
	rgb := []int32{
		255, 0, 0, // Red
		0, 255, 0, // Green
		0, 0, 255, // Blue
		255, 255, 255, // White
	}

	y, cb, cr := ConvertRGBToYCbCr(rgb, 2, 2)

	if len(y) != 4 || len(cb) != 4 || len(cr) != 4 {
		t.Fatalf("Expected 4 pixels per component, got Y:%d Cb:%d Cr:%d",
			len(y), len(cb), len(cr))
	}

	// Check Red pixel (approximate values)
	if abs(y[0]-76) > 2 {
		t.Errorf("Red Y: got %d, want ~76", y[0])
	}

	// Check White pixel
	if abs(y[3]-255) > 1 {
		t.Errorf("White Y: got %d, want 255", y[3])
	}
	if abs(cb[3]-0) > 1 {
		t.Errorf("White Cb: got %d, want 0", cb[3])
	}
	if abs(cr[3]-0) > 1 {
		t.Errorf("White Cr: got %d, want 0", cr[3])
	}
}

// TestConvertYCbCrToRGB tests image conversion from YCbCr to RGB
func TestConvertYCbCrToRGB(t *testing.T) {
	// Create a 2x2 YCbCr image
	y := []int32{0, 128, 255, 64}
	cb := []int32{0, 0, 0, 0}
	cr := []int32{0, 0, 0, 0}

	rgb := ConvertYCbCrToRGB(y, cb, cr, 2, 2)

	if len(rgb) != 12 { // 4 pixels * 3 components
		t.Fatalf("Expected 12 values, got %d", len(rgb))
	}

	// Check first pixel (black)
	if rgb[0] != 0 || rgb[1] != 0 || rgb[2] != 0 {
		t.Errorf("First pixel (black): got RGB(%d,%d,%d), want (0,0,0)",
			rgb[0], rgb[1], rgb[2])
	}

	// Check third pixel (white)
	if rgb[6] != 255 || rgb[7] != 255 || rgb[8] != 255 {
		t.Errorf("Third pixel (white): got RGB(%d,%d,%d), want (255,255,255)",
			rgb[6], rgb[7], rgb[8])
	}
}

// TestInterleaveComponents tests component interleaving
func TestInterleaveComponents(t *testing.T) {
	components := [][]int32{
		{0, 1, 2, 3},     // Component 0
		{10, 11, 12, 13}, // Component 1
		{20, 21, 22, 23}, // Component 2
	}

	result := InterleaveComponents(components)

	expected := []int32{
		0, 10, 20, // Pixel 0
		1, 11, 21, // Pixel 1
		2, 12, 22, // Pixel 2
		3, 13, 23, // Pixel 3
	}

	if len(result) != len(expected) {
		t.Fatalf("Expected length %d, got %d", len(expected), len(result))
	}

	for i, v := range expected {
		if result[i] != v {
			t.Errorf("Index %d: got %d, want %d", i, result[i], v)
		}
	}
}

// TestDeinterleaveComponents tests component deinterleaving
func TestDeinterleaveComponents(t *testing.T) {
	data := []int32{
		0, 10, 20, // Pixel 0
		1, 11, 21, // Pixel 1
		2, 12, 22, // Pixel 2
		3, 13, 23, // Pixel 3
	}

	components := DeinterleaveComponents(data, 3)

	if len(components) != 3 {
		t.Fatalf("Expected 3 components, got %d", len(components))
	}

	expectedC0 := []int32{0, 1, 2, 3}
	expectedC1 := []int32{10, 11, 12, 13}
	expectedC2 := []int32{20, 21, 22, 23}

	for i, v := range expectedC0 {
		if components[0][i] != v {
			t.Errorf("Component 0, index %d: got %d, want %d", i, components[0][i], v)
		}
	}

	for i, v := range expectedC1 {
		if components[1][i] != v {
			t.Errorf("Component 1, index %d: got %d, want %d", i, components[1][i], v)
		}
	}

	for i, v := range expectedC2 {
		if components[2][i] != v {
			t.Errorf("Component 2, index %d: got %d, want %d", i, components[2][i], v)
		}
	}
}

// TestRoundTripInterleaving tests interleave → deinterleave round-trip
func TestRoundTripInterleaving(t *testing.T) {
	original := [][]int32{
		{0, 1, 2, 3},
		{10, 11, 12, 13},
		{20, 21, 22, 23},
	}

	interleaved := InterleaveComponents(original)
	deinterleaved := DeinterleaveComponents(interleaved, 3)

	if len(deinterleaved) != len(original) {
		t.Fatalf("Expected %d components, got %d", len(original), len(deinterleaved))
	}

	for c := 0; c < len(original); c++ {
		for i := 0; i < len(original[c]); i++ {
			if deinterleaved[c][i] != original[c][i] {
				t.Errorf("Component %d, index %d: got %d, want %d",
					c, i, deinterleaved[c][i], original[c][i])
			}
		}
	}
}

// TestEdgeCases tests edge cases
func TestEdgeCases(t *testing.T) {
	t.Run("Empty components", func(t *testing.T) {
		result := InterleaveComponents([][]int32{})
		if result != nil {
			t.Error("Expected nil for empty components")
		}
	})

	t.Run("Empty data", func(t *testing.T) {
		result := DeinterleaveComponents([]int32{}, 3)
		if result != nil {
			t.Error("Expected nil for empty data")
		}
	})

	t.Run("Zero components", func(t *testing.T) {
		result := DeinterleaveComponents([]int32{1, 2, 3}, 0)
		if result != nil {
			t.Error("Expected nil for zero components")
		}
	})
}

// TestColorSpaceAccuracy tests conversion accuracy
func TestColorSpaceAccuracy(t *testing.T) {
	// Test a range of values to ensure reasonable accuracy
	maxError := 0.0
	totalError := 0.0
	count := 0

	for r := int32(0); r <= 255; r += 32 {
		for g := int32(0); g <= 255; g += 32 {
			for b := int32(0); b <= 255; b += 32 {
				// Round trip
				y, cb, cr := RGBToYCbCr(r, g, b)
				r2, g2, b2 := YCbCrToRGB(y, cb, cr)

				// Calculate error
				errR := float64(abs(r2 - r))
				errG := float64(abs(g2 - g))
				errB := float64(abs(b2 - b))

				maxErr := math.Max(math.Max(errR, errG), errB)
				if maxErr > maxError {
					maxError = maxErr
				}

				totalError += errR + errG + errB
				count += 3
			}
		}
	}

	avgError := totalError / float64(count)

	t.Logf("Color space conversion accuracy:")
	t.Logf("  Max error: %.2f", maxError)
	t.Logf("  Avg error: %.2f", avgError)

	if maxError > 10 {
		t.Errorf("Maximum error too high: %.2f (expected ≤ 10)", maxError)
	}

	if avgError > 2 {
		t.Errorf("Average error too high: %.2f (expected ≤ 2)", avgError)
	}
}

// Helper function for absolute value
func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}
