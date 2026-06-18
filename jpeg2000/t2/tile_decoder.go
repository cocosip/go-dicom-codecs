package t2

import (
	"fmt"
	"math"
	"sort"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
)

// BlockDecoder is an interface for T1 block decoders (EBCOT or HTJ2K)
type BlockDecoder interface {
	// DecodeWithBitplane decodes a code-block with known max bitplane
	DecodeWithBitplane(data []byte, numPasses int, maxBitplane int, roishift int) error
	// DecodeLayered decodes a code-block with TERMALL mode
	DecodeLayered(data []byte, passLengths []int, maxBitplane int, roishift int) error
	// GetData returns the decoded coefficients
	GetData() []int32
}

// BlockDecoderFactory creates block decoders for a specific code-block size
type BlockDecoderFactory func(width, height int, cblkstyle int) BlockDecoder

// TileDecoder decodes a single JPEG 2000 tile
type TileDecoder struct {
	// Tile information
	tile   *codestream.Tile
	siz    *codestream.SIZSegment
	cod    *codestream.CODSegment
	qcd    *codestream.QCDSegment
	tileX0 int
	tileY0 int
	tileX1 int
	tileY1 int

	// Component decoders
	components []*ComponentDecoder

	// Output
	decodedData [][]int32 // [component][pixel]

	// ROI
	roi *ROIInfo

	// HTJ2K support
	isHTJ2K             bool
	blockDecoderFactory BlockDecoderFactory

	// Error resilience
	resilient bool // Enable error resilience mode
	strict    bool // Strict mode: fail on any error
}

// ComponentDecoder decodes a single component within a tile
type ComponentDecoder struct {
	componentIdx int
	x0           int
	y0           int
	width        int
	height       int
	numLevels    int // Number of DWT levels

	// Resolution levels
	resolutions []*ResolutionLevel

	// Code-blocks for this component
	codeBlocks []*CodeBlockDecoder

	// Decoded coefficients (after EBCOT, before IDWT)
	coefficients []int32

	// Final decoded samples (after IDWT)
	samples []int32
}

// ResolutionLevel represents one resolution level of a component
// Fields reserved for future complete implementation
type ResolutionLevel struct {
	_ int               // level (reserved)
	_ int               // width (reserved)
	_ int               // height (reserved)
	_ []*SubbandDecoder // subbands (reserved)
}

// SubbandDecoder decodes a single subband
// Fields reserved for future complete implementation
type SubbandDecoder struct {
	_ codestream.SubbandType // subbandType (reserved)
	_ int                    // width (reserved)
	_ int                    // height (reserved)
	_ []*CodeBlockDecoder    // codeBlocks (reserved)
	_ []int32                // coeffs (reserved)
}

// CodeBlockDecoder decodes a single code-block
type CodeBlockDecoder struct {
	x0, y0    int
	x1, y1    int
	band      int
	data      []byte // Compressed data
	numPasses int
	t1Decoder BlockDecoder // Can be EBCOT T1 or HTJ2K decoder
	coeffs    []int32      // Decoded coefficients
}

// cbInfo holds per-code-block accumulation state from packets.
// key in maps uses "resolution:globalCBIdx".
type cbInfo struct {
	data             []byte
	maxBitplane      int
	maxBitplaneSet   bool
	zeroBitplanes    int
	zeroBitplanesSet bool
	resolution       int
	cbIdx            int
	passLengths      []int
	useTERMALL       bool
	totalPasses      int
}

// ROIInfo represents Region-of-Interest configuration for decoding and ROI scaling.
type ROIInfo struct {
	Rects            []ROIRect   // legacy/global
	RectsByComponent [][]ROIRect // per-component rectangles
	Shifts           []int       // per component shift (MaxShift or GeneralScaling)
	Styles           []byte      // per component Srgn style (0=MaxShift, 1=GeneralScaling)
	Masks            []*ROIMask  // per-component mask
}

// ROIRect is an axis-aligned rectangle.
type ROIRect struct {
	X0, Y0 int
	X1, Y1 int
}

// Intersects reports whether the ROI rectangle intersects the region [x0,y0)-[x1,y1).
func (r ROIRect) Intersects(x0, y0, x1, y1 int) bool {
	return r.X0 < x1 && x0 < r.X1 && r.Y0 < y1 && y0 < r.Y1
}

// ROIMask wraps a boolean mask.
type ROIMask struct {
	Width  int
	Height int
	Data   []bool
}

