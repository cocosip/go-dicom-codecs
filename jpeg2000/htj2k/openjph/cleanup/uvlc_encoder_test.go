package cleanup

import "testing"

func TestEncodeUVLC(t *testing.T) {
	tests := []struct {
		name       string
		u          uint32
		wantUPfx   uint8
		wantUSfx   uint8
		wantUExt   uint8
		wantPfxLen int
		wantSfxLen int
		wantExtLen int
	}{
		{
			name:       "u=0 (no encoding)",
			u:          0,
			wantUPfx:   0,
			wantUSfx:   0,
			wantUExt:   0,
			wantPfxLen: 0,
			wantSfxLen: 0,
			wantExtLen: 0,
		},
		{
			name:       "u=1",
			u:          1,
			wantUPfx:   1,
			wantUSfx:   0,
			wantUExt:   0,
			wantPfxLen: 1,
			wantSfxLen: 0,
			wantExtLen: 0,
		},
		{
			name:       "u=2",
			u:          2,
			wantUPfx:   2,
			wantUSfx:   0,
			wantUExt:   0,
			wantPfxLen: 2,
			wantSfxLen: 0,
			wantExtLen: 0,
		},
		{
			name:       testNameU3,
			u:          3,
			wantUPfx:   3,
			wantUSfx:   0,
			wantUExt:   0,
			wantPfxLen: 3,
			wantSfxLen: 1,
			wantExtLen: 0,
		},
		{
			name:       testNameU4,
			u:          4,
			wantUPfx:   3,
			wantUSfx:   1,
			wantUExt:   0,
			wantPfxLen: 3,
			wantSfxLen: 1,
			wantExtLen: 0,
		},
		{
			name:       "u=5",
			u:          5,
			wantUPfx:   5,
			wantUSfx:   0,
			wantUExt:   0,
			wantPfxLen: 3,
			wantSfxLen: 5,
			wantExtLen: 0,
		},
		{
			name:       "u=10",
			u:          10,
			wantUPfx:   5,
			wantUSfx:   5,
			wantUExt:   0,
			wantPfxLen: 3,
			wantSfxLen: 5,
			wantExtLen: 0,
		},
		{
			name:       "u=32",
			u:          32,
			wantUPfx:   5,
			wantUSfx:   27,
			wantUExt:   0,
			wantPfxLen: 3,
			wantSfxLen: 5,
			wantExtLen: 0,
		},
		{
			name:       "u=33 (first with extension)",
			u:          33,
			wantUPfx:   5,
			wantUSfx:   28,
			wantUExt:   0,
			wantPfxLen: 3,
			wantSfxLen: 5,
			wantExtLen: 4,
		},
		{
			name:       "u=37",
			u:          37,
			wantUPfx:   5,
			wantUSfx:   28,
			wantUExt:   1,
			wantPfxLen: 3,
			wantSfxLen: 5,
			wantExtLen: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := EncodeUVLC(tt.u)

			if cwd.Prefix != tt.wantUPfx {
				t.Errorf("Prefix = %d, want %d", cwd.Prefix, tt.wantUPfx)
			}
			if cwd.Suffix != tt.wantUSfx {
				t.Errorf("Suffix = %d, want %d", cwd.Suffix, tt.wantUSfx)
			}
			if cwd.Extension != tt.wantUExt {
				t.Errorf("Extension = %d, want %d", cwd.Extension, tt.wantUExt)
			}
			if cwd.PrefixLen != tt.wantPfxLen {
				t.Errorf("PrefixLen = %d, want %d", cwd.PrefixLen, tt.wantPfxLen)
			}
			if cwd.SuffixLen != tt.wantSfxLen {
				t.Errorf("SuffixLen = %d, want %d", cwd.SuffixLen, tt.wantSfxLen)
			}
			if cwd.ExtLen != tt.wantExtLen {
				t.Errorf("ExtLen = %d, want %d", cwd.ExtLen, tt.wantExtLen)
			}

			// Verify formula: u = u_pfx + u_sfx + 4*u_ext
			if tt.u > 0 {
				reconstructed := uint32(cwd.Prefix) + uint32(cwd.Suffix) + 4*uint32(cwd.Extension)
				if reconstructed != tt.u {
					t.Errorf("Formula check failed: %d + %d + 4*%d = %d, want %d",
						cwd.Prefix, cwd.Suffix, cwd.Extension, reconstructed, tt.u)
				}
			}
		})
	}
}

