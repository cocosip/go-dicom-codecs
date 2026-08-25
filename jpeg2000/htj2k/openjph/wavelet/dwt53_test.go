package wavelet

import (
	"math/rand/v2"
	"testing"
)

// TestForwardInverse53_1D tests perfect reconstruction for 1D transform
func TestForwardInverse53_1D(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"Size 2", 2},
		{"Size 4", 4},
		{"Size 8", 8},
		{"Size 16", 16},
		{"Size 32", 32},
		{"Size 64", 64},
		{"Size 100", 100}, // Non-power-of-2
		{"Size 127", 127}, // Odd size
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test signal
			original := make([]int32, tt.size)
			for i := range original {
				original[i] = int32(i*3 - 50) // Some pattern
			}

			// Copy for transform
			data := make([]int32, tt.size)
			copy(data, original)

			// Forward transform
			Forward53_1D(data)

			// Inverse transform
			Inverse53_1D(data)

			// Verify perfect reconstruction
			for i := range data {
				if data[i] != original[i] {
					t.Errorf("Perfect reconstruction failed at index %d: got %d, want %d", i, data[i], original[i])
				}
			}
		})
	}
}

func TestForwardInverse53_1DParity(t *testing.T) {
	tests := []struct {
		name string
		size int
		even bool
	}{
		{"Even start size 5", 5, true},
		{"Odd start size 5", 5, false},
		{"Even start size 8", 8, true},
		{"Odd start size 8", 8, false},
		{"Odd start size 1", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := make([]int32, tt.size)
			for i := range original {
				original[i] = int32(i*7 - 30)
			}

			data := make([]int32, tt.size)
			copy(data, original)

			Forward53_1DWithParity(data, tt.even)
			Inverse53_1DWithParity(data, tt.even)

			for i := range data {
				if data[i] != original[i] {
					t.Errorf("Parity reconstruction failed at %d: got %d, want %d", i, data[i], original[i])
				}
			}
		})
	}
}

// TestForwardInverse53_2D tests perfect reconstruction for 2D transform
func TestForwardInverse53_2D(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"2x2", 2, 2},
		{"4x4", 4, 4},
		{"8x8", 8, 8},
		{"16x16", 16, 16},
		{"32x32", 32, 32},
		{"64x64", 64, 64},
		{"128x64", 128, 64},   // Non-square
		{"100x100", 100, 100}, // Non-power-of-2
		{"33x17", 33, 17},     // Odd dimensions
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height

			// Create test image with gradient pattern
			original := make([]int32, size)
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					original[y*tt.width+x] = int32(x + y*2)
				}
			}

			// Copy for transform
			data := make([]int32, size)
			copy(data, original)

			// Forward transform (stride = width for single-level)
			Forward53_2D(data, tt.width, tt.height, tt.width)

			// Inverse transform (stride = width for single-level)
			Inverse53_2D(data, tt.width, tt.height, tt.width)

			// Verify perfect reconstruction
			errors := 0
			for i := range data {
				if data[i] != original[i] {
					errors++
					if errors <= 5 { // Show first 5 errors
						t.Errorf("Reconstruction error at index %d: got %d, want %d", i, data[i], original[i])
					}
				}
			}

			if errors > 0 {
				t.Errorf("Total reconstruction errors: %d/%d", errors, size)
			}
		})
	}
}

func TestForwardInverse53_2DParity(t *testing.T) {
	width, height := 17, 19
	size := width * height

	original := make([]int32, size)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			original[y*width+x] = int32(x*3 - y*2)
		}
	}

	data := make([]int32, size)
	copy(data, original)

	Forward53_2DWithParity(data, width, height, width, false, true)
	Inverse53_2DWithParity(data, width, height, width, false, true)

	for i := range data {
		if data[i] != original[i] {
			t.Errorf("2D parity reconstruction failed at %d: got %d, want %d", i, data[i], original[i])
			break
		}
	}
}

