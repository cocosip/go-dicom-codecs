package wavelet

import (
	"math"
	"testing"
)

// TestSubbandEnergy97 tests that energy is concentrated in LL subband
func TestSubbandEnergy97(t *testing.T) {
	width, height := 32, 32
	size := width * height

	// Create smooth gradient image
	data := make([]float64, size)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			data[y*width+x] = float64(x+y) / 2.0
		}
	}

	// Forward transform
	Forward97_2D(data, width, height, width)

	// Calculate energy in each subband
	nL := (width + 1) / 2
	mL := (height + 1) / 2

	energyLL := 0.0
	energyHL := 0.0
	energyLH := 0.0
	energyHH := 0.0

	// LL subband (top-left)
	for y := 0; y < mL; y++ {
		for x := 0; x < nL; x++ {
			val := data[y*width+x]
			energyLL += val * val
		}
	}

	// HL subband (top-right)
	for y := 0; y < mL; y++ {
		for x := nL; x < width; x++ {
			val := data[y*width+x]
			energyHL += val * val
		}
	}

	// LH subband (bottom-left)
	for y := mL; y < height; y++ {
		for x := 0; x < nL; x++ {
			val := data[y*width+x]
			energyLH += val * val
		}
	}

	// HH subband (bottom-right)
	for y := mL; y < height; y++ {
		for x := nL; x < width; x++ {
			val := data[y*width+x]
			energyHH += val * val
		}
	}

	totalEnergy := energyLL + energyHL + energyLH + energyHH

	// For smooth images, most energy should be in LL
	llRatio := energyLL / totalEnergy

	if llRatio < 0.95 {
		t.Logf("LL energy ratio: %.4f", llRatio)
		t.Logf("Energies - LL: %.2f, HL: %.2f, LH: %.2f, HH: %.2f",
			energyLL, energyHL, energyLH, energyHH)
	}

	// At least some energy concentration expected
	if llRatio < 0.5 {
		t.Errorf("LL energy ratio too low: %.4f (expected > 0.5)", llRatio)
	}
}

// TestEdgeCases97 tests edge cases for 9/7 transform
func TestEdgeCases97(t *testing.T) {
	t.Run("Size 1", func(t *testing.T) {
		data := []float64{42.5}
		Forward97_1D(data)
		// Size 1 should remain unchanged
		if data[0] != 42.5 {
			t.Errorf("Size 1 changed: got %f, want 42.5", data[0])
		}
	})

	t.Run("Size 2", func(t *testing.T) {
		data := []float32{10.5, 20.5}
		want := []uint32{0x3e7fffe0, 0x41a60001}

		Inverse97_1DFloat32WithParity(data, true)

		for i := range data {
			if bits := math.Float32bits(data[i]); bits != want[i] {
				t.Errorf("OpenJPH decode bits at %d: got %08x, want %08x",
					i, bits, want[i])
			}
		}
	})

	t.Run("All zeros", func(t *testing.T) {
		data := make([]float64, 16)
		Forward97_1D(data)

		// All zeros should remain zeros
		for i, v := range data {
			if math.Abs(v) > 1e-10 {
				t.Errorf("Zero preservation failed at %d: got %f", i, v)
			}
		}
	})

	t.Run("Constant signal", func(t *testing.T) {
		data := make([]float64, 16)
		for i := range data {
			data[i] = 100.5
		}

		Forward97_1D(data)

		// The OpenJPH 9/7 path operates on float32 samples. ARM64
		// rounding can leave residuals slightly above 1e-6 for a constant signal.
		nL := (len(data) + 1) / 2
		for i := nL; i < len(data); i++ {
			if math.Abs(data[i]) > 1e-5 {
				t.Errorf("High-pass coefficient should be near zero: got %f", data[i])
			}
		}
	})
}

// TestConversionFunctions tests int32 <-> float64 conversion
func TestConversionFunctions(t *testing.T) {
	t.Run("Int32 to Float64", func(t *testing.T) {
		input := []int32{-100, -1, 0, 1, 100, 1000}
		output := ConvertInt32ToFloat64(input)

		if len(output) != len(input) {
			t.Fatalf("Length mismatch: got %d, want %d", len(output), len(input))
		}

		for i := range input {
			expected := float64(input[i])
			if output[i] != expected {
				t.Errorf("Conversion failed at %d: got %f, want %f",
					i, output[i], expected)
			}
		}
	})

	t.Run("Float64 to Int32", func(t *testing.T) {
		input := []float64{-100.7, -1.4, 0.0, 1.5, 100.3, 1000.8}
		expected := []int32{-101, -1, 0, 2, 100, 1001}

		output := ConvertFloat64ToInt32(input)

		if len(output) != len(input) {
			t.Fatalf("Length mismatch: got %d, want %d", len(output), len(input))
		}

		for i := range input {
			if output[i] != expected[i] {
				t.Errorf("Conversion failed at %d: got %d, want %d",
					i, output[i], expected[i])
			}
		}
	})

	t.Run("Round trip", func(t *testing.T) {
		original := []int32{-50, -10, 0, 10, 50, 100}
		float := ConvertInt32ToFloat64(original)
		result := ConvertFloat64ToInt32(float)

		for i := range original {
			if result[i] != original[i] {
				t.Errorf("Round trip failed at %d: got %d, want %d",
					i, result[i], original[i])
			}
		}
	})
}

// Benchmark97_1D benchmarks 1D forward transform
func Benchmark97_1D(b *testing.B) {
	sizes := []int{64, 256, 1024}

	for _, size := range sizes {
		b.Run("", func(b *testing.B) {
			data := make([]float64, size)
			for i := range data {
				data[i] = float64(i % 100)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Forward97_1D(data)
			}
		})
	}
}

// Benchmark97_2D benchmarks 2D forward transform
func Benchmark97_2D(b *testing.B) {
	tests := []struct {
		width  int
		height int
	}{
		{64, 64},
		{256, 256},
		{512, 512},
	}

	for _, tt := range tests {
		b.Run("", func(b *testing.B) {
			size := tt.width * tt.height
			data := make([]float64, size)
			for i := range data {
				data[i] = float64(i % 100)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Forward97_2D(data, tt.width, tt.height, tt.width)
			}
		})
	}
}
