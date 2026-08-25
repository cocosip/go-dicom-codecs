package t2

import (
	"bytes"
	"fmt"
	"sort"
)

// PacketDecoder decodes JPEG 2000 packets
// Reference: ISO/IEC 15444-1:2019 Annex B
type PacketDecoder struct {
	// Input bitstream
	data   []byte
	offset int

	// Decoding parameters
	numComponents  int
	numLayers      int
	numResolutions int
	progression    ProgressionOrder
	imageWidth     int
	imageHeight    int
	cbWidth        int
	cbHeight       int
	numLevels      int
	codeBlockStyle uint8 // Code-block style (for TERMALL detection)

	// Per-component geometry (tile-component bounds)
	compBounds []componentBounds
	compDx     []int
	compDy     []int

	// Precinct parameters
	precinctWidth   int // Precinct width (0 = default size of 2^15)
	precinctHeight  int // Precinct height (0 = default size of 2^15)
	precinctWidths  []int
	precinctHeights []int

	// Parsed packets
	packets []Packet

	// Multi-layer state tracking
	// Maps "component:resolution:cbIndex" -> true if code-block was included in a previous layer
	cbIncluded map[string]bool

	// Precinct -> code-block order mapping per resolution (mirrors encoder traversal)
	cbPrecinctOrder map[int]map[int]map[int][]int
	// Precinct -> code-block positions (CBX/CBY) in packet header order
	cbPrecinctPositions map[int]map[int]map[int]map[int][]cbPosition
	// Precinct -> code-block grid dimensions used for tag-tree decoding
	cbPrecinctDims map[int]map[int]map[int]map[int]cbGridDim

	// Persisted code-block states per component/resolution/precinct for tag-tree decoding
	cbStates map[string]*packetHeaderContext

	// Error resilience
	resilient bool // Enable error resilience mode (warnings instead of errors)
	strict    bool // Strict mode: fail on any error
}

type cbGridDim struct {
	numCBX int
	numCBY int
}

type packetHeaderContext struct {
	incl   *TagTreeDecoder
	zbp    *TagTreeDecoder
	states []*CodeBlockState
}

// NewPacketDecoder creates a new packet decoder
func NewPacketDecoder(data []byte, numComponents, numLayers, numResolutions int, progression ProgressionOrder, codeBlockStyle uint8) *PacketDecoder {
	return &PacketDecoder{
		data:                data,
		offset:              0,
		numComponents:       numComponents,
		numLayers:           numLayers,
		numResolutions:      numResolutions,
		progression:         progression,
		imageWidth:          0,  // Will be set later if needed
		imageHeight:         0,  // Will be set later if needed
		cbWidth:             64, // Default code-block size
		cbHeight:            64, // Default code-block size
		numLevels:           numResolutions - 1,
		codeBlockStyle:      codeBlockStyle,
		packets:             make([]Packet, 0),
		cbIncluded:          make(map[string]bool),
		compBounds:          make([]componentBounds, numComponents),
		cbPrecinctOrder:     make(map[int]map[int]map[int][]int),
		cbPrecinctPositions: make(map[int]map[int]map[int]map[int][]cbPosition),
		cbPrecinctDims:      make(map[int]map[int]map[int]map[int]cbGridDim),
		resilient:           false,
		strict:              false,
	}
}

// SetResilient enables error resilience mode (warnings instead of fatal errors)
func (pd *PacketDecoder) SetResilient(resilient bool) {
	pd.resilient = resilient
}

// SetStrict enables strict mode (fail on any error, no resilience)
func (pd *PacketDecoder) SetStrict(strict bool) {
	pd.strict = strict
	if strict {
		pd.resilient = false // Strict mode overrides resilience
	}
}

