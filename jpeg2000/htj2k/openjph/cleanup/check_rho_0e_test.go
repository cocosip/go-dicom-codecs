package cleanup

import (
	"testing"
)

func TestCheckRho0E(t *testing.T) {
	t.Logf("Looking for VLC_tbl1 entries with CQ=4, Rho=0xE:")
	count := 0
	for _, e := range VLCTbl1 {
		if e.CQ == 4 && e.Rho == 0xE {
			t.Logf("  UOff=%d, EK=%d, E1=%d, Cwd=0x%X, Len=%d", e.UOff, e.EK, e.E1, e.Cwd, e.CwdLen)
			count++
		}
	}
	if count == 0 {
		t.Logf("  NO ENTRIES FOUND")
	}
}
