package jpeg2000

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

const updateJPEG2000BaselineEnv = "UPDATE_JPEG2000_BASELINE"

func TestClassicJPEG2000PreMigrationBytes(t *testing.T) {
	tests := []struct {
		name       string
		fixture    string
		wantSHA256 string
		encode     func(t *testing.T) []byte
	}{
		{
			name:       "mono_u16_lossless",
			fixture:    "classic-mono-u16-lossless.j2c",
			wantSHA256: "56da3b34eeffcc02a72caa60a931f18d1bb3f5006ec148467359ce89935ed853",
			encode:     encodeClassicMonoU16LosslessBaseline,
		},
		{
			name:       "rgb_u8_lossy",
			fixture:    "classic-rgb-u8-lossy.j2c",
			wantSHA256: "e72b2141404a7603c67f4b87c835c3f74610e550a3f0173f69d25dff667aba5f",
			encode:     encodeClassicRGBU8LossyBaseline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.encode(t)
			path := filepath.Join("testdata", "baseline", tt.fixture)
			assertFrozenCodestream(t, path, actual, tt.wantSHA256)
		})
	}
}

func encodeClassicMonoU16LosslessBaseline(t *testing.T) []byte {
	t.Helper()
	const width, height = 32, 24
	pixels := make([]byte, width*height*2)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value := uint16((x*257 + y*37 + x*y*3) & 0xffff)
			binary.LittleEndian.PutUint16(pixels[2*(y*width+x):], value)
		}
	}
	params := DefaultEncodeParams(width, height, 1, 16, false)
	params.NumLevels = 2
	encoded, err := NewEncoder(params).Encode(pixels)
	if err != nil {
		t.Fatalf("encode classic mono u16 lossless baseline: %v", err)
	}
	return encoded
}

func encodeClassicRGBU8LossyBaseline(t *testing.T) []byte {
	t.Helper()
	const width, height = 32, 24
	pixels := make([]byte, width*height*3)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := 3 * (y*width + x)
			pixels[offset] = byte((x*11 + y*3) & 0xff)
			pixels[offset+1] = byte((x*5 + y*13 + 17) & 0xff)
			pixels[offset+2] = byte((x*7 + y*19 + 31) & 0xff)
		}
	}
	params := DefaultEncodeParams(width, height, 3, 8, false)
	params.NumLevels = 2
	params.Lossless = false
	params.Quality = 73
	encoded, err := NewEncoder(params).Encode(pixels)
	if err != nil {
		t.Fatalf("encode classic RGB u8 lossy baseline: %v", err)
	}
	return encoded
}

func assertFrozenCodestream(t *testing.T, path string, actual []byte, wantSHA256 string) {
	t.Helper()
	if os.Getenv(updateJPEG2000BaselineEnv) == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create baseline directory: %v", err)
		}
		if err := os.WriteFile(path, actual, 0o644); err != nil {
			t.Fatalf("write baseline fixture: %v", err)
		}
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed baseline %s: %v", path, err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("codestream differs from committed baseline %s: got %d bytes, want %d", path, len(actual), len(expected))
	}
	sum := sha256.Sum256(actual)
	gotSHA256 := hex.EncodeToString(sum[:])
	if gotSHA256 != wantSHA256 {
		t.Fatalf("SHA256 for %s = %s, want literal %s", path, gotSHA256, wantSHA256)
	}
}
