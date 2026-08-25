package mqc

import "testing"

func TestMQCTablesMatchOpenJPEG(t *testing.T) {
	expected := loadOpenJPEGMQCTables(t)

	for i := 0; i < 47; i++ {
		if qeTable[i] != expected.qe[i] {
			t.Fatalf("state %d Qe mismatch: got 0x%04X want 0x%04X", i, qeTable[i], expected.qe[i])
		}
		if nmpsTable[i] != expected.nmps[i] {
			t.Fatalf("state %d NMPS mismatch: got %d want %d", i, nmpsTable[i], expected.nmps[i])
		}
		if nlpsTable[i] != expected.nlps[i] {
			t.Fatalf("state %d NLPS mismatch: got %d want %d", i, nlpsTable[i], expected.nlps[i])
		}
		if switchTable[i] != expected.switchB[i] {
			t.Fatalf("state %d switch mismatch: got %d want %d", i, switchTable[i], expected.switchB[i])
		}
	}
}

func TestMQCInitialization(t *testing.T) {
	enc := NewMQEncoder(19)
	if enc.a != 0x8000 {
		t.Fatalf("encoder A init = 0x%04X, want 0x8000", enc.a)
	}
	if enc.c != 0 {
		t.Fatalf("encoder C init = 0x%08X, want 0", enc.c)
	}
	if enc.ct != 12 {
		t.Fatalf("encoder ct init = %d, want 12", enc.ct)
	}
	for i, state := range enc.contexts {
		if state != 0 {
			t.Fatalf("encoder context %d init = %d, want 0", i, state)
		}
	}

	dec := NewMQDecoder([]byte{0x00}, 19)
	if dec.a != 0x8000 {
		t.Fatalf("decoder A init = 0x%04X, want 0x8000", dec.a)
	}
	for i, state := range dec.contexts {
		if state != 0 {
			t.Fatalf("decoder context %d init = %d, want 0", i, state)
		}
	}
}

func TestMQCRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		bits []int
	}{
		{name: "AllZero", bits: []int{0, 0, 0, 0, 0, 0, 0, 0}},
		{name: "AllOne", bits: []int{1, 1, 1, 1, 1, 1, 1, 1}},
		{name: "Alternating", bits: []int{0, 1, 0, 1, 0, 1, 0, 1}},
		{name: "Mixed", bits: []int{1, 0, 1, 1, 0, 0, 1, 0, 1, 1, 1, 0, 0, 0, 1, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewMQEncoder(1)
			for _, bit := range tt.bits {
				enc.Encode(bit, 0)
			}
			encoded := enc.Flush()

			dec := NewMQDecoder(encoded, 1)
			for i, expected := range tt.bits {
				if got := dec.Decode(0); got != expected {
					t.Fatalf("bit %d mismatch: got %d want %d", i, got, expected)
				}
			}
		})
	}
}

func TestMQCContextSwitching(t *testing.T) {
	numContexts := 5
	bitsPerContext := 20
	patterns := [][]int{
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1},
		{1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1},
		{1, 1, 0, 1, 0, 1, 1, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 1},
	}

	enc := NewMQEncoder(numContexts)
	for i := 0; i < bitsPerContext; i++ {
		for ctx := 0; ctx < numContexts; ctx++ {
			enc.Encode(patterns[ctx][i], ctx)
		}
	}
	encoded := enc.Flush()

	dec := NewMQDecoder(encoded, numContexts)
	for i := 0; i < bitsPerContext; i++ {
		for ctx := 0; ctx < numContexts; ctx++ {
			if got := dec.Decode(ctx); got != patterns[ctx][i] {
				t.Fatalf("context %d index %d mismatch: got %d want %d", ctx, i, got, patterns[ctx][i])
			}
		}
	}
}
