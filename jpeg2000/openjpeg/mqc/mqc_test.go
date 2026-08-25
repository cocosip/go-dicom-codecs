package mqc

import (
	"bytes"
	"testing"
)

// TestMQDecoderBasic tests basic MQ decoder functionality
func TestMQDecoderBasic(t *testing.T) {
	// Test with a simple encoded sequence
	// This is a placeholder - in real usage, data comes from EBCOT encoding

	data := []byte{0x00, 0x00, 0x00, 0x00}
	mqc := NewMQDecoder(data, 10)

	// Decode some bits
	// Note: Without a proper encoder, we can't validate exact values
	// This test mainly checks that the decoder doesn't crash
	for i := 0; i < 10; i++ {
		bit := mqc.Decode(0)
		if bit != 0 && bit != 1 {
			t.Errorf("Decoded bit should be 0 or 1, got %d", bit)
		}
	}
}

// TestMQDecoderContexts tests multiple contexts
func TestMQDecoderContexts(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00}
	mqc := NewMQDecoder(data, 100)

	// Test different contexts
	contexts := []int{0, 1, 5, 10, 50, 99}
	for _, ctx := range contexts {
		bit := mqc.Decode(ctx)
		if bit != 0 && bit != 1 {
			t.Errorf("Context %d: decoded bit should be 0 or 1, got %d", ctx, bit)
		}
	}
}

// TestMQDecoderStateTransitions tests state transitions
func TestMQDecoderStateTransitions(t *testing.T) {
	data := []byte{0xFF, 0xFF, 0xFF, 0xFF}
	mqc := NewMQDecoder(data, 10)

	// Decode multiple bits from same context
	// This should trigger state transitions
	ctx := 0
	for i := 0; i < 20; i++ {
		bit := mqc.Decode(ctx)
		_ = bit // We just want to ensure no panics

		// Get state after decode (lower 7 bits only, bit 7 is MPS)
		contextByte := mqc.GetContextState(ctx)
		state := contextByte & 0x7F
		if state > 46 {
			t.Errorf("Invalid state: %d (max is 46), full context byte: 0x%02x", state, contextByte)
		}
	}
}

// TestMQDecoderResetContext tests context reset
func TestMQDecoderResetContext(t *testing.T) {
	data := []byte{0x00, 0x00}
	mqc := NewMQDecoder(data, 5)

	ctx := 0

	// Decode some bits to change state
	for i := 0; i < 5; i++ {
		mqc.Decode(ctx)
	}

	// Get state before reset
	stateBefore := mqc.GetContextState(ctx)

	// Reset context
	mqc.ResetContext(ctx)

	// Get state after reset
	stateAfter := mqc.GetContextState(ctx)

	if stateAfter != 0 {
		t.Errorf("Context state after reset should be 0, got %d", stateAfter)
	}

	if stateBefore == stateAfter {
		// This is OK if it was already 0, but log it
		t.Logf("State was already 0 before reset")
	}
}

// TestMQDecoderByteStuffing tests byte stuffing handling
func TestMQDecoderByteStuffing(t *testing.T) {
	// Create data with 0xFF byte (which triggers byte stuffing)
	data := []byte{0xFF, 0x00, 0xFF, 0x00}
	mqc := NewMQDecoder(data, 5)

	// Decoder should handle byte stuffing without crashing
	for i := 0; i < 20; i++ {
		bit := mqc.Decode(0)
		if bit != 0 && bit != 1 {
			t.Errorf("Invalid bit value: %d", bit)
		}
	}
}

// TestMQDecoderExhaustedData tests behavior when data is exhausted
func TestMQDecoderExhaustedData(t *testing.T) {
	// Very short data
	data := []byte{0x80}
	mqc := NewMQDecoder(data, 5)

	// Try to decode more bits than available
	// Decoder should pad with 0xFF and not crash
	for i := 0; i < 50; i++ {
		bit := mqc.Decode(0)
		if bit != 0 && bit != 1 {
			t.Errorf("Invalid bit value: %d", bit)
		}
	}
}

