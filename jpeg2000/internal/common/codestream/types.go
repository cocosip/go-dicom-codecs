package codestream

// Codestream represents a complete JPEG 2000 codestream
type Codestream struct {
	// Main header
	SIZ *SIZSegment            // Image and tile size
	COD *CODSegment            // Coding style default
	QCD *QCDSegment            // Quantization default
	COC map[uint16]*COCSegment // Coding style component overrides
	QCC map[uint16]*QCCSegment // Quantization component overrides
	POC []POCSegment           // Progression order changes
	RGN []RGNSegment           // Region of Interest (main header only)
	COM []COMSegment           // Comments (optional)

	// Part 2 Multi-component transform (optional)
	MCT []MCTSegment
	MCC []MCCSegment
	MCO []MCOSegment

	// Tiles
	Tiles []*Tile

	// Original data (for debugging)
	Data []byte
}

// SIZSegment - Image and tile size marker segment
// ISO/IEC 15444-1 A.5.1
type SIZSegment struct {
	Rsiz   uint16 // Capabilities (0 = baseline)
	Xsiz   uint32 // Width of reference grid
	Ysiz   uint32 // Height of reference grid
	XOsiz  uint32 // Horizontal offset
	YOsiz  uint32 // Vertical offset
	XTsiz  uint32 // Width of one reference tile
	YTsiz  uint32 // Height of one reference tile
	XTOsiz uint32 // Horizontal offset of first tile
	YTOsiz uint32 // Vertical offset of first tile
	Csiz   uint16 // Number of components

	// Per-component parameters
	Components []ComponentSize
}

// ComponentSize holds per-component sizing information
type ComponentSize struct {
	Ssiz  uint8 // Precision and sign (bit 7 = sign, bits 0-6 = depth-1)
	XRsiz uint8 // Horizontal separation
	YRsiz uint8 // Vertical separation
}

// BitDepth returns the bit depth of the component
func (c *ComponentSize) BitDepth() int {
	return int(c.Ssiz&0x7F) + 1
}

// IsSigned returns true if the component is signed
func (c *ComponentSize) IsSigned() bool {
	return (c.Ssiz & 0x80) != 0
}

// CODSegment - Coding style default marker segment
// ISO/IEC 15444-1 A.6.1
type CODSegment struct {
	Scod uint8 // Coding style for all components
	// Scod bit interpretation:
	//   0: Entropy coder with/without partitions
	//   1: SOP marker segments
	//   2: EPH marker segments

	// SGcod - General coding style parameters
	ProgressionOrder           uint8  // 0=LRCP, 1=RLCP, 2=RPCL, 3=PCRL, 4=CPRL
	NumberOfLayers             uint16 // Number of layers
	MultipleComponentTransform uint8  // 0=none, 1=RCT or ICT

	// SPcod - Coding style parameters
	NumberOfDecompositionLevels uint8 // Number of decomposition levels
	CodeBlockWidth              uint8 // Code-block width exponent (2^(n+2))
	CodeBlockHeight             uint8 // Code-block height exponent (2^(n+2))
	CodeBlockStyle              uint8 // Code-block style
	Transformation              uint8 // Wavelet transformation: 0=9-7 irreversible, 1=5-3 reversible

	// Precinct sizes (if Scod bit 0 is set)
	PrecinctSizes []PrecinctSize // One per resolution level
}

// PrecinctSize holds precinct dimensions for a resolution level
type PrecinctSize struct {
	PPx uint8 // Precinct width exponent
	PPy uint8 // Precinct height exponent
}

// CodeBlockSize returns the actual code-block dimensions
func (c *CODSegment) CodeBlockSize() (width, height int) {
	width = 1 << (c.CodeBlockWidth + 2)
	height = 1 << (c.CodeBlockHeight + 2)
	return
}

// QCDSegment - Quantization default marker segment
// ISO/IEC 15444-1 A.6.4
type QCDSegment struct {
	Sqcd uint8 // Quantization style
	// Sqcd interpretation:
	//   bits 0-4: Quantization type (0=none, 1=scalar derived, 2=scalar expounded)
	//   bits 5-7: Number of guard bits

	SPqcd []byte // Quantization step size values
}

// QuantizationType returns the quantization type
func (q *QCDSegment) QuantizationType() int {
	return int(q.Sqcd & 0x1F)
}

// GuardBits returns the number of guard bits
func (q *QCDSegment) GuardBits() int {
	return int(q.Sqcd >> 5)
}

// COMSegment - Comment marker segment
type COMSegment struct {
	Rcom uint16 // Registration value (0=binary, 1=ISO/IEC 8859-15)
	Data []byte // Comment data
}