// TestForwardInverseMultilevel tests multilevel decomposition
func TestForwardInverseMultilevel(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		levels int
	}{
		{"64x64 1-level", 64, 64, 1},
		{"64x64 2-level", 64, 64, 2},
		{"64x64 3-level", 64, 64, 3},
		{"128x128 5-level", 128, 128, 5},
		{"256x256 6-level", 256, 256, 6},
		{"100x100 3-level", 100, 100, 3}, // Non-power-of-2
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height

			// Create test image
			original := make([]int32, size)
			rng := rand.New(rand.NewPCG(42, 0)) // Deterministic
			for i := range original {
				original[i] = int32(rng.IntN(256))
			}

			// Copy for transform
			data := make([]int32, size)
			copy(data, original)

			// Multilevel forward transform
			ForwardMultilevel(data, tt.width, tt.height, tt.levels)

			// Multilevel inverse transform
			InverseMultilevel(data, tt.width, tt.height, tt.levels)

			// Verify perfect reconstruction
			errors := 0
			maxError := int32(0)
			for i := range data {
				diff := data[i] - original[i]
				if diff < 0 {
					diff = -diff
				}
				if diff > maxError {
					maxError = diff
				}
				if data[i] != original[i] {
					errors++
				}
			}

			if errors > 0 {
				t.Errorf("Multilevel reconstruction failed: %d errors, max error = %d", errors, maxError)
			} else {
				t.Logf("Perfect reconstruction for %dx%d with %d levels", tt.width, tt.height, tt.levels)
			}
		})
	}
}

func TestForwardInverseMultilevelParity(t *testing.T) {
	width, height := 64, 48
	levels := 3
	x0, y0 := 1, 2
	size := width * height

	original := make([]int32, size)
	rng := rand.New(rand.NewPCG(7, 0))
	for i := range original {
		original[i] = int32(rng.IntN(1024) - 512)
	}

	data := make([]int32, size)
	copy(data, original)

	ForwardMultilevelWithParity(data, width, height, levels, x0, y0)
	InverseMultilevelWithParity(data, width, height, levels, x0, y0)

	for i := range data {
		if data[i] != original[i] {
			t.Errorf("Multilevel parity reconstruction failed at %d: got %d, want %d", i, data[i], original[i])
			break
		}
	}
}

// TestSubbandEnergy tests that energy is concentrated in LL subband
func TestSubbandEnergy(t *testing.T) {
	width, height := 64, 64
	size := width * height

	// Create smooth test image (mostly low-frequency content)
	data := make([]int32, size)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			data[y*width+x] = int32(100 + 50*((x+y)%10))
		}
	}

	// Forward transform (stride = width for single-level)
	Forward53_2D(data, width, height, width)

	// Calculate energy in each subband
	wL := (width + 1) / 2
	hL := (height + 1) / 2
	wH := width / 2
	hH := height / 2

	var energyLL, energyHL, energyLH, energyHH int64

	// LL subband (top-left)
	for y := 0; y < hL; y++ {
		for x := 0; x < wL; x++ {
			val := data[y*width+x]
			energyLL += int64(val) * int64(val)
		}
	}

	// HL subband (top-right)
	for y := 0; y < hL; y++ {
		for x := 0; x < wH; x++ {
			val := data[y*width+wL+x]
			energyHL += int64(val) * int64(val)
		}
	}

	// LH subband (bottom-left)
	for y := 0; y < hH; y++ {
		for x := 0; x < wL; x++ {
			val := data[(hL+y)*width+x]
			energyLH += int64(val) * int64(val)
		}
	}

	// HH subband (bottom-right)
	for y := 0; y < hH; y++ {
		for x := 0; x < wH; x++ {
			val := data[(hL+y)*width+wL+x]
			energyHH += int64(val) * int64(val)
		}
	}

	totalEnergy := energyLL + energyHL + energyLH + energyHH
	llPercent := float64(energyLL) / float64(totalEnergy) * 100

	t.Logf("Energy distribution:")
	t.Logf("  LL: %.2f%%", float64(energyLL)/float64(totalEnergy)*100)
	t.Logf("  HL: %.2f%%", float64(energyHL)/float64(totalEnergy)*100)
	t.Logf("  LH: %.2f%%", float64(energyLH)/float64(totalEnergy)*100)
	t.Logf("  HH: %.2f%%", float64(energyHH)/float64(totalEnergy)*100)

	// For smooth images, LL should have most of the energy
	if llPercent < 50 {
		t.Errorf("Expected LL subband to have >50%% energy, got %.2f%%", llPercent)
	}
}

