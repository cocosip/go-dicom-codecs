// Package codestream defines JPEG 2000 codestream markers and helpers.
package codestream

const unknownString = "UNKNOWN"

// JPEG 2000 Marker Codes
// Reference: ISO/IEC 15444-1:2019 Table A.1

// Delimiting markers and marker segments
const (
	// MarkerSOC - Start of codestream
	MarkerSOC uint16 = 0xFF4F

	// MarkerSOT - Start of tile-part
	MarkerSOT uint16 = 0xFF90

	// MarkerSOD - Start of data
	MarkerSOD uint16 = 0xFF93

	// MarkerEOC - End of codestream
	MarkerEOC uint16 = 0xFFD9
)

// Fixed information marker segments
const (
	// MarkerSIZ - Image and tile size
	MarkerSIZ uint16 = 0xFF51

	// MarkerCAP - Extended capabilities (JPEG 2000 Part 15 HTJ2K)
	MarkerCAP uint16 = 0xFF50
)

// Functional marker segments
const (
	// MarkerCOD - Coding style default
	MarkerCOD uint16 = 0xFF52

	// MarkerCOC - Coding style component
	MarkerCOC uint16 = 0xFF53

	// MarkerRGN - Region of interest
	MarkerRGN uint16 = 0xFF5E

	// MarkerQCD - Quantization default
	MarkerQCD uint16 = 0xFF5C

	// MarkerQCC - Quantization component
	MarkerQCC uint16 = 0xFF5D

	// MarkerPOC - Progression order change
	MarkerPOC uint16 = 0xFF5F
)

// Pointer marker segments
const (
	// MarkerTLM - Tile-part lengths
	MarkerTLM uint16 = 0xFF55

	// MarkerCPF - Corresponding profile values (JPEG 2000 Part 15 HTJ2K)
	MarkerCPF uint16 = 0xFF59

	// MarkerPLM - Packet length, main header
	MarkerPLM uint16 = 0xFF57

	// MarkerPLT - Packet length, tile-part header
	MarkerPLT uint16 = 0xFF58

	// MarkerPPM - Packed packet headers, main header
	MarkerPPM uint16 = 0xFF60

	// MarkerPPT - Packed packet headers, tile-part header
	MarkerPPT uint16 = 0xFF61
)

// Informational marker segments
const (
	// MarkerCRG - Component registration
	MarkerCRG uint16 = 0xFF63

	// MarkerCOM - Comment
	MarkerCOM uint16 = 0xFF64

	// Part 2 Multi-component transform markers (ISO/IEC 15444-2)
	MarkerMCT uint16 = 0xFF74 // Multi-component Transform
	MarkerMCC uint16 = 0xFF75 // Multiple Component Collection
	MarkerMCO uint16 = 0xFF77 // MCT ordering
)

// MarkerName returns the name of a marker code
func MarkerName(marker uint16) string {
	switch marker {
	// Delimiting markers
	case MarkerSOC:
		return "SOC"
	case MarkerSOT:
		return "SOT"
	case MarkerSOD:
		return "SOD"
	case MarkerEOC:
		return "EOC"

	// Fixed information
	case MarkerSIZ:
		return "SIZ"
	case MarkerCAP:
		return "CAP"

	// Functional
	case MarkerCOD:
		return "COD"
	case MarkerCOC:
		return "COC"
	case MarkerRGN:
		return "RGN"
	case MarkerQCD:
		return "QCD"
	case MarkerQCC:
		return "QCC"
	case MarkerPOC:
		return "POC"

	// Pointer
	case MarkerTLM:
		return "TLM"
	case MarkerCPF:
		return "CPF"
	case MarkerPLM:
		return "PLM"
	case MarkerPLT:
		return "PLT"
	case MarkerPPM:
		return "PPM"
	case MarkerPPT:
		return "PPT"

	// Informational
	case MarkerCRG:
		return "CRG"
	case MarkerCOM:
		return "COM"

	// Part 2 MCT/MCC
	case MarkerMCT:
		return "MCT"
	case MarkerMCC:
		return "MCC"
	case MarkerMCO:
		return "MCO"

	default:
		return unknownString
	}
}

// HasLength returns true if the marker has a length field
func HasLength(marker uint16) bool {
	switch marker {
	case MarkerSOC, MarkerSOD, MarkerEOC:
		return false
	default:
		return true
	}
}
