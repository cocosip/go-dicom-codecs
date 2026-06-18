package jpeg2000

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"testing"
)

func nearlyEqual(a, b, tol float64) bool {
	if a == 0 && b == 0 {
		return true
	}
	diff := math.Abs(a - b)
	denom := math.Max(math.Abs(a), math.Abs(b))
	if denom == 0 {
		return diff <= tol
	}
	return diff/denom <= tol
}

func TestCalculateQuantizationParams_StyleAndLengths(t *testing.T) {
	// quality 100 should still produce near-lossless lossy quantization
	numLevels := 4
	pNearLossless := CalculateQuantizationParams(100, numLevels, 16)
	if pNearLossless.Style != 2 {
		t.Fatalf("expected Style=2 for quality=100, got %d", pNearLossless.Style)
	}
	expectedSubbands := 3*numLevels + 1
	if len(pNearLossless.StepSizes) != expectedSubbands || len(pNearLossless.EncodedSteps) != expectedSubbands {
		t.Fatalf("quality=100 should emit full subband steps, got steps=%d encoded=%d want=%d", len(pNearLossless.StepSizes), len(pNearLossless.EncodedSteps), expectedSubbands)
	}
	if pNearLossless.GuardBits != 2 {
		t.Fatalf("expected GuardBits=2, got %d", pNearLossless.GuardBits)
	}

	// lossy mode
	pLossy := CalculateQuantizationParams(80, numLevels, 16)
	if pLossy.Style != 2 {
		t.Fatalf("expected Style=2 for lossy mode, got %d", pLossy.Style)
	}
	if len(pLossy.StepSizes) != expectedSubbands || len(pLossy.EncodedSteps) != expectedSubbands {
		t.Fatalf("unexpected subband count: got steps=%d encoded=%d want=%d", len(pLossy.StepSizes), len(pLossy.EncodedSteps), expectedSubbands)
	}

	scale := qualityScale(80)
	expected := calcOpenJPEGStepSizes97(numLevels, scale)
	if !nearlyEqual(pLossy.StepSizes[0], expected[0], 0.01) {
		t.Fatalf("LL step mismatch: got %.6f want %.6f", pLossy.StepSizes[0], expected[0])
	}
	if !nearlyEqual(pLossy.StepSizes[1], pLossy.StepSizes[2], 0.01) {
		t.Fatalf("expected HL1 and LH1 to match, got HL1=%.6f LH1=%.6f", pLossy.StepSizes[1], pLossy.StepSizes[2])
	}
}

func TestEncodedSteps_DecodeApprox(t *testing.T) {
	p := CalculateQuantizationParams(80, 3, 16)
	for i, enc := range p.EncodedSteps {
		decoded := DecodeQuantizationStep(enc, 16)
		original := p.StepSizes[i]
		if !nearlyEqual(decoded, original, 0.05) { // 5% tolerance
			t.Fatalf("decoded step not close to original for subband %d: got %.6f want %.6f", i, decoded, original)
		}
	}
}

func TestOpenJPEGLossyExplicitStepsMatchFoDicomBaseline(t *testing.T) {
	p := CalculateOpenJPEGQuantizationParams(5, 8)

	if p.Style != 2 {
		t.Fatalf("Style = %d, want OpenJPEG scalar expounded style 2", p.Style)
	}
	if p.GuardBits != 2 {
		t.Fatalf("GuardBits = %d, want OpenJPEG default 2", p.GuardBits)
	}
	got := make([]byte, 0, len(p.EncodedSteps)*2)
	for _, step := range p.EncodedSteps {
		got = append(got, byte(step>>8), byte(step))
	}

	const wantHex = "772076f076f076c06f006f006ee067506750676850055005504757d357d35762"
	if hex.EncodeToString(got) != wantHex {
		t.Fatalf("SPqcd = %s, want %s", hex.EncodeToString(got), wantHex)
	}
}

func TestOpenJPEGLossyRuntimeStepsUseEncodedQCDValues(t *testing.T) {
	p := CalculateOpenJPEGQuantizationParams(5, 8)
	if len(p.StepSizes) == 0 || len(p.EncodedSteps) == 0 {
		t.Fatal("missing OpenJPEG quantization steps")
	}

	got := p.StepSizes[0]
	want := decodeQuantizationStepWithGain(p.EncodedSteps[0], 8, 0)
	if got == want {
		t.Fatalf("test requires encoded QCD step to differ from source step")
	}

	runtime := OpenJPEGRuntimeQuantizationSteps(p.EncodedSteps, 5, 8)
	if runtime[0] != want {
		t.Fatalf("runtime LL step = %.12f, want QCD-decoded %.12f", runtime[0], want)
	}
}

