// Package mqc contains MQ-coder test alignment and validation utilities.
package mqc

import "testing"

func TestMQCProbabilityIntervalUpdate(t *testing.T) {
	t.Run("MPS update keeps A nonzero", func(t *testing.T) {
		enc := NewMQEncoder(1)
		enc.Encode(0, 0)
		if enc.a == 0 {
			t.Fatal("A should not be zero after MPS encode")
		}
	})

	t.Run("LPS transitions to NLPS state", func(t *testing.T) {
		enc := NewMQEncoder(1)
		initialState := uint8(0)
		enc.contexts[0] = initialState
		enc.Encode(1, 0)
		newState := enc.contexts[0] & 0x7F
		if newState != nlpsTable[initialState] {
			t.Fatalf("LPS state = %d, want %d", newState, nlpsTable[initialState])
		}
	})

	t.Run("LPS switch flag behavior", func(t *testing.T) {
		tests := []struct {
			state uint8
		}{
			{state: 0},
			{state: 1},
		}
		for _, tt := range tests {
			enc := NewMQEncoder(1)
			enc.contexts[0] = tt.state
			mpsBefore := enc.contexts[0] >> 7
			enc.Encode(1, 0)
			mpsAfter := enc.contexts[0] >> 7
			switched := mpsBefore != mpsAfter
			expectSwitch := switchTable[tt.state] == 1
			if switched != expectSwitch {
				t.Fatalf("state %d switch mismatch: got %v want %v", tt.state, switched, expectSwitch)
			}
		}
	})
}

func TestMQCRenormalizationKeepsAInRange(t *testing.T) {
	enc := NewMQEncoder(1)
	for i := 0; i < 200; i++ {
		enc.Encode(1, 0)
		if enc.a < 0x8000 {
			t.Fatalf("A fell below 0x8000 at %d: 0x%04X", i, enc.a)
		}
		if enc.a > 0xFFFF {
			t.Fatalf("A exceeded 0xFFFF at %d: 0x%04X", i, enc.a)
		}
	}
}

func TestMQCByteStuffingRoundTrip(t *testing.T) {
	enc := NewMQEncoder(1)
	for i := 0; i < 1000; i++ {
		enc.Encode(1, 0)
	}
	encoded := enc.Flush()

	dec := NewMQDecoder(encoded, 1)
	for i := 0; i < 1000; i++ {
		if bit := dec.Decode(0); bit != 1 {
			t.Fatalf("bit %d mismatch: got %d want 1", i, bit)
		}
	}
}

func TestMQCEncoderDecoderSymmetry(t *testing.T) {
	enc := NewMQEncoder(1)
	bits := []int{0, 1, 1, 0, 1, 0, 0, 1}
	for _, bit := range bits {
		enc.Encode(bit, 0)
	}
	encoded := enc.Flush()

	dec := NewMQDecoder(encoded, 1)
	for i, expected := range bits {
		if got := dec.Decode(0); got != expected {
			t.Fatalf("bit %d mismatch: got %d want %d", i, got, expected)
		}
	}
}
