package standard

// IDCTFast performs a fast integer inverse DCT on dequantized coefficients.
// The algorithm is the well-known stb_image IDCT (public domain), which is
// accurate to within one sample of the slow integer transform.
func IDCTFast(coef []int32, out []byte, stride int) {
	var val [64]int32

	// Columns.
	for i := 0; i < 8; i++ {
		d := i
		if coef[d+8] == 0 && coef[d+16] == 0 && coef[d+24] == 0 &&
			coef[d+32] == 0 && coef[d+40] == 0 && coef[d+48] == 0 && coef[d+56] == 0 {
			dc := coef[d] * 4
			val[d] = dc
			val[d+8] = dc
			val[d+16] = dc
			val[d+24] = dc
			val[d+32] = dc
			val[d+40] = dc
			val[d+48] = dc
			val[d+56] = dc
			continue
		}

		p2 := coef[d+16]
		p3 := coef[d+48]
		p1 := (p2 + p3) * 2217
		t2 := p1 + p3*(-7567)
		t3 := p1 + p2*3135
		p2 = coef[d]
		p3 = coef[d+32]
		t0 := (p2 + p3) << 12
		t1 := (p2 - p3) << 12
		x0 := t0 + t3
		x3 := t0 - t3
		x1 := t1 + t2
		x2 := t1 - t2

		t0 = coef[d+56]
		t1 = coef[d+40]
		t2 = coef[d+24]
		t3 = coef[d+8]
		p3 = t0 + t2
		p4 := t1 + t3
		p1 = t0 + t3
		p2 = t1 + t2
		p5 := (p3 + p4) * 4816
		t0 *= 1223
		t1 *= 8410
		t2 *= 12586
		t3 *= 6149
		p1 *= -3685
		p2 *= -10497
		p3 *= -8034
		p4 *= -1597
		p3 += p5
		p4 += p5
		t3 += p1 + p4
		t2 += p2 + p3
		t1 += p2 + p4
		t0 += p1 + p3

		x0 += 512
		x1 += 512
		x2 += 512
		x3 += 512
		val[d] = (x0 + t3) >> 10
		val[d+56] = (x0 - t3) >> 10
		val[d+8] = (x1 + t2) >> 10
		val[d+48] = (x1 - t2) >> 10
		val[d+16] = (x2 + t1) >> 10
		val[d+40] = (x2 - t1) >> 10
		val[d+24] = (x3 + t0) >> 10
		val[d+32] = (x3 - t0) >> 10
	}

	// Rows.
	for i := 0; i < 8; i++ {
		v := i * 8
		p2 := val[v+2]
		p3 := val[v+6]
		p1 := (p2 + p3) * 2217
		t2 := p1 + p3*(-7567)
		t3 := p1 + p2*3135
		p2 = val[v]
		p3 = val[v+4]
		t0 := (p2 + p3) << 12
		t1 := (p2 - p3) << 12
		x0 := t0 + t3
		x3 := t0 - t3
		x1 := t1 + t2
		x2 := t1 - t2

		t0 = val[v+7]
		t1 = val[v+5]
		t2 = val[v+3]
		t3 = val[v+1]
		p3 = t0 + t2
		p4 := t1 + t3
		p1 = t0 + t3
		p2 = t1 + t2
		p5 := (p3 + p4) * 4816
		t0 *= 1223
		t1 *= 8410
		t2 *= 12586
		t3 *= 6149
		p1 *= -3685
		p2 *= -10497
		p3 *= -8034
		p4 *= -1597
		p3 += p5
		p4 += p5
		t3 += p1 + p4
		t2 += p2 + p3
		t1 += p2 + p4
		t0 += p1 + p3

		x0 += 65536 + (128 << 17)
		x1 += 65536 + (128 << 17)
		x2 += 65536 + (128 << 17)
		x3 += 65536 + (128 << 17)
		o := i * stride
		out[o+0] = clampByte((x0 + t3) >> 17)
		out[o+7] = clampByte((x0 - t3) >> 17)
		out[o+1] = clampByte((x1 + t2) >> 17)
		out[o+6] = clampByte((x1 - t2) >> 17)
		out[o+2] = clampByte((x2 + t1) >> 17)
		out[o+5] = clampByte((x2 - t1) >> 17)
		out[o+3] = clampByte((x3 + t0) >> 17)
		out[o+4] = clampByte((x3 - t0) >> 17)
	}
}

func clampByte(v int32) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}
