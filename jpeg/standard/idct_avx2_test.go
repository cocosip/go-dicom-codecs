//go:build amd64 && !purego

package standard

import (
	"math/rand"
	"testing"
)

func TestIDCTAVX2MatchesFast(t *testing.T) {
	if !HasAVX2() {
		t.Skip("AVX2 not available")
	}
	rng := rand.New(rand.NewSource(3))
	var coef [64]int32
	want := make([]byte, 64)
	got := make([]byte, 64)
	maxDiff := 0
	for n := 0; n < 2000; n++ {
		for i := range coef {
			coef[i] = 0
			if rng.Intn(2) == 0 {
				coef[i] = int32(rng.Intn(4097) - 2048)
			}
		}
		IDCTFast(coef[:], want, 8)
		IDCT8x8(coef[:], got, 8)
		for i := range want {
			d := int(want[i]) - int(got[i])
			if d < 0 {
				d = -d
			}
			if d > maxDiff {
				maxDiff = d
			}
		}
	}
	if maxDiff > 1 {
		t.Fatalf("max diff = %d, want <= 1", maxDiff)
	}
	t.Logf("max diff = %d", maxDiff)
}

func TestIDCTAVX2Stride(t *testing.T) {
	if !HasAVX2() {
		t.Skip("AVX2 not available")
	}
	var coef [64]int32
	for i := range coef {
		coef[i] = int32(i*37 - 600)
	}
	want := make([]byte, 7*16+8)
	got := make([]byte, 7*16+8)
	IDCTFast(coef[:], want, 16)
	IDCT8x8(coef[:], got, 16)
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("byte %d: fast=%d avx2=%d", i, want[i], got[i])
		}
	}
}
