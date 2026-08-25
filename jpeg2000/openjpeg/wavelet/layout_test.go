package wavelet

import "testing"

func TestLLDimensions(t *testing.T) {
	tests := []struct {
		name           string
		width, height  int
		levels         int
		expectedWidth  int
		expectedHeight int
	}{
		{name: "No levels", width: 888, height: 459, levels: 0, expectedWidth: 888, expectedHeight: 459},
		{name: "One level odd height", width: 888, height: 459, levels: 1, expectedWidth: 444, expectedHeight: 230},
		{name: "OpenJPEG parity match", width: 888, height: 459, levels: 5, expectedWidth: 28, expectedHeight: 15},
		{name: "Power of two", width: 512, height: 512, levels: 4, expectedWidth: 32, expectedHeight: 32},
		{name: "Tiny with many levels", width: 2, height: 1, levels: 10, expectedWidth: 1, expectedHeight: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llW, llH := LLDimensions(tt.width, tt.height, tt.levels)
			if llW != tt.expectedWidth || llH != tt.expectedHeight {
				t.Fatalf("LLDimensions(%d,%d,%d) = %dx%d, want %dx%d",
					tt.width, tt.height, tt.levels, llW, llH, tt.expectedWidth, tt.expectedHeight)
			}
		})
	}
}

func TestLLDimensionsWithParity(t *testing.T) {
	tests := []struct {
		name           string
		width, height  int
		levels         int
		x0, y0         int
		expectedWidth  int
		expectedHeight int
	}{
		// x0 odd means horizontal low-pass length is floor(n/2).
		{name: "Odd x0 affects width", width: 7, height: 6, levels: 2, x0: 1, y0: 0, expectedWidth: 1, expectedHeight: 2},
		// y0 odd means vertical low-pass length is floor(n/2).
		{name: "Odd y0 affects height", width: 8, height: 7, levels: 2, x0: 0, y0: 1, expectedWidth: 2, expectedHeight: 1},
		{name: "Negative dimensions return zero", width: -1, height: 7, levels: 2, x0: 0, y0: 0, expectedWidth: 0, expectedHeight: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llW, llH := LLDimensionsWithParity(tt.width, tt.height, tt.levels, tt.x0, tt.y0)
			if llW != tt.expectedWidth || llH != tt.expectedHeight {
				t.Fatalf("LLDimensionsWithParity(%d,%d,%d,%d,%d) = %dx%d, want %dx%d",
					tt.width, tt.height, tt.levels, tt.x0, tt.y0, llW, llH, tt.expectedWidth, tt.expectedHeight)
			}
		})
	}
}
