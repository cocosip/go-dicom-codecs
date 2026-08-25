package t2

import "github.com/cocosip/go-dicom-codecs/jpeg2000/openjpeg/t1"

// Tier-2 Package Data Structures
// Reference: ISO/IEC 15444-1:2019 Annex B

// Packet represents a JPEG 2000 packet
// A packet contains compressed data for one resolution, one component,
// one layer, and one precinct
type Packet struct {
	// Packet header information
	HeaderPresent bool   // Whether packet has data (or is empty)
	Header        []byte // Raw packet header data

	// Packet body
	Body []byte // Compressed code-block contributions

	// Decoded header information
	LayerIndex      int             // Layer index
	ResolutionLevel int             // Resolution level
	ComponentIndex  int             // Component index
	PrecinctIndex   int             // Precinct index
	CodeBlockIncls  []CodeBlockIncl // Code-block inclusion information

	// Error resilience
	PartialBuffer bool // Insufficient data to complete packet
}

// CodeBlockIncl represents code-block inclusion and contribution information
type CodeBlockIncl struct {
	Included       bool   // Whether this code-block is included in this packet
	FirstInclusion bool   // Whether this is the first packet to include this code-block
	NumPasses      int    // Number of coding passes contributed
	DataLength     int    // Length of compressed data in bytes
	Data           []byte // Compressed code-block data
	ZeroBitplanes  int    // Number of missing MSB bit-planes (from tag tree)
	PassLengths    []int  // Cumulative byte length of each pass (for TERMALL mode)
	UseTERMALL     bool   // If true, passes are terminated (TERMALL mode)

	// Error resilience
	Corrupted bool // Codeblock data is corrupted or truncated
}

// Precinct represents a precinct in the codestream
// A precinct is a spatial partition of a resolution level
type Precinct struct {
	Index      int                  // Precinct index
	X0, Y0     int                  // Top-left coordinates
	X1, Y1     int                  // Bottom-right coordinates
	Width      int                  // Precinct width
	Height     int                  // Precinct height
	SubbandIdx int                  // Subband index (0=LL, 1=HL, 2=LH, 3=HH)
	CodeBlocks []*PrecinctCodeBlock // Code-blocks in this precinct

	// Tag-tree dimensions (for packet header encoding)
	NumCodeBlocksX int // Number of code-blocks in X direction
	NumCodeBlocksY int // Number of code-blocks in Y direction

	// Tag-trees for encoding (created per-packet)
	InclTree *TagTree // Inclusion tag-tree
	ZBPTree  *TagTree // Zero-bitplane tag-tree
}

// PrecinctCodeBlock represents a code-block within a precinct
type PrecinctCodeBlock struct {
	Index          int    // Code-block index within precinct
	X0, Y0         int    // Top-left coordinates
	X1, Y1         int    // Bottom-right coordinates
	CBX, CBY       int    // Code-block grid position within precinct
	Band           int    // Subband index (0=LL,1=HL,2=LH,3=HH)
	Included       bool   // Has been included in at least one packet
	NumPassesTotal int    // Total number of passes decoded so far
	ZeroBitPlanes  int    // Number of missing MSB bit-planes
	Data           []byte // Accumulated compressed data

	// Multi-layer support
	LayerPasses []int    // Number of passes included in each layer (cumulative)
	LayerData   [][]byte // Encoded data for each layer's passes
	PassLengths []int    // Cumulative byte length of each pass (used in TERMALL mode)

	// Coding style
	UseTERMALL bool // If true, each pass is terminated (TERMALL mode)

	// Rate-distortion support
	Passes       []t1.PassData // Detailed per-pass data (rate/distortion)
	CompleteData []byte        // Full MQ bitstream for all passes

	// Packet header state (OpenJPEG: numlenbits)
	NumLenBits int
}

// Layer represents a quality layer
// Layers provide progressive quality refinement
type Layer struct {
	Index   int      // Layer index (0 = lowest quality)
	Packets []Packet // Packets in this layer
}

// ProgressionOrder defines the order in which packets are encoded/decoded
type ProgressionOrder int

const (
	// ProgressionLRCP specifies Layer-Resolution-Component-Position ordering (default).
	ProgressionLRCP ProgressionOrder = 0

	// ProgressionRLCP specifies Resolution-Layer-Component-Position ordering (resolution scalability).
	ProgressionRLCP ProgressionOrder = 1

	// ProgressionRPCL specifies Resolution-Position-Component-Layer ordering (spatial random access).
	ProgressionRPCL ProgressionOrder = 2

	// ProgressionPCRL specifies Position-Component-Resolution-Layer ordering (spatial scalability).
	ProgressionPCRL ProgressionOrder = 3

	// ProgressionCPRL specifies Component-Position-Resolution-Layer ordering (component scalability).
	ProgressionCPRL ProgressionOrder = 4
)

