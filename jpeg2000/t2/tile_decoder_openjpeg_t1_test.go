package t2

import "testing"

func TestNormalizeOpenJPEGReversibleT1Coefficients(t *testing.T) {
	coeffs := []int32{-17, -3, -2, -1, 0, 1, 2, 3, 17}

	normalizeOpenJPEGReversibleT1Coefficients(coeffs)

	want := []int32{-8, -1, -1, 0, 0, 0, 1, 1, 8}
	for i := range want {
		if coeffs[i] != want[i] {
			t.Fatalf("coeffs[%d] = %d, want %d", i, coeffs[i], want[i])
		}
	}
}

func TestOpenJPEGIrreversibleDequantizationPreservesHalfStep(t *testing.T) {
	td := &TileDecoder{}
	data := []float32{96, -96}

	td.dequantizeSubbandFloat(data, 0, 0, 2, 1, 2, 4)

	want := []float32{192, -192}
	for i := range want {
		if data[i] != want[i] {
			t.Fatalf("data[%d] = %v, want %v", i, data[i], want[i])
		}
	}
}
