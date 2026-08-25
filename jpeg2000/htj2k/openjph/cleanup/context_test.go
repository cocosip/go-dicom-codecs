// Package htj2k provides tests for context computation and VLC decoding.
package cleanup

import (
	"testing"
)

func TestContextComputer(t *testing.T) {
	t.Run("BasicContextComputation", func(t *testing.T) {
		// Create 8x8 block
		cc := NewContextComputer(8, 8)

		// Test first row context (no top neighbors)
		ctx := cc.ComputeContext(0, 0, true)
		t.Logf("First quad (0,0) context: %d", ctx)
		if ctx != 0 {
			t.Errorf("Expected context 0 for first quad, got %d", ctx)
		}

		// Mark first quad as significant
		cc.UpdateQuadSignificance(0, 0, 0x0F) // All 4 samples significant

		// Test second quad in first row (context from left quad rho)
		ctx = cc.ComputeContext(1, 0, true)
		t.Logf("Second quad (1,0) context with left neighbors: %d", ctx)
		if ctx != 7 {
			t.Errorf("Expected context 7 for left rho=0x0F, got %d", ctx)
		}
	})

	t.Run("NonFirstRowContext", func(t *testing.T) {
		cc := NewContextComputer(8, 8)

		// Set up some significant samples in first row
		cc.UpdateQuadSignificance(0, 0, 0x0A) // rho bits 1 and 3
		cc.UpdateQuadSignificance(1, 0, 0x02) // rho bit 1

		// Test context for second row
		ctx := cc.ComputeContext(0, 1, false)
		t.Logf("Second row first quad (0,1) context: %d", ctx)
		if ctx != 5 {
			t.Errorf("Expected context 5 from top neighbors, got %d", ctx)
		}

		// Test context with both top and left neighbors
		cc.UpdateQuadSignificance(0, 1, 0x0C) // rho bits 2 and 3
		ctx = cc.ComputeContext(1, 1, false)
		t.Logf("Quad (1,1) with top and left neighbors context: %d", ctx)
		if ctx != 3 {
			t.Errorf("Expected context 3 with top+left neighbors, got %d", ctx)
		}
	})

	t.Run("SignificanceMap", func(t *testing.T) {
		cc := NewContextComputer(8, 8)

		// Test significance setting and querying
		cc.SetSignificant(2, 3, true)
		if !cc.IsSignificant(2, 3) {
			t.Error("Sample (2,3) should be significant")
		}

		// Test boundary conditions
		if cc.IsSignificant(-1, 0) {
			t.Error("Out-of-bounds sample should not be significant")
		}
		if cc.IsSignificant(10, 10) {
			t.Error("Out-of-bounds sample should not be significant")
		}
	})

	t.Run("QuadSignificanceUpdate", func(t *testing.T) {
		cc := NewContextComputer(8, 8)

		// Update quad with pattern 0b1010 (bottom-left and bottom-right significant)
		cc.UpdateQuadSignificance(1, 1, 0x0A)

		// Check individual samples
		// Quad at (1,1) corresponds to samples (2,2), (3,2), (2,3), (3,3)
		if cc.IsSignificant(2, 2) {
			t.Error("Sample (2,2) should not be significant (bit 0 = 0)")
		}
		if cc.IsSignificant(3, 2) {
			t.Error("Sample (3,2) should not be significant (bit 2 = 0)")
		}
		if !cc.IsSignificant(2, 3) {
			t.Error("Sample (2,3) should be significant (bit 1 = 1)")
		}
		if !cc.IsSignificant(3, 3) {
			t.Error("Sample (3,3) should be significant (bit 3 = 1)")
		}
	})

	t.Run("ContextProgression", func(t *testing.T) {
		cc := NewContextComputer(8, 8)

		// Simulate decoding progression
		contexts := []uint8{}

		// First row
		for qx := 0; qx < 4; qx++ {
			ctx := cc.ComputeContext(qx, 0, true)
			contexts = append(contexts, ctx)
			// Mark some quads as significant
			if qx%2 == 0 {
				cc.UpdateQuadSignificance(qx, 0, 0x0F)
			}
		}

		// Second row
		for qx := 0; qx < 4; qx++ {
			ctx := cc.ComputeContext(qx, 1, false)
			contexts = append(contexts, ctx)
			cc.UpdateQuadSignificance(qx, 1, 0x03)
		}

		t.Logf("Context progression: %v", contexts)

		// Verify contexts change as significance accumulates
		if contexts[0] != 0 {
			t.Errorf("First context should be 0, got %d", contexts[0])
		}

		// Check that non-first-row contexts differ from first-row
		firstRowUnique := make(map[uint8]bool)
		secondRowUnique := make(map[uint8]bool)
		for i := 0; i < 4; i++ {
			firstRowUnique[contexts[i]] = true
			secondRowUnique[contexts[i+4]] = true
		}

		t.Logf("First row unique contexts: %d", len(firstRowUnique))
		t.Logf("Second row unique contexts: %d", len(secondRowUnique))
	})
}