// String returns the progression order name
func (p ProgressionOrder) String() string {
	switch p {
	case ProgressionLRCP:
		return "LRCP"
	case ProgressionRLCP:
		return "RLCP"
	case ProgressionRPCL:
		return "RPCL"
	case ProgressionPCRL:
		return "PCRL"
	case ProgressionCPRL:
		return "CPRL"
	default:
		return "UNKNOWN"
	}
}

// Note: TagTree implementation moved to tagtree.go

// PacketIterator iterates through packets in a specific progression order
type PacketIterator struct {
	// Codestream parameters
	NumComponents    int
	NumLayers        int
	NumResolutions   int
	TileWidth        int
	TileHeight       int
	PrecinctWidth    int
	PrecinctHeight   int
	PrecinctWidths   []int
	PrecinctHeights  []int
	ProgressionOrder ProgressionOrder

	// Current position
	Layer      int
	Resolution int
	Component  int
	PrecinctX  int
	PrecinctY  int

	// State
	Done bool
}

// NewPacketIterator creates a new packet iterator
func NewPacketIterator(
	numComponents, numLayers, numResolutions int,
	tileWidth, tileHeight int,
	precinctWidth, precinctHeight int,
	progressionOrder ProgressionOrder,
) *PacketIterator {
	return &PacketIterator{
		NumComponents:    numComponents,
		NumLayers:        numLayers,
		NumResolutions:   numResolutions,
		TileWidth:        tileWidth,
		TileHeight:       tileHeight,
		PrecinctWidth:    precinctWidth,
		PrecinctHeight:   precinctHeight,
		ProgressionOrder: progressionOrder,
		Done:             false,
	}
}

func (pi *PacketIterator) resolutionDims(resolution int) (int, int) {
	numLevels := pi.NumResolutions - 1
	if numLevels < 0 {
		numLevels = 0
	}
	shift := numLevels - resolution
	if shift < 0 {
		shift = 0
	}
	return ceilDivPow2(pi.TileWidth, shift), ceilDivPow2(pi.TileHeight, shift)
}

func (pi *PacketIterator) precinctSizeForResolution(resolution int) (int, int) {
	pw := 0
	ph := 0
	if resolution >= 0 {
		if resolution < len(pi.PrecinctWidths) {
			pw = pi.PrecinctWidths[resolution]
		}
		if resolution < len(pi.PrecinctHeights) {
			ph = pi.PrecinctHeights[resolution]
		}
	}
	if pw == 0 {
		pw = pi.PrecinctWidth
	}
	if ph == 0 {
		ph = pi.PrecinctHeight
	}
	if pw == 0 {
		pw = 1 << 15
	}
	if ph == 0 {
		ph = 1 << 15
	}
	if len(pi.PrecinctWidths) == 0 && len(pi.PrecinctHeights) == 0 &&
		(pi.PrecinctWidth > 0 || pi.PrecinctHeight > 0) && pi.NumResolutions > 1 {
		shift := (pi.NumResolutions - 1) - resolution
		if shift < 0 {
			shift = 0
		}
		if shift > 0 {
			pw = intMax(1, pw>>shift)
			ph = intMax(1, ph>>shift)
		}
	}
	return pw, ph
}

func (pi *PacketIterator) precinctGridForResolution(resolution int) (int, int) {
	resW, resH := pi.resolutionDims(resolution)
	pw, ph := pi.precinctSizeForResolution(resolution)
	numPrecinctX := ceilDiv(resW, pw)
	numPrecinctY := ceilDiv(resH, ph)
	if numPrecinctX < 1 {
		numPrecinctX = 1
	}
	if numPrecinctY < 1 {
		numPrecinctY = 1
	}
	return numPrecinctX, numPrecinctY
}

