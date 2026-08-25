package t1

import "testing"

func TestLayout(t *testing.T) {
	width, height := 2, 2
	data := []int32{1, 2, 3, 4}
	
	t.Logf("Input array: %v", data)
	t.Logf("Width=%d, Height=%d", width, height)
	t.Logf("\n2D layout (row-major):")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			t.Logf("  pos[%d] = data[%d,%d] = %d", idx, x, y, data[idx])
		}
	}
	
	t.Logf("\nColumn-first order:")
	for x := 0; x < width; x++ {
		t.Logf("  Column %d:", x)
		for y := 0; y < height; y++ {
			idx := y*width + x
			t.Logf("    pos[%d] = data[%d,%d] = %d", idx, x, y, data[idx])
		}
	}
}
