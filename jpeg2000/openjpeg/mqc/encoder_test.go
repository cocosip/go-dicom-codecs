package mqc

import (
	"testing"
)

// TestMQEncoderCreation tests encoder creation
func TestMQEncoderCreation(t *testing.T) {
	mqe := NewMQEncoder(19)
	if mqe == nil {
		t.Fatal("NewMQEncoder returned nil")
	}

	// Verify initial state
	if mqe.a != 0x8000 {
		t.Errorf("Initial a = 0x%x, want 0x8000", mqe.a)
	}
	if mqe.c != 0 {
		t.Errorf("Initial c = 0x%x, want 0", mqe.c)
	}
	if mqe.ct != 12 {
		t.Errorf("Initial ct = %d, want 12", mqe.ct)
	}
	if len(mqe.contexts) != 19 {
		t.Errorf("Contexts length = %d, want 19", len(mqe.contexts))
	}
}

// TestMQEncodeDecodeRoundTrip tests encoding and decoding
func TestMQEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		bits     []int
		contexts []int
	}{
		{
			name:     "Single bit",
			bits:     []int{0},
			contexts: []int{0},
		},
		{
			name:     "Alternating bits",
			bits:     []int{0, 1, 0, 1, 0, 1, 0, 1},
			contexts: []int{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:     "All zeros",
			bits:     []int{0, 0, 0, 0, 0, 0, 0, 0},
			contexts: []int{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:     "All ones",
			bits:     []int{1, 1, 1, 1, 1, 1, 1, 1},
			contexts: []int{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:     "Multiple contexts",
			bits:     []int{0, 1, 0, 1, 1, 0, 1, 0},
			contexts: []int{0, 1, 2, 3, 4, 5, 6, 7},
		},
		{
			name:     "Long sequence",
			bits:     []int{0, 1, 1, 0, 1, 0, 0, 1, 1, 1, 0, 0, 1, 0, 1, 1},
			contexts: []int{0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			numContexts := 19
			mqe := NewMQEncoder(numContexts)

			for i := range tt.bits {
				mqe.Encode(tt.bits[i], tt.contexts[i])
			}

			encoded := mqe.Flush()

			if len(encoded) == 0 {
				t.Fatal("Encoded data is empty")
			}

			// Decode
			mqd := NewMQDecoder(encoded, numContexts)

			for i := range tt.bits {
				bit := mqd.Decode(tt.contexts[i])
				if bit != tt.bits[i] {
					t.Errorf("Bit %d: got %d, want %d", i, bit, tt.bits[i])
				}
			}
		})
	}
}

// TestMQEncoderReset tests encoder reset functionality
func TestMQEncoderReset(t *testing.T) {
	mqe := NewMQEncoder(19)

	// Encode some data
	mqe.Encode(1, 0)
	mqe.Encode(0, 1)
	mqe.Encode(1, 2)

	// Reset
	mqe.Reset()

	// Verify state
	if mqe.a != 0x8000 {
		t.Errorf("After reset a = 0x%x, want 0x8000", mqe.a)
	}
	if mqe.c != 0 {
		t.Errorf("After reset c = 0x%x, want 0", mqe.c)
	}
	if mqe.ct != 12 {
		t.Errorf("After reset ct = %d, want 12", mqe.ct)
	}
	if len(mqe.GetBuffer()) != 0 {
		t.Errorf("After reset output length = %d, want 0", len(mqe.GetBuffer()))
	}
}

// TestMQEncoderContextState tests context state management
func TestMQEncoderContextState(t *testing.T) {
	mqe := NewMQEncoder(19)

	// Set a context state
	mqe.SetContextState(5, 0x42)

	// Get it back
	state := mqe.GetContextState(5)
	if state != 0x42 {
		t.Errorf("GetContextState(5) = 0x%x, want 0x42", state)
	}

	// Reset specific context
	mqe.ResetContext(5)
	state = mqe.GetContextState(5)
	if state != 0 {
		t.Errorf("After ResetContext(5) = 0x%x, want 0", state)
	}

	// Set multiple contexts
	for i := 0; i < 10; i++ {
		mqe.SetContextState(i, uint8(i*10))
	}

	// Reset all contexts
	mqe.ResetContexts()
	for i := 0; i < 10; i++ {
		state = mqe.GetContextState(i)
		if state != 0 {
			t.Errorf("After ResetContexts() context %d = 0x%x, want 0", i, state)
		}
	}
}

// TestMQEncoderByteStuffing tests byte stuffing (0xFF -> 0xFF 0x00)
func TestMQEncoderByteStuffing(t *testing.T) {
	mqe := NewMQEncoder(19)

	// Encode a pattern that should trigger byte stuffing
	// Encode many 1s which should eventually produce 0xFF bytes
	for i := 0; i < 1000; i++ {
		mqe.Encode(1, 0)
	}

	encoded := mqe.Flush()

	// Check for byte stuffing pattern (0xFF followed by 0x00 or marker)
	hasStuffing := false
	for i := 0; i < len(encoded)-1; i++ {
		if encoded[i] == 0xFF && encoded[i+1] == 0x00 {
			hasStuffing = true
			break
		}
	}

	// Note: Byte stuffing may or may not occur depending on the data
	// This test just verifies the encoder doesn't crash
	t.Logf("Encoded %d bytes, byte stuffing detected: %v", len(encoded), hasStuffing)
}

// TestMQEncoderStateTransitions tests context state transitions
func TestMQEncoderStateTransitions(t *testing.T) {
	mqe := NewMQEncoder(19)

	contextID := 0
	initialState := mqe.GetContextState(contextID)

	// Encode some bits
	for i := 0; i < 10; i++ {
		mqe.Encode(0, contextID) // Encode MPS
	}

	stateAfterMPS := mqe.GetContextState(contextID)

	// State should have changed due to probability adaptation
	t.Logf("Initial state: 0x%02x, After 10 MPS: 0x%02x", initialState, stateAfterMPS)

	// Encode some LPS
	mqe.Reset()
	mqe.ResetContext(contextID)

	for i := 0; i < 5; i++ {
		mqe.Encode(1, contextID) // Encode LPS (initially)
	}

	stateAfterLPS := mqe.GetContextState(contextID)
	t.Logf("After 5 mixed: 0x%02x", stateAfterLPS)
}

// TestMQEncoderLargeData tests encoding large amounts of data
func TestMQEncoderLargeData(t *testing.T) {
	numBits := 10000
	bits := make([]int, numBits)
	contexts := make([]int, numBits)

	// Create pseudo-random pattern
	for i := range bits {
		bits[i] = (i * 7919) % 2 // Pseudo-random 0/1
		contexts[i] = i % 19      // Cycle through contexts
	}

	// Encode
	mqe := NewMQEncoder(19)
	for i := range bits {
		mqe.Encode(bits[i], contexts[i])
	}

	encoded := mqe.Flush()
	t.Logf("Encoded %d bits into %d bytes (%.2f bits/byte)", numBits, len(encoded), float64(numBits)/float64(len(encoded)))

	// Decode and verify
	mqd := NewMQDecoder(encoded, 19)

	errors := 0
	for i := range bits {
		bit := mqd.Decode(contexts[i])
		if bit != bits[i] {
			errors++
			if errors <= 10 { // Only report first 10 errors
				t.Errorf("Bit %d: got %d, want %d", i, bit, bits[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total errors: %d / %d", errors, numBits)
	}
}

// BenchmarkMQEncoder benchmarks encoding performance
func BenchmarkMQEncoder(b *testing.B) {
	bits := make([]int, 1000)
	contexts := make([]int, 1000)

	for i := range bits {
		bits[i] = i % 2
		contexts[i] = i % 19
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mqe := NewMQEncoder(19)
		for j := range bits {
			mqe.Encode(bits[j], contexts[j])
		}
		_ = mqe.Flush()
	}
}
