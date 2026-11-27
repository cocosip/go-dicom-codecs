//go:build amd64 && !purego

package standard

func cpuid(eax, ecx uint32) (a, b, c, d uint32)
func xgetbv(ecx uint32) uint64

var hasAVX2 = detectAVX2()

// HasAVX2 reports whether the current CPU and OS support AVX2.
func HasAVX2() bool {
	return hasAVX2
}

func detectAVX2() bool {
	maxLeaf, _, _, _ := cpuid(0, 0)
	if maxLeaf < 7 {
		return false
	}
	_, _, c1, _ := cpuid(1, 0)
	const (
		osxsave = 1 << 27
		avx     = 1 << 28
	)
	if c1&osxsave == 0 || c1&avx == 0 {
		return false
	}
	// XCR0 bits 1 and 2: XMM and YMM state saved by the OS.
	if xgetbv(0)&0x6 != 0x6 {
		return false
	}
	_, b7, _, _ := cpuid(7, 0)
	return b7&(1<<5) != 0
}
