package wavelet

import (
	"math"
	"reflect"
	"testing"
)

func TestOpenJPHReversible53ScalarVectors(t *testing.T) {
	for _, tt := range []struct {
		name  string
		even  bool
		input []int32
		want  []int32
	}{
		{name: "odd width even origin", even: true, input: []int32{-5, 2, 10, -3, 7}, want: []int32{-5, 7, 2, 0, -11}},
		{name: "odd width odd origin", input: []int32{-5, 2, 10, -3, 7}, want: []int32{3, 2, -7, 11, 10}},
		{name: "single odd origin", input: []int32{-7}, want: []int32{-14}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := append([]int32(nil), tt.input...)
			Forward53_1DWithParity(got, tt.even)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("forward = %v, want %v", got, tt.want)
			}
			Inverse53_1DWithParity(got, tt.even)
			if !reflect.DeepEqual(got, tt.input) {
				t.Fatalf("inverse = %v, want %v", got, tt.input)
			}
		})
	}
}

func TestOpenJPHIrreversible97ScalarFloatBits(t *testing.T) {
	for _, tt := range []struct {
		name        string
		even        bool
		input       []float32
		wantForward []uint32
		wantInverse []uint32
	}{
		{
			name: "odd width even origin", even: true,
			input:       []float32{-0.5, 0.25, -0.125, 0.375, 0.49609375},
			wantForward: []uint32{0xbe09d343, 0x3d989356, 0x3ef79ffa, 0x3f256ed3, 0x3dd8896e},
			wantInverse: []uint32{0xbf000000, 0x3e800000, 0xbe000000, 0x3ec00002, 0x3efe0001},
		},
		{
			name:        "even width odd origin",
			input:       []float32{0.125, -0.25, 0.375, -0.5, 0, 0.49609375},
			wantForward: []uint32{0x3d0f16a4, 0xbe78a5e3, 0x3eb3e039, 0x3e99e73d, 0x3f607b32, 0xbdb3a818},
			wantInverse: []uint32{0x3e000000, 0xbe800000, 0x3ec00000, 0xbeffffff, 0x32b40000, 0x3efdffff},
		},
		{
			name:        "single odd origin",
			input:       []float32{0.375},
			wantForward: []uint32{0x3f400000},
			wantInverse: []uint32{0x3ec00000},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := append([]float32(nil), tt.input...)
			Forward97_1DFloat32WithParity(got, tt.even)
			if bits := float32Bits(got); !reflect.DeepEqual(bits, tt.wantForward) {
				t.Fatalf("forward bits = %08x, want %08x", bits, tt.wantForward)
			}
			Inverse97_1DFloat32WithParity(got, tt.even)
			if bits := float32Bits(got); !reflect.DeepEqual(bits, tt.wantInverse) {
				t.Fatalf("inverse bits = %08x, want %08x", bits, tt.wantInverse)
			}
		})
	}
}

func TestOpenJPHIrreversible97OddOrigin2DFloatBits(t *testing.T) {
	got := []float32{
		-0.5, -0.375, -0.25,
		-0.125, 0, 0.125,
		0.25, 0.375, 0.49609375,
		0.125, -0.25, 0.375,
		-0.5, 0, 0.49609375,
	}
	wantForward := []uint32{
		0xbbd6afca, 0xbe220c55, 0x3e07cbe3,
		0x3dcbeaff, 0xbc6a0c0d, 0x3f09784b,
		0xbeec22cd, 0x3dbad17a, 0x3dbbf36a,
		0x3ee09dc7, 0xbe645491, 0xbe8ff32c,
		0xbd34c61b, 0xbf8d7824, 0xbda65717,
	}
	wantInverse := []uint32{
		0xbf000001, 0xbec00001, 0xbe800006,
		0xbdfffff9, 0x32000000, 0x3e000002,
		0x3e7ffffe, 0x3ec00003, 0x3efe0000,
		0x3dfffffc, 0xbe7fffff, 0x3ec00001,
		0xbf000002, 0x33800000, 0x3efe0000,
	}

	Forward97_2DFloat32WithParity(got, 3, 5, 3, false, false)
	if bits := float32Bits(got); !reflect.DeepEqual(bits, wantForward) {
		t.Errorf("forward bits = %08x, want %08x", bits, wantForward)
	}
	Inverse97_2DFloat32WithParity(got, 3, 5, 3, false, false)
	if bits := float32Bits(got); !reflect.DeepEqual(bits, wantInverse) {
		t.Errorf("inverse bits = %08x, want %08x", bits, wantInverse)
	}
}

func TestOpenJPHIrreversibleOutputScalesBeforeNativeSIMDRounding(t *testing.T) {
	input := []float32{0.001953125, -0.001953125, 0.25, -0.25}
	want := []int32{0, 0, 64, -64}

	got := ConvertFloat32ToInt32OpenJPH(input, 8)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("irreversible output samples = %v, want %v", got, want)
	}
}

func float32Bits(values []float32) []uint32 {
	bits := make([]uint32, len(values))
	for i, value := range values {
		bits[i] = math.Float32bits(value)
	}
	return bits
}