// SetImageDimensions sets the image and code-block dimensions
func (pd *PacketDecoder) SetImageDimensions(width, height, cbWidth, cbHeight int) {
	pd.imageWidth = width
	pd.imageHeight = height
	pd.cbWidth = cbWidth
	pd.cbHeight = cbHeight
	if pd.compBounds == nil {
		pd.compBounds = make([]componentBounds, pd.numComponents)
	}
	for i := range pd.compBounds {
		if pd.compBounds[i].x1 == 0 && pd.compBounds[i].y1 == 0 {
			pd.compBounds[i] = componentBounds{x0: 0, y0: 0, x1: width, y1: height}
		}
	}
}

// SetComponentBounds sets the tile-component bounds for a component.
func (pd *PacketDecoder) SetComponentBounds(component, x0, y0, x1, y1 int) {
	if component < 0 || component >= pd.numComponents {
		return
	}
	if pd.compBounds == nil {
		pd.compBounds = make([]componentBounds, pd.numComponents)
	}
	pd.compBounds[component] = componentBounds{x0: x0, y0: y0, x1: x1, y1: y1}
}

// SetComponentSampling sets the sampling factors for a component.
func (pd *PacketDecoder) SetComponentSampling(component, dx, dy int) {
	if component < 0 || component >= pd.numComponents {
		return
	}
	if pd.compDx == nil {
		pd.compDx = make([]int, pd.numComponents)
	}
	if pd.compDy == nil {
		pd.compDy = make([]int, pd.numComponents)
	}
	pd.compDx[component] = dx
	pd.compDy[component] = dy
}

func (pd *PacketDecoder) componentBoundsFor(component int) componentBounds {
	if component >= 0 && component < len(pd.compBounds) {
		b := pd.compBounds[component]
		if b.x1 != 0 || b.y1 != 0 {
			return b
		}
	}
	return componentBounds{x0: 0, y0: 0, x1: pd.imageWidth, y1: pd.imageHeight}
}

func (pd *PacketDecoder) componentSamplingFor(component int) (int, int) {
	dx := 1
	dy := 1
	if component >= 0 && component < len(pd.compDx) && pd.compDx[component] > 0 {
		dx = pd.compDx[component]
	}
	if component >= 0 && component < len(pd.compDy) && pd.compDy[component] > 0 {
		dy = pd.compDy[component]
	}
	return dx, dy
}

// SetPrecinctSize sets the precinct dimensions
func (pd *PacketDecoder) SetPrecinctSize(width, height int) {
	pd.precinctWidth = width
	pd.precinctHeight = height
	pd.precinctWidths = nil
	pd.precinctHeights = nil
}

// SetPrecinctSizes sets per-resolution precinct sizes (in pixels).
func (pd *PacketDecoder) SetPrecinctSizes(widths, heights []int) {
	pd.precinctWidths = append([]int(nil), widths...)
	pd.precinctHeights = append([]int(nil), heights...)
}

func (pd *PacketDecoder) precinctSizeForResolution(resolution int) (int, int) {
	pw := 0
	ph := 0
	if resolution >= 0 {
		if resolution < len(pd.precinctWidths) {
			pw = pd.precinctWidths[resolution]
		}
		if resolution < len(pd.precinctHeights) {
			ph = pd.precinctHeights[resolution]
		}
	}
	if pw == 0 {
		pw = pd.precinctWidth
	}
	if ph == 0 {
		ph = pd.precinctHeight
	}
	if pw == 0 {
		pw = 1 << 15
	}
	if ph == 0 {
		ph = 1 << 15
	}
	return pw, ph
}

func (pd *PacketDecoder) precinctCBDimensions(component, resolution, precinctIdx, band int) (int, int) {
	if pd.cbPrecinctDims != nil {
		if compMap, ok := pd.cbPrecinctDims[component]; ok {
			if resMap, ok := compMap[resolution]; ok {
				if bandMap, ok := resMap[precinctIdx]; ok {
					if dim, ok := bandMap[band]; ok {
						if dim.numCBX > 0 && dim.numCBY > 0 {
							return dim.numCBX, dim.numCBY
						}
					}
				}
			}
		}
	}
	return 0, 0
}

