package htj2k

import (
	"testing"
)

// TestUVLCEncodeDecode tests U-VLC encoding/decoding in isolation
func TestUVLCEncodeDecode(t *testing.T) {
	tests := []struct {
		name string
		u    uint32
	}{
		{"u=1", 1},
		{"u=2", 2},
		{testNameU3, 3},
		{testNameU4, 4},
		{"u=5", 5},
		{"u=10", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create VLC encoder
			vlc := NewVLCEncoder()

			// Encode U-VLC
			cwd := EncodeUVLC(tt.u)
			if err := cwd.EncodeToStream(vlc); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Flush VLC
			vlcData := vlc.Flush()
			t.Logf("Encoded u=%d -> %d bytes: %v", tt.u, len(vlcData), vlcData)
			t.Logf("Codeword: prefix=%d(%d bits), suffix=%d(%d bits), ext=%d(%d bits)",
				cwd.Prefix, cwd.PrefixLen, cwd.Suffix, cwd.SuffixLen, cwd.Extension, cwd.ExtLen)

			// Create standard reverse VLC decoder over a cleanup suffix.
			writeScupLocator(vlcData, len(vlcData))
			vlcDec := NewVLCReverseDecoder(vlcData).(*VLCDecoder)

			// Manually decode following the same logic as decodeUVLCSimple
			// Read first bit
			bit1, ok := vlcDec.readBits(1)
			if !ok {
				t.Fatalf("Failed to read bit1")
			}
			t.Logf("bit1=%d", bit1)

			if bit1 == 1 {
				// Prefix "1" -> u=1
				decoded := uint32(1)
				t.Logf("Decoded (prefix=1): u=%d", decoded)
				if decoded != tt.u {
					t.Errorf("Mismatch: got %d, want %d", decoded, tt.u)
				}
				return
			}

			// Read second bit
			bit2, ok := vlcDec.readBits(1)
			if !ok {
				t.Fatalf("Failed to read bit2")
			}
			t.Logf("bit2=%d", bit2)

			if bit2 == 1 {
				// Prefix "01" -> u=2
				decoded := uint32(2)
				t.Logf("Decoded (prefix=01): u=%d", decoded)
				if decoded != tt.u {
					t.Errorf("Mismatch: got %d, want %d", decoded, tt.u)
				}
				return
			}

			// Read third bit
			bit3, ok := vlcDec.readBits(1)
			if !ok {
				t.Fatalf("Failed to read bit3")
			}
			t.Logf("bit3=%d", bit3)

			if bit3 == 1 {
				// Prefix "001" -> u=3-4
				suffix, ok := vlcDec.readBits(1)
				if !ok {
					t.Fatalf("Failed to read suffix")
				}
				decoded := 3 + suffix
				t.Logf("Decoded (prefix=001): suffix=%d, u=%d", suffix, decoded)
				if decoded != tt.u {
					t.Errorf("Mismatch: got %d, want %d", decoded, tt.u)
				}
				return
			}

			// Prefix "000" -> u=5+
			suffix, ok := vlcDec.readBits(5)
			if !ok {
				t.Fatalf("Failed to read 5-bit suffix")
			}
			t.Logf("5-bit suffix=%d", suffix)

			if suffix >= 28 {
				ext, ok := vlcDec.readBits(4)
				if !ok {
					t.Fatalf("Failed to read extension")
				}
				decoded := 5 + suffix + 4*ext
				t.Logf("Decoded (prefix=000, ext): suffix=%d, ext=%d, u=%d", suffix, ext, decoded)
				if decoded != tt.u {
					t.Errorf("Mismatch: got %d, want %d", decoded, tt.u)
				}
			} else {
				decoded := 5 + suffix
				t.Logf("Decoded (prefix=000): suffix=%d, u=%d", suffix, decoded)
				if decoded != tt.u {
					t.Errorf("Mismatch: got %d, want %d", decoded, tt.u)
				}
			}
		})
	}
}
