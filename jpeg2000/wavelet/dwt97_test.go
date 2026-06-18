package wavelet

import (
	"math"
	"testing"
)

func TestForward97_1DUsesOpenJPEGFloat32Arithmetic(t *testing.T) {
	input := []float64{0, 17, 33, 71, 129, 251, 502, 777, 1023}
	got := append([]float64(nil), input...)
	want := openJPEGForward97_1DFloat32(input, true)

	Forward97_1DWithParity(got, true)

	for i := range got {
		if got[i] != float64(want[i]) {
			t.Fatalf("coefficient %d = %.10f, want OpenJPEG float32 %.10f", i, got[i], want[i])
		}
	}
}

func openJPEGForward97_1DFloat32(input []float64, even bool) []float32 {
	data := make([]float32, len(input))
	for i, v := range input {
		data[i] = float32(v)
	}
	width := len(data)
	var sn, dn int32
	if even {
		sn = int32((width + 1) >> 1)
		dn = int32(width) - sn
	} else {
		sn = int32(width >> 1)
		dn = int32(width) - sn
	}

	var a, b int32
	if even {
		a = 0
		b = 1
	} else {
		a = 1
		b = 0
	}

	openJPEGEncodeStep2Float32(data, a, b+1, dn, min32(dn, sn-b), float32(alpha97))
	openJPEGEncodeStep2Float32(data, b, a+1, sn, min32(sn, dn-a), float32(beta97))
	openJPEGEncodeStep2Float32(data, a, b+1, dn, min32(dn, sn-b), float32(gamma97))
	openJPEGEncodeStep2Float32(data, b, a+1, sn, min32(sn, dn-a), float32(delta97))

	if a == 0 {
		openJPEGEncodeStep1CombinedFloat32(data, sn, dn, float32(invK97), float32(K97))
	} else {
		openJPEGEncodeStep1CombinedFloat32(data, dn, sn, float32(K97), float32(invK97))
	}

	out := make([]float32, len(data))
	if even {
		for i := int32(0); i < sn; i++ {
			out[i] = data[2*i]
		}
		for i := int32(0); i < dn; i++ {
			out[sn+i] = data[2*i+1]
		}
	} else {
		for i := int32(0); i < sn; i++ {
			out[i] = data[2*i+1]
		}
		for i := int32(0); i < dn; i++ {
			out[sn+i] = data[2*i]
		}
	}
	return out
}

func openJPEGEncodeStep2Float32(data []float32, flStart, fwStart, end, m int32, c float32) {
	imax := min32(end, m)
	if imax > 0 {
		fw := fwStart
		fl := flStart
		data[fw-1] += (data[fl] + data[fw]) * c
		fw += 2
		for i := int32(1); i < imax; i++ {
			data[fw-1] += (data[fw-2] + data[fw]) * c
			fw += 2
		}
	}
	if m < end {
		fw := fwStart + 2*m
		data[fw-1] += (2 * data[fw-2]) * c
	}
}

func openJPEGEncodeStep1CombinedFloat32(data []float32, itersC1, itersC2 int32, c1, c2 float32) {
	itersCommon := min32(itersC1, itersC2)
	var i int32
	fw := int32(0)
	for i = 0; i < itersCommon; i++ {
		data[fw] *= c1
		data[fw+1] *= c2
		fw += 2
	}
	if i < itersC1 {
		data[fw] *= c1
	} else if i < itersC2 {
		data[fw+1] *= c2
	}
}

func TestInverse97_1DMatchesOpenJPEGDecode(t *testing.T) {
	tests := []struct {
		name string
		data []float32
		even bool
	}{
		{"Even size 2", []float32{2, 3}, true},
		{"Even size 5", []float32{1.25, -2.5, 3.75, -4.5, 5.25}, true},
		{"Odd size 5", []float32{1.25, -2.5, 3.75, -4.5, 5.25}, false},
		{"Even size 8", []float32{-7, -3, 0, 2, 5, 9, 13, 17}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := append([]float32(nil), tt.data...)
			want := openJPEGInverse97_1DFloat32(tt.data, tt.even)

			Inverse97_1DOpenJPEGWithParity(got, tt.even)

			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("sample %d = %.10f, want OpenJPEG %.10f", i, got[i], want[i])
				}
			}
		})
	}
}

func TestInverse97_1DWrapperUsesOpenJPEGDecode(t *testing.T) {
	input := []float64{1.25, -2.5, 3.75, -4.5, 5.25}
	got := append([]float64(nil), input...)
	want32 := openJPEGInverse97_1DFloat32([]float32{1.25, -2.5, 3.75, -4.5, 5.25}, false)

	Inverse97_1DWithParity(got, false)

	for i := range got {
		if got[i] != float64(want32[i]) {
			t.Fatalf("sample %d = %.10f, want wrapper to expose OpenJPEG %.10f", i, got[i], want32[i])
		}
	}
}

func TestInverseMultilevel97MatchesOpenJPEGDecode(t *testing.T) {
	width, height := 4, 3
	input := []float32{
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
	}
	got := append([]float32(nil), input...)
	want := append([]float32(nil), input...)

	openJPEGInverse97_2DFloat32(want, width, height, width, false, true)
	InverseMultilevel97OpenJPEGWithParity(got, width, height, 1, 1, 0)

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("sample %d = %.10f, want OpenJPEG %.10f", i, got[i], want[i])
		}
	}
}