func TestQualityMonotonicity_LLStep(t *testing.T) {
	qualities := []int{1, 20, 50, 80, 90, 95, 99}
	var llSteps []float64
	for _, q := range qualities {
		p := CalculateQuantizationParams(q, 5, 16)
		if p.Style == 0 {
			// skip lossless
			continue
		}
		llSteps = append(llSteps, p.StepSizes[0])
	}
	for i := 1; i < len(llSteps); i++ {
		if !(llSteps[i] < llSteps[i-1]) {
			t.Fatalf("expected LL step to decrease with higher quality: prev=%.6f curr=%.6f", llSteps[i-1], llSteps[i])
		}
	}
}

func TestQuantizeDequantizeErrorByQuality(t *testing.T) {
	// Deterministic synthetic coefficients to avoid flaky comparisons.
	rng := rand.New(rand.NewSource(42))
	coeffs := make([]int32, 5000)
	for i := range coeffs {
		coeffs[i] = int32(rng.Intn(41) - 20)
	}

	// Compare average absolute reconstruction error across low qualities where
	// LL quantization step remains >1 and therefore produces measurable loss.
	type pair struct {
		q   int
		err float64
	}
	qs := []int{1, 3, 5}
	results := make([]pair, 0, len(qs))
	for _, q := range qs {
		p := CalculateQuantizationParams(q, 5, 16)
		step := p.StepSizes[len(p.StepSizes)-1]
		qd := QuantizeCoefficients(coeffs, step)
		dq := DequantizeCoefficients(qd, step)
		var sum float64
		for i := range coeffs {
			sum += math.Abs(float64(coeffs[i] - dq[i]))
		}
		avg := sum / float64(len(coeffs))
		results = append(results, pair{q: q, err: avg})
	}

	// Lower quality should be no better than higher quality.
	if !(results[0].err > results[1].err) {
		t.Fatalf("expected error(q=%d)>error(q=%d), got %.6f <= %.6f", results[0].q, results[1].q, results[0].err, results[1].err)
	}
	if !(results[1].err > results[2].err) {
		t.Fatalf("expected error(q=%d)>error(q=%d), got %.6f <= %.6f", results[1].q, results[2].q, results[1].err, results[2].err)
	}
}

func TestBoundaryQualityValues(t *testing.T) {
	tests := []struct {
		name          string
		quality       int
		expectedStyle int
	}{
		{"Quality 1 (max compression)", 1, 2},
		{"Quality 50 (medium)", 50, 2},
		{"Quality 99 (near-lossless)", 99, 2},
		{"Quality 100 (near-lossless)", 100, 2},
		{"Quality below 1 (clamped)", 0, 2},
		{"Quality above 100 (clamped)", 101, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := CalculateQuantizationParams(tt.quality, 5, 16)
			if p.Style != tt.expectedStyle {
				t.Errorf("quality=%d: expected Style=%d, got %d", tt.quality, tt.expectedStyle, p.Style)
			}
			if p.GuardBits != 2 {
				t.Errorf("expected GuardBits=2, got %d", p.GuardBits)
			}
		})
	}
}

func TestDifferentBitDepths(t *testing.T) {
	bitDepths := []int{8, 12, 16}
	quality := 80
	numLevels := 3

	for _, bd := range bitDepths {
		t.Run(fmt.Sprintf("BitDepth_%d", bd), func(t *testing.T) {
			p := CalculateQuantizationParams(quality, numLevels, bd)

			expectedSubbands := 3*numLevels + 1
			if len(p.StepSizes) != expectedSubbands {
				t.Errorf("bitDepth=%d: expected %d subbands, got %d", bd, expectedSubbands, len(p.StepSizes))
			}

			for i, step := range p.StepSizes {
				encoded := p.EncodedSteps[i]
				decoded := DecodeQuantizationStep(encoded, bd)
				if !nearlyEqual(decoded, step, 0.05) {
					t.Errorf("bitDepth=%d, subband=%d: encoded step not recovered accurately: original=%.6f decoded=%.6f",
						bd, i, step, decoded)
				}
			}
		})
	}
}

func TestDifferentDecompositionLevels(t *testing.T) {
	levels := []int{1, 3, 5, 6}
	quality := 80
	bitDepth := 16

	for _, level := range levels {
		t.Run(fmt.Sprintf("Levels_%d", level), func(t *testing.T) {
			p := CalculateQuantizationParams(quality, level, bitDepth)

			expectedSubbands := 3*level + 1
			if len(p.StepSizes) != expectedSubbands {
				t.Errorf("numLevels=%d: expected %d subbands, got %d", level, expectedSubbands, len(p.StepSizes))
			}

			for i, step := range p.StepSizes {
				if step <= 0 {
					t.Errorf("numLevels=%d, subband=%d: invalid step size %.6f", level, i, step)
				}
			}
		})
	}
}

