package codestream

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestParserWithVariousTileSizes tests parsing with different tile configurations
func TestParserWithVariousTileSizes(t *testing.T) {
	tests := []struct {
		name       string
		imageW     uint32
		imageH     uint32
		tileW      uint32
		tileH      uint32
		expectErr  bool
	}{
		{"Single tile", 512, 512, 512, 512, false},
		{"2x2 tiles", 1024, 1024, 512, 512, false},
		{"3x3 tiles", 1536, 1536, 512, 512, false},
		{"Non-square", 640, 480, 320, 240, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := createMinimalJ2K(tt.imageW, tt.imageH, tt.tileW, tt.tileH)
			parser := NewParser(data)
			cs, err := parser.Parse()

			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectErr && cs != nil {
				if cs.SIZ == nil {
					t.Fatal("SIZ segment is nil")
				}
				// Verify dimensions
				gotW := cs.SIZ.Xsiz - cs.SIZ.XOsiz
				gotH := cs.SIZ.Ysiz - cs.SIZ.YOsiz
				if gotW != tt.imageW || gotH != tt.imageH {
					t.Errorf("Image size mismatch: got %dx%d, want %dx%d",
						gotW, gotH, tt.imageW, tt.imageH)
				}
			}
		})
	}
}

// TestParserWithDifferentComponents tests multi-component parsing
func TestParserWithDifferentComponents(t *testing.T) {
	tests := []struct {
		name       string
		components int
		bitDepth   int
	}{
		{"Grayscale 8-bit", 1, 8},
		{"Grayscale 12-bit", 1, 12},
		{"Grayscale 16-bit", 1, 16},
		{"RGB 8-bit", 3, 8},
		{"RGB 12-bit", 3, 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := createJ2KWithComponents(256, 256, tt.components, tt.bitDepth)
			parser := NewParser(data)
			cs, err := parser.Parse()

			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if int(cs.SIZ.Csiz) != tt.components {
				t.Errorf("Component count mismatch: got %d, want %d",
					cs.SIZ.Csiz, tt.components)
			}

			if len(cs.SIZ.Components) != tt.components {
				t.Errorf("Components array length mismatch: got %d, want %d",
					len(cs.SIZ.Components), tt.components)
			}

			for i, comp := range cs.SIZ.Components {
				if comp.BitDepth() != tt.bitDepth {
					t.Errorf("Component %d bit depth mismatch: got %d, want %d",
						i, comp.BitDepth(), tt.bitDepth)
				}
			}
		})
	}
}

// TestParserMalformedData tests error handling with invalid data
func TestParserMalformedData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"Empty data", []byte{}},
		{"Only SOC", []byte{0xFF, 0x4F}},
		{"Missing EOC", createIncompleteJ2K()},
		{"Invalid marker", []byte{0xFF, 0x4F, 0xFF, 0xFF}},
		{"Truncated SIZ", createTruncatedSIZ()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.data)
			_, err := parser.Parse()

			if err == nil {
				t.Error("Expected error for malformed data but got none")
			}
		})
	}
}

// TestParserCODParameters tests various COD parameter combinations
func TestParserCODParameters(t *testing.T) {
	tests := []struct {
		name        string
		numLevels   uint8
		progression uint8
		numLayers   uint16
	}{
		{"0 levels", 0, 0, 1},
		{"1 level", 1, 0, 1},
		{"3 levels", 3, 0, 1},
		{"5 levels", 5, 0, 1},
		{"Multiple layers", 3, 0, 5},
		{"RLCP progression", 3, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := createJ2KWithCOD(256, 256, tt.numLevels, tt.progression, tt.numLayers)
			parser := NewParser(data)
			cs, err := parser.Parse()

			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if cs.COD == nil {
				t.Fatal("COD segment is nil")
			}

			if cs.COD.NumberOfDecompositionLevels != tt.numLevels {
				t.Errorf("Decomposition levels mismatch: got %d, want %d",
					cs.COD.NumberOfDecompositionLevels, tt.numLevels)
			}

			if cs.COD.ProgressionOrder != tt.progression {
				t.Errorf("Progression order mismatch: got %d, want %d",
					cs.COD.ProgressionOrder, tt.progression)
			}

			if cs.COD.NumberOfLayers != tt.numLayers {
				t.Errorf("Number of layers mismatch: got %d, want %d",
					cs.COD.NumberOfLayers, tt.numLayers)
			}
		})
	}
}

// Helper functions to create test data

func createMinimalJ2K(imageW, imageH, tileW, tileH uint32) []byte {
	buf := bytes.Buffer{}

	// SOC
	_ = binary.Write(&buf, binary.BigEndian, uint16(0xFF4F))

	// SIZ
	writeSIZSegment(&buf, imageW, imageH, tileW, tileH, 1, 8)

	// COD
	writeCODSegment(&buf, 0, 0, 1)

	// QCD
	writeQCDSegment(&buf, 8)

	// EOC
	_ = binary.Write(&buf, binary.BigEndian, uint16(0xFFD9))

	return buf.Bytes()
}