func (pd *PacketDecoder) precinctCBPositions(component, resolution, precinctIdx, band int) []cbPosition {
	if pd.cbPrecinctPositions == nil {
		return nil
	}
	if compMap, ok := pd.cbPrecinctPositions[component]; ok {
		if resMap, ok := compMap[resolution]; ok {
			if bandMap, ok := resMap[precinctIdx]; ok {
				return bandMap[band]
			}
		}
	}
	return nil
}

// DecodePackets decodes all packets according to progression order
func (pd *PacketDecoder) DecodePackets() ([]Packet, error) {
	// Packet headers use OpenJPEG-style bit stuffing handled by PacketHeaderParser.
	// Packet bodies are raw code-block data (no byte stuffing).
	pd.offset = 0

	pd.buildPrecinctOrder()

	switch pd.progression {
	case ProgressionLRCP:
		return pd.decodeComplete(pd.decodeLRCP())
	case ProgressionRLCP:
		return pd.decodeComplete(pd.decodeRLCP())
	case ProgressionRPCL:
		return pd.decodeComplete(pd.decodeRPCL())
	case ProgressionPCRL:
		return pd.decodeComplete(pd.decodePCRL())
	case ProgressionCPRL:
		return pd.decodeComplete(pd.decodeCPRL())
	default:
		return nil, fmt.Errorf("unsupported progression order: %v", pd.progression)
	}
}

// decodeComplete is a helper to log packet count when debug is on.
func (pd *PacketDecoder) decodeComplete(pkts []Packet, err error) ([]Packet, error) {
	return pkts, err
}

// decodeLRCP decodes packets in Layer-Resolution-Component-Position order
func (pd *PacketDecoder) decodeLRCP() ([]Packet, error) {
	for layer := 0; layer < pd.numLayers; layer++ {
		for res := 0; res < pd.numResolutions; res++ {
			for comp := 0; comp < pd.numComponents; comp++ {
				precincts := pd.precinctIndicesForResolution(comp, res)
				for _, precinctIdx := range precincts {
					packet, err := pd.decodePacket(layer, res, comp, precinctIdx)
					if err != nil {
						return nil, fmt.Errorf("failed to decode packet (L=%d,R=%d,C=%d,P=%d): %w",
							layer, res, comp, precinctIdx, err)
					}
					pd.packets = append(pd.packets, packet)
				}
			}
		}
	}

	return pd.packets, nil
}

// decodeRLCP decodes packets in Resolution-Layer-Component-Position order
func (pd *PacketDecoder) decodeRLCP() ([]Packet, error) {
	for res := 0; res < pd.numResolutions; res++ {
		for layer := 0; layer < pd.numLayers; layer++ {
			for comp := 0; comp < pd.numComponents; comp++ {
				precincts := pd.precinctIndicesForResolution(comp, res)
				for _, precinctIdx := range precincts {
					packet, err := pd.decodePacket(layer, res, comp, precinctIdx)
					if err != nil {
						return nil, fmt.Errorf("failed to decode packet (R=%d,L=%d,C=%d,P=%d): %w",
							res, layer, comp, precinctIdx, err)
					}
					pd.packets = append(pd.packets, packet)
				}
			}
		}
	}

	return pd.packets, nil
}

// decodeRPCL decodes packets in Resolution-Position-Component-Layer order
func (pd *PacketDecoder) decodeRPCL() ([]Packet, error) {
	posMaps := pd.buildPositionMaps()
	for res := 0; res < pd.numResolutions; res++ {
		positions := posMaps.byRes[res]
		for _, pos := range positions {
			for comp := 0; comp < pd.numComponents; comp++ {
				resMap := posMaps.byCompRes[comp][res]
				if resMap == nil {
					continue
				}
				precinctIdx, ok := resMap[pos]
				if !ok {
					continue
				}
				for layer := 0; layer < pd.numLayers; layer++ {
					packet, err := pd.decodePacket(layer, res, comp, precinctIdx)
					if err != nil {
						return nil, fmt.Errorf("failed to decode packet (R=%d,P=%d,C=%d,L=%d): %w",
							res, precinctIdx, comp, layer, err)
					}
					pd.packets = append(pd.packets, packet)
				}
			}
		}
	}
	return pd.packets, nil
}

