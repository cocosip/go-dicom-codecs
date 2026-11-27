package extended

import (
	"math"
	"math/rand"
	"testing"
)

func slowIDCT12(coef *[64]int64) [64]float64 {
	var cos [8][8]float64
	for frequency := 0; frequency < 8; frequency++ {
		for sample := 0; sample < 8; sample++ {
			cos[frequency][sample] = math.Cos(float64((2*sample+1)*frequency) * math.Pi / 16)
		}
	}
	var out [64]float64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			sum := 0.0
			for v := 0; v < 8; v++ {
				for u := 0; u < 8; u++ {
					scale := 1.0
					if u == 0 {
						scale /= math.Sqrt2
					}
					if v == 0 {
						scale /= math.Sqrt2
					}
					sum += scale * float64(coef[v*8+u]) * cos[u][x] * cos[v][y]
				}
			}
			out[y*8+x] = math.Round(sum/4 + 2048)
		}
	}
	return out
}

// The integer AAN constants are 12-bit fixed point, so a 12-bit block can be
// off by a few counts; that is far below the quantization step of a 12-bit
// lossy JPEG.
func TestIDCT12MatchesSlow(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	var coef [64]int64
	maxDiff := 0.0
	for n := 0; n < 100; n++ {
		for i := range coef {
			coef[i] = 0
			if rng.Intn(2) == 0 {
				coef[i] = int64(rng.Intn(32769) - 16384)
			}
		}
		want := slowIDCT12(&coef)
		var got [64]int64
		idct12Block(&coef, &got)
		for i := range want {
			d := math.Abs(want[i] - float64(got[i]))
			if d > maxDiff {
				maxDiff = d
			}
		}
	}
	if maxDiff > 10 {
		t.Fatalf("max diff = %f, want <= 10", maxDiff)
	}
	t.Logf("max diff = %f", maxDiff)
}
