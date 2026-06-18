package t1

import "testing"

func TestOpenJPEGDecodeReconstructionValues(t *testing.T) {
	tests := []struct {
		name     string
		bitplane int
		sign     int
		want     int32
	}{
		{name: "positive significance uses one plus half", bitplane: 6, sign: 0, want: 96},
		{name: "negative significance uses one plus half", bitplane: 6, sign: 1, want: -96},
		{name: "lowest bitplane has no half bit", bitplane: 0, sign: 0, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reconstructSignificantValue(tt.bitplane, tt.sign); got != tt.want {
				t.Fatalf("reconstructSignificantValue(%d, %d) = %d, want %d", tt.bitplane, tt.sign, got, tt.want)
			}
		})
	}
}

func TestOpenJPEGDecodeRefinementValues(t *testing.T) {
	tests := []struct {
		name     string
		current  int32
		bitplane int
		bit      int
		want     int32
	}{
		{name: "positive zero refinement subtracts poshalf", current: 96, bitplane: 5, bit: 0, want: 80},
		{name: "positive one refinement adds poshalf", current: 96, bitplane: 5, bit: 1, want: 112},
		{name: "negative zero refinement adds poshalf", current: -96, bitplane: 5, bit: 0, want: -80},
		{name: "negative one refinement subtracts poshalf", current: -96, bitplane: 5, bit: 1, want: -112},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := refineReconstructedValue(tt.current, tt.bitplane, tt.bit)
			if got != tt.want {
				t.Fatalf("refineReconstructedValue(%d, %d, %d) = %d, want %d", tt.current, tt.bitplane, tt.bit, got, tt.want)
			}
		})
	}
}