func TestQuantizationWithSpecialCoefficients(t *testing.T) {
	tests := []struct {
		name   string
		coeffs []int32
	}{
		{"All zeros", []int32{0, 0, 0, 0, 0}},
		{"All positive", []int32{100, 200, 300, 400, 500}},
		{"All negative", []int32{-100, -200, -300, -400, -500}},
		{"Mixed signs", []int32{-500, -100, 0, 100, 500}},
		{"Large values", []int32{32000, -32000, 16000, -16000, 0}},
		{"Small values", []int32{1, -1, 2, -2, 0}},
	}

	stepSize := 10.0

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quantized := QuantizeCoefficients(tt.coeffs, stepSize)
			dequantized := DequantizeCoefficients(quantized, stepSize)

			if len(dequantized) != len(tt.coeffs) {
				t.Errorf("length mismatch: got %d, want %d", len(dequantized), len(tt.coeffs))
			}

			for i, orig := range tt.coeffs {
				if orig != 0 && dequantized[i] != 0 {
					origSign := orig >= 0
					deqSign := dequantized[i] >= 0
					if origSign != deqSign {
						t.Errorf("sign not preserved at index %d: orig=%d dequant=%d", i, orig, dequantized[i])
					}
				}
			}
		})
	}
}

func TestSubbandGainRelationships(t *testing.T) {
	quality := 80
	numLevels := 5
	p := CalculateQuantizationParams(quality, numLevels, 16)
	expected := calcOpenJPEGStepSizes97(numLevels, qualityScale(quality))

	if len(p.StepSizes) != len(expected) {
		t.Fatalf("step count mismatch: got %d want %d", len(p.StepSizes), len(expected))
	}

	for i := range p.StepSizes {
		if !nearlyEqual(p.StepSizes[i], expected[i], 0.01) {
			t.Errorf("subband %d: step mismatch got %.6f want %.6f", i, p.StepSizes[i], expected[i])
		}
	}
}

func TestQuantizationZeroStepSize(t *testing.T) {
	coeffs := []int32{100, -200, 300, -400, 0}

	quantized := QuantizeCoefficients(coeffs, 0)
	if len(quantized) != len(coeffs) {
		t.Errorf("length mismatch: got %d, want %d", len(quantized), len(coeffs))
	}
	for i := range coeffs {
		if quantized[i] != coeffs[i] {
			t.Errorf("index %d: expected no change with zero step, got %d want %d", i, quantized[i], coeffs[i])
		}
	}

	dequantized := DequantizeCoefficients(coeffs, 0)
	if len(dequantized) != len(coeffs) {
		t.Errorf("length mismatch: got %d, want %d", len(dequantized), len(coeffs))
	}
	for i := range coeffs {
		if dequantized[i] != coeffs[i] {
			t.Errorf("index %d: expected no change with zero step, got %d want %d", i, dequantized[i], coeffs[i])
		}
	}
}

func TestEncodedStepsPrecision(t *testing.T) {
	testCases := []struct {
		quality  int
		bitDepth int
		maxError float64
	}{
		{quality: 1, bitDepth: 8, maxError: 0.10},
		{quality: 50, bitDepth: 12, maxError: 0.05},
		{quality: 90, bitDepth: 16, maxError: 0.05},
		{quality: 99, bitDepth: 16, maxError: 0.065},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Q%d_BD%d", tc.quality, tc.bitDepth), func(t *testing.T) {
			p := CalculateQuantizationParams(tc.quality, 5, tc.bitDepth)
			if p.Style == 0 {
				return
			}

			for i, encoded := range p.EncodedSteps {
				decoded := DecodeQuantizationStep(encoded, tc.bitDepth)
				original := p.StepSizes[i]

				if !nearlyEqual(decoded, original, tc.maxError) {
					t.Errorf("subband %d: encoding error too large: original=%.6f decoded=%.6f relError=%.4f maxAllowed=%.4f",
						i, original, decoded, math.Abs(original-decoded)/original, tc.maxError)
				}
			}
		})
	}
}

func TestQualityStepSizeRange(t *testing.T) {
	for quality := 1; quality <= 99; quality += 10 {
		p := CalculateQuantizationParams(quality, 5, 16)

		if p.Style == 0 {
			continue
		}

		for i, step := range p.StepSizes {
			if step <= 0 {
				t.Errorf("quality=%d subband=%d: step size must be positive, got %.6f", quality, i, step)
			}
			maxAllowed := 200.0
			if quality <= 10 {
				maxAllowed = 500.0
			}
			if step > maxAllowed {
				t.Errorf("quality=%d subband=%d: step size too large: %.6f (max allowed: %.1f)", quality, i, step, maxAllowed)
			}
		}
	}
}

func BenchmarkCalculateQuantizationParams(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateQuantizationParams(80, 5, 16)
	}
}

func BenchmarkQuantizeCoefficients(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	coeffs := make([]int32, 10000)
	for i := range coeffs {
		coeffs[i] = int32(rng.Intn(1<<14) - (1 << 13))
	}
	stepSize := 10.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		QuantizeCoefficients(coeffs, stepSize)
	}
}

func BenchmarkDequantizeCoefficients(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	coeffs := make([]int32, 10000)
	for i := range coeffs {
		coeffs[i] = int32(rng.Intn(2000) - 1000)
	}
	stepSize := 10.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DequantizeCoefficients(coeffs, stepSize)
	}
}
