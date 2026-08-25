package openjpeg

import (
	"math"
	"testing"
)

// TestDiagnosticHighQuality analyzes quantization behavior at high quality levels.
func TestDiagnosticHighQuality(t *testing.T) {
	t.Log("=== Diagnostic Test: High Quality Quantization Parameters ===")

	qualities := []int{80, 85, 90, 92, 94, 95, 96, 97, 98, 99}
	numLevels := 5
	bitDepth := 16

	t.Logf("Configuration: numLevels=%d, bitDepth=%d", numLevels, bitDepth)
	t.Logf("%-8s | %-12s | %-12s | %-12s | %-12s", "Quality", "Scale", "LL Step", "HL1 Step", "HH1 Step")
	t.Log("---------|--------------|--------------|--------------|--------------")

	var prevLLStep float64
	for _, quality := range qualities {
		p := CalculateQuantizationParams(quality, numLevels, bitDepth)
		if p.Style == 0 {
			t.Logf("%-8d | LOSSLESS MODE (no quantization)", quality)
			continue
		}

		scale := qualityScale(quality)
		llStep := p.StepSizes[0]
		hl1Step := p.StepSizes[1]
		hh1Step := p.StepSizes[3]

		t.Logf("%-8d | %12.8f | %12.8f | %12.8f | %12.8f", quality, scale, llStep, hl1Step, hh1Step)

		if prevLLStep > 0 && llStep > prevLLStep {
			t.Errorf("LL step increased from %.8f to %.8f (should decrease)", prevLLStep, llStep)
		}
		prevLLStep = llStep
	}
}

// TestDiagnosticEncodingPipeline tests the full encoding pipeline at high quality.
func TestDiagnosticEncodingPipeline(t *testing.T) {
	t.Log("=== Diagnostic Test: Full Encoding Pipeline at High Quality ===")

	width := 4
	height := 4
	data := []int32{
		0, 50, 100, 150,
		50, 100, 150, 200,
		100, 150, 200, 250,
		150, 200, 250, 255,
	}

	qualities := []int{80, 90, 95, 99}
	bitDepth := 16

	t.Logf("Test data: %d x %d gradient pattern", width, height)

	for _, quality := range qualities {
		t.Logf("--- Quality %d ---", quality)

		p := CalculateQuantizationParams(quality, 2, bitDepth)
		if p.Style == 0 {
			t.Log("  Mode: LOSSLESS (no quantization)")
			continue
		}

		t.Logf("  LL step size: %.8f", p.StepSizes[0])

		quantized := QuantizeCoefficients(data, p.StepSizes[0])
		dequantized := DequantizeCoefficients(quantized, p.StepSizes[0])

		var maxError int32
		var totalError int64
		var errorCount int
		for i := range data {
			err := data[i] - dequantized[i]
			if err < 0 {
				err = -err
			}
			if err > 0 {
				errorCount++
			}
			if err > maxError {
				maxError = err
			}
			totalError += int64(err)
		}

		avgError := float64(totalError) / float64(len(data))
		psnr := calculatePSNRInt32(data, dequantized, 255.0)

		t.Logf("  Max error: %d", maxError)
		t.Logf("  Avg error: %.4f", avgError)
		t.Logf("  Error pixels: %d / %d (%.1f%%)", errorCount, len(data), float64(errorCount)*100/float64(len(data)))
		t.Logf("  PSNR: %.2f dB", psnr)
	}
}

// TestDiagnosticFormulaComparison verifies the quality scale is monotonic.
func TestDiagnosticFormulaComparison(t *testing.T) {
	t.Log("=== Diagnostic Test: Quality Scale Monotonicity ===")

	var prev float64
	for q := 90; q <= 99; q++ {
		scale := qualityScale(q)
		t.Logf("Q=%d: scale=%.10f", q, scale)
		if prev > 0 && scale > prev {
			t.Errorf("scale increased from %.10f to %.10f at Q=%d", prev, scale, q)
		}
		prev = scale
	}
}

func calculatePSNRInt32(original, decoded []int32, maxValue float64) float64 {
	if len(original) != len(decoded) {
		return 0
	}

	var mse float64
	for i := range original {
		diff := float64(original[i] - decoded[i])
		mse += diff * diff
	}
	mse /= float64(len(original))

	if mse == 0 {
		return math.Inf(1)
	}

	return 10 * math.Log10((maxValue*maxValue)/mse)
}
