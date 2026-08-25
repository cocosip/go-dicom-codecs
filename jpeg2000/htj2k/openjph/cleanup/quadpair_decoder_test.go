package cleanup

import (
	"testing"
)

func TestQuadPairDecoder_InitialLinePair(t *testing.T) {
	// Test initial line-pair (first row) decoding
	// This tests the special case where both quads have ulf=1

	// Create minimal VLC data for testing
	// This is a simplified test - in real usage, data comes from actual encoding
	vlcData := make([]byte, 64)

	decoder := NewQuadPairDecoder(vlcData, 4, 4) // 4x4 quads

	// Decode first quad-pair in initial line (qy=0)
	pair, err := decoder.DecodeQuadPair(0, 0)
	if err != nil {
		t.Fatalf("DecodeQuadPair() error = %v", err)
	}

	// Verify it's recognized as initial line-pair
	if !pair.IsInitialLinePair {
		t.Errorf("Expected IsInitialLinePair=true, got false")
	}

	// Verify both quads are present (width=4, so first pair has both)
	if !pair.HasSecondQuad {
		t.Errorf("Expected HasSecondQuad=true for pair 0 in width=4")
	}

	t.Logf("Initial line-pair decoded: q1.rho=%04b, q2.rho=%04b", pair.Rho1, pair.Rho2)
}

func TestQuadPairDecoder_NonInitialLinePair(t *testing.T) {
	// Test non-initial line-pair (rows after first)

	vlcData := make([]byte, 64)
	decoder := NewQuadPairDecoder(vlcData, 4, 4)

	// Decode quad-pair in second row (qy=1)
	pair, err := decoder.DecodeQuadPair(0, 1)
	if err != nil {
		t.Fatalf("DecodeQuadPair() error = %v", err)
	}

	// Verify it's NOT initial line-pair
	if pair.IsInitialLinePair {
		t.Errorf("Expected IsInitialLinePair=false for qy=1, got true")
	}

	t.Logf("Non-initial line-pair decoded: q1.rho=%04b, q2.rho=%04b", pair.Rho1, pair.Rho2)
}

func TestQuadPairDecoder_OddWidth(t *testing.T) {
	// Test quad-pair decoding when QW is odd
	// The last quad-pair in each row should only have first quad

	vlcData := make([]byte, 64)
	decoder := NewQuadPairDecoder(vlcData, 5, 4) // 5 quads wide (odd)

	// Last quad-pair (g=2) in a row of width 5
	// q1 = 2*2 = 4, q2 = 2*2+1 = 5 (out of bounds)
	pair, err := decoder.DecodeQuadPair(2, 0)
	if err != nil {
		t.Fatalf("DecodeQuadPair() error = %v", err)
	}

	// Verify second quad is not present
	if pair.HasSecondQuad {
		t.Errorf("Expected HasSecondQuad=false for last pair in odd width, got true")
	}

	t.Logf("Odd-width last pair: HasSecondQuad=%v", pair.HasSecondQuad)
}

func TestQuadPairDecoder_ULFFlags(t *testing.T) {
	// Test unsigned residual offset flag handling

	tests := []struct {
		name     string
		qy       int
		wantInit bool
	}{
		{
			name:     "InitialRow",
			qy:       0,
			wantInit: true,
		},
		{
			name:     "SecondRow",
			qy:       1,
			wantInit: false,
		},
		{
			name:     "ThirdRow",
			qy:       2,
			wantInit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vlcData := make([]byte, 64)
			decoder := NewQuadPairDecoder(vlcData, 4, 4)

			pair, err := decoder.DecodeQuadPair(0, tt.qy)
			if err != nil {
				t.Fatalf("DecodeQuadPair() error = %v", err)
			}

			if pair.IsInitialLinePair != tt.wantInit {
				t.Errorf("IsInitialLinePair = %v, want %v", pair.IsInitialLinePair, tt.wantInit)
			}

			// ULF flags should be 0 or 1
			if pair.ULF1 > 1 {
				t.Errorf("ULF1 = %d, should be 0 or 1", pair.ULF1)
			}
			if pair.ULF2 > 1 {
				t.Errorf("ULF2 = %d, should be 0 or 1", pair.ULF2)
			}

			t.Logf("Row %d: ULF1=%d, ULF2=%d, Uq1=%d, Uq2=%d",
				tt.qy, pair.ULF1, pair.ULF2, pair.Uq1, pair.Uq2)
		})
	}
}

func TestQuadPairDecoder_DecodeAllQuadPairs(t *testing.T) {
	// Test decoding all quad-pairs in a block

	tests := []struct {
		name         string
		widthQuads   int
		heightQuads  int
		wantPairsPer int
		wantTotal    int
	}{
		{
			name:         "4x4 block",
			widthQuads:   4,
			heightQuads:  4,
			wantPairsPer: 2, // ceil(4/2) = 2 pairs per row
			wantTotal:    8, // 2 pairs * 4 rows = 8 total
		},
		{
			name:         "5x3 block (odd width)",
			widthQuads:   5,
			heightQuads:  3,
			wantPairsPer: 3, // ceil(5/2) = 3 pairs per row
			wantTotal:    9, // 3 pairs * 3 rows = 9 total
		},
		{
			name:         "2x2 block (minimal)",
			widthQuads:   2,
			heightQuads:  2,
			wantPairsPer: 1, // ceil(2/2) = 1 pair per row
			wantTotal:    2, // 1 pair * 2 rows = 2 total
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vlcData := make([]byte, 256)
			decoder := NewQuadPairDecoder(vlcData, tt.widthQuads, tt.heightQuads)

			pairs, err := decoder.DecodeAllQuadPairs(tt.heightQuads)
			if err != nil {
				t.Fatalf("DecodeAllQuadPairs() error = %v", err)
			}

			if len(pairs) != tt.wantTotal {
				t.Errorf("Got %d quad-pairs, want %d", len(pairs), tt.wantTotal)
			}

			// Verify first row is all initial line-pairs
			pairsInFirstRow := 0
			for _, pair := range pairs {
				if pair.IsInitialLinePair {
					pairsInFirstRow++
				}
			}

			if pairsInFirstRow != tt.wantPairsPer {
				t.Errorf("Got %d initial line-pairs, want %d", pairsInFirstRow, tt.wantPairsPer)
			}

			t.Logf("Decoded %d quad-pairs (%d pairs/row x %d rows)",
				len(pairs), tt.wantPairsPer, tt.heightQuads)
		})
	}
}

