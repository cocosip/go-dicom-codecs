//go:build !amd64 || purego

package standard

// HasAVX2 reports whether the current CPU and OS support AVX2.
func HasAVX2() bool {
	return false
}
