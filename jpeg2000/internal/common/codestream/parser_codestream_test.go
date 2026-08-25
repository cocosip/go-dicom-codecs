package codestream

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParserCOCQCCAndPOC(t *testing.T) {
	var buf bytes.Buffer

	_ = binary.Write(&buf, binary.BigEndian, MarkerSOC)
	writeSIZSegment(&buf, 256, 256, 256, 256, 2, 8)
	writeCODSegment(&buf, 1, 0, 1)
	writeQCDSegment(&buf, 8)
	writeCOCSegment(&buf, 1, 2, 3, 4, 1)
	writeQCCSegment(&buf, 1, 2, 0x1234)
	writePOCSegment(&buf, 2, []POCEntry{{
		RSpoc:  0,
		CSpoc:  0,
		LYEpoc: 1,
		REpoc:  1,
		CEpoc:  2,
		Ppoc:   0,
	}})
	_ = binary.Write(&buf, binary.BigEndian, MarkerEOC)

	cs, err := NewParser(buf.Bytes()).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if cs.COC == nil || cs.COC[1] == nil {
		t.Fatal("Expected COC for component 1")
	}
	if cs.COC[1].NumberOfDecompositionLevels != 2 {
		t.Errorf("COC levels = %d, want 2", cs.COC[1].NumberOfDecompositionLevels)
	}
	if cs.QCC == nil || cs.QCC[1] == nil {
		t.Fatal("Expected QCC for component 1")
	}
	if len(cs.QCC[1].SPqcc) != 2 {
		t.Fatalf("QCC SPqcc length = %d, want 2", len(cs.QCC[1].SPqcc))
	}
	if len(cs.POC) != 1 || len(cs.POC[0].Entries) != 1 {
		t.Fatalf("Expected 1 POC entry, got %d", len(cs.POC))
	}

	resolved := cs.ComponentCOD(nil, 1)
	if resolved == nil || resolved.CodeBlockWidth != 3 {
		t.Fatalf("Resolved COD missing or incorrect CodeBlockWidth")
	}
}

func TestParserTilePartConcatenation(t *testing.T) {
	var buf bytes.Buffer

	_ = binary.Write(&buf, binary.BigEndian, MarkerSOC)
	writeSIZSegment(&buf, 64, 64, 64, 64, 1, 8)
	writeCODSegment(&buf, 0, 0, 1)
	writeQCDSegment(&buf, 8)

	writeTilePart(&buf, 0, 0, 2, []byte{0x01, 0x02})
	writeTilePart(&buf, 0, 1, 2, []byte{0x03, 0x04, 0x05})

	_ = binary.Write(&buf, binary.BigEndian, MarkerEOC)

	cs, err := NewParser(buf.Bytes()).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(cs.Tiles) != 1 {
		t.Fatalf("Expected 1 tile, got %d", len(cs.Tiles))
	}
	tile := cs.Tiles[0]
	if tile.SOT == nil || tile.SOT.TNsot != 2 {
		t.Fatalf("Expected TNsot=2, got %v", tile.SOT)
	}
	want := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	if !bytes.Equal(tile.Data, want) {
		t.Fatalf("Tile data mismatch: got %v, want %v", tile.Data, want)
	}
}

func writeCOCSegment(buf *bytes.Buffer, comp uint16, numLevels, cbw, cbh, transform uint8) {
	cocData := bytes.Buffer{}

	_ = binary.Write(&cocData, binary.BigEndian, uint8(comp))
	_ = binary.Write(&cocData, binary.BigEndian, uint8(0))  // Scoc
	_ = binary.Write(&cocData, binary.BigEndian, numLevels) // Levels
	_ = binary.Write(&cocData, binary.BigEndian, cbw)       // CB width
	_ = binary.Write(&cocData, binary.BigEndian, cbh)       // CB height
	_ = binary.Write(&cocData, binary.BigEndian, uint8(0))  // CB style
	_ = binary.Write(&cocData, binary.BigEndian, transform) // Transform

	_ = binary.Write(buf, binary.BigEndian, MarkerCOC)
	_ = binary.Write(buf, binary.BigEndian, uint16(cocData.Len()+2))
	buf.Write(cocData.Bytes())
}

func writeQCCSegment(buf *bytes.Buffer, comp uint16, sqcc uint8, step uint16) {
	qccData := bytes.Buffer{}

	_ = binary.Write(&qccData, binary.BigEndian, uint8(comp))
	_ = binary.Write(&qccData, binary.BigEndian, sqcc)
	_ = binary.Write(&qccData, binary.BigEndian, step)

	_ = binary.Write(buf, binary.BigEndian, MarkerQCC)
	_ = binary.Write(buf, binary.BigEndian, uint16(qccData.Len()+2))
	buf.Write(qccData.Bytes())
}

func writePOCSegment(buf *bytes.Buffer, csiz uint16, entries []POCEntry) {
	data := bytes.Buffer{}
	for _, e := range entries {
		_ = binary.Write(&data, binary.BigEndian, e.RSpoc)
		if csiz > 256 {
			_ = binary.Write(&data, binary.BigEndian, e.CSpoc)
		} else {
			_ = binary.Write(&data, binary.BigEndian, uint8(e.CSpoc))
		}
		_ = binary.Write(&data, binary.BigEndian, e.LYEpoc)
		_ = binary.Write(&data, binary.BigEndian, e.REpoc)
		if csiz > 256 {
			_ = binary.Write(&data, binary.BigEndian, e.CEpoc)
		} else {
			_ = binary.Write(&data, binary.BigEndian, uint8(e.CEpoc))
		}
		_ = binary.Write(&data, binary.BigEndian, e.Ppoc)
	}
	_ = binary.Write(buf, binary.BigEndian, MarkerPOC)
	_ = binary.Write(buf, binary.BigEndian, uint16(data.Len()+2))
	buf.Write(data.Bytes())
}

func writeTilePart(buf *bytes.Buffer, tileIdx uint16, partIdx, total uint8, data []byte) {
	_ = binary.Write(buf, binary.BigEndian, MarkerSOT)
	_ = binary.Write(buf, binary.BigEndian, uint16(10)) // Lsot
	_ = binary.Write(buf, binary.BigEndian, tileIdx)
	psot := uint32(14 + len(data))
	_ = binary.Write(buf, binary.BigEndian, psot)
	_ = binary.Write(buf, binary.BigEndian, partIdx)
	_ = binary.Write(buf, binary.BigEndian, total)
	_ = binary.Write(buf, binary.BigEndian, MarkerSOD)
	buf.Write(data)
}
