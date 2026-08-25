package t1

import (
	"testing"
)

// TestT1DecoderBasic tests basic decoder creation and structure
func TestT1DecoderBasic(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		cblkstyle int
	}{
		{"4x4 block", 4, 4, 0},
		{"8x8 block", 8, 8, 0},
		{"16x16 block", 16, 16, 0},
		{"32x32 block", 32, 32, 0},
		{"64x64 block", 64, 64, 0},
		{"Non-square 8x4", 8, 4, 0},
		{"With reset context", 32, 32, 0x02},
		{"With termall", 32, 32, 0x04},
		{"With segmentation", 32, 32, 0x20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewT1Decoder(tt.width, tt.height, tt.cblkstyle)

			if decoder == nil {
				t.Fatal("decoder is nil")
			}

			if decoder.width != tt.width {
				t.Errorf("width = %d, want %d", decoder.width, tt.width)
			}

			if decoder.height != tt.height {
				t.Errorf("height = %d, want %d", decoder.height, tt.height)
			}

			// Check data and flags arrays are properly sized (with padding)
			expectedSize := (tt.width + 2) * (tt.height + 2)
			if len(decoder.data) != expectedSize {
				t.Errorf("data size = %d, want %d", len(decoder.data), expectedSize)
			}

			if len(decoder.flags) != expectedSize {
				t.Errorf("flags size = %d, want %d", len(decoder.flags), expectedSize)
			}
		})
	}
}

// TestContextModeling tests the context modeling functions
func TestContextModeling(t *testing.T) {
	t.Run("Zero Coding Context", func(t *testing.T) {
		tests := []struct {
			name  string
			flags uint32
		}{
			{"No significant neighbors", 0},
			{"One horizontal neighbor", T1SigE},
			{"One vertical neighbor", T1SigN},
			{"One diagonal neighbor", T1SigNE},
			{"All neighbors significant", T1SigN | T1SigS | T1SigE | T1SigW | T1SigNE | T1SigNW | T1SigSE | T1SigSW},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := getZeroCodingContext(tt.flags, 0)
				// Context should be in valid range [0, 8]
				if ctx < CTXZCSTART || ctx > CTXZCSTART+8 {
					t.Errorf("getZeroCodingContext() = %d, out of valid range [%d, %d]",
						ctx, CTXZCSTART, CTXZCSTART+8)
				}
			})
		}
	})

	t.Run("Sign Coding Context", func(t *testing.T) {
		// Basic test - context should be in range [9, 13]
		tests := []struct {
			name  string
			flags uint32
		}{
			{"No sign neighbors", 0},
			{"Positive east", T1SigE | T1SignE},
			{"Negative east", T1SigE},
			{"Mixed signs", T1SigE | T1SignE | T1SigW},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := getSignCodingContext(tt.flags)
				// Context should be in absolute range [9, 13]
				if ctx < 9 || ctx > 13 {
					t.Errorf("getSignCodingContext() = %d, want in range [9, 13]", ctx)
				}
			})
		}
	})

	t.Run("Magnitude Refinement Context", func(t *testing.T) {
		tests := []struct {
			name  string
			flags uint32
		}{
			{"No neighbors", 0},
			{"One neighbor", T1SigE},
			{"Three neighbors", T1SigE | T1SigW | T1SigN},
			{"All neighbors", T1SigN | T1SigS | T1SigE | T1SigW | T1SigNE | T1SigNW | T1SigSE | T1SigSW},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := getMagRefinementContext(tt.flags)
				// Context should be in range [CTX_MR_START+0, CTX_MR_START+2] = [14, 16]
				if ctx < CTXMRSTART || ctx > CTXMRSTART+2 {
					t.Errorf("getMagRefinementContext() = %d, out of valid range [%d, %d]",
						ctx, CTXMRSTART, CTXMRSTART+2)
				}
			})
		}
	})
}

