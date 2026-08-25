package codestream

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestParserBasic tests basic parser functionality
func TestParserBasic(t *testing.T) {
	// Create a minimal JPEG 2000 codestream
	var buf bytes.Buffer

	// SOC marker
	_ = binary.Write(&buf, binary.BigEndian, MarkerSOC)

	// SIZ segment
	_ = binary.Write(&buf, binary.BigEndian, MarkerSIZ)
	_ = binary.Write(&buf, binary.BigEndian, uint16(41)) // Length: 38 + 3*1 components
	_ = binary.Write(&buf, binary.BigEndian, uint16(0))  // Rsiz (baseline)
	_ = binary.Write(&buf, binary.BigEndian, uint32(256)) // Xsiz
	_ = binary.Write(&buf, binary.BigEndian, uint32(256)) // Ysiz
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))   // XOsiz
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))   // YOsiz
	_ = binary.Write(&buf, binary.BigEndian, uint32(256)) // XTsiz
	_ = binary.Write(&buf, binary.BigEndian, uint32(256)) // YTsiz
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))   // XTOsiz
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))   // YTOsiz
	_ = binary.Write(&buf, binary.BigEndian, uint16(1))   // Csiz (1 component)
	// Component 0
	_ = binary.Write(&buf, binary.BigEndian, uint8(7))  // Ssiz (8-bit unsigned)
	_ = binary.Write(&buf, binary.BigEndian, uint8(1))  // XRsiz
	_ = binary.Write(&buf, binary.BigEndian, uint8(1))  // YRsiz

	// COD segment
	_ = binary.Write(&buf, binary.BigEndian, MarkerCOD)
	_ = binary.Write(&buf, binary.BigEndian, uint16(12)) // Length
	_ = binary.Write(&buf, binary.BigEndian, uint8(0))   // Scod
	_ = binary.Write(&buf, binary.BigEndian, uint8(0))   // Progression order (LRCP)
	_ = binary.Write(&buf, binary.BigEndian, uint16(1))  // Number of layers
	_ = binary.Write(&buf, binary.BigEndian, uint8(0))   // MCT
	_ = binary.Write(&buf, binary.BigEndian, uint8(5))   // Decomposition levels
	_ = binary.Write(&buf, binary.BigEndian, uint8(2))   // Code-block width (2^4 = 16)
	_ = binary.Write(&buf, binary.BigEndian, uint8(2))   // Code-block height (2^4 = 16)
	_ = binary.Write(&buf, binary.BigEndian, uint8(0))   // Code-block style
	_ = binary.Write(&buf, binary.BigEndian, uint8(1))   // Transformation (5-3 reversible)

	// QCD segment
	_ = binary.Write(&buf, binary.BigEndian, MarkerQCD)
	_ = binary.Write(&buf, binary.BigEndian, uint16(5)) // Length (2 + 1 Sqcd + 2 data)
	_ = binary.Write(&buf, binary.BigEndian, uint8(0))  // Sqcd (no quantization)
	_ = binary.Write(&buf, binary.BigEndian, uint16(0)) // Dummy quantization data

	// EOC marker
	_ = binary.Write(&buf, binary.BigEndian, MarkerEOC)

	// Parse
	parser := NewParser(buf.Bytes())
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify SIZ
	if cs.SIZ == nil {
		t.Fatal("SIZ is nil")
	}
	if cs.SIZ.Xsiz != 256 || cs.SIZ.Ysiz != 256 {
		t.Errorf("Expected 256x256, got %dx%d", cs.SIZ.Xsiz, cs.SIZ.Ysiz)
	}
	if cs.SIZ.Csiz != 1 {
		t.Errorf("Expected 1 component, got %d", cs.SIZ.Csiz)
	}
	if cs.SIZ.Components[0].BitDepth() != 8 {
		t.Errorf("Expected 8-bit, got %d-bit", cs.SIZ.Components[0].BitDepth())
	}

	// Verify COD
	if cs.COD == nil {
		t.Fatal("COD is nil")
	}
	if cs.COD.NumberOfDecompositionLevels != 5 {
		t.Errorf("Expected 5 decomposition levels, got %d", cs.COD.NumberOfDecompositionLevels)
	}
	if cs.COD.Transformation != 1 {
		t.Errorf("Expected 5-3 transform, got %d", cs.COD.Transformation)
	}

	// Verify QCD
	if cs.QCD == nil {
		t.Fatal("QCD is nil")
	}

	t.Logf("Successfully parsed minimal JPEG 2000 codestream")
}

// TestMarkerName tests marker name function
func TestMarkerName(t *testing.T) {
	tests := []struct {
		marker   uint16
		expected string
	}{
		{MarkerSOC, "SOC"},
		{MarkerSIZ, "SIZ"},
		{MarkerCOD, "COD"},
		{MarkerQCD, "QCD"},
		{MarkerSOT, "SOT"},
		{MarkerSOD, "SOD"},
		{MarkerEOC, "EOC"},
		{0xFFFF, "UNKNOWN"},
	}

	for _, tt := range tests {
		name := MarkerName(tt.marker)
		if name != tt.expected {
			t.Errorf("MarkerName(0x%04X) = %s, want %s", tt.marker, name, tt.expected)
		}
	}
}

// TestHasLength tests marker length function
func TestHasLength(t *testing.T) {
	if HasLength(MarkerSOC) {
		t.Error("SOC should not have length")
	}
	if HasLength(MarkerSOD) {
		t.Error("SOD should not have length")
	}
	if HasLength(MarkerEOC) {
		t.Error("EOC should not have length")
	}
	if !HasLength(MarkerSIZ) {
		t.Error("SIZ should have length")
	}
	if !HasLength(MarkerCOD) {
		t.Error("COD should have length")
	}
}

// TestComponentSize tests component size methods
func TestComponentSize(t *testing.T) {
	tests := []struct {
		ssiz     uint8
		bitDepth int
		signed   bool
	}{
		{0x07, 8, false},  // 8-bit unsigned
		{0x87, 8, true},   // 8-bit signed
		{0x0B, 12, false}, // 12-bit unsigned
		{0x8B, 12, true},  // 12-bit signed
	}

	for _, tt := range tests {
		cs := ComponentSize{Ssiz: tt.ssiz}
		if cs.BitDepth() != tt.bitDepth {
			t.Errorf("BitDepth() = %d, want %d", cs.BitDepth(), tt.bitDepth)
		}
		if cs.IsSigned() != tt.signed {
			t.Errorf("IsSigned() = %v, want %v", cs.IsSigned(), tt.signed)
		}
	}
}

// TestCODCodeBlockSize tests code-block size calculation
func TestCODCodeBlockSize(t *testing.T) {
	cod := &CODSegment{
		CodeBlockWidth:  2, // 2^(2+2) = 16
		CodeBlockHeight: 3, // 2^(3+2) = 32
	}

	w, h := cod.CodeBlockSize()
	if w != 16 || h != 32 {
		t.Errorf("CodeBlockSize() = %dx%d, want 16x32", w, h)
	}
}

// TestSubbandType tests subband type string
func TestSubbandType(t *testing.T) {
	tests := []struct {
		typ      SubbandType
		expected string
	}{
		{SubbandLL, "LL"},
		{SubbandHL, "HL"},
		{SubbandLH, "LH"},
		{SubbandHH, "HH"},
	}

	for _, tt := range tests {
		if tt.typ.String() != tt.expected {
			t.Errorf("SubbandType.String() = %s, want %s", tt.typ.String(), tt.expected)
		}
	}
}