// decodePCRL decodes packets in Position-Component-Resolution-Layer order
func (pd *PacketDecoder) decodePCRL() ([]Packet, error) {
	posMaps := pd.buildPositionMaps()
	for _, pos := range posMaps.all {
		for comp := 0; comp < pd.numComponents; comp++ {
			for res := 0; res < pd.numResolutions; res++ {
				resMap := posMaps.byCompRes[comp][res]
				if resMap == nil {
					continue
				}
				precinctIdx, ok := resMap[pos]
				if !ok {
					continue
				}
				for layer := 0; layer < pd.numLayers; layer++ {
					packet, err := pd.decodePacket(layer, res, comp, precinctIdx)
					if err != nil {
						return nil, fmt.Errorf("failed to decode packet (P=%d,C=%d,R=%d,L=%d): %w",
							precinctIdx, comp, res, layer, err)
					}
					pd.packets = append(pd.packets, packet)
				}
			}
		}
	}
	return pd.packets, nil
}

// decodeCPRL decodes packets in Component-Position-Resolution-Layer order
func (pd *PacketDecoder) decodeCPRL() ([]Packet, error) {
	posMaps := pd.buildPositionMaps()
	for comp := 0; comp < pd.numComponents; comp++ {
		positions := posMaps.byComp[comp]
		for _, pos := range positions {
			for res := 0; res < pd.numResolutions; res++ {
				resMap := posMaps.byCompRes[comp][res]
				if resMap == nil {
					continue
				}
				precinctIdx, ok := resMap[pos]
				if !ok {
					continue
				}
				for layer := 0; layer < pd.numLayers; layer++ {
					packet, err := pd.decodePacket(layer, res, comp, precinctIdx)
					if err != nil {
						return nil, fmt.Errorf("failed to decode packet (C=%d,P=%d,R=%d,L=%d): %w",
							comp, precinctIdx, res, layer, err)
					}
					pd.packets = append(pd.packets, packet)
				}
			}
		}
	}
	return pd.packets, nil
}