// COCSegment - Coding style component marker segment
// ISO/IEC 15444-1 A.6.2
type COCSegment struct {
	Component uint16 // Component index
	Scoc      uint8  // Coding style for this component

	// SPcoc - Coding style parameters (same fields as COD SPcod)
	NumberOfDecompositionLevels uint8
	CodeBlockWidth              uint8
	CodeBlockHeight             uint8
	CodeBlockStyle              uint8
	Transformation              uint8
	PrecinctSizes               []PrecinctSize
}

// QCCSegment - Quantization component marker segment
// ISO/IEC 15444-1 A.6.5
type QCCSegment struct {
	Component uint16 // Component index
	Sqcc      uint8  // Quantization style
	SPqcc     []byte // Quantization step size values
}

// POCEntry represents one progression order change entry.
type POCEntry struct {
	RSpoc  uint8  // Start resolution
	CSpoc  uint16 // Start component
	LYEpoc uint16 // End layer
	REpoc  uint8  // End resolution
	CEpoc  uint16 // End component
	Ppoc   uint8  // Progression order
}

// POCSegment - Progression order change marker segment
// ISO/IEC 15444-1 A.6.6
type POCSegment struct {
	Entries []POCEntry
}

// MCTArrayType enumerates multi-component transform array types.
type MCTArrayType uint8

// MCTElementType enumerates element types used in MCT arrays.
type MCTElementType uint8

// MCTArrayType values define transform array usage.
const (
	MCTArrayDependency  MCTArrayType = 0
	MCTArrayDecorrelate MCTArrayType = 1
	MCTArrayOffset      MCTArrayType = 2
)

// MCTElementType values define element representation.
const (
	MCTElementInt16   MCTElementType = 0
	MCTElementInt32   MCTElementType = 1
	MCTElementFloat32 MCTElementType = 2
	MCTElementFloat64 MCTElementType = 3
)

// MCTSegment describes a multi-component transform segment (Part 2).
type MCTSegment struct {
	Index       uint8
	ElementType MCTElementType
	ArrayType   MCTArrayType
	Data        []byte
}

// MCCSegment describes a Multiple Component Collection segment (Part 2).
type MCCSegment struct {
	Index              uint8
	CollectionType     uint8
	NumComponents      uint16
	ComponentIDs       []uint16
	OutputComponentIDs []uint16
	DecorrelateIndex   uint8
	OffsetIndex        uint8
	Reversible         bool
}

// MCOSegment describes MCT ordering segment (Part 2).
type MCOSegment struct {
	NumStages    uint8
	StageIndices []uint8
}

// RGNSegment - Region of Interest marker segment (MaxShift)
// ISO/IEC 15444-1 A.6.3
type RGNSegment struct {
	Crgn  uint16 // Component index
	Srgn  uint8  // ROI style (0 = MaxShift)
	SPrgn uint8  // ROI shift value (number of most significant bit-planes to skip)
}

// Tile represents a single tile in the codestream
type Tile struct {
	Index int                    // Tile index
	SOT   *SOTSegment            // Start of tile
	COD   *CODSegment            // Coding style (optional, overrides default)
	QCD   *QCDSegment            // Quantization (optional, overrides default)
	COC   map[uint16]*COCSegment // Coding style component overrides
	QCC   map[uint16]*QCCSegment // Quantization component overrides
	POC   []POCSegment           // Progression order changes
	RGN   []*RGNSegment          // ROI (optional, tile-specific ROI)
	Data  []byte                 // Compressed tile data (after SOD marker)

	// Decoded components (filled during decode)
	Components []*TileComponent
}

// TileCOD returns the tile-level COD, falling back to main header defaults.
func (cs *Codestream) TileCOD(tile *Tile) *CODSegment {
	if tile != nil && tile.COD != nil {
		return tile.COD
	}
	if cs == nil {
		return nil
	}
	return cs.COD
}

// TileQCD returns the tile-level QCD, falling back to main header defaults.
func (cs *Codestream) TileQCD(tile *Tile) *QCDSegment {
	if tile != nil && tile.QCD != nil {
		return tile.QCD
	}
	if cs == nil {
		return nil
	}
	return cs.QCD
}

// ComponentCOD resolves COD/COC inheritance for a component.
func (cs *Codestream) ComponentCOD(tile *Tile, component int) *CODSegment {
	if cs == nil || component < 0 {
		return nil
	}
	base := cs.TileCOD(tile)
	if base == nil {
		return nil
	}
	out := cloneCOD(base)
	if cs.COC != nil {
		if coc := cs.COC[uint16(component)]; coc != nil {
			out = applyCOC(out, coc)
		}
	}
	if tile != nil && tile.COC != nil {
		if coc := tile.COC[uint16(component)]; coc != nil {
			out = applyCOC(out, coc)
		}
	}
	return out
}