func createJ2KWithComponents(w, h uint32, components, bitDepth int) []byte {
	buf := bytes.Buffer{}

	// SOC
	_ = binary.Write(&buf, binary.BigEndian, uint16(0xFF4F))

	// SIZ with multiple components
	writeSIZSegment(&buf, w, h, w, h, uint16(components), bitDepth)

	// COD
	writeCODSegment(&buf, 0, 0, 1)

	// QCD
	writeQCDSegment(&buf, bitDepth)

	// EOC
	_ = binary.Write(&buf, binary.BigEndian, uint16(0xFFD9))

	return buf.Bytes()
}

func createJ2KWithCOD(w, h uint32, numLevels, progression uint8, numLayers uint16) []byte {
	buf := bytes.Buffer{}

	// SOC
	_ = binary.Write(&buf, binary.BigEndian, uint16(0xFF4F))

	// SIZ
	writeSIZSegment(&buf, w, h, w, h, 1, 8)

	// COD with specific parameters
	writeCODSegment(&buf, numLevels, progression, numLayers)

	// QCD
	writeQCDSegment(&buf, 8)

	// EOC
	_ = binary.Write(&buf, binary.BigEndian, uint16(0xFFD9))

	return buf.Bytes()
}

func createIncompleteJ2K() []byte {
	buf := bytes.Buffer{}
	// SOC
	_ = binary.Write(&buf, binary.BigEndian, uint16(0xFF4F))
	// SIZ but no EOC
	writeSIZSegment(&buf, 256, 256, 256, 256, 1, 8)
	return buf.Bytes()
}

func createTruncatedSIZ() []byte {
	buf := bytes.Buffer{}
	// SOC
	_ = binary.Write(&buf, binary.BigEndian, uint16(0xFF4F))
	// SIZ marker
	_ = binary.Write(&buf, binary.BigEndian, uint16(0xFF51))
	// Incomplete SIZ (just length)
	_ = binary.Write(&buf, binary.BigEndian, uint16(10))
	return buf.Bytes()
}

func writeSIZSegment(buf *bytes.Buffer, w, h, tw, th uint32, comps uint16, bitDepth int) {
	sizData := bytes.Buffer{}

	_ = binary.Write(&sizData, binary.BigEndian, uint16(0))        // Rsiz
	_ = binary.Write(&sizData, binary.BigEndian, w)                // Xsiz
	_ = binary.Write(&sizData, binary.BigEndian, h)                // Ysiz
	_ = binary.Write(&sizData, binary.BigEndian, uint32(0))        // XOsiz
	_ = binary.Write(&sizData, binary.BigEndian, uint32(0))        // YOsiz
	_ = binary.Write(&sizData, binary.BigEndian, tw)               // XTsiz
	_ = binary.Write(&sizData, binary.BigEndian, th)               // YTsiz
	_ = binary.Write(&sizData, binary.BigEndian, uint32(0))        // XTOsiz
	_ = binary.Write(&sizData, binary.BigEndian, uint32(0))        // YTOsiz
	_ = binary.Write(&sizData, binary.BigEndian, comps)            // Csiz

	for i := 0; i < int(comps); i++ {
		ssiz := uint8(bitDepth - 1)
		_ = binary.Write(&sizData, binary.BigEndian, ssiz)     // Ssiz
		_ = binary.Write(&sizData, binary.BigEndian, uint8(1)) // XRsiz
		_ = binary.Write(&sizData, binary.BigEndian, uint8(1)) // YRsiz
	}

	// Write marker and length
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF51))
	_ = binary.Write(buf, binary.BigEndian, uint16(sizData.Len()+2))
	buf.Write(sizData.Bytes())
}

func writeCODSegment(buf *bytes.Buffer, numLevels, progression uint8, numLayers uint16) {
	codData := bytes.Buffer{}

	_ = binary.Write(&codData, binary.BigEndian, uint8(0))      // Scod
	_ = binary.Write(&codData, binary.BigEndian, progression)   // SGcod - progression
	_ = binary.Write(&codData, binary.BigEndian, numLayers)     // Layers
	_ = binary.Write(&codData, binary.BigEndian, uint8(0))      // MCT
	_ = binary.Write(&codData, binary.BigEndian, numLevels)     // Levels
	_ = binary.Write(&codData, binary.BigEndian, uint8(4))      // CB width
	_ = binary.Write(&codData, binary.BigEndian, uint8(4))      // CB height
	_ = binary.Write(&codData, binary.BigEndian, uint8(0))      // CB style
	_ = binary.Write(&codData, binary.BigEndian, uint8(1))      // Transformation

	// Write marker and length
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF52))
	_ = binary.Write(buf, binary.BigEndian, uint16(codData.Len()+2))
	buf.Write(codData.Bytes())
}

func writeQCDSegment(buf *bytes.Buffer, bitDepth int) {
	qcdData := bytes.Buffer{}

	_ = binary.Write(&qcdData, binary.BigEndian, uint8(0))           // Sqcd
	_ = binary.Write(&qcdData, binary.BigEndian, uint8(bitDepth+1)) // SPqcd

	// Write marker and length
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF5C))
	_ = binary.Write(buf, binary.BigEndian, uint16(qcdData.Len()+2))
	buf.Write(qcdData.Bytes())
}
