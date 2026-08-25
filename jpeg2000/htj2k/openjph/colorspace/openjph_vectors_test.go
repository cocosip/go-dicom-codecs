package colorspace

import (
	"math"
	"testing"
)

func TestOpenJPHRCTScalarVectors(t *testing.T) {
	tests := []struct {
		r, g, b   int32
		y, cb, cr int32
	}{
		{r: -5, g: 2, b: 10, y: 2, cb: 8, cr: -7},
		{r: -1, g: 0, b: 0, y: -1, cb: 0, cr: -1},
		{r: 32767, g: -32768, b: 1, y: -8192, cb: 32769, cr: 65535},
	}
	for _, tt := range tests {
		y, cb, cr := RCTForward(tt.r, tt.g, tt.b)
		if y != tt.y || cb != tt.cb || cr != tt.cr {
			t.Fatalf("RCTForward(%d,%d,%d) = (%d,%d,%d), want (%d,%d,%d)",
				tt.r, tt.g, tt.b, y, cb, cr, tt.y, tt.cb, tt.cr)
		}
		r, g, b := RCTInverse(y, cb, cr)
		if r != tt.r || g != tt.g || b != tt.b {
			t.Fatalf("RCTInverse(%d,%d,%d) = (%d,%d,%d), want (%d,%d,%d)",
				y, cb, cr, r, g, b, tt.r, tt.g, tt.b)
		}
	}
}

func TestOpenJPHICTScalarFloatBits(t *testing.T) {
	tests := []struct {
		r, g, b          float32
		forward, inverse [3]uint32
	}{
		{
			r: -0.5, g: 0, b: 0.49609375,
			forward: [3]uint32{0xbdbe5a1c, 0x3eaa3247, 0xbe94a742},
			inverse: [3]uint32{0xbeffffff, 0xb2000000, 0x3efe0001},
		},
		{
			r: 0.125, g: -0.25, b: 0.375,
			forward: [3]uint32{0xbd8872b0, 0x3e7f3496, 0x3e0bf5c6},
			inverse: [3]uint32{0x3dfffffe, 0xbe7fffff, 0x3ebfffff},
		},
	}

	for _, tt := range tests {
		y, cb, cr := ICTForwardFloat32(tt.r, tt.g, tt.b)
		gotForward := [3]uint32{math.Float32bits(y), math.Float32bits(cb), math.Float32bits(cr)}
		if gotForward != tt.forward {
			t.Fatalf("ICTForwardFloat32 bits = %08x, want %08x", gotForward, tt.forward)
		}
		r, g, b := ICTInverseFloat32(y, cb, cr)
		gotInverse := [3]uint32{math.Float32bits(r), math.Float32bits(g), math.Float32bits(b)}
		if gotInverse != tt.inverse {
			t.Fatalf("ICTInverseFloat32 bits = %08x, want %08x", gotInverse, tt.inverse)
		}
	}
}