func TestVLCDecoderWithContext(t *testing.T) {
	t.Run("ContextBasedDecoding", func(t *testing.T) {
		// Create test VLC data
		testData := []byte{0xFF, 0x3F, 0x1F, 0x0F}
		decoder := NewVLCDecoder(testData)

		// Test decoding with different contexts
		contexts := []uint8{0, 1, 2, 3, 4}

		for _, ctx := range contexts {
			rho, uOff, eK, e1, found := decoder.DecodeQuadWithContext(ctx, true)
			if found {
				t.Logf("Context %d: rho=0x%X, u_off=%d, e_k=%d, e_1=%d",
					ctx, rho, uOff, eK, e1)
			}

			// Reset decoder for next test
			decoder = NewVLCDecoder(testData)
		}
	})

	t.Run("FirstRowVsNonFirstRow", func(t *testing.T) {
		testData := []byte{0x06, 0x3F, 0x00, 0x7F}
		decoder := NewVLCDecoder(testData)

		// Decode as first row
		rho1, uOff1, eK1, e11, found1 := decoder.DecodeQuadWithContext(1, true)
		if !found1 {
			t.Fatal("First row decoding failed")
		}
		t.Logf("First row:     rho=0x%X, u_off=%d, e_k=%d, e_1=%d",
			rho1, uOff1, eK1, e11)

		// Reset and decode as non-first row
		decoder = NewVLCDecoder(testData)
		rho2, uOff2, eK2, e12, found2 := decoder.DecodeQuadWithContext(1, false)
		if !found2 {
			t.Fatal("Non-first row decoding failed")
		}
		t.Logf("Non-first row: rho=0x%X, u_off=%d, e_k=%d, e_1=%d",
			rho2, uOff2, eK2, e12)

		// The decoded values may differ due to different table selection
	})
}

func BenchmarkContextComputation(b *testing.B) {
	cc := NewContextComputer(64, 64)

	// Set up a realistic significance pattern
	for qy := 0; qy < 32; qy++ {
		for qx := 0; qx < 32; qx++ {
			if (qx+qy)%3 == 0 {
				cc.UpdateQuadSignificance(qx, qy, 0x0F)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qx := i % 32
		qy := (i / 32) % 32
		isFirstRow := qy == 0
		_ = cc.ComputeContext(qx, qy, isFirstRow)
	}
}

func BenchmarkVLCDecoderWithContext(b *testing.B) {
	testData := make([]byte, 1024)
	for i := range testData {
		testData[i] = uint8(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoder := NewVLCDecoder(testData)
		ctx := uint8(i % 16)
		isFirstRow := (i % 2) == 0
		_, _, _, _, _ = decoder.DecodeQuadWithContext(ctx, isFirstRow)
	}
}
