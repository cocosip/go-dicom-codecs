package extended

// idct12Block applies the integer stb_image inverse DCT to one dequantized
// 12-bit block. int64 intermediates avoid overflow at 12-bit coefficient range.
func idct12Block(coef *[64]int64, out *[64]int64) {
	var ws [64]int64

	for c := 0; c < 8; c++ {
		p2 := coef[2*8+c]
		p3 := coef[6*8+c]
		p1 := (p2 + p3) * 2217
		t2 := p1 + p3*(-7567)
		t3 := p1 + p2*3135

		p2 = coef[0*8+c]
		p3 = coef[4*8+c]
		t0 := (p2 + p3) << 12
		t1 := (p2 - p3) << 12

		x0 := t0 + t3
		x3 := t0 - t3
		x1 := t1 + t2
		x2 := t1 - t2

		t0 = coef[7*8+c]
		t1 = coef[5*8+c]
		t2 = coef[3*8+c]
		t3 = coef[1*8+c]

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

		x0 += 2048
		x1 += 2048
		x2 += 2048
		x3 += 2048

		ws[0*8+c] = (x0 + t3) >> 12
		ws[7*8+c] = (x0 - t3) >> 12
		ws[1*8+c] = (x1 + t2) >> 12
		ws[6*8+c] = (x1 - t2) >> 12
		ws[2*8+c] = (x2 + t1) >> 12
		ws[5*8+c] = (x2 - t1) >> 12
		ws[3*8+c] = (x3 + t0) >> 12
		ws[4*8+c] = (x3 - t0) >> 12
	}

	const bias = 16384 + (2048 << 15)
	for r := 0; r < 8; r++ {
		p2 := ws[r*8+2]
		p3 := ws[r*8+6]
		p1 := (p2 + p3) * 2217
		t2 := p1 + p3*(-7567)
		t3 := p1 + p2*3135

		p2 = ws[r*8+0]
		p3 = ws[r*8+4]
		t0 := (p2 + p3) << 12
		t1 := (p2 - p3) << 12

		x0 := t0 + t3
		x3 := t0 - t3
		x1 := t1 + t2
		x2 := t1 - t2

		t0 = ws[r*8+7]
		t1 = ws[r*8+5]
		t2 = ws[r*8+3]
		t3 = ws[r*8+1]

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

		x0 += bias
		x1 += bias
		x2 += bias
		x3 += bias

		out[r*8+0] = (x0 + t3) >> 15
		out[r*8+7] = (x0 - t3) >> 15
		out[r*8+1] = (x1 + t2) >> 15
		out[r*8+6] = (x1 - t2) >> 15
		out[r*8+2] = (x2 + t1) >> 15
		out[r*8+5] = (x2 - t1) >> 15
		out[r*8+3] = (x3 + t0) >> 15
		out[r*8+4] = (x3 - t0) >> 15
	}
}
