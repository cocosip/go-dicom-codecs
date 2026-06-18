package validation

import (
	"math"
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
)

// TestDWT53Reversibility verifies DWT 5/3 is perfectly reversible.
func TestDWT53Reversibility(t *testing.T) {
	testCases := []struct {
		name string
		size int
	}{
		{"Small (8 samples)", 8},
		{"Medium (32 samples)", 32},
		{"Large (128 samples)", 128},
		{"Very Large (512 samples)", 512},
		{"Odd Size (63 samples)", 63},
		{"Power of 2 (256 samples)", 256},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := make([]int32, tc.size)
			for i := range original {
				original[i] = int32((i*7 + i*i/3) % 1024)
			}

			data := append([]int32(nil), original...)
			wavelet.Forward53_1D(data)
			wavelet.Inverse53_1D(data)

			for i := range original {
				if data[i] != original[i] {
					t.Fatalf("sample %d = %d, want %d", i, data[i], original[i])
				}
			}
		})
	}
}

// TestDWT97Precision verifies public DWT 9/7 inverse wrappers use the
// OpenJPEG irreversible float32 decode path.
func TestDWT97Precision(t *testing.T) {
	testCases := []struct {
		name string
		size int
	}{
		{"Small (8 samples)", 8},
		{"Medium (32 samples)", 32},
		{"Large (128 samples)", 128},
		{"Very Large (512 samples)", 512},
		{"Odd Size (63 samples)", 63},
		{"Power of 2 (256 samples)", 256},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := make([]float64, tc.size)
			for i := range input {
				x := float64(i) / float64(tc.size)
				input[i] = 100.0*math.Sin(2*math.Pi*x) +
					50.0*math.Cos(6*math.Pi*x) +
					float64(i%10)*10.0
			}

			got := append([]float64(nil), input...)
			want := wavelet.ConvertFloat64ToFloat32(input)

			wavelet.Inverse97_1D(got)
			wavelet.Inverse97_1DOpenJPEGWithParity(want, true)

			for i := range got {
				if got[i] != float64(want[i]) {
					t.Fatalf("OpenJPEG decode mismatch at %d: got %.6f, want %.6f",
						i, got[i], want[i])
				}
			}
		})
	}
}

// TestDWT53MultiLevel verifies multi-level DWT 5/3 decomposition.
func TestDWT53MultiLevel(t *testing.T) {
	size := 128
	levels := 3

	original := make([]int32, size)
	for i := range original {
		original[i] = int32((i * 11) % 512)
	}

	data := append([]int32(nil), original...)

	currentSize := size
	for level := 0; level < levels; level++ {
		if currentSize <= 1 {
			break
		}
		wavelet.Forward53_1D(data[:currentSize])
		currentSize = (currentSize + 1) / 2
	}

	currentSize = (size + (1 << (levels - 1))) >> levels
	for level := levels - 1; level >= 0; level-- {
		reconstructSize := currentSize * 2
		if reconstructSize > size {
			reconstructSize = size
		}
		wavelet.Inverse53_1D(data[:reconstructSize])
		currentSize = reconstructSize
	}

	for i := range original {
		if data[i] != original[i] {
			t.Fatalf("sample %d = %d, want %d", i, data[i], original[i])
		}
	}
}

// TestDWT97MultiLevel verifies multi-level DWT 9/7 OpenJPEG decode parity.
func TestDWT97MultiLevel(t *testing.T) {
	size := 128
	levels := 3

	input := make([]float64, size)
	for i := range input {
		x := float64(i) / float64(size)
		input[i] = 100.0 * math.Sin(4*math.Pi*x)
	}

	got := append([]float64(nil), input...)
	want := wavelet.ConvertFloat64ToFloat32(input)

	currentSize := (size + (1 << (levels - 1))) >> levels
	for level := levels - 1; level >= 0; level-- {
		reconstructSize := currentSize * 2
		if reconstructSize > size {
			reconstructSize = size
		}
		wavelet.Inverse97_1D(got[:reconstructSize])
		wavelet.Inverse97_1DOpenJPEGWithParity(want[:reconstructSize], true)
		currentSize = reconstructSize
	}

	for i := range got {
		if got[i] != float64(want[i]) {
			t.Fatalf("OpenJPEG multi-level decode mismatch at %d: got %.6f, want %.6f",
				i, got[i], want[i])
		}
	}
}

func TestDWTValidationSummary(t *testing.T) {
	t.Log("DWT 5/3 reversible path: perfect reconstruction verified")
	t.Log("DWT 9/7 irreversible path: OpenJPEG float32 decode parity verified")
}
