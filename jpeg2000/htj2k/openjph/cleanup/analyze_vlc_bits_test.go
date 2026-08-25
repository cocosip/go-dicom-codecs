// Package htj2k contains tests that analyze VLC bit operations for HTJ2K.
package cleanup

import (
	"testing"
)

// TestAnalyzeVLCBitStream analyzes the VLC bit stream in detail
func TestAnalyzeVLCBitStream(t *testing.T) {
	// Encode a single codeword
	enc := NewVLCEncoder()

	t.Logf("=== ENCODER INITIAL STATE ===")
	t.Logf("vlcBuf: %v", enc.vlcBuf)
	t.Logf("vlcTmp: 0x%02X (%08b)", enc.vlcTmp, enc.vlcTmp)
	t.Logf("vlcBits: %d", enc.vlcBits)
	t.Logf("vlcLast: 0x%02X", enc.vlcLast)

	// Encode codeword 0x06 (0110), length 4
	t.Logf("\n=== ENCODING CODEWORD 0x06 (0110), LENGTH 4 ===")

	cwd := uint32(0x06) // 0110
	length := 4

	// Manually trace emitVLCBits
	vlcBits := enc.vlcBits
	vlcTmp := enc.vlcTmp
	vlcBuf := make([]byte, len(enc.vlcBuf))
	copy(vlcBuf, enc.vlcBuf)

	for i := 0; i < length; i++ {
		bit := cwd & 1
		cwd = cwd >> 1

		t.Logf("Bit %d: value=%d", i, bit)
		t.Logf("  Before: vlcTmp=0x%02X (%08b), vlcBits=%d", vlcTmp, vlcTmp, vlcBits)

		vlcTmp = vlcTmp | uint8(bit<<vlcBits)
		vlcBits++

		t.Logf("  After:  vlcTmp=0x%02X (%08b), vlcBits=%d", vlcTmp, vlcTmp, vlcBits)

		if vlcBits == 8 {
			vlcBuf = append(vlcBuf, vlcTmp)
			t.Logf("  Flush byte: 0x%02X (%08b)", vlcTmp, vlcTmp)
			vlcTmp = 0
			vlcBits = 0
		}
	}

	t.Logf("\n=== AFTER ENCODING ===")
	t.Logf("vlcBuf: %v", vlcBuf)
	t.Logf("vlcTmp: 0x%02X (%08b)", vlcTmp, vlcTmp)
	t.Logf("vlcBits: %d", vlcBits)

	// Now actually encode
	err := enc.EncodeCxtVLC(0, 1, 0, 0, 0, true)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	vlcData := enc.Flush()
	t.Logf("\n=== ENCODED DATA (after Flush) ===")
	t.Logf("VLC data: %v", vlcData)
	for i, b := range vlcData {
		t.Logf("  Byte %d: 0x%02X (%08b)", i, b, b)
	}

	// Analyze bit positions
	t.Logf("\n=== BIT ANALYSIS ===")
	t.Logf("Original buffer before reverse: %v", enc.vlcBuf)
	t.Logf("After reverse: %v", vlcData)

	// The encoded codeword 0110 should be somewhere in these bytes
	// Let's find it
	t.Logf("\nSearching for codeword pattern 0110 (0x6)...")

	// Convert bytes to bit string
	allBits := ""
	for i := len(vlcData) - 1; i >= 0; i-- {
		for j := 0; j < 8; j++ {
			if (vlcData[i]>>j)&1 == 1 {
				allBits += "1"
			} else {
				allBits += "0"
			}
		}
	}
	t.Logf("All bits (reading backward, LSB first): %s", allBits)
	t.Logf("Bit positions: 0-3: %s (initial padding)", allBits[0:4])
	if len(allBits) >= 8 {
		t.Logf("Bit positions: 4-7: %s (codeword)", allBits[4:8])
	}
}

