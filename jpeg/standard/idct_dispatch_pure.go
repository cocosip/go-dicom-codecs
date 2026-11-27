//go:build !amd64 || purego

package standard

// IDCT8x8 applies the inverse DCT to one dequantized block.
func IDCT8x8(coef []int32, out []byte, stride int) {
	IDCTFast(coef, out, stride)
}

func clampIDCTByte(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}
