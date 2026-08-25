package cleanup

import (
	"testing"
)

// TestVLCRoundTrip tests VLC encoding and decoding round-trip
func TestVLCRoundTrip(t *testing.T) {
	// Test encoding the exact parameters from our failing case
	context := uint8(0)
	rho := uint8(1)
	uOff := uint8(0)
	ek := uint8(0)
	e1 := uint8(0)

	t.Logf("Encoding: context=%d, rho=%d, uOff=%d, ek=%d, e1=%d", context, rho, uOff, ek, e1)

	// Encode
	enc := NewVLCEncoder()

	// Check if entry exists in table
	key := encodeKey{cq: context, rho: rho, uOff: uOff, ek: ek, e1: e1}
	entry, found := enc.encodeMap0[key]
	t.Logf("Table lookup: found=%v", found)
	if found {
		t.Logf("  Entry: codeword=0x%02X (%08b), length=%d", entry.Codeword, entry.Codeword, entry.Length)
	}

	err := enc.EncodeCxtVLC(context, rho, uOff, ek, e1, true)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	vlcData := enc.Flush()
	t.Logf("Encoded VLC data: %v", vlcData)
	for i, b := range vlcData {
		t.Logf("  Byte %d: 0x%02X = %08b", i, b, b)
	}

	// Decode
	dec := NewVLCDecoder(vlcData)

	// Check what 7 bits are read
	dec2 := NewVLCDecoder(vlcData)
	bits7, _ := dec2.readBits(7)
	t.Logf("After init, reading 7 bits: 0x%02X (%07b)", bits7, bits7)

	decRho, decUOff, decEk, decE1, found := dec.DecodeInitialRow(context)
	t.Logf("Decoded: rho=%d, uOff=%d, ek=%d, e1=%d, found=%v", decRho, decUOff, decEk, decE1, found)

	// Check
	if !found {
		t.Errorf("VLC decode failed")
	}
	if decRho != rho {
		t.Errorf("Rho mismatch: encoded=%d, decoded=%d", rho, decRho)
	}
	if decUOff != uOff {
		t.Errorf("UOff mismatch: encoded=%d, decoded=%d", uOff, decUOff)
	}
}
