package htj2k

import (
	"testing"
)

// TestVLCEncodeDecodeRoundtrip tests that VLC encoding and decoding are inverses
func TestVLCEncodeDecodeRoundtrip(t *testing.T) {
	// Test case: encode and decode a simple quad
	// For samples like [7,7,7,7], all have magnitude > 1, so uOff=1
	context := uint8(0)
	rho := uint8(0xF) // All significant
	uOff := uint8(1)  // uOff=1 because all samples need u encoding
	ek := uint8(0xE)  // Example EMB pattern
	e1 := uint8(0x6)  // Example EMB pattern
	isFirstRow := true

	// Create encoder
	encoder := NewVLCEncoder()

	// Encode
	length, err := encoder.EncodeCxtVLCWithLen(context, rho, uOff, ek, e1, isFirstRow)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded: context=%d, rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X", context, rho, uOff, ek, e1)
	t.Logf("VLC codeword length: %d bits", length)

	// Get encoded data
	vlcData := encoder.Flush()
	writeScupLocator(vlcData, len(vlcData))
	t.Logf("VLC data: %d bytes: %v", len(vlcData), vlcData)

	// Print in binary
	for i, b := range vlcData {
		t.Logf("  VLC[%d]: 0x%02X = %08b", i, b, b)
	}

	// Create decoder
	decoder := NewVLCReverseDecoder(vlcData)

	// Decode
	decodedRho, decodedUOff, decodedEK, decodedE1, found := decoder.DecodeQuadWithContext(context, isFirstRow)
	if !found {
		t.Fatalf("Decode failed: no match found")
	}

	t.Logf("Decoded: rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X", decodedRho, decodedUOff, decodedEK, decodedE1)

	// Verify
	if decodedRho != rho {
		t.Errorf("rho mismatch: expected 0x%X, got 0x%X", rho, decodedRho)
	}
	if decodedUOff != uOff {
		t.Errorf("uOff mismatch: expected %d, got %d", uOff, decodedUOff)
	}
	if decodedEK != ek {
		t.Errorf("ek mismatch: expected 0x%X, got 0x%X", ek, decodedEK)
	}
	if decodedE1 != e1 {
		t.Errorf("e1 mismatch: expected 0x%X, got 0x%X", e1, decodedE1)
	}

	if decodedRho == rho && decodedUOff == uOff && decodedEK == ek && decodedE1 == e1 {
		t.Logf("✓ VLC roundtrip successful!")
	}
}

// TestVLCMultipleQuads tests encoding multiple quads
func TestVLCMultipleQuads(t *testing.T) {
	encoder := NewVLCEncoder()

	// Encode first quad
	_, err := encoder.EncodeCxtVLCWithLen(0, 0xF, 0, 0xE, 0x6, true)
	if err != nil {
		t.Fatalf("Encode quad 1 failed: %v", err)
	}

	// Encode second quad (simulate second quad in pair)
	// In a pair, second quad would have different context
	_, err = encoder.EncodeCxtVLCWithLen(0, 0xF, 0, 0xE, 0x6, true)
	if err != nil {
		t.Fatalf("Encode quad 2 failed: %v", err)
	}

	vlcData := encoder.Flush()
	writeScupLocator(vlcData, len(vlcData))
	t.Logf("Encoded %d quads into %d bytes", 2, len(vlcData))

	// Decode
	decoder := NewVLCReverseDecoder(vlcData)

	// Decode first quad
	rho1, uOff1, ek1, e1_1, found1 := decoder.DecodeQuadWithContext(0, true)
	if !found1 {
		t.Fatalf("Decode quad 1 failed")
	}
	t.Logf("Quad 1: rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X", rho1, uOff1, ek1, e1_1)

	// Decode second quad
	rho2, uOff2, ek2, e1_2, found2 := decoder.DecodeQuadWithContext(0, true)
	if !found2 {
		t.Fatalf("Decode quad 2 failed")
	}
	t.Logf("Quad 2: rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X", rho2, uOff2, ek2, e1_2)

	// Both should match
	if rho1 == 0xF && rho2 == 0xF {
		t.Logf("✓ Multiple quads roundtrip successful!")
	}
}
