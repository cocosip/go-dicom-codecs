package t1

// CalculateMaxBitplane calculates the maximum bitplane for a set of coefficients
func CalculateMaxBitplane(data []int32) int {
	maxAbs := int32(0)
	for _, v := range data {
		abs := v
		if abs < 0 {
			abs = -abs
		}
		if abs > maxAbs {
			maxAbs = abs
		}
	}

	if maxAbs == 0 {
		return -1 // All zeros
	}

	bitplane := 0
	for maxAbs > 0 {
		maxAbs >>= 1
		bitplane++
	}
	return bitplane - 1
}
