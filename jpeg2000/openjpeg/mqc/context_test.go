package mqc

import (
	"testing"
)

// TestContextPreservation verifies that NewMQDecoderWithContexts correctly preserves contexts
func TestContextPreservation(t *testing.T) {
	// Create first decoder and decode some data
	data1 := []byte{0x1F, 0xFE, 0x00}
	mqc1 := NewMQDecoder(data1, 19)

	// Decode a few symbols to modify context states
	_ = mqc1.Decode(0)
	_ = mqc1.Decode(1)
	_ = mqc1.Decode(2)

	// Get contexts after decoding
	contexts1 := mqc1.GetContexts()
	t.Logf("Contexts after first decoder: %v", contexts1[:5])

	// Create second decoder with inherited contexts
	data2 := []byte{0x00, 0x1F, 0xFE, 0x00}
	mqc2 := NewMQDecoderWithContexts(data2, contexts1)

	// Verify contexts were copied
	contexts2 := mqc2.GetContexts()
	t.Logf("Contexts in second decoder: %v", contexts2[:5])

	if len(contexts1) != len(contexts2) {
		t.Errorf("Context length mismatch: got %d, want %d", len(contexts2), len(contexts1))
	}

	for i := range contexts1 {
		if contexts1[i] != contexts2[i] {
			t.Errorf("Context[%d] mismatch: got %d, want %d", i, contexts2[i], contexts1[i])
		}
	}

	t.Log("Context preservation test passed!")
}