func (r *ROIInfo) intersects(compIdx, x0, y0, x1, y1 int) bool {
	if r == nil {
		return false
	}

	// Prefer mask if available
	if compIdx >= 0 && compIdx < len(r.Masks) && r.Masks[compIdx] != nil {
		m := r.Masks[compIdx]
		// sample mask region; if any pixel inside ROI
		if m.Width > 0 && m.Height > 0 && len(m.Data) == m.Width*m.Height {
			clamp := func(v, maxVal int) int {
				if v < 0 {
					return 0
				}
				if v > maxVal {
					return maxVal
				}
				return v
			}
			x0c := clamp(x0, m.Width)
			y0c := clamp(y0, m.Height)
			x1c := clamp(x1, m.Width)
			y1c := clamp(y1, m.Height)
			for yy := y0c; yy < y1c; yy++ {
				row := yy * m.Width
				for xx := x0c; xx < x1c; xx++ {
					if m.Data[row+xx] {
						return true
					}
				}
			}
		}
	}

	if compIdx >= 0 && compIdx < len(r.RectsByComponent) && len(r.RectsByComponent[compIdx]) > 0 {
		for _, rect := range r.RectsByComponent[compIdx] {
			if rect.Intersects(x0, y0, x1, y1) {
				return true
			}
		}
		return false
	}

	for _, rect := range r.Rects {
		if rect.Intersects(x0, y0, x1, y1) {
			return true
		}
	}
	return false
}

func (r *ROIInfo) context(compIdx, x0, y0, x1, y1 int) (int, byte, bool) {
	if r == nil || compIdx < 0 || compIdx >= len(r.Shifts) {
		return 0, 0, false
	}
	shift := r.Shifts[compIdx]
	style := byte(0)
	if compIdx < len(r.Styles) {
		style = r.Styles[compIdx]
	}
	if shift <= 0 {
		return 0, style, false
	}
	inside := false
	if compIdx >= 0 && compIdx < len(r.Masks) && r.Masks[compIdx] != nil {
		blockMask := r.blockMask(compIdx, x0, y0, x1, y1)
		if len(blockMask) > 0 && len(blockMask[0]) > 0 {
			inside = maskAnyTrue(blockMask)
		}
	}
	if !inside {
		inside = r.intersects(compIdx, x0, y0, x1, y1)
	}
	return shift, style, inside
}

// blockMask extracts a boolean mask for the given block region.
func (r *ROIInfo) blockMask(compIdx, x0, y0, x1, y1 int) [][]bool {
	if r == nil || compIdx < 0 || compIdx >= len(r.Masks) || r.Masks[compIdx] == nil {
		return nil
	}
	m := r.Masks[compIdx]
	if m.Width <= 0 || m.Height <= 0 || len(m.Data) != m.Width*m.Height {
		return nil
	}
	w := x1 - x0
	h := y1 - y0
	if w <= 0 || h <= 0 {
		return nil
	}
	out := make([][]bool, h)
	for j := 0; j < h; j++ {
		out[j] = make([]bool, w)
		srcY := y0 + j
		if srcY < 0 || srcY >= m.Height {
			continue
		}
		rowOffset := srcY * m.Width
		for i := 0; i < w; i++ {
			srcX := x0 + i
			if srcX < 0 || srcX >= m.Width {
				continue
			}
			out[j][i] = m.Data[rowOffset+srcX]
		}
	}
	return out
}

// NewTileDecoder creates a new tile decoder
func NewTileDecoder(
	tile *codestream.Tile,
	siz *codestream.SIZSegment,
	cod *codestream.CODSegment,
	qcd *codestream.QCDSegment,
	roi *ROIInfo,
	isHTJ2K bool,
	blockDecoderFactory BlockDecoderFactory,
) *TileDecoder {
	numTilesX := int((siz.Xsiz - siz.XTOsiz + siz.XTsiz - 1) / siz.XTsiz)
	if numTilesX <= 0 {
		numTilesX = 1
	}
	tileX := tile.Index % numTilesX
	tileY := tile.Index / numTilesX
	tileGridX0 := int(siz.XTOsiz) + tileX*int(siz.XTsiz)
	tileGridY0 := int(siz.YTOsiz) + tileY*int(siz.YTsiz)
	tileGridX1 := tileGridX0 + int(siz.XTsiz)
	tileGridY1 := tileGridY0 + int(siz.YTsiz)
	tileX0 := tileGridX0
	tileY0 := tileGridY0
	if tileX0 < int(siz.XOsiz) {
		tileX0 = int(siz.XOsiz)
	}
	if tileY0 < int(siz.YOsiz) {
		tileY0 = int(siz.YOsiz)
	}
	tileX1 := tileGridX1
	tileY1 := tileGridY1
	if tileX1 > int(siz.Xsiz) {
		tileX1 = int(siz.Xsiz)
	}
	if tileY1 > int(siz.Ysiz) {
		tileY1 = int(siz.Ysiz)
	}

	td := &TileDecoder{
		tile:                tile,
		siz:                 siz,
		cod:                 cod,
		qcd:                 qcd,
		roi:                 roi,
		isHTJ2K:             isHTJ2K,
		blockDecoderFactory: blockDecoderFactory,
		tileX0:              tileX0,
		tileY0:              tileY0,
		tileX1:              tileX1,
		tileY1:              tileY1,
		resilient:           false,
		strict:              false,
	}

	return td
}

// SetResilient enables error resilience mode (warnings instead of fatal errors)
func (td *TileDecoder) SetResilient(resilient bool) {
	td.resilient = resilient
}

// SetStrict enables strict mode (fail on any error, no resilience)
func (td *TileDecoder) SetStrict(strict bool) {
	td.strict = strict
	if strict {
		td.resilient = false // Strict mode overrides resilience
	}
}

