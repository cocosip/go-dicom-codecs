package openjpeg

import "testing"

func TestROIMaskRectAndDownsample(t *testing.T) {
	mask := newROIMask(8, 8)
	mask.setRect(2, 2, 6, 6)

	// Points inside/outside
	if !mask.get(2, 2) || !mask.get(5, 5) {
		t.Fatalf("expected inside points to be true")
	}
	if mask.get(1, 1) || mask.get(7, 7) {
		t.Fatalf("expected outside points to be false")
	}

	// Downsample by step=2 over cropped region
	ds := mask.downsample(0, 0, 8, 8, 2)
	if len(ds) != 4 || len(ds[0]) != 4 {
		t.Fatalf("unexpected downsample size")
	}
	// The center blocks should be true
	if !ds[1][1] || !ds[2][2] {
		t.Fatalf("expected center blocks to be true")
	}
	// Corner block should be false
	if ds[0][0] {
		t.Fatalf("expected corner block false")
	}
}

func TestRasterizePolygon(t *testing.T) {
	pts := []Point{
		{X: 2, Y: 2},
		{X: 6, Y: 2},
		{X: 6, Y: 6},
		{X: 2, Y: 6},
	}
	mask := rasterizePolygon(8, 8, pts)
	if mask == nil {
		t.Fatalf("mask is nil")
	}
	inside := mask.get(3, 3)
	outside := mask.get(1, 1)
	if !inside || outside {
		t.Fatalf("polygon rasterization failed, inside=%v outside=%v", inside, outside)
	}
}