func openJPEGInverse97_1DFloat32(input []float32, even bool) []float32 {
	data := append([]float32(nil), input...)
	width := len(data)
	var sn, dn int32
	if even {
		sn = int32((width + 1) >> 1)
		dn = int32(width) - sn
	} else {
		sn = int32(width >> 1)
		dn = int32(width) - sn
	}

	var a, b int32
	if even {
		a = 0
		b = 1
	} else {
		a = 1
		b = 0
	}

	openJPEGInterleaveHFloat32(data, dn, sn, even)
	openJPEGDecodeStep1Float32(data, a, sn, float32(K97))
	openJPEGDecodeStep1Float32(data, b, dn, float32(twoInvK97))
	openJPEGDecodeStep2Float32(data, b, a+1, sn, min32(sn, dn-a), float32(-delta97))
	openJPEGDecodeStep2Float32(data, a, b+1, dn, min32(dn, sn-b), float32(-gamma97))
	openJPEGDecodeStep2Float32(data, b, a+1, sn, min32(sn, dn-a), float32(-beta97))
	openJPEGDecodeStep2Float32(data, a, b+1, dn, min32(dn, sn-b), float32(-alpha97))
	return data
}

func openJPEGInverse97_2DFloat32(data []float32, width, height, stride int, evenRow, evenCol bool) {
	if width > 1 {
		row := make([]float32, width)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				row[x] = data[y*stride+x]
			}
			row = openJPEGInverse97_1DFloat32(row, evenRow)
			for x := 0; x < width; x++ {
				data[y*stride+x] = row[x]
			}
		}
	}

	if height > 1 {
		col := make([]float32, height)
		for x := 0; x < width; x++ {
			for y := 0; y < height; y++ {
				col[y] = data[y*stride+x]
			}
			col = openJPEGInverse97_1DFloat32(col, evenCol)
			for y := 0; y < height; y++ {
				data[y*stride+x] = col[y]
			}
		}
	}
}

func openJPEGInterleaveHFloat32(data []float32, dn, sn int32, even bool) {
	tmp := make([]float32, int(dn+sn))
	if even {
		for i := int32(0); i < sn; i++ {
			tmp[2*i] = data[i]
		}
		for i := int32(0); i < dn; i++ {
			tmp[2*i+1] = data[sn+i]
		}
	} else {
		for i := int32(0); i < sn; i++ {
			tmp[2*i+1] = data[i]
		}
		for i := int32(0); i < dn; i++ {
			tmp[2*i] = data[sn+i]
		}
	}
	copy(data, tmp)
}

func openJPEGDecodeStep1Float32(data []float32, start, end int32, c float32) {
	for i := int32(0); i < end; i++ {
		data[start+2*i] *= c
	}
}

func openJPEGDecodeStep2Float32(data []float32, flStart, fwStart, end, m int32, c float32) {
	imax := min32(end, m)
	if imax > 0 {
		fw := fwStart
		fl := flStart
		data[fw-1] += (data[fl] + data[fw]) * c
		fw += 2
		for i := int32(1); i < imax; i++ {
			data[fw-1] += (data[fw-2] + data[fw]) * c
			fw += 2
		}
	}
	if m < end {
		fw := fwStart + 2*m
		data[fw-1] += (2 * data[fw-2]) * c
	}
}

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
		data := []float64{10.5, 20.5}
		want := openJPEGInverse97_1DFloat32([]float32{10.5, 20.5}, true)

		Inverse97_1D(data)

		for i := range data {
			if data[i] != float64(want[i]) {
				t.Errorf("OpenJPEG decode mismatch at %d: got %f, want %f",
					i, data[i], want[i])
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

		// High-pass coefficients should be near zero
		nL := (len(data) + 1) / 2
		for i := nL; i < len(data); i++ {
			if math.Abs(data[i]) > 1e-6 {
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

// TestLossyNature97 tests that OpenJPEG's irreversible decode path is lossy
// and deterministic when converted back to integer samples.
func TestLossyNature97(t *testing.T) {
	width, height := 32, 32
	size := width * height

	// Create int32 test data
	original := make([]int32, size)
	for i := range original {
		original[i] = int32(i % 256)
	}

	data := ConvertInt32ToFloat32(original)

	ForwardMultilevel97Float32WithParity(data, width, height, 2, 0, 0)
	InverseMultilevel97OpenJPEGWithParity(data, width, height, 2, 0, 0)

	result := ConvertFloat32ToInt32OpenJPEG(data)

	// Calculate error
	differences := 0
	maxError := int32(0)
	for i := range original {
		diff := result[i] - original[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > 0 {
			differences++
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("Pixels with differences: %d / %d", differences, size)
	t.Logf("Max error: %d", maxError)

	if differences == 0 {
		t.Fatal("expected irreversible OpenJPEG 9/7 decode path to lose information")
	}
	if maxError == 0 {
		t.Fatal("expected non-zero maximum error")
	}
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