// TestVLCDecoderBitReading tests how VLC decoder reads bits
func TestVLCDecoderBitReading(t *testing.T) {
	// Use the encoded data from encoder
	vlcData := []byte{111, 255} // From previous test

	t.Logf("=== DECODER INPUT ===")
	t.Logf("VLC data: %v", vlcData)
	for i, b := range vlcData {
		t.Logf("  Byte %d: 0x%02X (%08b)", i, b, b)
	}

	// Create decoder WITHOUT initialization
	dec := &VLCDecoder{
		data: vlcData,
		pos:  len(vlcData),
	}
	dec.buildLookupTables()

	t.Logf("\n=== READING BITS (no init skip) ===")
	for i := 0; i < 16; i++ {
		bit, ok := dec.readBits(1)
		if !ok {
			t.Logf("Bit %d: exhausted", i)
			break
		}
		t.Logf("Bit %d: %d", i, bit)
	}

	// Create decoder WITH initialization skip
	dec2 := NewVLCDecoder(vlcData)

	t.Logf("\n=== READING BITS (with 4-bit init skip) ===")
	for i := 0; i < 12; i++ {
		bit, ok := dec2.readBits(1)
		if !ok {
			t.Logf("Bit %d: exhausted", i)
			break
		}
		t.Logf("Bit %d: %d", i, bit)
	}

	// Read 4 bits at once
	dec3 := NewVLCDecoder(vlcData)
	bits4, ok := dec3.readBits(4)
	t.Logf("\n=== READING 4 BITS (after init skip) ===")
	t.Logf("4 bits: 0x%X (%04b), ok=%v", bits4, bits4, ok)

	// Read 7 bits at once
	dec4 := NewVLCDecoder(vlcData)
	bits7, ok := dec4.readBits(7)
	t.Logf("\n=== READING 7 BITS (after init skip) ===")
	t.Logf("7 bits: 0x%02X (%07b), ok=%v", bits7, bits7, ok)

	// Manual calculation of what bits SHOULD be read
	t.Logf("\n=== EXPECTED BIT READING ===")
	t.Logf("Total bit stream (LSB to MSB): 01101111 11111111")
	t.Logf("After skipping 4 bits, remaining: %s", "01101111 11111111"[4:])
	t.Logf("Next 7 bits should be: bits 4-10")
	t.Logf("Byte 111 = 01101111:")
	t.Logf("  Bits 4-7: 0110")
	t.Logf("Byte 255 = 11111111:")
	t.Logf("  Bits 0-2 (from byte 255): 111")
	t.Logf("Combined (bits 4-10): 0110111 = 0x%02X = %d", 0b0110111, 0b0110111)

	// Check table lookup
	context := uint8(0)
	key := (uint32(context) << 7) | bits7
	t.Logf("\n=== TABLE LOOKUP ===")
	t.Logf("Key: (context << 7) | bits7 = (%d << 7) | %d = %d", context, bits7, key)

	if key < 1024 {
		packed := dec4.tbl0[key]
		t.Logf("Table entry at key %d: 0x%04X", key, packed)
		if packed != 0 {
			rho := (packed >> 4) & 0xF
			uOff := (packed >> 3) & 0x1
			t.Logf("  Decoded: rho=%d, uOff=%d", rho, uOff)
		}
	}

	// Try the expected key
	expectedBits := uint32(0b0110111) // 55
	expectedKey := (uint32(context) << 7) | expectedBits
	t.Logf("\n=== EXPECTED TABLE LOOKUP ===")
	t.Logf("Expected key: %d (bits=0x%02X)", expectedKey, expectedBits)
	if expectedKey < 1024 {
		packed := dec4.tbl0[expectedKey]
		t.Logf("Table entry: 0x%04X", packed)
		if packed != 0 {
			rho := (packed >> 4) & 0xF
			uOff := (packed >> 3) & 0x1
			cwdLen := packed & 0x7
			t.Logf("  Decoded: rho=%d, uOff=%d, cwd_len=%d", rho, uOff, cwdLen)
		}
	}
}