// Next advances to the next packet and returns its coordinates
func (pi *PacketIterator) Next() (layer, resolution, component, precinctX, precinctY int, ok bool) {
	if pi.Done {
		return 0, 0, 0, 0, 0, false
	}

	// Store current position
	layer = pi.Layer
	resolution = pi.Resolution
	component = pi.Component
	precinctX = pi.PrecinctX
	precinctY = pi.PrecinctY

	// Advance to next position based on progression order
	switch pi.ProgressionOrder {
	case ProgressionLRCP:
		pi.advanceLRCP()
	case ProgressionRLCP:
		pi.advanceRLCP()
	case ProgressionRPCL:
		pi.advanceRPCL()
	case ProgressionPCRL:
		pi.advancePCRL()
	case ProgressionCPRL:
		pi.advanceCPRL()
	}

	return layer, resolution, component, precinctX, precinctY, true
}

// advanceLRCP advances in Layer-Resolution-Component-Position order
func (pi *PacketIterator) advanceLRCP() {
	numPrecinctX, numPrecinctY := pi.precinctGridForResolution(pi.Resolution)

	pi.PrecinctX++
	if pi.PrecinctX >= numPrecinctX {
		pi.PrecinctX = 0
		pi.PrecinctY++
		if pi.PrecinctY >= numPrecinctY {
			pi.PrecinctY = 0
			pi.Component++
			if pi.Component >= pi.NumComponents {
				pi.Component = 0
				pi.Resolution++
				if pi.Resolution >= pi.NumResolutions {
					pi.Resolution = 0
					pi.Layer++
					if pi.Layer >= pi.NumLayers {
						pi.Done = true
					}
				}
			}
		}
	}
}

// advanceRLCP advances in Resolution-Layer-Component-Position order
func (pi *PacketIterator) advanceRLCP() {
	numPrecinctX, numPrecinctY := pi.precinctGridForResolution(pi.Resolution)

	pi.PrecinctX++
	if pi.PrecinctX >= numPrecinctX {
		pi.PrecinctX = 0
		pi.PrecinctY++
		if pi.PrecinctY >= numPrecinctY {
			pi.PrecinctY = 0
			pi.Component++
			if pi.Component >= pi.NumComponents {
				pi.Component = 0
				pi.Layer++
				if pi.Layer >= pi.NumLayers {
					pi.Layer = 0
					pi.Resolution++
					if pi.Resolution >= pi.NumResolutions {
						pi.Done = true
					}
				}
			}
		}
	}
}

// advanceRPCL advances in Resolution-Position-Component-Layer order
func (pi *PacketIterator) advanceRPCL() {
	pi.Layer++
	if pi.Layer >= pi.NumLayers {
		pi.Layer = 0
		pi.Component++
		if pi.Component >= pi.NumComponents {
			pi.Component = 0
			numPrecinctX, numPrecinctY := pi.precinctGridForResolution(pi.Resolution)
			pi.PrecinctX++
			if pi.PrecinctX >= numPrecinctX {
				pi.PrecinctX = 0
				pi.PrecinctY++
				if pi.PrecinctY >= numPrecinctY {
					pi.PrecinctY = 0
					pi.Resolution++
					if pi.Resolution >= pi.NumResolutions {
						pi.Done = true
					}
				}
			}
		}
	}
}

// advancePCRL advances in Position-Component-Resolution-Layer order
func (pi *PacketIterator) advancePCRL() {
	pi.Layer++
	if pi.Layer >= pi.NumLayers {
		pi.Layer = 0
		pi.Resolution++
		if pi.Resolution >= pi.NumResolutions {
			pi.Resolution = 0
			pi.Component++
			if pi.Component >= pi.NumComponents {
				pi.Component = 0
				numPrecinctX, numPrecinctY := pi.precinctGridForResolution(pi.Resolution)
				pi.PrecinctX++
				if pi.PrecinctX >= numPrecinctX {
					pi.PrecinctX = 0
					pi.PrecinctY++
					if pi.PrecinctY >= numPrecinctY {
						pi.Done = true
					}
				}
			}
		}
	}
}

// advanceCPRL advances in Component-Position-Resolution-Layer order
func (pi *PacketIterator) advanceCPRL() {
	pi.Layer++
	if pi.Layer >= pi.NumLayers {
		pi.Layer = 0
		pi.Resolution++
		if pi.Resolution >= pi.NumResolutions {
			pi.Resolution = 0
			numPrecinctX, numPrecinctY := pi.precinctGridForResolution(pi.Resolution)
			pi.PrecinctX++
			if pi.PrecinctX >= numPrecinctX {
				pi.PrecinctX = 0
				pi.PrecinctY++
				if pi.PrecinctY >= numPrecinctY {
					pi.PrecinctY = 0
					pi.Component++
					if pi.Component >= pi.NumComponents {
						pi.Done = true
					}
				}
			}
		}
	}
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
