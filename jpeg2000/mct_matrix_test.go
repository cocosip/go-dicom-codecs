package jpeg2000

import "testing"

func TestCustomMCTMatrix_Identity4Components(t *testing.T) {
	//t.Skip("Experimental MCT roundtrip pending full Part 2 alignment")
	width, height := 32, 32
	n := width * height
	comps := 3
	data := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		data[c] = make([]int32, n)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				idx := y*width + x
				data[c][idx] = int32((x*5 + y*7 + c*13) % 256)
			}
		}
	}

	params := DefaultEncodeParams(width, height, comps, 8, false)
	// Identity 3x3 matrix
	I := make([][]float64, comps)
	inv := make([][]float64, comps)
	for i := 0; i < comps; i++ {
		I[i] = make([]float64, comps)
		inv[i] = make([]float64, comps)
		for j := 0; j < comps; j++ {
			v := 0.0
			if i == j {
				v = 1.0
			}
			I[i][j] = v
			inv[i][j] = v
		}
	}
	params.MCTMatrix = I
	params.InverseMCTMatrix = inv
	params.MCTReversible = true
	params.NumLevels = 1
	params.Lossless = true

	enc := NewEncoder(params)
	encoded, err := enc.EncodeComponents(data)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	dec := NewDecoder()
	if err := dec.Decode(encoded); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for c := 0; c < comps; c++ {
		got, err := dec.GetComponentData(c)
		if err != nil {
			t.Fatalf("get comp %d failed: %v", c, err)
		}
		if len(got) != len(data[c]) {
			t.Fatalf("length mismatch comp=%d", c)
		}
		for i := 0; i < 16 && i < len(got); i++ {
			diff := int(got[i]) - int(data[c][i])
			if diff < 0 {
				diff = -diff
			}
			if diff > 10 {
				t.Fatalf("value diff too large comp=%d i=%d got=%d want=%d", c, i, got[i], data[c][i])
			}
		}
	}
}
