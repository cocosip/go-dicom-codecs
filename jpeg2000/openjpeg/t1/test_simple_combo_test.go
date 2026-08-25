package t1

import "testing"

func TestSimpleCombo(t *testing.T) {
	tests := []struct {
		name string
		data []int32
	}{
		{"[1,2] in positions 0,3", []int32{1, 0, 0, 2}},
		{"[1,1] in positions 0,3", []int32{1, 0, 0, 1}},
		{"[2,2] in positions 0,3", []int32{2, 0, 0, 2}},
		{"[1,2] in positions 0,1", []int32{1, 2, 0, 0}},
		{"[1,3,0,0] adjacent", []int32{1, 3, 0, 0}},
		{"[3,1,0,0] adjacent", []int32{3, 1, 0, 0}},
		{"[1,0,1,0] same column", []int32{1, 0, 1, 0}},
		{"[2,0,0,4] values 2,4", []int32{2, 0, 0, 4}},
		{"[3,0,0,6] values 3,6", []int32{3, 0, 0, 6}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxVal := int32(0)
			for _, v := range tt.data {
				if v > maxVal {
					maxVal = v
				}
			}

			maxBitplane := 0
			if maxVal > 0 {
				for maxVal > 0 {
					maxVal >>= 1
					maxBitplane++
				}
				maxBitplane--
			}

			numPasses := (maxBitplane * 3) + 1

			enc := NewT1Encoder(2, 2, 0)
			encoded, err := enc.Encode(tt.data, numPasses, 0)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			dec := NewT1Decoder(2, 2, 0)
			err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			decoded := dec.GetData()

			for i := range tt.data {
				if decoded[i] != tt.data[i] {
					t.Errorf("Position %d: expected=%d, got=%d", i, tt.data[i], decoded[i])
				}
			}
		})
	}
}