// TestSignPrediction tests the sign prediction functionality
func TestSignPrediction(t *testing.T) {
	tests := []struct {
		name  string
		flags uint32
	}{
		{"No neighbors", 0},
		{"Positive neighbors", T1SigE | T1SignE | T1SigW | T1SignW},
		{"Negative neighbors", T1SigE | T1SigW},
		{"Mixed neighbors", T1SigE | T1SignE | T1SigW},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := getSignPrediction(tt.flags)
			// Prediction should be 0 or 1
			if pred != 0 && pred != 1 {
				t.Errorf("getSignPrediction() = %d, want 0 or 1", pred)
			}
		})
	}
}

// TestGetData tests the GetData method
func TestGetData(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"4x4", 4, 4},
		{"8x8", 8, 8},
		{"16x16", 16, 16},
		{"8x4", 8, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewT1Decoder(tt.width, tt.height, 0)

			// Set some test data
			paddedWidth := tt.width + 2
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					idx := (y+1)*paddedWidth + (x + 1)
					decoder.data[idx] = int32(y*tt.width + x)
				}
			}

			// Get data without padding
			result := decoder.GetData()

			if len(result) != tt.width*tt.height {
				t.Errorf("result length = %d, want %d", len(result), tt.width*tt.height)
			}

			// Verify values
			for i := 0; i < len(result); i++ {
				if result[i] != int32(i) {
					t.Errorf("result[%d] = %d, want %d", i, result[i], i)
				}
			}
		})
	}
}

// TestUpdateNeighborFlags tests neighbor flag updates
func TestUpdateNeighborFlags(t *testing.T) {
	t.Run("Center position", func(t *testing.T) {
		decoder := NewT1Decoder(8, 8, 0)
		paddedWidth := 8 + 2

		// Set a coefficient as significant at position (4, 4)
		x, y := 4, 4
		idx := (y+1)*paddedWidth + (x + 1)
		decoder.flags[idx] = T1Sig | T1Sign

		// Update neighbor flags
		decoder.updateNeighborFlags(x, y, idx)

		// Check north neighbor
		nIdx := (y)*paddedWidth + (x + 1)
		if decoder.flags[nIdx]&T1SigS == 0 {
			t.Error("North neighbor should have T1SigS flag")
		}
		if decoder.flags[nIdx]&T1SignS == 0 {
			t.Error("North neighbor should have T1SignS flag")
		}

		// Check south neighbor
		sIdx := (y+2)*paddedWidth + (x + 1)
		if decoder.flags[sIdx]&T1SigN == 0 {
			t.Error("South neighbor should have T1SigN flag")
		}

		// Check east neighbor
		eIdx := (y+1)*paddedWidth + (x + 2)
		if decoder.flags[eIdx]&T1SigW == 0 {
			t.Error("East neighbor should have T1SigW flag")
		}

		// Check west neighbor
		wIdx := (y+1)*paddedWidth + x
		if decoder.flags[wIdx]&T1SigE == 0 {
			t.Error("West neighbor should have T1SigE flag")
		}

		// Check diagonal neighbors
		neIdx := (y)*paddedWidth + (x + 2)
		if decoder.flags[neIdx]&T1SigSW == 0 {
			t.Error("Northeast neighbor should have T1SigSW flag")
		}
	})

	t.Run("Corner position", func(t *testing.T) {
		decoder := NewT1Decoder(8, 8, 0)

		// Top-left corner
		x, y := 0, 0
		paddedWidth := 8 + 2
		idx := (y+1)*paddedWidth + (x + 1)
		decoder.flags[idx] = T1Sig

		decoder.updateNeighborFlags(x, y, idx)

		// Only south and east neighbors should be updated
		sIdx := (y+2)*paddedWidth + (x + 1)
		if decoder.flags[sIdx]&T1SigN == 0 {
			t.Error("South neighbor should be updated at corner")
		}

		eIdx := (y+1)*paddedWidth + (x + 2)
		if decoder.flags[eIdx]&T1SigW == 0 {
			t.Error("East neighbor should be updated at corner")
		}
	})
}

