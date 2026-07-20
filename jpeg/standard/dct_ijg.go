package standard

const (
	ijgConstBits = 13
	ijgPass1Bits = 2

	ijgFix0298631336 = 2446
	ijgFix0390180644 = 3196
	ijgFix0541196100 = 4433
	ijgFix0765366865 = 6270
	ijgFix0899976223 = 7373
	ijgFix1175875602 = 9633
	ijgFix1501321110 = 12299
	ijgFix1847759065 = 15137
	ijgFix1961570560 = 16069
	ijgFix2053119869 = 16819
	ijgFix2562915447 = 20995
	ijgFix3072711026 = 25172
)

// DCTISlow performs the IJG integer forward DCT. Its coefficients retain the
// factor-of-eight scale expected by libjpeg's integer quantizer.
func DCTISlow(input []byte, stride int, coef []int32) {
	var data [64]int32
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			data[y*8+x] = int32(input[y*stride+x]) - 128
		}
	}

	for y := 0; y < 8; y++ {
		row := y * 8
		tmp0 := data[row] + data[row+7]
		tmp7 := data[row] - data[row+7]
		tmp1 := data[row+1] + data[row+6]
		tmp6 := data[row+1] - data[row+6]
		tmp2 := data[row+2] + data[row+5]
		tmp5 := data[row+2] - data[row+5]
		tmp3 := data[row+3] + data[row+4]
		tmp4 := data[row+3] - data[row+4]

		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2

		data[row] = (tmp10 + tmp11) << ijgPass1Bits
		data[row+4] = (tmp10 - tmp11) << ijgPass1Bits

		z1 := (tmp12 + tmp13) * ijgFix0541196100
		data[row+2] = ijgDescale(z1+tmp13*ijgFix0765366865, ijgConstBits-ijgPass1Bits)
		data[row+6] = ijgDescale(z1-tmp12*ijgFix1847759065, ijgConstBits-ijgPass1Bits)

		z1 = tmp4 + tmp7
		z2 := tmp5 + tmp6
		z3 := tmp4 + tmp6
		z4 := tmp5 + tmp7
		z5 := (z3 + z4) * ijgFix1175875602
		tmp4 *= ijgFix0298631336
		tmp5 *= ijgFix2053119869
		tmp6 *= ijgFix3072711026
		tmp7 *= ijgFix1501321110
		z1 *= -ijgFix0899976223
		z2 *= -ijgFix2562915447
		z3 *= -ijgFix1961570560
		z4 *= -ijgFix0390180644
		z3 += z5
		z4 += z5

		data[row+7] = ijgDescale(tmp4+z1+z3, ijgConstBits-ijgPass1Bits)
		data[row+5] = ijgDescale(tmp5+z2+z4, ijgConstBits-ijgPass1Bits)
		data[row+3] = ijgDescale(tmp6+z2+z3, ijgConstBits-ijgPass1Bits)
		data[row+1] = ijgDescale(tmp7+z1+z4, ijgConstBits-ijgPass1Bits)
	}

	for x := 0; x < 8; x++ {
		tmp0 := data[x] + data[56+x]
		tmp7 := data[x] - data[56+x]
		tmp1 := data[8+x] + data[48+x]
		tmp6 := data[8+x] - data[48+x]
		tmp2 := data[16+x] + data[40+x]
		tmp5 := data[16+x] - data[40+x]
		tmp3 := data[24+x] + data[32+x]
		tmp4 := data[24+x] - data[32+x]

		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2

		coef[x] = ijgDescale(tmp10+tmp11, ijgPass1Bits)
		coef[32+x] = ijgDescale(tmp10-tmp11, ijgPass1Bits)

		z1 := (tmp12 + tmp13) * ijgFix0541196100
		coef[16+x] = ijgDescale(z1+tmp13*ijgFix0765366865, ijgConstBits+ijgPass1Bits)
		coef[48+x] = ijgDescale(z1-tmp12*ijgFix1847759065, ijgConstBits+ijgPass1Bits)

		z1 = tmp4 + tmp7
		z2 := tmp5 + tmp6
		z3 := tmp4 + tmp6
		z4 := tmp5 + tmp7
		z5 := (z3 + z4) * ijgFix1175875602
		tmp4 *= ijgFix0298631336
		tmp5 *= ijgFix2053119869
		tmp6 *= ijgFix3072711026
		tmp7 *= ijgFix1501321110
		z1 *= -ijgFix0899976223
		z2 *= -ijgFix2562915447
		z3 *= -ijgFix1961570560
		z4 *= -ijgFix0390180644
		z3 += z5
		z4 += z5

		coef[56+x] = ijgDescale(tmp4+z1+z3, ijgConstBits+ijgPass1Bits)
		coef[40+x] = ijgDescale(tmp5+z2+z4, ijgConstBits+ijgPass1Bits)
		coef[24+x] = ijgDescale(tmp6+z2+z3, ijgConstBits+ijgPass1Bits)
		coef[8+x] = ijgDescale(tmp7+z1+z4, ijgConstBits+ijgPass1Bits)
	}
}

func ijgDescale(value int32, shift uint) int32 {
	return (value + (1 << (shift - 1))) >> shift
}