// TestQeTable tests that Qe table has correct values
func TestQeTable(t *testing.T) {
	if len(qeTable) != 47 {
		t.Errorf("qeTable should have 47 entries, got %d", len(qeTable))
	}

	// First entry should be 0x5601
	if qeTable[0] != 0x5601 {
		t.Errorf("qeTable[0] should be 0x5601, got 0x%04X", qeTable[0])
	}

	// Last entry should be 0x5601
	if qeTable[46] != 0x5601 {
		t.Errorf("qeTable[46] should be 0x5601, got 0x%04X", qeTable[46])
	}
}

// TestStateTransitionTables tests state transition tables
func TestStateTransitionTables(t *testing.T) {
	if len(nmpsTable) != 47 {
		t.Errorf("nmpsTable should have 47 entries, got %d", len(nmpsTable))
	}

	if len(nlpsTable) != 47 {
		t.Errorf("nlpsTable should have 47 entries, got %d", len(nlpsTable))
	}

	// All next states should be valid (0-46)
	for i, state := range nmpsTable {
		if state > 46 {
			t.Errorf("nmpsTable[%d] = %d is invalid (max 46)", i, state)
		}
	}

	for i, state := range nlpsTable {
		if state > 46 {
			t.Errorf("nlpsTable[%d] = %d is invalid (max 46)", i, state)
		}
	}
}

// TestMQDecoderMultipleContexts tests decoding with multiple independent contexts
func TestMQDecoderMultipleContexts(t *testing.T) {
	data := []byte{0xAA, 0x55, 0xAA, 0x55} // Alternating pattern
	numContexts := 10
	mqc := NewMQDecoder(data, numContexts)

	// Decode from different contexts
	results := make(map[int][]int)
	for ctx := 0; ctx < numContexts; ctx++ {
		results[ctx] = make([]int, 0)
		for i := 0; i < 5; i++ {
			bit := mqc.Decode(ctx)
			results[ctx] = append(results[ctx], bit)
		}
	}

	// Each context should maintain its own state
	// Just verify we got some bits
	for ctx, bits := range results {
		if len(bits) != 5 {
			t.Errorf("Context %d: expected 5 bits, got %d", ctx, len(bits))
		}
	}
}

// TestMQDecoderZeroData tests decoder with all-zero data
func TestMQDecoderZeroData(t *testing.T) {
	data := make([]byte, 10)
	mqc := NewMQDecoder(data, 5)

	// Decode some bits - should not crash
	for i := 0; i < 30; i++ {
		bit := mqc.Decode(i % 5)
		if bit != 0 && bit != 1 {
			t.Errorf("Invalid bit value: %d", bit)
		}
	}
}

// TestMQDecoderAllOnesData tests decoder with all 0xFF data
func TestMQDecoderAllOnesData(t *testing.T) {
	data := bytes.Repeat([]byte{0xFF}, 10)
	mqc := NewMQDecoder(data, 5)

	// Decode some bits - should handle byte stuffing
	for i := 0; i < 30; i++ {
		bit := mqc.Decode(i % 5)
		if bit != 0 && bit != 1 {
			t.Errorf("Invalid bit value: %d", bit)
		}
	}
}

// BenchmarkMQDecode benchmarks MQ decoding
func BenchmarkMQDecode(b *testing.B) {
	data := bytes.Repeat([]byte{0xAA, 0x55}, 1000)
	mqc := NewMQDecoder(data, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mqc.Decode(i % 10)
	}
}

// BenchmarkMQDecodeMultiContext benchmarks decoding with multiple contexts
func BenchmarkMQDecodeMultiContext(b *testing.B) {
	data := bytes.Repeat([]byte{0xAA, 0x55, 0x33, 0xCC}, 1000)
	numContexts := 100
	mqc := NewMQDecoder(data, numContexts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mqc.Decode(i % numContexts)
	}
}
