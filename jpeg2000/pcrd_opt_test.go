package jpeg2000

import (
	"math"
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
)

func TestFindOptimalLambda(t *testing.T) {
	passes := [][]t1.PassData{
		{
			{PassIndex: 0, Rate: 10, ActualBytes: 10, Distortion: 100},
			{PassIndex: 1, Rate: 20, ActualBytes: 20, Distortion: 180},
			{PassIndex: 2, Rate: 30, ActualBytes: 30, Distortion: 240},
		},
		{
			{PassIndex: 0, Rate: 8, ActualBytes: 8, Distortion: 60},
			{PassIndex: 1, Rate: 16, ActualBytes: 16, Distortion: 100},
			{PassIndex: 2, Rate: 24, ActualBytes: 24, Distortion: 130},
		},
	}
	total := float64(getPassBytes(passes[0], 3) + getPassBytes(passes[1], 3))
	target := total * 0.5
	lambda, sel, rate := FindOptimalLambda(passes, target, 0.05, nil)
	if lambda <= 0 {
		t.Errorf("lambda <= 0")
	}
	if len(sel) != 2 {
		t.Errorf("selected size = %d", len(sel))
	}
	if math.Abs(rate-target) > target*0.2 {
		t.Errorf("rate %.2f not within tolerance of target %.2f", rate, target)
	}
	for i := range sel {
		if sel[i] < 0 || sel[i] > len(passes[i]) {
			t.Errorf("sel[%d]=%d out of range", i, sel[i])
		}
	}
}

func TestTruncateAtLambda(t *testing.T) {
	passes := [][]t1.PassData{{
		{PassIndex: 0, Rate: 10, ActualBytes: 10, Distortion: 100},
		{PassIndex: 1, Rate: 20, ActualBytes: 20, Distortion: 180},
		{PassIndex: 2, Rate: 30, ActualBytes: 30, Distortion: 240},
	}}
	slopes, cum, _ := computeIncrementals(passes)
	s1, r1 := truncateAtLambda(passes, slopes, cum, slopes[0][1], nil)
	if s1[0] < 2 {
		t.Errorf("expected at least 2 passes for lambda at slope1")
	}
	s2, r2 := truncateAtLambda(passes, slopes, cum, slopes[0][2]-0.001, nil)
	if s2[0] != 3 {
		t.Errorf("expected all passes for low lambda")
	}
	if r2 < r1 {
		t.Errorf("rate should increase when lambda decreases")
	}
}

func TestLayerBudgetStrategies(t *testing.T) {
	total := 300.0
	b1 := ComputeLayerBudgets(total, 3, "EQUAL_RATE")
	b2 := ComputeLayerBudgets(total, 3, "EXPONENTIAL")
	b3 := ComputeLayerBudgets(total, 3, "EQUAL_QUALITY")
	b4 := ComputeLayerBudgets(total, 3, "ADAPTIVE")
	sets := [][]float64{b1, b2, b3, b4}
	for si, bs := range sets {
		if len(bs) != 3 {
			t.Errorf("set %d size %d", si, len(bs))
		}
		if bs[0] <= 0 || bs[2] != total {
			t.Errorf("set %d bounds invalid", si)
		}
		if !(bs[0] < bs[1] && bs[1] < bs[2]) {
			t.Errorf("set %d not increasing", si)
		}
	}
}

func TestAllocateLayersWithLambda(t *testing.T) {
	passes := [][]t1.PassData{
		{
			{PassIndex: 0, Rate: 10, ActualBytes: 10, Distortion: 100},
			{PassIndex: 1, Rate: 20, ActualBytes: 20, Distortion: 180},
			{PassIndex: 2, Rate: 30, ActualBytes: 30, Distortion: 240},
		},
		{
			{PassIndex: 0, Rate: 8, ActualBytes: 8, Distortion: 60},
			{PassIndex: 1, Rate: 16, ActualBytes: 16, Distortion: 100},
			{PassIndex: 2, Rate: 24, ActualBytes: 24, Distortion: 130},
		},
	}
	total := float64(getPassBytes(passes[0], 3) + getPassBytes(passes[1], 3))
	budgets := ComputeLayerBudgets(total*0.8, 3, "EXPONENTIAL")
	alloc := AllocateLayersWithLambda(passes, 3, budgets, 0.05)
	if alloc.NumLayers != 3 {
		t.Errorf("NumLayers=%d", alloc.NumLayers)
	}
	for cb := range alloc.CodeBlockPasses {
		lp := alloc.CodeBlockPasses[cb]
		if lp[0] > lp[1] || lp[1] > lp[2] {
			t.Errorf("monotonic failed: %v", lp)
		}
		if lp[2] > len(passes[cb]) {
			t.Errorf("out of range")
		}
	}
}

func TestAllocateLayersOpenJPEGThresholdAppendsFullFinalLayer(t *testing.T) {
	passes := [][]t1.PassData{
		{
			{Rate: 10, Distortion: 100},
			{Rate: 20, Distortion: 150},
			{Rate: 30, Distortion: 170},
		},
		{
			{Rate: 8, Distortion: 80},
			{Rate: 16, Distortion: 110},
		},
	}

	alloc := AllocateLayersOpenJPEGThreshold(passes, []float64{18, 0})

	if alloc.NumLayers != 2 {
		t.Fatalf("NumLayers = %d, want 2", alloc.NumLayers)
	}
	if got := alloc.CodeBlockPasses[0][1]; got != len(passes[0]) {
		t.Fatalf("final layer block 0 passes = %d, want all", got)
	}
	if got := alloc.CodeBlockPasses[1][1]; got != len(passes[1]) {
		t.Fatalf("final layer block 1 passes = %d, want all", got)
	}
	if alloc.CodeBlockPasses[0][0] > alloc.CodeBlockPasses[0][1] ||
		alloc.CodeBlockPasses[1][0] > alloc.CodeBlockPasses[1][1] {
		t.Fatalf("allocation is not monotonic: %+v", alloc.CodeBlockPasses)
	}
}

func TestSelectOpenJPEGThresholdUsesDBLEpsilonMargin(t *testing.T) {
	const dblEpsilon = 2.220446049250313e-16
	threshold := 1.0
	passes := [][]t1.PassData{{
		{Rate: 1, ActualBytes: 1, Distortion: threshold - dblEpsilon/2},
	}}

	selected := selectOpenJPEGThreshold(passes, []int{0}, threshold)

	if got := selected[0]; got != 1 {
		t.Fatalf("selected passes = %d, want 1 for OpenJPEG DBL_EPSILON threshold margin", got)
	}
}

func TestOpenJPEGThresholdUsesPassRateNotActualBytes(t *testing.T) {
	passes := [][]t1.PassData{{
		{Rate: 10, ActualBytes: 7, Distortion: 10},
		{Rate: 20, ActualBytes: 14, Distortion: 17},
	}}

	selected := selectOpenJPEGThreshold(passes, []int{0}, 0.8)

	if got := selected[0]; got != 1 {
		t.Fatalf("selected passes = %d, want 1 when using OpenJPEG pass->rate slopes", got)
	}
}