// Decode decodes the tile and returns the pixel data for each component
func (td *TileDecoder) Decode() ([][]int32, error) {
	// Initialize component decoders
	numComponents := int(td.siz.Csiz)
	td.components = make([]*ComponentDecoder, numComponents)
	td.decodedData = make([][]int32, numComponents)

	for i := 0; i < numComponents; i++ {
		dx := int(td.siz.Components[i].XRsiz)
		dy := int(td.siz.Components[i].YRsiz)
		if dx <= 0 {
			dx = 1
		}
		if dy <= 0 {
			dy = 1
		}
		compX0 := ceilDiv(td.tileX0, dx)
		compY0 := ceilDiv(td.tileY0, dy)
		compX1 := ceilDiv(td.tileX1, dx)
		compY1 := ceilDiv(td.tileY1, dy)
		compWidth := compX1 - compX0
		compHeight := compY1 - compY0
		if compWidth < 0 {
			compWidth = 0
		}
		if compHeight < 0 {
			compHeight = 0
		}
		comp := &ComponentDecoder{
			componentIdx: i,
			x0:           compX0,
			y0:           compY0,
			width:        compWidth,
			height:       compHeight,
			numLevels:    int(td.cod.NumberOfDecompositionLevels),
		}

		td.components[i] = comp
	}

	// Parse packets ONCE for all components
	packetDec := NewPacketDecoder(
		td.tile.Data,
		int(td.siz.Csiz),
		int(td.cod.NumberOfLayers),
		int(td.cod.NumberOfDecompositionLevels)+1, // numResolutions = numLevels + 1
		ProgressionOrder(td.cod.ProgressionOrder),
		td.cod.CodeBlockStyle,
	)

	// Propagate error resilience settings
	packetDec.SetResilient(td.resilient)
	packetDec.SetStrict(td.strict)

	// Set image dimensions and code-block size
	cbWidth, cbHeight := td.cod.CodeBlockSize()
	if numComponents > 0 {
		packetDec.SetImageDimensions(td.components[0].width, td.components[0].height, cbWidth, cbHeight)
	} else {
		packetDec.SetImageDimensions(int(td.siz.Xsiz), int(td.siz.Ysiz), cbWidth, cbHeight)
	}
	for i := 0; i < numComponents; i++ {
		comp := td.components[i]
		packetDec.SetComponentBounds(i, comp.x0, comp.y0, comp.x0+comp.width, comp.y0+comp.height)
		packetDec.SetComponentSampling(i, int(td.siz.Components[i].XRsiz), int(td.siz.Components[i].YRsiz))
	}

	// Set precinct sizes if defined in COD segment
	if len(td.cod.PrecinctSizes) > 0 {
		widths := make([]int, len(td.cod.PrecinctSizes))
		heights := make([]int, len(td.cod.PrecinctSizes))
		for i, ps := range td.cod.PrecinctSizes {
			widths[i] = 1 << ps.PPx
			heights[i] = 1 << ps.PPy
		}
		packetDec.SetPrecinctSizes(widths, heights)
	}

	packets, err := packetDec.DecodePackets()
	if err != nil {
		return nil, fmt.Errorf("failed to decode packets: %w", err)
	}

	td.decodeAllCodeBlocks(packets)

	// Process each component
	for i := 0; i < numComponents; i++ {
		comp := td.components[i]

		// Assemble subbands
		td.assembleSubbands(comp)

		// Apply IDWT
		if err := td.applyIDWT(comp); err != nil {
			return nil, fmt.Errorf("IDWT failed for component %d: %w", i, err)
		}

		// Level shift - DISABLED: DC shift should be applied at codec level (decoder.go), not here
		// to match OpenJPEG pipeline: T1^-1 -> DWT^-1 -> MCT^-1 -> DC shift^-1
		// td.levelShift(comp)

		td.decodedData[i] = comp.samples
	}

	return td.decodedData, nil
}

// decodeAllCodeBlocks decodes code-blocks for all components from packets
// decodeAllCodeBlocks decodes code-blocks for all components from packets.
// params: packets - parsed packet list for the tile
// returns: error - non-nil on any decoding failure
func (td *TileDecoder) decodeAllCodeBlocks(packets []Packet) {
	cbWidth, cbHeight := td.cod.CodeBlockSize()
	for _, comp := range td.components {
		precinctOrder := td.buildPrecinctOrder(comp, cbWidth, cbHeight)
		cbDataMap := td.gatherCBData(comp, precinctOrder, packets)
		codeBlocks := td.buildAndDecodeCodeBlocks(comp, cbWidth, cbHeight, cbDataMap)
		comp.resolutions = make([]*ResolutionLevel, comp.numLevels+1)
		comp.codeBlocks = codeBlocks
	}
}