// TestEdgeCases tests edge cases
func TestEdgeCases(t *testing.T) {
	t.Run("Size 1", func(t *testing.T) {
		data := []int32{42}
		original := []int32{42}

		Forward53_1D(data)
		Inverse53_1D(data)

		if data[0] != original[0] {
			t.Errorf("Size 1 failed: got %d, want %d", data[0], original[0])
		}
	})

	t.Run("Size 1x1", func(t *testing.T) {
		data := []int32{42}
		original := []int32{42}

		Forward53_2D(data, 1, 1, 1)
		Inverse53_2D(data, 1, 1, 1)

		if data[0] != original[0] {
			t.Errorf("Size 1x1 failed: got %d, want %d", data[0], original[0])
		}
	})

	t.Run("All zeros", func(t *testing.T) {
		data := make([]int32, 64)
		original := make([]int32, 64)

		Forward53_1D(data)
		Inverse53_1D(data)

		for i := range data {
			if data[i] != original[i] {
				t.Errorf("All zeros failed at %d", i)
				break
			}
		}
	})

	t.Run("Constant value", func(t *testing.T) {
		data := make([]int32, 64)
		for i := range data {
			data[i] = 100
		}
		original := make([]int32, 64)
		copy(original, data)

		Forward53_1D(data)
		Inverse53_1D(data)

		for i := range data {
			if data[i] != original[i] {
				t.Errorf("Constant value failed at %d: got %d, want %d", i, data[i], original[i])
				break
			}
		}
	})
}

// TestRangeLimits tests with extreme values
func TestRangeLimits(t *testing.T) {
	t.Run("8-bit range", func(t *testing.T) {
		size := 64
		data := make([]int32, size)
		for i := range data {
			data[i] = int32(i % 256)
		}
		original := make([]int32, size)
		copy(original, data)

		Forward53_1D(data)
		Inverse53_1D(data)

		for i := range data {
			if data[i] != original[i] {
				t.Errorf("8-bit range failed at %d: got %d, want %d", i, data[i], original[i])
			}
		}
	})

	t.Run("12-bit range", func(t *testing.T) {
		size := 64
		data := make([]int32, size)
		for i := range data {
			data[i] = int32(i * 64) // 0 to ~4000
		}
		original := make([]int32, size)
		copy(original, data)

		Forward53_1D(data)
		Inverse53_1D(data)

		for i := range data {
			if data[i] != original[i] {
				t.Errorf("12-bit range failed at %d: got %d, want %d", i, data[i], original[i])
			}
		}
	})

	t.Run("16-bit range", func(t *testing.T) {
		size := 64
		data := make([]int32, size)
		for i := range data {
			data[i] = int32(i * 1000) // Large values
		}
		original := make([]int32, size)
		copy(original, data)

		Forward53_1D(data)
		Inverse53_1D(data)

		for i := range data {
			if data[i] != original[i] {
				t.Errorf("16-bit range failed at %d: got %d, want %d", i, data[i], original[i])
			}
		}
	})
}

// BenchmarkForward53_1D benchmarks 1D forward transform
func BenchmarkForward53_1D(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}

	for _, size := range sizes {
		b.Run(string(rune(size)), func(b *testing.B) {
			data := make([]int32, size)
			for i := range data {
				data[i] = int32(i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Forward53_1D(data)
			}
		})
	}
}

// BenchmarkForward53_2D benchmarks 2D forward transform
func BenchmarkForward53_2D(b *testing.B) {
	sizes := []struct {
		width  int
		height int
	}{
		{64, 64},
		{256, 256},
		{512, 512},
		{1024, 1024},
	}

	for _, size := range sizes {
		name := string(rune(size.width)) + "x" + string(rune(size.height))
		b.Run(name, func(b *testing.B) {
			data := make([]int32, size.width*size.height)
			for i := range data {
				data[i] = int32(i % 256)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Forward53_2D(data, size.width, size.height, size.width)
			}
		})
	}
}
