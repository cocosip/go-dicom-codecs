//go:build amd64 && !purego

package standard

// idct8x8AVX2 applies the integer IDCT to one dequantized 8x8 block.
// coef must point to 64 int32 values and out to at least 7*stride+8 bytes.
//
//go:noescape
func idct8x8AVX2(coef *[64]int32, out *[64]byte, stride int)