// gatherCBData accumulates per-code-block data across all packets for a component.
// params: comp - component decoder, precinctOrder - mapping from resolution/precinct to global CB indices, packets - tile packets
// returns: map key "res:cbIdx" -> cbInfo with data, passes and flags
func (td *TileDecoder) gatherCBData(comp *ComponentDecoder, precinctOrder map[int]map[int][]int, packets []Packet) map[string]cbInfo {
	cbDataMap := make(map[string]cbInfo)
	for i := range packets {
		packet := &packets[i]
		if packet.ComponentIndex != comp.componentIdx {
			continue
		}
		resOrder := precinctOrder[packet.ResolutionLevel]
		if resOrder == nil {
			continue
		}
		cbOrder := resOrder[packet.PrecinctIndex]
		if cbOrder == nil {
			continue
		}
		dataOffset := 0
		for cbIdx, cbIncl := range packet.CodeBlockIncls {
			if !cbIncl.Included {
				continue
			}
			if cbIdx >= len(cbOrder) {
				dataOffset += cbIncl.DataLength
				continue
			}
			actualCBIdx := cbOrder[cbIdx]
			var cbData []byte
			if cbIncl.DataLength > 0 && dataOffset+cbIncl.DataLength <= len(packet.Body) {
				cbData = make([]byte, cbIncl.DataLength)
				copy(cbData, packet.Body[dataOffset:dataOffset+cbIncl.DataLength])
			} else {
				cbData = []byte{}
			}
			key := fmt.Sprintf("%d:%d", packet.ResolutionLevel, actualCBIdx)
			existing := cbDataMap[key]
			if len(existing.data) > 0 {
				existing.data = append(existing.data, cbData...)
			} else {
				existing.data = cbData
				existing.resolution = packet.ResolutionLevel
				existing.cbIdx = cbIdx
			}
			dataOffset += cbIncl.DataLength
			if len(cbIncl.PassLengths) > 0 {
				if existing.passLengths == nil {
					existing.passLengths = make([]int, len(cbIncl.PassLengths))
					total := 0
					for i, pl := range cbIncl.PassLengths {
						total += pl
						existing.passLengths[i] = total
					}
				} else {
					total := existing.passLengths[len(existing.passLengths)-1]
					for _, pl := range cbIncl.PassLengths {
						total += pl
						existing.passLengths = append(existing.passLengths, total)
					}
				}
			}
			if cbIncl.UseTERMALL {
				existing.useTERMALL = true
			}
			existing.totalPasses += cbIncl.NumPasses
			if !existing.zeroBitplanesSet && cbIncl.ZeroBitplanes >= 0 {
				existing.zeroBitplanes = cbIncl.ZeroBitplanes
				existing.zeroBitplanesSet = true
			}
			cbDataMap[key] = existing
		}
	}
	return cbDataMap
}

// buildAndDecodeCodeBlocks creates CodeBlockDecoders for all positions and performs decoding.
// params: comp - component, cbWidth/cbHeight - code-block dims, cbDataMap - accumulated per-block info
// returns: slice of decoded CodeBlockDecoder
func (td *TileDecoder) buildAndDecodeCodeBlocks(comp *ComponentDecoder, cbWidth, cbHeight int, cbDataMap map[string]cbInfo) []*CodeBlockDecoder {
	codeBlocks := make([]*CodeBlockDecoder, 0)
	globalCBIdx := 0
	for res := 0; res <= comp.numLevels; res++ {
		_, _, _, _, bands := bandInfosForResolution(comp.width, comp.height, comp.x0, comp.y0, comp.numLevels, res)
		for _, bandInfo := range bands {
			if bandInfo.width <= 0 || bandInfo.height <= 0 {
				continue
			}
			numCBX := (bandInfo.width + cbWidth - 1) / cbWidth
			numCBY := (bandInfo.height + cbHeight - 1) / cbHeight
			band := bandInfo.band
			for cby := 0; cby < numCBY; cby++ {
				for cbx := 0; cbx < numCBX; cbx++ {
					cbIdx := globalCBIdx
					globalCBIdx++
					key := fmt.Sprintf("%d:%d", res, cbIdx)
					info, exists := cbDataMap[key]
					if !exists {
						info = cbInfo{data: []byte{}, resolution: res, cbIdx: cbIdx, maxBitplane: -1, maxBitplaneSet: true}
					}
					if !info.maxBitplaneSet {
						info.maxBitplane = td.estimateMaxBitplane(comp, res, band, info)
						info.maxBitplaneSet = true
					}
					localX0 := cbx * cbWidth
					localY0 := cby * cbHeight
					localX1 := localX0 + cbWidth
					localY1 := localY0 + cbHeight
					if localX1 > bandInfo.width {
						localX1 = bandInfo.width
					}
					if localY1 > bandInfo.height {
						localY1 = bandInfo.height
					}
					x0 := bandInfo.offsetX + localX0
					y0 := bandInfo.offsetY + localY0
					x1 := bandInfo.offsetX + localX1
					y1 := bandInfo.offsetY + localY1
					actualWidth := x1 - x0
					actualHeight := y1 - y0
					if actualWidth <= 0 || actualHeight <= 0 {
						continue
					}
					numPasses := info.totalPasses
					if numPasses == 0 {
						cblkNumbps := info.maxBitplane
						if cblkNumbps > 0 {
							numPasses = (cblkNumbps * 3) - 2
						} else if info.maxBitplane >= 0 {
							numPasses = 1
						}
					}
					cbd := &CodeBlockDecoder{
						x0:        x0,
						y0:        y0,
						x1:        x1,
						y1:        y1,
						band:      band,
						data:      info.data,
						numPasses: numPasses,
						t1Decoder: func() BlockDecoder {
							if td.isHTJ2K && td.blockDecoderFactory != nil {
								return td.blockDecoderFactory(actualWidth, actualHeight, int(td.cod.CodeBlockStyle))
							}
							dec := t1.NewT1Decoder(actualWidth, actualHeight, int(td.cod.CodeBlockStyle))
							dec.SetOpenJPEGReconstruction(true)
							return dec
						}(),
					}
					if orientSetter, ok := cbd.t1Decoder.(interface{ SetOrientation(int) }); ok {
						orientSetter.SetOrientation(band)
					}
					if td.isHTJ2K {
						if contextSetter, ok := cbd.t1Decoder.(interface {
							SetCodingContext(bandNumbps int, zeroBitplanes int)
						}); ok {
							if bandNumbps, ok := bandNumbpsFromQCD(td.qcd, comp.numLevels, res, band); ok {
								contextSetter.SetCodingContext(bandNumbps, info.zeroBitplanes)
							}
						}
					}
					if td.shouldDecode(info) {
						td.decodeCodeBlock(comp, cbd, info, actualWidth, actualHeight)
					} else {
						cbd.coeffs = make([]int32, actualWidth*actualHeight)
					}
					codeBlocks = append(codeBlocks, cbd)
				}
			}
		}
	}
	return codeBlocks
}

