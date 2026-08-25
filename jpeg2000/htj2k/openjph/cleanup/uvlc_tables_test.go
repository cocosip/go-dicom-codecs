package cleanup

import "testing"

// Sanity checks for generated UVLC tables to ensure table generation stays stable.
func TestUVLCTablesNonEmpty(t *testing.T) {
	nonZero0 := 0
	for _, v := range UVLCTbl0 {
		if v != 0 {
			nonZero0++
		}
	}
	nonZero1 := 0
	for _, v := range UVLCTbl1 {
		if v != 0 {
			nonZero1++
		}
	}
	if nonZero0 == 0 {
		t.Fatalf("UVLCTbl0 has no entries")
	}
	if nonZero1 == 0 {
		t.Fatalf("UVLCTbl1 has no entries")
	}
}

func TestUVLCBiasInitialModes(t *testing.T) {
	// Mode 3 (both u_off=1, mel=0) and mode 4 (both u_off=1, mel=1) should have bias entries.
	mode3Has := false
	mode4Has := false
	for head := 0; head < 64; head++ {
		if UVLCBias[(3<<6)|head] != 0 {
			mode3Has = true
		}
		if UVLCBias[(4<<6)|head] != 0 {
			mode4Has = true
		}
	}
	if !mode3Has {
		t.Errorf("UVLCBias missing non-zero entries for mode 3 (mel=0)")
	}
	if !mode4Has {
		t.Errorf("UVLCBias missing non-zero entries for mode 4 (mel=1)")
	}
}
