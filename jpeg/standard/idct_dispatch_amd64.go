//go:build amd64 && !purego

package standard

import "unsafe"

// IDCT8x8 applies the inverse DCT to one dequantized block. It uses the AVX2
// kernel when the CPU supports it and falls back to the portable integer IDCT.
func IDCT8x8(coef []int32, out []byte, stride int) {
	if HasAVX2() && len(coef) >= 64 && stride >= 8 && len(out) >= 7*stride+8 {
		idct8x8AVX2((*[64]int32)(unsafe.Pointer(&coef[0])), (*[64]byte)(unsafe.Pointer(&out[0])), stride)
		return
	}
	IDCTFast(coef, out, stride)
}
