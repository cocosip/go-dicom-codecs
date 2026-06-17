package htj2k

import "testing"

func TestHTEncoderUsesOpenJPHScupLocator(t *testing.T) {
	data := []int32{
		1, 0, 2, 0,
		0, 3, 0, 4,
		5, 0, 6, 0,
		0, 7, 0, 8,
	}

	block, _ := encodeDecodeOpenJPHCleanupForTest(t, 4, 4, data)
	if len(block) < 2 {
		t.Fatalf("encoded block too short: %d", len(block))
	}

	scup := standardScup(block)
	if scup < 2 || scup > len(block) {
		t.Fatalf("standard Scup = %d, want 2..%d", scup, len(block))
	}

	if looksLikeLegacyFooter(block) {
		t.Fatalf("encoded block still uses legacy 4-byte mel/vlc footer: tail=% X", block[len(block)-4:])
	}
}

func TestHTEncoderUsesMinimumOpenJPHCleanupForZeroBlock(t *testing.T) {
	block, decoded := encodeDecodeOpenJPHCleanupForTest(t, 4, 4, make([]int32, 16))

	if len(block) != 0 {
		t.Fatalf("encoded zero block length = %d, want 0 because OpenJPH omits empty codeblocks", len(block))
	}
	for i, v := range decoded {
		if v != 0 {
			t.Fatalf("decoded[%d] = %d, want 0", i, v)
		}
	}
}

func standardScup(block []byte) int {
	lcup := len(block)
	return int(block[lcup-1])<<4 | int(block[lcup-2]&0x0F)
}

func looksLikeLegacyFooter(block []byte) bool {
	if len(block) < 4 {
		return false
	}
	lcup := len(block)
	melLen := int(block[lcup-4]) | int(block[lcup-3])<<8
	vlcLen := int(block[lcup-2]) | int(block[lcup-1])<<8
	return melLen > 0 && vlcLen > 0 && melLen+vlcLen+4 <= lcup
}
