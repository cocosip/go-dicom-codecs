// Package colorspace provides color transform utilities for JPEG 2000.
package colorspace

import "math"

// ICTForward applies the irreversible color transform (JPEG 2000 ICT).
// This matches OpenJPEG: no 128 offset; input is already level-shifted.
func ICTForward(r, g, b int32) (y, cb, cr int32) {
	y = int32(math.Round(0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)))
	cb = int32(math.Round(-0.16875*float64(r) - 0.331260*float64(g) + 0.5*float64(b)))
	cr = int32(math.Round(0.5*float64(r) - 0.41869*float64(g) - 0.08131*float64(b)))
	return
}

// ICTInverse applies the inverse irreversible color transform (JPEG 2000 ICT).
func ICTInverse(y, cb, cr int32) (r, g, b int32) {
	r = int32(math.Round(float64(y) + 1.402*float64(cr)))
	g = int32(math.Round(float64(y) - 0.34413*float64(cb) - 0.71414*float64(cr)))
	b = int32(math.Round(float64(y) + 1.772*float64(cb)))
	return
}

// ApplyICTToComponents converts RGB components to YCbCr (ICT) components.
func ApplyICTToComponents(r, g, b []int32) (y, cb, cr []int32) {
	n := len(r)
	y = make([]int32, n)
	cb = make([]int32, n)
	cr = make([]int32, n)
	for i := 0; i < n; i++ {
		y[i], cb[i], cr[i] = ICTForward(r[i], g[i], b[i])
	}
	return
}

// ApplyInverseICTToComponents converts YCbCr (ICT) components to RGB components.
func ApplyInverseICTToComponents(y, cb, cr []int32) (r, g, b []int32) {
	n := len(y)
	r = make([]int32, n)
	g = make([]int32, n)
	b = make([]int32, n)
	for i := 0; i < n; i++ {
		r[i], g[i], b[i] = ICTInverse(y[i], cb[i], cr[i])
	}
	return
}
