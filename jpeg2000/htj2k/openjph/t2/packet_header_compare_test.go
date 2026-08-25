package t2

import (
	"fmt"
	"testing"
)

func TestPacketHeaderEncoding(t *testing.T) {
	// Create a simple precinct with a few codeblocks
	precinct := &Precinct{
		Index:          0,
		NumCodeBlocksX: 2,
		NumCodeBlocksY: 2,
	}

	// Add some codeblocks
	for cby := 0; cby < 2; cby++ {
		for cbx := 0; cbx < 2; cbx++ {
			cb := &PrecinctCodeBlock{
				Index:          cby*2 + cbx,
				CBX:            cbx,
				CBY:            cby,
				Included:       false,
				NumPassesTotal: 10,
				ZeroBitPlanes:  2,
				Data:           make([]byte, 100),
			}
			precinct.CodeBlocks = append(precinct.CodeBlocks, cb)
		}
	}

	// Create packet encoder
	pe := NewPacketEncoder(1, 1, 1, ProgressionLRCP)

	// Encode with old method
	headerOld, _ := pe.encodePacketHeaderLayered(precinct, 0, 0)

	// Reset precinct
	for _, cb := range precinct.CodeBlocks {
		cb.Included = false
	}
	precinct.InclTree = nil
	precinct.ZBPTree = nil

	// Encode with tag-tree method
	headerNew, _, err := pe.encodePacketHeaderWithTagTree(precinct, 0, 0)
	if err != nil {
		t.Fatalf("Tag-tree encoding failed: %v", err)
	}

	fmt.Printf("Old header size: %d bytes\n", len(headerOld))
	fmt.Printf("Tag-tree header size: %d bytes\n", len(headerNew))
	fmt.Printf("Difference: %d bytes (%.1f%%)\n",
		len(headerNew)-len(headerOld),
		float64(len(headerNew)-len(headerOld))/float64(len(headerOld))*100)

	// For small grids (2x2), tag-tree overhead may equal simple encoding
	// For larger grids, tag-tree should be smaller or equal
	if len(headerNew) > len(headerOld) {
		t.Errorf("Tag-tree encoding should not be larger! Old: %d, New: %d",
			len(headerOld), len(headerNew))
	}
}