// decodePacket decodes a single packet
func (pd *PacketDecoder) decodePacket(layer, resolution, component, precinctIdx int) (Packet, error) {
	packet := Packet{
		LayerIndex:      layer,
		ResolutionLevel: resolution,
		ComponentIndex:  component,
		PrecinctIndex:   precinctIdx,
	}

	// Check if we've reached end of data
	if pd.offset >= len(pd.data) {
		packet.HeaderPresent = false
		return packet, nil
	}

	bands := pd.bandsForResolution(resolution)
	bandStates := make([]*packetHeaderBand, 0, len(bands))

	if pd.cbStates == nil {
		pd.cbStates = make(map[string]*packetHeaderContext)
	}

	for _, band := range bands {
		numCBX, numCBY := pd.precinctCBDimensions(component, resolution, precinctIdx, band)
		if numCBX == 0 || numCBY == 0 {
			continue
		}
		positions := pd.precinctCBPositions(component, resolution, precinctIdx, band)
		stateKey := fmt.Sprintf("%d:%d:%d:%d", component, resolution, precinctIdx, band)
		ctx := pd.cbStates[stateKey]
		if ctx == nil {
			ctx = &packetHeaderContext{}
			pd.cbStates[stateKey] = ctx
		}
		bandStates = append(bandStates, &packetHeaderBand{
			numCBX:          numCBX,
			numCBY:          numCBY,
			cbPositions:     positions,
			inclTagTree:     ctx.incl,
			zbpTagTree:      ctx.zbp,
			codeBlockStates: ctx.states,
		})
	}

	termAll := (pd.codeBlockStyle & 0x04) != 0
	header, cbIncls, bytesRead, headerPresent, err := parsePacketHeaderMulti(pd.data[pd.offset:], layer, bandStates, termAll)
	if err != nil {
		return packet, fmt.Errorf("failed to parse packet header: %w", err)
	}

	packet.HeaderPresent = headerPresent
	if !packet.HeaderPresent {
		pd.offset += bytesRead
		return packet, nil
	}

	pd.offset += bytesRead
	for i, band := range bands {
		stateKey := fmt.Sprintf("%d:%d:%d:%d", component, resolution, precinctIdx, band)
		ctx := pd.cbStates[stateKey]
		if ctx == nil || i >= len(bandStates) {
			continue
		}
		ctx.states = bandStates[i].codeBlockStates
		ctx.incl = bandStates[i].inclTagTree
		ctx.zbp = bandStates[i].zbpTagTree
	}

	packet.Header = header
	packet.CodeBlockIncls = cbIncls

	// Decode packet body (code-block contributions)
	body := &bytes.Buffer{}
	partialBuffer := false
	for i := range packet.CodeBlockIncls {
		cbIncl := &packet.CodeBlockIncls[i] // Get pointer to modify the slice element
		if cbIncl.Included && cbIncl.DataLength > 0 {
			// Read code-block data (raw bytes)
			if cbIncl.DataLength == 0 {
				continue
			}
			if pd.offset >= len(pd.data) {
				partialBuffer = true
				break
			}

			// Segment length validation with overflow checks (OpenJPEG alignment)
			// Check for integer overflow: offset + length < offset
			if pd.offset+cbIncl.DataLength < pd.offset {
				if pd.strict {
					return packet, fmt.Errorf("segment length overflow detected for codeblock %d", i)
				}
				if pd.resilient {
					// Mark this and remaining blocks as corrupted
					partialBuffer = true
					break
				}
				return packet, fmt.Errorf("segment length overflow detected for codeblock %d", i)
			}

			// Check bounds: offset + length > total data length
			if pd.offset+cbIncl.DataLength > len(pd.data) {
				if pd.strict {
					return packet, fmt.Errorf("segment length exceeds available data for codeblock %d: offset=%d, length=%d, total=%d",
						i, pd.offset, cbIncl.DataLength, len(pd.data))
				}
				if pd.resilient {
					// Truncate to remaining space and mark partial
					cbIncl.DataLength = len(pd.data) - pd.offset
					partialBuffer = true
				} else {
					// Default: truncate gracefully
					cbIncl.DataLength = len(pd.data) - pd.offset
				}
			}

			// JPWL error handling: segment length limit
			// JPEG2000 Part 1 allows up to 64KB per codeblock (16-bit length field)
			// Increased from 8192 to support HTJ2K which produces larger codeblocks
			const maxSegmentLength = 65535
			if cbIncl.DataLength > maxSegmentLength {
				if pd.strict {
					return packet, fmt.Errorf("segment length exceeds limit for codeblock %d: %d > %d",
						i, cbIncl.DataLength, maxSegmentLength)
				}
				// Truncate to limit (OpenJPEG behavior)
				remainingSpace := len(pd.data) - pd.offset
				if remainingSpace < maxSegmentLength {
					cbIncl.DataLength = remainingSpace
				} else {
					cbIncl.DataLength = maxSegmentLength
				}
			}

			cbData := pd.data[pd.offset : pd.offset+cbIncl.DataLength]
			cbIncl.Data = cbData
			body.Write(cbData)
			pd.offset += cbIncl.DataLength

			// Mark if we detected partial buffer condition
			if partialBuffer {
				cbIncl.Corrupted = true
			}
		}
	}
	packet.Body = body.Bytes()
	packet.PartialBuffer = partialBuffer

	return packet, nil
}

// GetPackets returns the decoded packets
func (pd *PacketDecoder) GetPackets() []Packet {
	return pd.packets
}

// cbEntry represents a code-block entry for precinct ordering
type cbEntry struct {
	cbx    int
	cby    int
	global int
}

// buildPrecinctOrder builds the mapping from precinct index to code-block order
// for each resolution, mirroring the encoder traversal.
func (pd *PacketDecoder) buildPrecinctOrder() {
	if len(pd.cbPrecinctOrder) > 0 {
		return
	}

	pd.initializePrecinctMaps()

	cbw, cbh := pd.cbWidth, pd.cbHeight
	for comp := 0; comp < pd.numComponents; comp++ {
		pd.initializeComponentMaps(comp)
		pd.buildComponentPrecinctOrder(comp, cbw, cbh)
	}
}