// estimateMaxBitplane computes max bitplane from QCD or pass counts.
// params: comp - component, res/band - resolution/subband, info - cbInfo
// returns: max bitplane value
func (td *TileDecoder) estimateMaxBitplane(comp *ComponentDecoder, res, band int, info cbInfo) int {
	zbp := info.zeroBitplanes
	if !info.zeroBitplanesSet {
		zbp = 0
	}
	var maxBP int
	maxFromPass := -1
	if info.totalPasses > 0 {
		cblkNumbps := (info.totalPasses + 2) / 3
		if cblkNumbps <= 0 {
			maxFromPass = -1
		} else {
			maxFromPass = cblkNumbps
		}
	}
	maxFromQCD := -1
	if bandNumbps, ok := bandNumbpsFromQCD(td.qcd, comp.numLevels, res, band); ok && bandNumbps > 0 {
		cblkNumbps := bandNumbps - zbp
		if cblkNumbps > 0 {
			maxFromQCD = cblkNumbps
		}
	}
	if maxFromPass >= 0 && maxFromQCD >= 0 {
		if maxFromPass > maxFromQCD {
			maxBP = maxFromPass
		} else {
			maxBP = maxFromQCD
		}
	} else if maxFromQCD >= 0 {
		maxBP = maxFromQCD
	} else if maxFromPass >= 0 {
		maxBP = maxFromPass
	} else {
		componentBitDepth := int(td.siz.Components[comp.componentIdx].Ssiz&0x7F) + 1
		effectiveBitDepth := componentBitDepth + comp.numLevels
		maxBP = effectiveBitDepth - zbp
	}
	if maxBP < -1 {
		maxBP = -1
	}
	return maxBP
}

// shouldDecode decides whether a block should be decoded based on data/lengths/maxBP.
// params: info - cbInfo
// returns: true if decoding is needed
func (td *TileDecoder) shouldDecode(info cbInfo) bool {
	if len(info.data) == 0 || info.maxBitplane < 0 {
		return false
	}
	if len(info.passLengths) > 0 {
		allZero := true
		for _, pl := range info.passLengths {
			if pl > 0 {
				allZero = false
				break
			}
		}
		if allZero {
			return false
		}
	}
	return true
}

// decodeCodeBlock performs layered or bitplane decoding and applies ROI and normalization.
// params: comp - component, cbd - code-block decoder, info - cbInfo, actualWidth/Height - block dims
// returns: none (cbd.coeffs populated)
func (td *TileDecoder) decodeCodeBlock(comp *ComponentDecoder, cbd *CodeBlockDecoder, info cbInfo, actualWidth, actualHeight int) {
	var err error
	if len(info.passLengths) > 0 {
		if t1Dec, ok := cbd.t1Decoder.(interface {
			DecodeLayeredWithMode(data []byte, passLengths []int, maxBitplane int, roishift int, useTERMALL bool, resetContexts bool) error
		}); ok {
			resetContexts := (td.cod.CodeBlockStyle & 0x02) != 0
			err = t1Dec.DecodeLayeredWithMode(info.data, info.passLengths, info.maxBitplane, 0, info.useTERMALL, resetContexts)
		} else {
			err = cbd.t1Decoder.DecodeLayered(info.data, info.passLengths, info.maxBitplane, 0)
		}
	} else {
		err = cbd.t1Decoder.DecodeWithBitplane(info.data, cbd.numPasses, info.maxBitplane, 0)
	}
	if err != nil {
		cbd.coeffs = make([]int32, actualWidth*actualHeight)
		return
	}
	cbd.coeffs = cbd.t1Decoder.GetData()
	var shiftVal int
	var style byte
	var inside bool
	if td.roi != nil {
		shiftVal, style, inside = td.roi.context(comp.componentIdx, cbd.x0, cbd.y0, cbd.x1, cbd.y1)
		if style == 0 && shiftVal > 0 {
			applyInverseMaxShift(cbd.coeffs, shiftVal)
		}
	}
	if !td.isHTJ2K && td.cod.Transformation == 1 {
		normalizeOpenJPEGReversibleT1Coefficients(cbd.coeffs)
	}
	if td.roi != nil && style == 1 && shiftVal > 0 && inside {
		blockMask := td.roi.blockMask(comp.componentIdx, cbd.x0, cbd.y0, cbd.x1, cbd.y1)
		if len(blockMask) > 0 && len(blockMask[0]) > 0 {
			applyInverseGeneralScalingMasked(cbd.coeffs, blockMask, shiftVal)
		} else {
			applyInverseGeneralScaling(cbd.coeffs, shiftVal)
		}
	}
}