func TestQuadPairDecoder_SpecialCases(t *testing.T) {
	// Test special U-VLC decoding cases

	t.Run("BothULF1_InitialPair", func(t *testing.T) {
		// When both quads in initial line-pair have ulf=1
		// This should use Formula (4): u = 2 + u_pfx + u_sfx + 4*u_ext

		// This test verifies the logic path, actual values depend on VLC data
		vlcData := make([]byte, 64)
		decoder := NewQuadPairDecoder(vlcData, 4, 4)

		pair, err := decoder.DecodeQuadPair(0, 0) // Initial line-pair
		if err != nil {
			t.Fatalf("DecodeQuadPair() error = %v", err)
		}

		// If both ULF are 1, verify we're in initial line-pair
		if pair.ULF1 == 1 && pair.ULF2 == 1 {
			if !pair.IsInitialLinePair {
				t.Errorf("Both ULF=1 but not initial line-pair")
			}
			t.Logf("Both quads have ULF=1 in initial pair: Uq1=%d, Uq2=%d", pair.Uq1, pair.Uq2)
		}
	})

	t.Run("Uq1_GreaterThan2", func(t *testing.T) {
		// When first quad has uq1>2, second quad uses simplified decoding
		// (single bit: ubit+1, giving either 1 or 2)

		vlcData := make([]byte, 64)
		decoder := NewQuadPairDecoder(vlcData, 4, 4)

		pair, err := decoder.DecodeQuadPair(0, 1) // Non-initial row
		if err != nil {
			t.Fatalf("DecodeQuadPair() error = %v", err)
		}

		// If first quad has uq1>2 and both have ulf=1
		if pair.ULF1 == 1 && pair.Uq1 > 2 && pair.ULF2 == 1 {
			// Second quad should be 1 or 2 (from simplified decoding)
			if pair.Uq2 != 1 && pair.Uq2 != 2 {
				t.Errorf("With uq1>2, expected uq2 ∈ {1,2}, got %d", pair.Uq2)
			}
			t.Logf("Simplified decoding used: Uq1=%d, Uq2=%d", pair.Uq1, pair.Uq2)
		}
	})
}

func TestGetQuadInfo(t *testing.T) {
	// Test helper function for extracting quad info

	pair := &QuadPairResult{
		Rho1:  0b0101,
		ULF1:  1,
		Uq1:   5,
		E1_1:  2,
		EMax1: 7,

		Rho2:  0b1010,
		ULF2:  0,
		Uq2:   0,
		E1_2:  0,
		EMax2: 3,
	}

	// Get first quad info
	rho1, ulf1, uq1, e1_1, emax1 := GetQuadInfo(pair, 0)
	if rho1 != 0b0101 || ulf1 != 1 || uq1 != 5 || e1_1 != 2 || emax1 != 7 {
		t.Errorf("First quad info mismatch: rho=%04b, ulf=%d, uq=%d, e1=%d, emax=%d",
			rho1, ulf1, uq1, e1_1, emax1)
	}

	// Get second quad info
	rho2, ulf2, uq2, e1_2, emax2 := GetQuadInfo(pair, 1)
	if rho2 != 0b1010 || ulf2 != 0 || uq2 != 0 || e1_2 != 0 || emax2 != 3 {
		t.Errorf("Second quad info mismatch: rho=%04b, ulf=%d, uq=%d, e1=%d, emax=%d",
			rho2, ulf2, uq2, e1_2, emax2)
	}
}

func TestQuadPairDecoder_Integration(t *testing.T) {
	// Integration test with realistic block dimensions

	t.Run("8x8_block", func(t *testing.T) {
		// 8x8 sample block = 4x4 quad block
		vlcData := make([]byte, 256)
		decoder := NewQuadPairDecoder(vlcData, 4, 4)

		pairs, err := decoder.DecodeAllQuadPairs(4)
		if err != nil {
			t.Fatalf("DecodeAllQuadPairs() error = %v", err)
		}

		// Should have 8 quad-pairs (2 per row * 4 rows)
		if len(pairs) != 8 {
			t.Errorf("Expected 8 quad-pairs for 4x4 quad block, got %d", len(pairs))
		}

		// Count quads in each row
		var rowCounts [4]int
		pairIdx := 0
		for qy := 0; qy < 4; qy++ {
			for g := 0; g < 2; g++ { // 2 pairs per row
				pair := pairs[pairIdx]
				pairIdx++

				// First quad always exists
				rowCounts[qy]++

				// Second quad exists if HasSecondQuad
				if pair.HasSecondQuad {
					rowCounts[qy]++
				}
			}
		}

		// Each row should have 4 quads
		for qy := 0; qy < 4; qy++ {
			if rowCounts[qy] != 4 {
				t.Errorf("Row %d has %d quads, expected 4", qy, rowCounts[qy])
			}
		}

		t.Logf("Successfully decoded 8x8 block: %d quad-pairs", len(pairs))
	})
}
