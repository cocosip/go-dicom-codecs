package mqc

import (
	"fmt"
	"testing"
)

// TestVerifyTables prints the first few entries of each table for manual verification
func TestVerifyTables(t *testing.T) {
	fmt.Println("=== QE TABLE (first 10 entries) ===")
	for i := 0; i < 10; i++ {
		fmt.Printf("qeTable[%d] = 0x%04x\n", i, qeTable[i])
	}

	fmt.Println("\n=== NMPS TABLE (first 10 entries) ===")
	for i := 0; i < 10; i++ {
		fmt.Printf("nmpsTable[%d] = %d\n", i, nmpsTable[i])
	}

	fmt.Println("\n=== NLPS TABLE (first 10 entries) ===")
	for i := 0; i < 10; i++ {
		fmt.Printf("nlpsTable[%d] = %d\n", i, nlpsTable[i])
	}

	fmt.Println("\n=== SWITCH TABLE (first 10 entries) ===")
	for i := 0; i < 10; i++ {
		fmt.Printf("switchTable[%d] = %d\n", i, switchTable[i])
	}

	// Expected values from OpenJPEG/JPEG2000 spec:
	// qeTable[0] should be 0x5601
	// qeTable[1] should be 0x3401
	// qeTable[6] should be 0x0221 or 0x5601 depending on spec version

	if qeTable[0] != 0x5601 {
		t.Errorf("qeTable[0] = 0x%x, want 0x5601", qeTable[0])
	}
	if qeTable[1] != 0x3401 {
		t.Errorf("qeTable[1] = 0x%x, want 0x3401", qeTable[1])
	}
}