// buildPrecinctOrder returns the mapping of precinct index -> ordered list of global code-block indices for each resolution.
func (td *TileDecoder) buildPrecinctOrder(comp *ComponentDecoder, cbWidth, cbHeight int) map[int]map[int][]int {
	order := make(map[int]map[int][]int)
	globalCBIdx := 0

	for res := 0; res <= comp.numLevels; res++ {
		pw, ph := td.precinctSizeForResolution(res)
		order[res] = make(map[int][]int)
		type cbEntry struct {
			cbx    int
			cby    int
			global int
		}
		precinctBands := make(map[int]map[int][]cbEntry)
		addEntry := func(pIdx, band, cbxLocal, cbyLocal, global int) {
			if precinctBands[pIdx] == nil {
				precinctBands[pIdx] = make(map[int][]cbEntry)
			}
			precinctBands[pIdx][band] = append(precinctBands[pIdx][band], cbEntry{cbx: cbxLocal, cby: cbyLocal, global: global})
		}

		resW, resH, resX0, resY0, bands := bandInfosForResolution(comp.width, comp.height, comp.x0, comp.y0, comp.numLevels, res)
		if resW <= 0 || resH <= 0 {
			continue
		}
		startX := floorDiv(resX0, pw) * pw
		startY := floorDiv(resY0, ph) * ph
		endX := ceilDiv(resX0+resW, pw) * pw
		numPrecinctX := (endX - startX) / pw
		if numPrecinctX < 1 {
			numPrecinctX = 1
		}

		for _, bandInfo := range bands {
			if bandInfo.width <= 0 || bandInfo.height <= 0 {
				continue
			}
			numCBX := (bandInfo.width + cbWidth - 1) / cbWidth
			numCBY := (bandInfo.height + cbHeight - 1) / cbHeight
			for cby := 0; cby < numCBY; cby++ {
				for cbx := 0; cbx < numCBX; cbx++ {
					cbX0 := cbx * cbWidth
					cbY0 := cby * cbHeight
					absResX0 := resX0 + cbX0
					absResY0 := resY0 + cbY0
					px := (absResX0 - startX) / pw
					py := (absResY0 - startY) / ph
					pIdx := py*numPrecinctX + px
					localX := absResX0 - (startX + px*pw)
					localY := absResY0 - (startY + py*ph)
					cbxLocal := localX / cbWidth
					cbyLocal := localY / cbHeight
					addEntry(pIdx, bandInfo.band, cbxLocal, cbyLocal, globalCBIdx)
					globalCBIdx++
				}
			}
		}

		bandOrder := []int{0}
		if res > 0 {
			bandOrder = []int{1, 2, 3}
		}
		for pIdx, bandMap := range precinctBands {
			for _, band := range bandOrder {
				entries := bandMap[band]
				if len(entries) == 0 {
					continue
				}
				sort.Slice(entries, func(i, j int) bool {
					if entries[i].cby != entries[j].cby {
						return entries[i].cby < entries[j].cby
					}
					return entries[i].cbx < entries[j].cbx
				})
				for _, entry := range entries {
					order[res][pIdx] = append(order[res][pIdx], entry.global)
				}
			}
		}
	}

	return order
}

// precinctSizeForResolution returns the precinct size in pixels for a resolution (default 2^15).
func (td *TileDecoder) precinctSizeForResolution(resolution int) (int, int) {
	if td.cod != nil && resolution >= 0 && resolution < len(td.cod.PrecinctSizes) {
		ppx := td.cod.PrecinctSizes[resolution].PPx
		ppy := td.cod.PrecinctSizes[resolution].PPy
		return 1 << ppx, 1 << ppy
	}
	return 1 << 15, 1 << 15
}