func (pd *PacketDecoder) initializePrecinctMaps() {
	if pd.cbPrecinctPositions == nil {
		pd.cbPrecinctPositions = make(map[int]map[int]map[int]map[int][]cbPosition)
	}
	if pd.cbPrecinctDims == nil {
		pd.cbPrecinctDims = make(map[int]map[int]map[int]map[int]cbGridDim)
	}
}

func (pd *PacketDecoder) initializeComponentMaps(comp int) {
	if pd.cbPrecinctOrder[comp] == nil {
		pd.cbPrecinctOrder[comp] = make(map[int]map[int][]int)
	}
	if pd.cbPrecinctPositions[comp] == nil {
		pd.cbPrecinctPositions[comp] = make(map[int]map[int]map[int][]cbPosition)
	}
	if pd.cbPrecinctDims[comp] == nil {
		pd.cbPrecinctDims[comp] = make(map[int]map[int]map[int]cbGridDim)
	}
}

func (pd *PacketDecoder) buildComponentPrecinctOrder(comp, cbw, cbh int) {
	b := pd.componentBoundsFor(comp)
	compWidth := b.x1 - b.x0
	compHeight := b.y1 - b.y0
	if compWidth <= 0 || compHeight <= 0 {
		return
	}

	globalCBIdx := 0
	for res := 0; res < pd.numResolutions; res++ {
		globalCBIdx = pd.buildResolutionPrecinctOrder(comp, res, b, compWidth, compHeight, cbw, cbh, globalCBIdx)
	}
}

func (pd *PacketDecoder) buildResolutionPrecinctOrder(comp, res int, b componentBounds, compWidth, compHeight, cbw, cbh, globalCBIdx int) int {
	pw, ph := pd.precinctSizeForResolution(res)
	if pd.cbPrecinctOrder[comp][res] == nil {
		pd.cbPrecinctOrder[comp][res] = make(map[int][]int)
	}

	precinctBands := make(map[int]map[int][]cbEntry)

	resW, resH, resX0, resY0, bands := bandInfosForResolution(compWidth, compHeight, b.x0, b.y0, pd.numLevels, res)
	if resW <= 0 || resH <= 0 {
		return globalCBIdx
	}

	startX := floorDiv(resX0, pw) * pw
	startY := floorDiv(resY0, ph) * ph
	endX := ceilDiv(resX0+resW, pw) * pw
	numPrecinctX := (endX - startX) / pw
	if numPrecinctX < 1 {
		numPrecinctX = 1
	}

	globalCBIdx = pd.collectCodeBlockEntries(bands, cbw, cbh, resX0, resY0, startX, startY, pw, ph, numPrecinctX, precinctBands, globalCBIdx)

	bandOrder := pd.getBandOrder(res)
	pd.storePrecinctBands(comp, res, precinctBands, bandOrder)

	return globalCBIdx
}

func (pd *PacketDecoder) collectCodeBlockEntries(bands []bandInfo, cbw, cbh, resX0, resY0, startX, startY, pw, ph, numPrecinctX int, precinctBands map[int]map[int][]cbEntry, globalCBIdx int) int {
	for _, bandInfo := range bands {
		if bandInfo.width <= 0 || bandInfo.height <= 0 {
			continue
		}
		numCBX := (bandInfo.width + cbw - 1) / cbw
		numCBY := (bandInfo.height + cbh - 1) / cbh
		for cby := 0; cby < numCBY; cby++ {
			for cbx := 0; cbx < numCBX; cbx++ {
				cbX0 := cbx * cbw
				cbY0 := cby * cbh
				absResX0 := resX0 + cbX0
				absResY0 := resY0 + cbY0
				px := (absResX0 - startX) / pw
				py := (absResY0 - startY) / ph
				pIdx := py*numPrecinctX + px
				localX := absResX0 - (startX + px*pw)
				localY := absResY0 - (startY + py*ph)
				cbxLocal := localX / cbw
				cbyLocal := localY / cbh

				if precinctBands[pIdx] == nil {
					precinctBands[pIdx] = make(map[int][]cbEntry)
				}
				precinctBands[pIdx][bandInfo.band] = append(precinctBands[pIdx][bandInfo.band], cbEntry{cbx: cbxLocal, cby: cbyLocal, global: globalCBIdx})
				globalCBIdx++
			}
		}
	}
	return globalCBIdx
}

