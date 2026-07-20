package standard

// IDCTISlow performs libjpeg's integer inverse DCT and dequantization.
func IDCTISlow(coef []int32, qtable [64]int32, out []byte, stride int) {
	var workspace [64]int32
	for x := 0; x < 8; x++ {
		z2 := coef[16+x] * qtable[16+x]
		z3 := coef[48+x] * qtable[48+x]
		z1 := (z2 + z3) * ijgFix0541196100
		tmp2 := z1 - z3*ijgFix1847759065
		tmp3 := z1 + z2*ijgFix0765366865
		z2 = coef[x] * qtable[x]
		z3 = coef[32+x] * qtable[32+x]
		tmp0 := (z2 + z3) << ijgConstBits
		tmp1 := (z2 - z3) << ijgConstBits
		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2

		tmp0 = coef[56+x] * qtable[56+x]
		tmp1 = coef[40+x] * qtable[40+x]
		tmp2 = coef[24+x] * qtable[24+x]
		tmp3 = coef[8+x] * qtable[8+x]
		z1 = tmp0 + tmp3
		z2 = tmp1 + tmp2
		z3 = tmp0 + tmp2
		z4 := tmp1 + tmp3
		z5 := (z3 + z4) * ijgFix1175875602
		tmp0 *= ijgFix0298631336
		tmp1 *= ijgFix2053119869
		tmp2 *= ijgFix3072711026
		tmp3 *= ijgFix1501321110
		z1 *= -ijgFix0899976223
		z2 *= -ijgFix2562915447
		z3 *= -ijgFix1961570560
		z4 *= -ijgFix0390180644
		z3 += z5
		z4 += z5
		tmp0 += z1 + z3
		tmp1 += z2 + z4
		tmp2 += z2 + z3
		tmp3 += z1 + z4

		workspace[x] = ijgDescale(tmp10+tmp3, ijgConstBits-ijgPass1Bits)
		workspace[56+x] = ijgDescale(tmp10-tmp3, ijgConstBits-ijgPass1Bits)
		workspace[8+x] = ijgDescale(tmp11+tmp2, ijgConstBits-ijgPass1Bits)
		workspace[48+x] = ijgDescale(tmp11-tmp2, ijgConstBits-ijgPass1Bits)
		workspace[16+x] = ijgDescale(tmp12+tmp1, ijgConstBits-ijgPass1Bits)
		workspace[40+x] = ijgDescale(tmp12-tmp1, ijgConstBits-ijgPass1Bits)
		workspace[24+x] = ijgDescale(tmp13+tmp0, ijgConstBits-ijgPass1Bits)
		workspace[32+x] = ijgDescale(tmp13-tmp0, ijgConstBits-ijgPass1Bits)
	}

	for y := 0; y < 8; y++ {
		row := y * 8
		z2 := workspace[row+2]
		z3 := workspace[row+6]
		z1 := (z2 + z3) * ijgFix0541196100
		tmp2 := z1 - z3*ijgFix1847759065
		tmp3 := z1 + z2*ijgFix0765366865
		tmp0 := (workspace[row] + workspace[row+4]) << ijgConstBits
		tmp1 := (workspace[row] - workspace[row+4]) << ijgConstBits
		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2

		tmp0 = workspace[row+7]
		tmp1 = workspace[row+5]
		tmp2 = workspace[row+3]
		tmp3 = workspace[row+1]
		z1 = tmp0 + tmp3
		z2 = tmp1 + tmp2
		z3 = tmp0 + tmp2
		z4 := tmp1 + tmp3
		z5 := (z3 + z4) * ijgFix1175875602
		tmp0 *= ijgFix0298631336
		tmp1 *= ijgFix2053119869
		tmp2 *= ijgFix3072711026
		tmp3 *= ijgFix1501321110
		z1 *= -ijgFix0899976223
		z2 *= -ijgFix2562915447
		z3 *= -ijgFix1961570560
		z4 *= -ijgFix0390180644
		z3 += z5
		z4 += z5
		tmp0 += z1 + z3
		tmp1 += z2 + z4
		tmp2 += z2 + z3
		tmp3 += z1 + z4

		out[row/8*stride+0] = byte(Clamp(int(ijgDescale(tmp10+tmp3, ijgConstBits+ijgPass1Bits+3))+128, 0, 255))
		out[row/8*stride+7] = byte(Clamp(int(ijgDescale(tmp10-tmp3, ijgConstBits+ijgPass1Bits+3))+128, 0, 255))
		out[row/8*stride+1] = byte(Clamp(int(ijgDescale(tmp11+tmp2, ijgConstBits+ijgPass1Bits+3))+128, 0, 255))
		out[row/8*stride+6] = byte(Clamp(int(ijgDescale(tmp11-tmp2, ijgConstBits+ijgPass1Bits+3))+128, 0, 255))
		out[row/8*stride+2] = byte(Clamp(int(ijgDescale(tmp12+tmp1, ijgConstBits+ijgPass1Bits+3))+128, 0, 255))
		out[row/8*stride+5] = byte(Clamp(int(ijgDescale(tmp12-tmp1, ijgConstBits+ijgPass1Bits+3))+128, 0, 255))
		out[row/8*stride+3] = byte(Clamp(int(ijgDescale(tmp13+tmp0, ijgConstBits+ijgPass1Bits+3))+128, 0, 255))
		out[row/8*stride+4] = byte(Clamp(int(ijgDescale(tmp13-tmp0, ijgConstBits+ijgPass1Bits+3))+128, 0, 255))
	}
}
