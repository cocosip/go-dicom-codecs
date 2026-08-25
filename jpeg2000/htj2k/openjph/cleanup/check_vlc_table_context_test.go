package cleanup

import (
	"testing"
)

// TestCheckVLCTableContext checks VLC table entries for different contexts
func TestCheckVLCTableContext(t *testing.T) {
	// For quad (1,1) with rho=0xF (all 4 samples significant)
	// We need to check what codeword exists for different contexts

	rho := uint8(0xF)
	uOff := uint8(1) // Assuming uOff=1
	ek := uint8(0xF)
	e1 := uint8(0xF)

	t.Logf("Looking for VLC entries with rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X", rho, uOff, ek, e1)

	// Check non-initial row table (tbl1) for different contexts
	for ctx := uint8(0); ctx < 8; ctx++ {
		found := false
		var codeword uint8
		var length uint8

		for _, entry := range VLCTbl1 {
			if entry.CQ == ctx && entry.Rho == rho && entry.UOff == uOff {
				// Found a match (ignoring ek/e1 for now)
				found = true
				codeword = entry.Cwd
				length = entry.CwdLen
				t.Logf("  Context %d: found entry - codeword=0x%02X, length=%d, ek=%d, e1=%d",
					ctx, codeword, length, entry.EK, entry.E1)
				break
			}
		}

		if !found {
			t.Logf("  Context %d: NO ENTRY FOUND", ctx)
		}
	}

	// Also check with different ek/e1 values
	t.Logf("\nChecking context=2 and context=7 with various ek/e1:")

	for ctx := uint8(2); ctx <= 7; ctx += 5 {
		t.Logf("\nContext %d:", ctx)
		matchCount := 0
		for _, entry := range VLCTbl1 {
			if entry.CQ == ctx && entry.Rho == rho && entry.UOff == uOff {
				t.Logf("    ek=%d, e1=%d: codeword=0x%02X, length=%d",
					entry.EK, entry.E1, entry.Cwd, entry.CwdLen)
				matchCount++
			}
		}
		if matchCount == 0 {
			t.Logf("    No entries found")
		}
	}
}
