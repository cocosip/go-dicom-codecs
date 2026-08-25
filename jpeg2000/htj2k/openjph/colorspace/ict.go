// Package colorspace provides color transform utilities for JPEG 2000.
package colorspace

import "math"

var (
	openJPHBetaCb   = math.Float32frombits(0x3f107833)
	openJPHBetaCr   = math.Float32frombits(0x3f3698a6)
	openJPHGammaCbG = math.Float32frombits(0x3eb032a1)
	openJPHGammaCrG = math.Float32frombits(0x3f36d1a2)
	openJPHGammaCbB = math.Float32frombits(0x3fe2d0e5)
	openJPHGammaCrR = math.Float32frombits(0x3fb374bc)
)

const (
	openJPHAlphaR float32 = 0.299
	openJPHAlphaG float32 = 0.587
	openJPHAlphaB float32 = 0.114
)

// ICTForwardFloat32 matches OpenJPH's scalar gen_ict_forward operation order.
func ICTForwardFloat32(r, g, b float32) (y, cb, cr float32) {
	y = float32Add(float32Add(float32Mul(openJPHAlphaR, r), float32Mul(openJPHAlphaG, g)), float32Mul(openJPHAlphaB, b))
	cb = float32Mul(openJPHBetaCb, float32Sub(b, y))
	cr = float32Mul(openJPHBetaCr, float32Sub(r, y))
	return
}

// ICTInverseFloat32 matches OpenJPH's scalar gen_ict_backward operation order.
func ICTInverseFloat32(y, cb, cr float32) (r, g, b float32) {
	g = float32Sub(float32Sub(y, float32Mul(openJPHGammaCrG, cr)), float32Mul(openJPHGammaCbG, cb))
	r = float32Add(y, float32Mul(openJPHGammaCrR, cr))
	b = float32Add(y, float32Mul(openJPHGammaCbB, cb))
	return
}

func float32Add(a, b float32) float32 {
	return math.Float32frombits(math.Float32bits(a + b))
}

func float32Sub(a, b float32) float32 {
	return math.Float32frombits(math.Float32bits(a - b))
}

func float32Mul(a, b float32) float32 {
	return math.Float32frombits(math.Float32bits(a * b))
}

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