func (pd *PacketDecoder) getBandOrder(res int) []int {
	if res == 0 {
		return []int{0}
	}
	return []int{1, 2, 3}
}

func (pd *PacketDecoder) storePrecinctBands(comp, res int, precinctBands map[int]map[int][]cbEntry, bandOrder []int) {
	for pIdx, bandMap := range precinctBands {
		for _, band := range bandOrder {
			entries := bandMap[band]
			if len(entries) == 0 {
				continue
			}
			pd.sortAndStoreEntries(comp, res, pIdx, band, entries)
		}
	}
}

func (pd *PacketDecoder) sortAndStoreEntries(comp, res, pIdx, band int, entries []cbEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].cby != entries[j].cby {
			return entries[i].cby < entries[j].cby
		}
		return entries[i].cbx < entries[j].cbx
	})

	pd.ensurePrecinctMapsInitialized(comp, res, pIdx)

	positions := make([]cbPosition, 0, len(entries))
	maxX, maxY := 0, 0
	for _, entry := range entries {
		pd.cbPrecinctOrder[comp][res][pIdx] = append(pd.cbPrecinctOrder[comp][res][pIdx], entry.global)
		positions = append(positions, cbPosition{X: entry.cbx, Y: entry.cby})
		if entry.cbx+1 > maxX {
			maxX = entry.cbx + 1
		}
		if entry.cby+1 > maxY {
			maxY = entry.cby + 1
		}
	}
	pd.cbPrecinctPositions[comp][res][pIdx][band] = positions
	pd.cbPrecinctDims[comp][res][pIdx][band] = cbGridDim{numCBX: maxX, numCBY: maxY}
}

func (pd *PacketDecoder) ensurePrecinctMapsInitialized(comp, res, pIdx int) {
	if pd.cbPrecinctPositions[comp][res] == nil {
		pd.cbPrecinctPositions[comp][res] = make(map[int]map[int][]cbPosition)
	}
	if pd.cbPrecinctPositions[comp][res][pIdx] == nil {
		pd.cbPrecinctPositions[comp][res][pIdx] = make(map[int][]cbPosition)
	}
	if pd.cbPrecinctDims[comp][res] == nil {
		pd.cbPrecinctDims[comp][res] = make(map[int]map[int]cbGridDim)
	}
	if pd.cbPrecinctDims[comp][res][pIdx] == nil {
		pd.cbPrecinctDims[comp][res][pIdx] = make(map[int]cbGridDim)
	}
}

func (pd *PacketDecoder) bandsForResolution(resolution int) []int {
	if resolution == 0 {
		return []int{0}
	}
	return []int{1, 2, 3}
}

// precinctIndicesForResolution returns sorted precinct indices that contain code-blocks for a resolution.
func (pd *PacketDecoder) precinctIndicesForResolution(component, resolution int) []int {
	byComp, ok := pd.cbPrecinctOrder[component]
	if !ok {
		return nil
	}
	order := byComp[resolution]
	indices := make([]int, 0, len(order))
	for idx := range order {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	return indices
}

func (pd *PacketDecoder) buildPositionMaps() *positionMaps {
	return buildPositionMaps(positionInputs{
		numComponents:     pd.numComponents,
		numResolutions:    pd.numResolutions,
		precinctIndices:   pd.precinctIndicesForResolution,
		componentBounds:   pd.componentBoundsFor,
		componentSampling: pd.componentSamplingFor,
		precinctSize:      pd.precinctSizeForResolution,
	})
}