// assembleSubbands assembles code-block coefficients into subband arrays
func (td *TileDecoder) assembleSubbands(comp *ComponentDecoder) {
	// Initialize the full coefficient array
	comp.coefficients = make([]int32, comp.width*comp.height)

	if len(comp.codeBlocks) == 0 {
		// No code-blocks decoded - use zeros
		return
	}

	// The code-blocks are organized in the same order as they were encoded:
	// - Resolution 0: LL subband (single subband at top-left)
	// - Resolution 1..N: HL, LH, HH subbands for each resolution level
	//
	// The wavelet coefficient array layout after DWT is:
	// For numLevels=1 (one decomposition):
	// +-------+-------+
	// |  LL   |  HL   |
	// +-------+-------+
	// |  LH   |  HH   |
	// +-------+-------+

	// Simply copy all code-blocks at their x0,y0 positions
	// The encoder has already set the correct x0,y0 for each code-block
	for _, cb := range comp.codeBlocks {
		x0 := cb.x0
		y0 := cb.y0
		x1 := cb.x1
		y1 := cb.y1
		actualWidth := x1 - x0
		actualHeight := y1 - y0

		// Copy decoded coefficients from code-block to full array
		for y := 0; y < actualHeight; y++ {
			for x := 0; x < actualWidth; x++ {
				srcIdx := y*actualWidth + x
				dstIdx := (y0+y)*comp.width + (x0 + x)

				if srcIdx < len(cb.coeffs) && dstIdx < len(comp.coefficients) {
					comp.coefficients[dstIdx] = cb.coeffs[srcIdx]
				}
			}
		}
	}
}

// applyIDWT applies the inverse discrete wavelet transform
func (td *TileDecoder) applyIDWT(comp *ComponentDecoder) error {
	if comp.numLevels == 0 {
		// No wavelet transform - coefficients are samples
		comp.samples = comp.coefficients
		return nil
	}

	switch td.cod.Transformation {
	case 1:
		// 5/3 reversible wavelet (lossless)
		comp.samples = make([]int32, len(comp.coefficients))
		copy(comp.samples, comp.coefficients)
		wavelet.InverseMultilevelWithParity(comp.samples, comp.width, comp.height, comp.numLevels, comp.x0, comp.y0)
	case 0:
		// 9/7 irreversible wavelet (lossy)
		var floatCoeffs []float32
		qType := 0
		if td.qcd != nil {
			qType = td.qcd.QuantizationType()
		}
		if qType == 1 || qType == 2 {
			bitDepth := td.siz.Components[comp.componentIdx].BitDepth()
			floatCoeffs = td.applyDequantizationBySubbandFloat(comp.coefficients, comp.width, comp.height, comp.numLevels, bitDepth, comp.x0, comp.y0)
		} else {
			floatCoeffs = wavelet.ConvertInt32ToFloat32(comp.coefficients)
		}
		wavelet.InverseMultilevel97OpenJPEGWithParity(floatCoeffs, comp.width, comp.height, comp.numLevels, comp.x0, comp.y0)
		comp.samples = wavelet.ConvertFloat32ToInt32OpenJPEG(floatCoeffs)
	default:
		return fmt.Errorf("unsupported wavelet transformation type: %d", td.cod.Transformation)
	}

	return nil
}

// applyDequantizationBySubbandFloat applies dequantization to each subband separately.
// coeffs: quantized wavelet coefficients in subband layout
// width, height: dimensions of the full image
// numLevels: number of wavelet decomposition levels
func (td *TileDecoder) applyDequantizationBySubbandFloat(coeffs []int32, width, height, numLevels, bitDepth, x0, y0 int) []float32 {
	if td.qcd == nil || len(td.qcd.SPqcd) == 0 {
		// No dequantization
		return wavelet.ConvertInt32ToFloat32(coeffs)
	}

	stepSizes := td.decodeQuantizationSteps(numLevels, bitDepth)
	if len(stepSizes) == 0 {
		return wavelet.ConvertInt32ToFloat32(coeffs)
	}

	floatCoeffs := wavelet.ConvertInt32ToFloat32(coeffs)

	subbandIdx := 0

	// LL subband (resolution 0)
	_, _, _, _, bands := bandInfosForResolution(width, height, x0, y0, numLevels, 0)
	if len(bands) > 0 && subbandIdx < len(stepSizes) {
		b := bands[0]
		if b.width > 0 && b.height > 0 {
			td.dequantizeSubbandFloat(floatCoeffs, b.offsetX, b.offsetY, b.width, b.height, width, stepSizes[subbandIdx])
		}
	}
	subbandIdx++

	// HL/LH/HH subbands from low to high resolution
	for res := 1; res <= numLevels; res++ {
		_, _, _, _, bands = bandInfosForResolution(width, height, x0, y0, numLevels, res)
		for _, b := range bands {
			if subbandIdx < len(stepSizes) && b.width > 0 && b.height > 0 {
				td.dequantizeSubbandFloat(floatCoeffs, b.offsetX, b.offsetY, b.width, b.height, width, stepSizes[subbandIdx])
			}
			subbandIdx++
		}
	}

	return floatCoeffs
}