func TestEncodeUVLCInitialPair(t *testing.T) {
	tests := []struct {
		name string
		u    uint32
		want uint32 // Expected encoding of (u - 2)
	}{
		{
			name: "u=2 (minimum)",
			u:    2,
			want: 0, // Encode 0
		},
		{
			name: testNameU3,
			u:    3,
			want: 1, // Encode 1
		},
		{
			name: testNameU4,
			u:    4,
			want: 2, // Encode 2
		},
		{
			name: "u=7",
			u:    7,
			want: 5, // Encode 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := EncodeUVLCInitialPair(tt.u)

			// Verify it encodes (u - 2)
			expected := EncodeUVLC(tt.want)

			if cwd.Prefix != expected.Prefix {
				t.Errorf("Prefix = %d, want %d", cwd.Prefix, expected.Prefix)
			}
			if cwd.Suffix != expected.Suffix {
				t.Errorf("Suffix = %d, want %d", cwd.Suffix, expected.Suffix)
			}
			if cwd.Extension != expected.Extension {
				t.Errorf("Extension = %d, want %d", cwd.Extension, expected.Extension)
			}
		})
	}
}

func TestEncodePrefixBits(t *testing.T) {
	tests := []struct {
		name       string
		uPfx       uint8
		wantBits   uint32
		wantLength int
	}{
		{
			name:       "u_pfx=1: '1'",
			uPfx:       1,
			wantBits:   0b1, // "1": bit0=1 鈫?0x1
			wantLength: 1,
		},
		{
			name:       "u_pfx=2: '01'",
			uPfx:       2,
			wantBits:   0b10, // "01": bit0=0, bit1=1 鈫?0x2
			wantLength: 2,
		},
		{
			name:       "u_pfx=3: '001'",
			uPfx:       3,
			wantBits:   0b100, // "001": bit0=0, bit1=0, bit2=1 鈫?0x4
			wantLength: 3,
		},
		{
			name:       "u_pfx=5: '000'",
			uPfx:       5,
			wantBits:   0b000, // "000": bit0=0, bit1=0, bit2=0 鈫?0x0
			wantLength: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bits, length := EncodePrefixBits(tt.uPfx)

			if bits != tt.wantBits {
				t.Errorf("bits = %b, want %b", bits, tt.wantBits)
			}
			if length != tt.wantLength {
				t.Errorf("length = %d, want %d", length, tt.wantLength)
			}
		})
	}
}

func TestUVLCCodeword_TotalLength(t *testing.T) {
	tests := []struct {
		name string
		cwd  UVLCCodeword
		want int
	}{
		{
			name: "u=1: 1 bit total",
			cwd: UVLCCodeword{
				PrefixLen: 1,
			},
			want: 1,
		},
		{
			name: "u=3: 4 bits total (3+1)",
			cwd: UVLCCodeword{
				PrefixLen: 3,
				SuffixLen: 1,
			},
			want: 4,
		},
		{
			name: "u=10: 8 bits total (3+5)",
			cwd: UVLCCodeword{
				PrefixLen: 3,
				SuffixLen: 5,
			},
			want: 8,
		},
		{
			name: "u=37: 12 bits total (3+5+4)",
			cwd: UVLCCodeword{
				PrefixLen: 3,
				SuffixLen: 5,
				ExtLen:    4,
			},
			want: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cwd.TotalLength()
			if got != tt.want {
				t.Errorf("TotalLength() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUVLCEncodeDecodeRoundTrip(t *testing.T) {
	// Test that encoding and decoding are inverse operations

	testValues := []uint32{0, 1, 2, 3, 4, 5, 10, 15, 20, 32, 33, 37, 50, 100}

	for _, u := range testValues {
		t.Run("", func(t *testing.T) {
			if u == 0 {
				// Skip u=0 (no encoding)
				return
			}

			// Encode
			cwd := EncodeUVLC(u)

			// Verify formula
			decoded := uint32(cwd.Prefix) + uint32(cwd.Suffix) + 4*uint32(cwd.Extension)

			if decoded != u {
				t.Errorf("Round-trip failed: encoded %d, decoded %d", u, decoded)
				t.Errorf("  Components: pfx=%d, sfx=%d, ext=%d", cwd.Prefix, cwd.Suffix, cwd.Extension)
			}
		})
	}
}
