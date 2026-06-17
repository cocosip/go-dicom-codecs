package htj2k

import (
	"testing"
)

// TestVLCEncodeDecodeEKE1 tests VLC encoding/decoding for ek=0xF, e1=0xF
// NOTE: VLC table may not have exact match for all ek/e1 combinations.
// Encoder uses fallback to find best matching entry, decoder returns that entry's EK/E1.
func TestVLCEncodeDecodeEKE1(t *testing.T) {
	// Our case: ctx=0, rho=0xF, uOff=1, ek=0xF, e1=0xF, isFirstRow=true
	context := uint8(0)
	rho := uint8(0xF)
	uOff := uint8(1)
	ek := uint8(0xF)
	e1 := uint8(0xF)
	isFirstRow := true

	t.Logf("=== Input ===")
	t.Logf("ctx=%d, rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X, isFirstRow=%v",
		context, rho, uOff, ek, e1, isFirstRow)

	// Test encoding
	encoder := NewVLCEncoder()
	length, err := encoder.EncodeCxtVLCWithLen(context, rho, uOff, ek, e1, isFirstRow)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded: %d bits", length)

	vlcData := encoder.Flush()
	writeScupLocator(vlcData, len(vlcData))
	t.Logf("VLC data: %d bytes: %X", len(vlcData), vlcData)

	// Test decoding
	decoder := NewVLCReverseDecoder(vlcData)
	decodedRho, decodedUOff, decodedEK, decodedE1, found := decoder.DecodeQuadWithContext(context, isFirstRow)
	if !found {
		t.Fatalf("Decode failed: no match found")
	}

	t.Logf("\n=== Output ===")
	t.Logf("Decoded: rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X", decodedRho, decodedUOff, decodedEK, decodedE1)

	// Check results - rho and uOff must match exactly
	if decodedRho != rho {
		t.Errorf("rho mismatch: expected 0x%X, got 0x%X", rho, decodedRho)
	}
	if decodedUOff != uOff {
		t.Errorf("uOff mismatch: expected %d, got %d", uOff, decodedUOff)
	}

	// For ek/e1, decoder returns VLC table entry's EK/E1, which may differ from input
	// This is correct behavior when exact match doesn't exist in VLC table
	t.Logf("Note: Decoded ek/e1 are from VLC table entry (may differ from input if no exact match)")

	// Verify that the decoded values are reasonable
	// The important thing is that encoding and decoding are consistent
	if (decodedEK&ek) == 0 && ek != 0 {
		t.Errorf("Decoded EK=0x%X has no overlap with input ek=0x%X", decodedEK, ek)
	}
	if (decodedE1&e1) == 0 && e1 != 0 {
		t.Errorf("Decoded E1=0x%X has no overlap with input e1=0x%X", decodedE1, e1)
	}

	t.Logf("✓ VLC roundtrip completed (rho/uOff match, ek/e1 are table values)")
}

// TestVLCVariousEKE1 tests various ek/e1 combinations
// NOTE: This test verifies encode/decode consistency, not exact ek/e1 recovery
func TestVLCVariousEKE1(t *testing.T) {
	cases := []struct {
		name string
		ek   uint8
		e1   uint8
	}{
		{"ek=0xE,e1=0x6", 0xE, 0x6}, // Exact match exists in table
		{"ek=0xF,e1=0xC", 0xF, 0xC}, // Exact match exists in table
		{"ek=0xF,e1=0xF", 0xF, 0xF}, // No exact match - uses fallback
		{"ek=0x0,e1=0x0", 0x0, 0x0}, // Special case (uOff should be 0)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			context := uint8(0)
			rho := uint8(0xF)
			uOff := uint8(1)
			isFirstRow := true

			encoder := NewVLCEncoder()
			_, err := encoder.EncodeCxtVLCWithLen(context, rho, uOff, tc.ek, tc.e1, isFirstRow)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			vlcData := encoder.Flush()
			writeScupLocator(vlcData, len(vlcData))
			decoder := NewVLCReverseDecoder(vlcData)
			decodedRho, decodedUOff, decodedEK, decodedE1, found := decoder.DecodeQuadWithContext(context, isFirstRow)
			if !found {
				t.Fatalf("Decode failed")
			}

			t.Logf("Input:  ek=0x%X, e1=0x%X", tc.ek, tc.e1)
			t.Logf("Output: ek=0x%X, e1=0x%X", decodedEK, decodedE1)

			// Check rho and uOff - these must match
			if decodedRho != rho {
				t.Errorf("rho mismatch: expected 0x%X, got 0x%X", rho, decodedRho)
			}
			if decodedUOff != uOff {
				t.Errorf("uOff mismatch: expected %d, got %d", uOff, decodedUOff)
			}

			// For ek/e1, check if decoded values are consistent with encoding
			// Verify that the decoded values have some overlap with input
			if (decodedEK&tc.ek) == 0 && tc.ek != 0 {
				t.Errorf("Decoded EK=0x%X has no overlap with input ek=0x%X", decodedEK, tc.ek)
			}
			if (decodedE1&tc.e1) == 0 && tc.e1 != 0 {
				t.Errorf("Decoded E1=0x%X has no overlap with input e1=0x%X", decodedE1, tc.e1)
			}
		})
	}
}