// dequantizeSubbandFloat dequantizes a single subband.
// data: full coefficient array (float domain)
// x0, y0: top-left corner of subband
// w, h: dimensions of subband
// stride: row stride (width of full image)
// stepSize: quantization step size
func (td *TileDecoder) dequantizeSubbandFloat(data []float32, x0, y0, w, h, stride int, stepSize float64) {
	if stepSize <= 0 {
		return
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := (y0+y)*stride + (x0 + x)
			if idx < len(data) {
				// OpenJPEG T1 decode reconstructs half-step centered values and
				// irreversible dequantization applies 0.5 * band->stepsize.
				data[idx] *= float32(0.5 * stepSize)
			}
		}
	}
}

func normalizeOpenJPEGReversibleT1Coefficients(coeffs []int32) {
	for i := range coeffs {
		coeffs[i] /= 2
	}
}

func (td *TileDecoder) decodeQuantizationSteps(numLevels, bitDepth int) []float64 {
	if td.qcd == nil || len(td.qcd.SPqcd) == 0 {
		return nil
	}
	qType := td.qcd.QuantizationType()
	if qType != 1 && qType != 2 {
		return nil
	}

	numSubbands := 3*numLevels + 1
	steps := make([]float64, numSubbands)

	switch qType {
	case 1:
		if len(td.qcd.SPqcd) < 2 {
			return nil
		}
		encoded := uint16(td.qcd.SPqcd[0])<<8 | uint16(td.qcd.SPqcd[1])
		baseExpn := int((encoded >> 11) & 0x1f)
		baseMant := int(encoded & 0x7ff)
		for idx := 0; idx < numSubbands; idx++ {
			expn := baseExpn
			if idx > 0 {
				expn -= (idx - 1) / 3
				if expn < 0 {
					expn = 0
				}
			}
			log2Gain := td.log2GainForSubband(idx)
			steps[idx] = decodeQuantStep(expn, baseMant, bitDepth, log2Gain)
		}
	case 2:
		maxBands := len(td.qcd.SPqcd) / 2
		if maxBands == 0 {
			return nil
		}
		if maxBands < numSubbands {
			numSubbands = maxBands
			steps = steps[:numSubbands]
		}
		for idx := 0; idx < numSubbands; idx++ {
			offset := idx * 2
			encoded := uint16(td.qcd.SPqcd[offset])<<8 | uint16(td.qcd.SPqcd[offset+1])
			expn := int((encoded >> 11) & 0x1f)
			mant := int(encoded & 0x7ff)
			log2Gain := td.log2GainForSubband(idx)
			steps[idx] = decodeQuantStep(expn, mant, bitDepth, log2Gain)
		}
	}

	return steps
}

func (td *TileDecoder) log2GainForSubband(idx int) int {
	if td.cod.Transformation == 0 {
		return 0
	}
	if idx == 0 {
		return 0
	}
	orient := (idx-1)%3 + 1
	if orient == 3 {
		return 2
	}
	return 1
}

func decodeQuantStep(expn, mant, bitDepth, log2Gain int) float64 {
	rb := bitDepth + log2Gain
	return math.Ldexp(1.0+float64(mant)/2048.0, rb-expn)
}

// GetComponentData returns the decoded data for a specific component
func (td *TileDecoder) GetComponentData(componentIdx int) ([]int32, error) {
	if componentIdx < 0 || componentIdx >= len(td.decodedData) {
		return nil, fmt.Errorf("invalid component index: %d", componentIdx)
	}

	return td.decodedData[componentIdx], nil
}

// GetAllComponentsData returns decoded data for all components
func (td *TileDecoder) GetAllComponentsData() [][]int32 {
	return td.decodedData
}

// applyInverseGeneralScaling divides coefficients by 2^shift in-place.
func applyInverseGeneralScaling(data []int32, shift int) {
	if shift <= 0 {
		return
	}
	factor := int32(1 << shift)
	for i := range data {
		data[i] /= factor
	}
}

// applyInverseGeneralScalingMasked divides only masked coefficients by 2^shift.
func applyInverseGeneralScalingMasked(data []int32, mask [][]bool, shift int) {
	if shift <= 0 || len(mask) == 0 || len(mask[0]) == 0 {
		return
	}
	factor := int32(1 << shift)
	height := len(mask)
	width := len(mask[0])
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if mask[y][x] {
				idx := y*width + x
				if idx >= 0 && idx < len(data) {
					data[idx] /= factor
				}
			}
		}
	}
}

// applyInverseMaxShift reverses MaxShift ROI scaling using the standard threshold rule.
func applyInverseMaxShift(data []int32, shift int) {
	if shift <= 0 {
		return
	}
	if shift >= 31 {
		for i := range data {
			data[i] = 0
		}
		return
	}
	thresh := int32(1 << shift)
	for i, val := range data {
		mag := val
		if mag < 0 {
			mag = -mag
		}
		if mag >= thresh {
			mag >>= shift
			if val < 0 {
				data[i] = -mag
			} else {
				data[i] = mag
			}
		}
	}
}

func maskAnyTrue(mask [][]bool) bool {
	for y := 0; y < len(mask); y++ {
		for x := 0; x < len(mask[y]); x++ {
			if mask[y][x] {
				return true
			}
		}
	}
	return false
}
