package openjph

import (
	"reflect"
	"testing"
)

func TestConvertPixelDataMatchesOpenJPHWrapperTraversal(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		bitDepth int
		isSigned bool
		input    []byte
		want     [][]int32
	}{
		{
			name:     "8-bit interleaved by pixel",
			width:    2,
			height:   1,
			bitDepth: 8,
			input:    []byte{1, 2, 3, 4, 5, 6},
			want:     [][]int32{{1, 4}, {2, 5}, {3, 6}},
		},
		{
			name:     "16-bit unsigned restarts every component at row start",
			width:    2,
			height:   1,
			bitDepth: 16,
			input:    []byte{1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0},
			want:     [][]int32{{1, 2}, {1, 2}, {1, 2}},
		},
		{
			name:     "16-bit signed restarts every component at row start",
			width:    2,
			height:   1,
			bitDepth: 16,
			isSigned: true,
			input:    []byte{0xff, 0xff, 0x00, 0x80, 3, 0, 4, 0, 5, 0, 6, 0},
			want:     [][]int32{{-1, -32768}, {-1, -32768}, {-1, -32768}},
		},
		{
			name:     "16-bit row stride excludes component count",
			width:    2,
			height:   2,
			bitDepth: 16,
			input: []byte{
				1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0,
				7, 0, 8, 0, 9, 0, 10, 0, 11, 0, 12, 0,
			},
			want: [][]int32{{1, 2, 3, 4}, {1, 2, 3, 4}, {1, 2, 3, 4}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := DefaultEncodeParams(tt.width, tt.height, 3, tt.bitDepth, tt.isSigned)
			encoder := NewEncoder(params)
			if err := encoder.convertPixelData(tt.input); err != nil {
				t.Fatalf("convertPixelData() error = %v", err)
			}
			if !reflect.DeepEqual(encoder.data, tt.want) {
				t.Fatalf("convertPixelData() = %v, want %v", encoder.data, tt.want)
			}
		})
	}
}

func TestUsesColorTransformMatchesFoDicomComponentRule(t *testing.T) {
	for _, components := range []int{1, 2, 3} {
		params := DefaultEncodeParams(2, 1, components, 8, false)
		params.EnableMCT = components > 1
		got := NewEncoder(params).usesColorTransform()
		want := components > 1
		if got != want {
			t.Errorf("components=%d: usesColorTransform() = %v, want %v", components, got, want)
		}
	}
}
