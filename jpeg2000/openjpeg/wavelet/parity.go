package wavelet

func splitLengths(n int, even bool) (low int) {
	if even {
		return (n + 1) / 2
	}
	return n / 2
}

func isEven(value int) bool {
	return value&1 == 0
}

func nextCoord(value int) int {
	return (value + 1) >> 1
}