// TestContextTables tests the initialization of context lookup tables
func TestContextTables(t *testing.T) {
	t.Run("Sign Context LUT", func(t *testing.T) {
		// All values should be in valid absolute range [9, 13] (CTX_SC_START to CTX_SC_END)
		// Updated: lut_ctxno_sc now stores absolute values from OpenJPEG
		for i, v := range lutCtxnoSc {
			if v < CTXSCSTART || v > CTXSCEND {
				t.Errorf("lut_ctxno_sc[%d] = %d, out of range [%d, %d]", i, v, CTXSCSTART, CTXSCEND)
			}
		}
	})

	t.Run("Sign Prediction LUT", func(t *testing.T) {
		// All values should be 0 or 1
		for i, v := range lutSpb {
			if v != 0 && v != 1 {
				t.Errorf("lut_spb[%d] = %d, want 0 or 1", i, v)
			}
		}
	})
}

// TestCodeBlockStyle tests code block style flag parsing
func TestCodeBlockStyle(t *testing.T) {
	tests := []struct {
		name             string
		cblkstyle        int
		wantResetCtx     bool
		wantTermAll      bool
		wantSegmentation bool
	}{
		{"No flags", 0x00, false, false, false},
		{"Reset context", 0x02, true, false, false},
		{"Terminate all", 0x04, false, true, false},
		{"Segmentation", 0x20, false, false, true},
		{"All flags", 0x26, true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewT1Decoder(32, 32, tt.cblkstyle)

			if decoder.resetctx != tt.wantResetCtx {
				t.Errorf("resetctx = %v, want %v", decoder.resetctx, tt.wantResetCtx)
			}

			if decoder.termall != tt.wantTermAll {
				t.Errorf("termall = %v, want %v", decoder.termall, tt.wantTermAll)
			}

			if decoder.segmentation != tt.wantSegmentation {
				t.Errorf("segmentation = %v, want %v", decoder.segmentation, tt.wantSegmentation)
			}
		})
	}
}

// TestEmptyData tests decoder behavior with empty data
func TestEmptyData(t *testing.T) {
	decoder := NewT1Decoder(32, 32, 0)
	err := decoder.Decode([]byte{}, 0, 0)

	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
}

// TestConstants tests that all constants are properly defined
func TestConstants(t *testing.T) {
	t.Run("Context ranges", func(t *testing.T) {
		if CTXZCSTART != 0 {
			t.Errorf("CTX_ZC_START = %d, want 0", CTXZCSTART)
		}
		if CTXZCEND != 8 {
			t.Errorf("CTX_ZC_END = %d, want 8", CTXZCEND)
		}
		if CTXSCSTART != 9 {
			t.Errorf("CTX_SC_START = %d, want 9", CTXSCSTART)
		}
		if CTXRL != 17 {
			t.Errorf("CTX_RL = %d, want 17", CTXRL)
		}
		if CTXUNI != 18 {
			t.Errorf("CTX_UNI = %d, want 18", CTXUNI)
		}
		if NUMCONTEXTS != 19 {
			t.Errorf("NUM_CONTEXTS = %d, want 19", NUMCONTEXTS)
		}
	})

	t.Run("Flag bits", func(t *testing.T) {
		// Verify flags don't overlap (except for combined masks)
		flags := []uint32{
			T1Sig, T1Refine, T1Visit,
			T1SigN, T1SigS, T1SigE, T1SigW,
			T1SigNE, T1SigNW, T1SigSE, T1SigSW,
			T1Sign, T1SignN, T1SignS, T1SignE, T1SignW,
		}

		for i := 0; i < len(flags); i++ {
			for j := i + 1; j < len(flags); j++ {
				if flags[i]&flags[j] != 0 {
					t.Errorf("Flags overlap: 0x%X & 0x%X = 0x%X",
						flags[i], flags[j], flags[i]&flags[j])
				}
			}
		}
	})
}
