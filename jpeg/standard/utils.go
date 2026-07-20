// Package standard contains shared helpers and tables used across JPEG codecs.
package standard

// Clamp clamps a value between min and max
func Clamp(v, minVal, maxVal int) int {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

// DivCeil performs ceiling division
func DivCeil(a, b int) int {
	return (a + b - 1) / b
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two integers
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ZigZag maps a JPEG scan position to its natural 8x8 coefficient index.
var ZigZag = [64]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}

// Unzig maps a natural 8x8 coefficient index to its JPEG scan position.
var Unzig [64]int

func init() {
	for i := range ZigZag {
		Unzig[ZigZag[i]] = i
	}
}