// ComponentQCD resolves QCD/QCC inheritance for a component.
func (cs *Codestream) ComponentQCD(tile *Tile, component int) *QCDSegment {
	if cs == nil || component < 0 {
		return nil
	}
	base := cs.TileQCD(tile)
	if base == nil {
		return nil
	}
	out := cloneQCD(base)
	if cs.QCC != nil {
		if qcc := cs.QCC[uint16(component)]; qcc != nil {
			out = applyQCC(out, qcc)
		}
	}
	if tile != nil && tile.QCC != nil {
		if qcc := tile.QCC[uint16(component)]; qcc != nil {
			out = applyQCC(out, qcc)
		}
	}
	return out
}

func cloneCOD(src *CODSegment) *CODSegment {
	if src == nil {
		return nil
	}
	dst := *src
	if src.PrecinctSizes != nil {
		dst.PrecinctSizes = append([]PrecinctSize(nil), src.PrecinctSizes...)
	}
	return &dst
}

func cloneQCD(src *QCDSegment) *QCDSegment {
	if src == nil {
		return nil
	}
	dst := *src
	if src.SPqcd != nil {
		dst.SPqcd = append([]byte(nil), src.SPqcd...)
	}
	return &dst
}

func applyCOC(base *CODSegment, coc *COCSegment) *CODSegment {
	if base == nil {
		return nil
	}
	out := cloneCOD(base)
	if coc == nil {
		return out
	}
	out.Scod = coc.Scoc
	out.NumberOfDecompositionLevels = coc.NumberOfDecompositionLevels
	out.CodeBlockWidth = coc.CodeBlockWidth
	out.CodeBlockHeight = coc.CodeBlockHeight
	out.CodeBlockStyle = coc.CodeBlockStyle
	out.Transformation = coc.Transformation
	if len(coc.PrecinctSizes) > 0 {
		out.PrecinctSizes = append([]PrecinctSize(nil), coc.PrecinctSizes...)
	} else {
		out.PrecinctSizes = nil
	}
	return out
}

func applyQCC(base *QCDSegment, qcc *QCCSegment) *QCDSegment {
	if base == nil {
		return nil
	}
	out := cloneQCD(base)
	if qcc == nil {
		return out
	}
	out.Sqcd = qcc.Sqcc
	if qcc.SPqcc != nil {
		out.SPqcd = append([]byte(nil), qcc.SPqcc...)
	} else {
		out.SPqcd = nil
	}
	return out
}

// SOTSegment - Start of tile-part marker segment
// ISO/IEC 15444-1 A.4.2
type SOTSegment struct {
	Isot  uint16 // Tile index
	Psot  uint32 // Tile-part length
	TPsot uint8  // Tile-part index
	TNsot uint8  // Number of tile-parts
}

// TileComponent represents a single component within a tile
type TileComponent struct {
	Index       int           // Component index
	Width       int           // Component width
	Height      int           // Component height
	Resolutions []*Resolution // Resolution levels (0 = LL subband, 1+ = HL/LH/HH)
}

// Resolution represents one resolution level
type Resolution struct {
	Level    int        // Resolution level (0 = lowest)
	Width    int        // Width at this resolution
	Height   int        // Height at this resolution
	Subbands []*Subband // Subbands (LL, HL, LH, HH)
}

// Subband represents one subband (LL, HL, LH, or HH)
type Subband struct {
	Type   SubbandType // LL, HL, LH, or HH
	Width  int         // Subband width
	Height int         // Subband height

	// Code-blocks within this subband
	CodeBlocks []*CodeBlock

	// Coefficients (filled during decode)
	Coefficients []int32
}

// SubbandType identifies the subband orientation
type SubbandType int

// Subband type constants for LL/HL/LH/HH orientations
const (
	SubbandLL SubbandType = iota // Low-Low (approximation)
	SubbandHL                    // High-Low (horizontal detail)
	SubbandLH                    // Low-High (vertical detail)
	SubbandHH                    // High-High (diagonal detail)
)

// String returns the subband type name
func (s SubbandType) String() string {
	switch s {
	case SubbandLL:
		return "LL"
	case SubbandHL:
		return "HL"
	case SubbandLH:
		return "LH"
	case SubbandHH:
		return "HH"
	default:
		return unknownString
	}
}

// CodeBlock represents one code-block
type CodeBlock struct {
	X0, Y0 int    // Top-left position in subband
	X1, Y1 int    // Bottom-right position in subband
	Data   []byte // Compressed data
	Passes int    // Number of coding passes

	// Decoded coefficients (filled during decode)
	Coefficients []int32
}

// Width returns the code-block width
func (cb *CodeBlock) Width() int {
	return cb.X1 - cb.X0
}

// Height returns the code-block height
func (cb *CodeBlock) Height() int {
	return cb.Y1 - cb.Y0
}
