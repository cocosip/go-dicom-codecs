package jpeg2000

import (
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/colorspace"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
)

// Decoder implements JPEG 2000 decoding
type Decoder struct {
	// Codestream
	cs *codestream.Codestream

	// Custom block decoder factory (for HTJ2K support)
	blockDecoderFactory t2.BlockDecoderFactory

	// ROI
	roi       *ROIParams
	roiConfig *ROIConfig
	roiShifts []int
	RoiRects  [][]RoiRect // per-component rectangles
	roiSrgn   []byte      // per-component ROI style (Srgn)
	roiMasks  []*roiMask  // per-component ROI mask

	// Decoded image data
	width      int
	height     int
	components int
	bitDepth   int
	isSigned   bool

	// Decoded pixel data per component
	data [][]int32

	// Custom MCT inverse (experimental, parsed from COM)
	mctInverse [][]float64
	// Optional per-component offsets
	mctOffsets []int32
	bindings   []mctBinding

	// Error resilience configuration
	resilient bool // Enable error resilience mode (warnings instead of errors)
	strict    bool // Strict mode: fail on any error (default: false for resilience)
}

type mctBinding struct {
	compIDs    []int
	matrixF    [][]float64
	matrixI    [][]int32
	offsets    []int32
	reversible bool
}

// NewDecoder creates a new JPEG 2000 decoder
func NewDecoder() *Decoder {
	return &Decoder{}
}

// SetROI sets the ROI rectangle for decoding (required if ROI is used in the codestream).
func (d *Decoder) SetROI(roi *ROIParams) {
	d.roi = roi
}

// SetROIConfig sets ROI configuration (MVP: multiple rectangles, MaxShift).
func (d *Decoder) SetROIConfig(cfg *ROIConfig) {
	d.roiConfig = cfg
}

// SetBlockDecoderFactory sets a custom block decoder factory (e.g., for HTJ2K support)
func (d *Decoder) SetBlockDecoderFactory(factory t2.BlockDecoderFactory) {
	d.blockDecoderFactory = factory
}

// SetResilient enables error resilience mode (warnings instead of fatal errors)
func (d *Decoder) SetResilient(resilient bool) {
	d.resilient = resilient
}

// SetStrict enables strict mode (fail on any error, no resilience)
func (d *Decoder) SetStrict(strict bool) {
	d.strict = strict
	if strict {
		d.resilient = false // Strict mode overrides resilience
	}
}

// Decode decodes a JPEG 2000 codestream
func (d *Decoder) Decode(data []byte) error {
	// Parse codestream
	parser := codestream.NewParser(data)
	cs, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse codestream: %w", err)
	}

	d.cs = cs

	// Extract image parameters
	if err := d.extractImageParameters(); err != nil {
		return fmt.Errorf("failed to extract image parameters: %w", err)
	}

	// Capture ROI shift values from RGN segments
	d.captureROIShifts()

	// Extract ROI geometry from COM marker (if present)
	d.extractROIFromCOM()
	d.extractMCTFromMarkers()
	d.extractBindings()

	// Resolve ROI geometry (legacy ROI or ROIConfig)
	if err := d.resolveROI(); err != nil {
		return fmt.Errorf("invalid ROI configuration: %w", err)
	}

	if err := d.decodeTiles(); err != nil {
		return fmt.Errorf("failed to decode tiles: %w", err)
	}

	return nil
}

// extractImageParameters extracts image parameters from SIZ segment
func (d *Decoder) extractImageParameters() error {
	if d.cs.SIZ == nil {
		return fmt.Errorf("missing SIZ segment")
	}

	siz := d.cs.SIZ

	d.width = int(siz.Xsiz - siz.XOsiz)
	d.height = int(siz.Ysiz - siz.YOsiz)
	d.components = int(siz.Csiz)

	if d.components == 0 {
		return fmt.Errorf("invalid number of components: %d", d.components)
	}

	// Use first component's parameters
	d.bitDepth = siz.Components[0].BitDepth()
	d.isSigned = siz.Components[0].IsSigned()

	return nil
}

// captureROIShifts builds per-component ROI shift table from RGN segments.
// If no RGN is present, shifts remain zero.
func (d *Decoder) captureROIShifts() {
	if d.cs == nil || d.cs.SIZ == nil {
		return
	}
	d.roiShifts = make([]int, d.components)
	d.roiSrgn = make([]byte, d.components)
	for _, rgn := range d.cs.RGN {
		if int(rgn.Crgn) < len(d.roiShifts) {
			d.roiShifts[int(rgn.Crgn)] = int(rgn.SPrgn)
			d.roiSrgn[int(rgn.Crgn)] = rgn.Srgn
		}
	}
}

// extractROIFromCOM extracts ROI geometry from COM marker (private metadata).
// This allows automatic ROI reconstruction without external parameters.
func (d *Decoder) extractROIFromCOM() {
	if d.cs == nil || len(d.cs.COM) == 0 {
		return
	}

	// If user already provided ROIConfig, don't override
	if d.roiConfig != nil && !d.roiConfig.IsEmpty() {
		return
	}

	// Search for our private ROI COM marker
	for _, com := range d.cs.COM {
		// Check for magic string "JP2ROI"
		if len(com.Data) < 7 {
			continue
		}
		if string(com.Data[0:6]) != "JP2ROI" {
			continue
		}

		// Parse version
		version := com.Data[6]
		if version != 1 {
			continue // Unknown version
		}

		// Parse ROI configuration
		cfg, err := parseROIFromCOMData(com.Data[7:])
		if err != nil {
			// Invalid format, skip
			continue
		}

		// Use this ROI configuration
		d.roiConfig = cfg
		return
	}
}

// extractMCTFromMarkers parses a fallback inverse matrix from MCT markers (legacy mode).
func (d *Decoder) extractMCTFromMarkers() {
	if d.cs == nil {
		return
	}
	if len(d.cs.MCC) == 0 && len(d.cs.MCT) > 0 && d.components > 0 {
		var inv [][]float64
		var offs []int32
		for _, seg := range d.cs.MCT {
			if seg.ArrayType == codestream.MCTArrayDecorrelate && inv == nil {
				if m := decodeMCTMatrix(seg, d.components); m != nil {
					inv = m
				}
			}
			if seg.ArrayType == codestream.MCTArrayOffset && offs == nil {
				if o := decodeMCTOffsets(seg, d.components); o != nil {
					offs = o
				}
			}
		}
		if inv != nil {
			d.mctInverse = inv
		}
		if offs != nil {
			d.mctOffsets = offs
		}
		if d.mctInverse != nil || d.mctOffsets != nil {
			return
		}
	}
	// Fallback to COM-based payload
	if len(d.cs.COM) > 0 {
		for _, com := range d.cs.COM {
			if len(com.Data) < 7 {
				continue
			}
			if string(com.Data[0:6]) != "JP2MCT" {
				continue
			}
			version := com.Data[6]
			if version != 1 {
				continue
			}
			if len(com.Data) < 11 {
				continue
			}
			rows := int(com.Data[7])<<8 | int(com.Data[8])
			cols := int(com.Data[9])<<8 | int(com.Data[10])
			offset := 11
			if offset >= len(com.Data) {
				continue
			}
			// reversible flag
			offset++
			need := rows * cols * 4
			if offset+need > len(com.Data) {
				continue
			}
			inv := make([][]float64, rows)
			for r := 0; r < rows; r++ {
				inv[r] = make([]float64, cols)
				for c := 0; c < cols; c++ {
					v := uint32(com.Data[offset])<<24 | uint32(com.Data[offset+1])<<16 | uint32(com.Data[offset+2])<<8 | uint32(com.Data[offset+3])
					offset += 4
					inv[r][c] = float64(math.Float32frombits(v))
				}
			}
			d.mctInverse = inv
			return
		}
	}
}

func (d *Decoder) extractBindings() {
	if d.cs == nil {
		return
	}
	if len(d.cs.MCC) == 0 {
		return
	}
	mctByIndex := make(map[uint8]codestream.MCTSegment, len(d.cs.MCT))
	for _, seg := range d.cs.MCT {
		mctByIndex[seg.Index] = seg
	}
	mccByIndex := make(map[uint8]codestream.MCCSegment, len(d.cs.MCC))
	for _, seg := range d.cs.MCC {
		mccByIndex[seg.Index] = seg
	}
	var order []uint8
	if len(d.cs.MCO) > 0 && len(d.cs.MCO[0].StageIndices) > 0 {
		order = append(order, d.cs.MCO[0].StageIndices...)
	} else {
		for _, seg := range d.cs.MCC {
			order = append(order, seg.Index)
		}
	}
	for _, idx := range order {
		seg, ok := mccByIndex[idx]
		if !ok {
			continue
		}
		if seg.CollectionType != 0 && seg.CollectionType != 1 {
			continue
		}
		compIDs := seg.ComponentIDs
		if len(compIDs) == 0 && seg.NumComponents > 0 {
			compIDs = make([]uint16, seg.NumComponents)
			for i := range compIDs {
				compIDs[i] = uint16(i)
			}
		}
		if len(seg.OutputComponentIDs) > 0 && !sameUint16Slice(seg.OutputComponentIDs, compIDs) {
			continue
		}
		if len(compIDs) == 0 {
			continue
		}
		compIdx := make([]int, len(compIDs))
		for i := range compIDs {
			compIdx[i] = int(compIDs[i])
		}
		var matF [][]float64
		var matI [][]int32
		if seg.DecorrelateIndex != 0 {
			if m, ok := mctByIndex[seg.DecorrelateIndex]; ok && m.ArrayType == codestream.MCTArrayDecorrelate {
				matF, matI = decodeMCTMatrixWithInts(m, len(compIDs))
			}
		}
		var offsVals []int32
		if seg.OffsetIndex != 0 {
			if m, ok := mctByIndex[seg.OffsetIndex]; ok && m.ArrayType == codestream.MCTArrayOffset {
				offsVals = decodeMCTOffsets(m, len(compIDs))
			}
		}
		if matF == nil && matI == nil && offsVals == nil {
			continue
		}
		b := mctBinding{
			compIDs:    compIdx,
			matrixF:    matF,
			matrixI:    matI,
			offsets:    offsVals,
			reversible: seg.Reversible,
		}
		d.bindings = append(d.bindings, b)
	}
}

func mctElementSize(et codestream.MCTElementType) int {
	switch et {
	case codestream.MCTElementInt16:
		return 2
	case codestream.MCTElementInt32, codestream.MCTElementFloat32:
		return 4
	case codestream.MCTElementFloat64:
		return 8
	default:
		return 0
	}
}

func decodeMCTMatrix(seg codestream.MCTSegment, comps int) [][]float64 {
	elemSize := mctElementSize(seg.ElementType)
	if comps <= 0 || elemSize == 0 {
		return nil
	}
	need := comps * comps * elemSize
	if len(seg.Data) < need {
		return nil
	}
	mat := make([][]float64, comps)
	off := 0
	for r := 0; r < comps; r++ {
		mat[r] = make([]float64, comps)
		for c := 0; c < comps; c++ {
			switch seg.ElementType {
			case codestream.MCTElementInt16:
				v := int16(uint16(seg.Data[off])<<8 | uint16(seg.Data[off+1]))
				mat[r][c] = float64(v)
			case codestream.MCTElementInt32:
				v := int32(uint32(seg.Data[off])<<24 | uint32(seg.Data[off+1])<<16 | uint32(seg.Data[off+2])<<8 | uint32(seg.Data[off+3]))
				mat[r][c] = float64(v)
			case codestream.MCTElementFloat32:
				v := uint32(seg.Data[off])<<24 | uint32(seg.Data[off+1])<<16 | uint32(seg.Data[off+2])<<8 | uint32(seg.Data[off+3])
				mat[r][c] = float64(math.Float32frombits(v))
			case codestream.MCTElementFloat64:
				v := uint64(seg.Data[off])<<56 | uint64(seg.Data[off+1])<<48 | uint64(seg.Data[off+2])<<40 | uint64(seg.Data[off+3])<<32 | uint64(seg.Data[off+4])<<24 | uint64(seg.Data[off+5])<<16 | uint64(seg.Data[off+6])<<8 | uint64(seg.Data[off+7])
				mat[r][c] = math.Float64frombits(v)
			}
			off += elemSize
		}
	}
	return mat
}

func decodeMCTMatrixWithInts(seg codestream.MCTSegment, comps int) ([][]float64, [][]int32) {
	switch seg.ElementType {
	case codestream.MCTElementInt16, codestream.MCTElementInt32:
		elemSize := mctElementSize(seg.ElementType)
		if comps <= 0 || elemSize == 0 {
			return nil, nil
		}
		need := comps * comps * elemSize
		if len(seg.Data) < need {
			return nil, nil
		}
		matF := make([][]float64, comps)
		matI := make([][]int32, comps)
		off := 0
		for r := 0; r < comps; r++ {
			matF[r] = make([]float64, comps)
			matI[r] = make([]int32, comps)
			for c := 0; c < comps; c++ {
				var v int32
				if seg.ElementType == codestream.MCTElementInt16 {
					v = int32(int16(uint16(seg.Data[off])<<8 | uint16(seg.Data[off+1])))
				} else {
					v = int32(uint32(seg.Data[off])<<24 | uint32(seg.Data[off+1])<<16 | uint32(seg.Data[off+2])<<8 | uint32(seg.Data[off+3]))
				}
				matI[r][c] = v
				matF[r][c] = float64(v)
				off += elemSize
			}
		}
		return matF, matI
	default:
		return decodeMCTMatrix(seg, comps), nil
	}
}

func decodeMCTOffsets(seg codestream.MCTSegment, comps int) []int32 {
	elemSize := mctElementSize(seg.ElementType)
	if comps <= 0 || elemSize == 0 {
		return nil
	}
	need := comps * elemSize
	if len(seg.Data) < need {
		return nil
	}
	offs := make([]int32, comps)
	off := 0
	for i := 0; i < comps; i++ {
		switch seg.ElementType {
		case codestream.MCTElementInt16:
			v := int16(uint16(seg.Data[off])<<8 | uint16(seg.Data[off+1]))
			offs[i] = int32(v)
		case codestream.MCTElementInt32:
			v := int32(uint32(seg.Data[off])<<24 | uint32(seg.Data[off+1])<<16 | uint32(seg.Data[off+2])<<8 | uint32(seg.Data[off+3]))
			offs[i] = v
		case codestream.MCTElementFloat32:
			v := uint32(seg.Data[off])<<24 | uint32(seg.Data[off+1])<<16 | uint32(seg.Data[off+2])<<8 | uint32(seg.Data[off+3])
			offs[i] = int32(math.Float32frombits(v))
		case codestream.MCTElementFloat64:
			v := uint64(seg.Data[off])<<56 | uint64(seg.Data[off+1])<<48 | uint64(seg.Data[off+2])<<40 | uint64(seg.Data[off+3])<<32 | uint64(seg.Data[off+4])<<24 | uint64(seg.Data[off+5])<<16 | uint64(seg.Data[off+6])<<8 | uint64(seg.Data[off+7])
			offs[i] = int32(math.Float64frombits(v))
		}
		off += elemSize
	}
	return offs
}

func sameUint16Slice(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// resolveROI normalizes ROI inputs (legacy ROI or ROIConfig) into internal rectangles.
func (d *Decoder) resolveROI() error {
	d.RoiRects = nil

	// ROIConfig takes priority when present
	if d.roiConfig != nil && !d.roiConfig.IsEmpty() {
		if err := d.roiConfig.Validate(d.width, d.height); err != nil {
			return err
		}
		srgn, shifts, rects, err := d.roiConfig.ResolveRectangles(d.width, d.height, d.components)
		if err != nil {
			return err
		}
		d.roiShifts = shifts
		d.RoiRects = rects
		d.roiMasks = buildMasksFromConfig(d.width, d.height, d.components, rects, d.roiConfig)
		if len(shifts) > 0 {
			d.roiSrgn = make([]byte, len(shifts))
			for i := range d.roiSrgn {
				d.roiSrgn[i] = srgn
			}
		}
		return nil
	}

	// Legacy single-rectangle ROI
	if d.roi != nil {
		if !d.roi.IsValid(d.width, d.height) {
			return fmt.Errorf("invalid ROI parameters for decoded image: %+v", *d.roi)
		}
		d.roiShifts = make([]int, d.components)
		d.roiSrgn = make([]byte, d.components)
		d.RoiRects = make([][]RoiRect, d.components)
		d.roiMasks = make([]*roiMask, d.components)
		for c := 0; c < d.components; c++ {
			d.roiShifts[c] = d.roi.Shift
			d.roiSrgn[c] = 0
			d.RoiRects[c] = []RoiRect{{
				x0: d.roi.X0,
				y0: d.roi.Y0,
				x1: d.roi.X0 + d.roi.Width,
				y1: d.roi.Y0 + d.roi.Height,
			}}
			d.roiMasks[c] = newROIMask(d.width, d.height)
			d.roiMasks[c].setRect(d.roi.X0, d.roi.Y0, d.roi.X0+d.roi.Width, d.roi.Y0+d.roi.Height)
		}
	}

	return nil
}

// decodeTiles decodes all tiles in the codestream
func (d *Decoder) decodeTiles() error {
	if len(d.cs.Tiles) == 0 {
		return fmt.Errorf("no tiles found in codestream")
	}
	assembler := NewTileAssembler(d.cs.SIZ)
	roiInfo := d.buildDecoderROIInfo()
	if err := d.decodeAllTiles(assembler, roiInfo); err != nil {
		return err
	}
	d.data = assembler.GetImageData()
	d.applyInverseTransforms()
	d.applyInverseDCLevelShift()
	return nil
}

func (d *Decoder) buildDecoderROIInfo() *t2.ROIInfo {
	if len(d.roiShifts) != d.components || d.components == 0 {
		return nil
	}
	roiInfo := &t2.ROIInfo{
		Shifts: d.roiShifts,
		Styles: d.roiSrgn,
	}
	if len(d.RoiRects) > 0 {
		rectsByComp := make([][]t2.ROIRect, len(d.RoiRects))
		for comp := range d.RoiRects {
			rects := d.RoiRects[comp]
			rectsByComp[comp] = make([]t2.ROIRect, len(rects))
			for i, r := range rects {
				rectsByComp[comp][i] = t2.ROIRect{
					X0: r.x0,
					Y0: r.y0,
					X1: r.x1,
					Y1: r.y1,
				}
			}
		}
		roiInfo.RectsByComponent = rectsByComp
	}
	if len(d.roiMasks) > 0 {
		roiInfo.Masks = make([]*t2.ROIMask, len(d.roiMasks))
		for i := range d.roiMasks {
			if d.roiMasks[i] != nil {
				roiInfo.Masks[i] = &t2.ROIMask{
					Width:  d.roiMasks[i].width,
					Height: d.roiMasks[i].height,
					Data:   d.roiMasks[i].data,
				}
			}
		}
	}
	return roiInfo
}

func (d *Decoder) decodeAllTiles(assembler *TileAssembler, roiInfo *t2.ROIInfo) error {
	for tileIdx, tile := range d.cs.Tiles {
		cod, qcd := d.resolveTileCODQCD(tile)
		isHTJ2K := cod != nil && (cod.CodeBlockStyle&0x40) != 0
		var blockDecoderFactory t2.BlockDecoderFactory
		if isHTJ2K && d.blockDecoderFactory != nil {
			blockDecoderFactory = d.blockDecoderFactory
		}
		tileDecoder := t2.NewTileDecoder(tile, d.cs.SIZ, cod, qcd, roiInfo, isHTJ2K, blockDecoderFactory)
		tileData, err := tileDecoder.Decode()
		if err != nil {
			return fmt.Errorf("failed to decode tile %d: %w", tileIdx, err)
		}
		err = assembler.AssembleTile(tileIdx, tileData)
		if err != nil {
			return fmt.Errorf("failed to assemble tile %d: %w", tileIdx, err)
		}
	}
	return nil
}

func (d *Decoder) resolveTileCODQCD(tile *codestream.Tile) (*codestream.CODSegment, *codestream.QCDSegment) {
	cod := d.cs.TileCOD(tile)
	qcd := d.cs.TileQCD(tile)
	if d.components == 1 {
		if resolved := d.cs.ComponentCOD(tile, 0); resolved != nil {
			cod = resolved
		}
		if resolved := d.cs.ComponentQCD(tile, 0); resolved != nil {
			qcd = resolved
		}
	}
	return cod, qcd
}

func (d *Decoder) applyInverseTransforms() {
	if len(d.bindings) > 0 {
		d.applyDecoderMCTBindings()
	} else if d.mctInverse != nil && len(d.mctInverse) == d.components {
		d.applyDecoderInverseCustomMCT()
	} else if d.cs != nil && d.cs.COD != nil && d.components == 3 {
		d.applyDecoderStandardInverseMCT()
	}
}

func (d *Decoder) applyDecoderMCTBindings() {
	n := d.width * d.height
	for _, b := range d.bindings {
		if len(b.compIDs) == 0 {
			continue
		}
		useInt := b.reversible && b.matrixI != nil && len(b.matrixI) == len(b.compIDs)
		if useInt {
			d.applyIntegerMatrixTransform(&b, n)
		} else if b.matrixF != nil && len(b.matrixF) == len(b.compIDs) {
			d.applyFloatMatrixTransform(&b, n)
		}
		d.applyBindingOffsets(&b, n)
	}
}

func (d *Decoder) applyIntegerMatrixTransform(b *mctBinding, n int) {
	r := len(b.matrixI)
	c := len(b.matrixI[0])
	for i := 0; i < n; i++ {
		out := make([]int32, r)
		for rr := 0; rr < r; rr++ {
			var sum int64
			for kk := 0; kk < c; kk++ {
				sum += int64(b.matrixI[rr][kk]) * int64(d.data[b.compIDs[kk]][i])
			}
			out[rr] = int32(sum)
		}
		for rr := 0; rr < r; rr++ {
			d.data[b.compIDs[rr]][i] = out[rr]
		}
	}
}

func (d *Decoder) applyFloatMatrixTransform(b *mctBinding, n int) {
	m := b.matrixF
	r := len(m)
	c := len(m[0])
	for i := 0; i < n; i++ {
		out := make([]int32, r)
		for rr := 0; rr < r; rr++ {
			sum := 0.0
			for kk := 0; kk < c; kk++ {
				sum += m[rr][kk] * float64(d.data[b.compIDs[kk]][i])
			}
			out[rr] = int32(math.Round(sum))
		}
		for rr := 0; rr < r; rr++ {
			d.data[b.compIDs[rr]][i] = out[rr]
		}
	}
}

func (d *Decoder) applyBindingOffsets(b *mctBinding, n int) {
	if b.offsets != nil && len(b.offsets) == len(b.compIDs) {
		for idx, cid := range b.compIDs {
			off := b.offsets[idx]
			if off != 0 {
				for i := 0; i < n; i++ {
					d.data[cid][i] += off
				}
			}
		}
	}
}

func (d *Decoder) applyDecoderInverseCustomMCT() {
	n := d.width * d.height
	comps := d.components
	out := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		out[c] = make([]int32, n)
	}
	for i := 0; i < n; i++ {
		for r := 0; r < comps; r++ {
			sum := 0.0
			for k := 0; k < comps; k++ {
				sum += d.mctInverse[r][k] * float64(d.data[k][i])
			}
			out[r][i] = int32(math.Round(sum))
		}
	}
	d.data = out
	if d.mctOffsets != nil && len(d.mctOffsets) == d.components {
		for c := 0; c < comps; c++ {
			off := d.mctOffsets[c]
			if off != 0 {
				for i := 0; i < n; i++ {
					d.data[c][i] += off
				}
			}
		}
	}
}

func (d *Decoder) applyDecoderStandardInverseMCT() {
	if d.cs.COD.MultipleComponentTransform == 1 {
		if d.cs.COD.Transformation == 1 {
			r, g, b := colorspace.ApplyInverseRCTToComponents(d.data[0], d.data[1], d.data[2])
			d.data[0], d.data[1], d.data[2] = r, g, b
		} else {
			r, g, b := colorspace.ApplyInverseICTToComponents(d.data[0], d.data[1], d.data[2])
			d.data[0], d.data[1], d.data[2] = r, g, b
		}
	}
}

// GetImageData returns the decoded image data for all components
func (d *Decoder) GetImageData() [][]int32 {
	return d.data
}

// GetComponentData returns the decoded data for a specific component
func (d *Decoder) GetComponentData(componentIdx int) ([]int32, error) {
	if componentIdx < 0 || componentIdx >= len(d.data) {
		return nil, fmt.Errorf("invalid component index: %d", componentIdx)
	}
	return d.data[componentIdx], nil
}

// Width returns the image width
func (d *Decoder) Width() int {
	return d.width
}

// Height returns the image height
func (d *Decoder) Height() int {
	return d.height
}

// Components returns the number of components
func (d *Decoder) Components() int {
	return d.components
}

// BitDepth returns the bit depth
func (d *Decoder) BitDepth() int {
	return d.bitDepth
}

// IsSigned returns whether the data is signed
func (d *Decoder) IsSigned() bool {
	return d.isSigned
}

// GetPixelData returns interleaved pixel data in a byte array
// Suitable for use with the Codec interface
func (d *Decoder) GetPixelData() []byte {
	if d.components == 1 {
		// Grayscale
		return d.getGrayscalePixelData()
	}
	// RGB or multi-component
	return d.getInterleavedPixelData()
}

// getGrayscalePixelData returns grayscale pixel data
func (d *Decoder) getGrayscalePixelData() []byte {
	numPixels := d.width * d.height

	if d.bitDepth <= 8 {
		// 8-bit
		result := make([]byte, numPixels)
		for i := 0; i < numPixels; i++ {
			val := d.data[0][i]

			// For signed data, convert to 2's complement representation
			if d.isSigned {
				// Signed data: clamp to signed range
				minVal := -(1 << (d.bitDepth - 1))
				maxVal := (1 << (d.bitDepth - 1)) - 1
				if val < int32(minVal) {
					val = int32(minVal)
				} else if val > int32(maxVal) {
					val = int32(maxVal)
				}
				// Convert to unsigned representation for storage (2's complement)
				if val < 0 {
					val += (1 << d.bitDepth)
				}
			} else {
				// Unsigned data: clamp to [0, 2^bitDepth-1]
				if val < 0 {
					val = 0
				}
				maxVal := (1 << d.bitDepth) - 1
				if val > int32(maxVal) {
					val = int32(maxVal)
				}
			}

			result[i] = byte(val)
		}
		return result
	}

	// 16-bit (or 12-bit stored as 16-bit)
	result := make([]byte, numPixels*2)
	for i := 0; i < numPixels; i++ {
		val := d.data[0][i]

		// For signed data, convert to 2's complement representation
		// For unsigned data, clamp to valid range
		if d.isSigned {
			// Signed data: clamp to signed range
			minVal := -(1 << (d.bitDepth - 1))
			maxVal := (1 << (d.bitDepth - 1)) - 1
			if val < int32(minVal) {
				val = int32(minVal)
			} else if val > int32(maxVal) {
				val = int32(maxVal)
			}
			// Convert to unsigned representation for storage (2's complement)
			if val < 0 {
				val += (1 << d.bitDepth)
			}
		} else {
			// Unsigned data: clamp to [0, maxVal]
			if val < 0 {
				val = 0
			}
			maxVal := (1 << d.bitDepth) - 1
			if val > int32(maxVal) {
				val = int32(maxVal)
			}
		}

		// Little-endian
		result[i*2] = byte(val)
		result[i*2+1] = byte(val >> 8)
	}
	return result
}

// getInterleavedPixelData returns interleaved RGB/multi-component pixel data
func (d *Decoder) getInterleavedPixelData() []byte {
	numPixels := d.width * d.height

	if d.bitDepth <= 8 {
		// 8-bit per component
		result := make([]byte, numPixels*d.components)
		for i := 0; i < numPixels; i++ {
			for c := 0; c < d.components; c++ {
				val := d.data[c][i]

				// For signed data, convert to 2's complement representation
				if d.isSigned {
					// Signed data: clamp to signed range
					minVal := -(1 << (d.bitDepth - 1))
					maxVal := (1 << (d.bitDepth - 1)) - 1
					if val < int32(minVal) {
						val = int32(minVal)
					} else if val > int32(maxVal) {
						val = int32(maxVal)
					}
					// Convert to unsigned representation for storage (2's complement)
					if val < 0 {
						val += (1 << d.bitDepth)
					}
				} else {
					// Unsigned data: clamp to [0, 2^bitDepth-1]
					if val < 0 {
						val = 0
					}
					maxVal := (1 << d.bitDepth) - 1
					if val > int32(maxVal) {
						val = int32(maxVal)
					}
				}

				result[i*d.components+c] = byte(val)
			}
		}
		return result
	}

	// 16-bit per component
	result := make([]byte, numPixels*d.components*2)
	for i := 0; i < numPixels; i++ {
		for c := 0; c < d.components; c++ {
			val := d.data[c][i]

			// For signed data, convert to 2's complement representation
			// For unsigned data, clamp to valid range
			if d.isSigned {
				// Signed data: clamp to signed range
				minVal := -(1 << (d.bitDepth - 1))
				maxVal := (1 << (d.bitDepth - 1)) - 1
				if val < int32(minVal) {
					val = int32(minVal)
				} else if val > int32(maxVal) {
					val = int32(maxVal)
				}
				// Convert to unsigned representation for storage (2's complement)
				if val < 0 {
					val += (1 << d.bitDepth)
				}
			} else {
				// Unsigned data: clamp to [0, maxVal]
				if val < 0 {
					val = 0
				}
				maxVal := (1 << d.bitDepth) - 1
				if val > int32(maxVal) {
					val = int32(maxVal)
				}
			}

			idx := (i*d.components + c) * 2
			result[idx] = byte(val)
			result[idx+1] = byte(val >> 8)
		}
	}
	return result
}

// applyInverseDCLevelShift applies inverse DC level shift for unsigned data
// For unsigned data: add 2^(bitDepth-1) to convert back from signed range
func (d *Decoder) applyInverseDCLevelShift() {
	if d.isSigned {
		// Signed data - no level shift needed
		return
	}

	// Unsigned data - add 2^(bitDepth-1)
	shift := int32(1 << (d.bitDepth - 1))

	for c := 0; c < d.components; c++ {
		for i := 0; i < len(d.data[c]); i++ {
			d.data[c][i] += shift
		}
	}
}

// parseROIFromCOMData parses ROI configuration from COM marker data.
func parseROIFromCOMData(data []byte) (*ROIConfig, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("COM data too short")
	}

	// Read number of ROI regions (2 bytes)
	numRegions := int(data[0])<<8 | int(data[1])
	offset := 2

	cfg := &ROIConfig{
		ROIs: make([]ROIRegion, 0, numRegions),
	}

	for i := 0; i < numRegions; i++ {
		if offset >= len(data) {
			return nil, fmt.Errorf("unexpected end of COM data")
		}

		// Read shape type (1 byte)
		shapeType := data[offset]
		offset++

		// Read number of components (1 byte)
		if offset >= len(data) {
			return nil, fmt.Errorf("unexpected end of COM data")
		}
		numComps := int(data[offset])
		offset++

		// Read component indices
		if offset+numComps > len(data) {
			return nil, fmt.Errorf("unexpected end of COM data")
		}
		comps := make([]int, numComps)
		for j := 0; j < numComps; j++ {
			comps[j] = int(data[offset])
			offset++
		}

		roi := ROIRegion{
			Components: comps,
		}

		// Parse geometry based on shape type
		switch shapeType {
		case 0: // Rectangle
			if offset+16 > len(data) {
				return nil, fmt.Errorf("unexpected end of COM data")
			}
			x0 := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
			y0 := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])
			x1 := int(data[offset+8])<<24 | int(data[offset+9])<<16 | int(data[offset+10])<<8 | int(data[offset+11])
			y1 := int(data[offset+12])<<24 | int(data[offset+13])<<16 | int(data[offset+14])<<8 | int(data[offset+15])
			offset += 16
			roi.Rect = &ROIParams{X0: x0, Y0: y0, Width: x1 - x0, Height: y1 - y0}
			roi.Shape = ROIShapeRectangle

		case 1: // Polygon
			if offset+2 > len(data) {
				return nil, fmt.Errorf("unexpected end of COM data")
			}
			numPoints := int(data[offset])<<8 | int(data[offset+1])
			offset += 2
			if offset+numPoints*8 > len(data) {
				return nil, fmt.Errorf("unexpected end of COM data")
			}
			points := make([]Point, numPoints)
			for j := 0; j < numPoints; j++ {
				x := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
				y := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])
				points[j] = Point{X: x, Y: y}
				offset += 8
			}
			roi.Polygon = points
			roi.Shape = ROIShapePolygon

		case 2: // Mask (placeholder only - actual mask data not stored)
			if offset+8 > len(data) {
				return nil, fmt.Errorf("unexpected end of COM data")
			}
			width := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
			height := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])
			offset += 8
			// Create empty mask data structure (decoder still needs external mask)
			roi.MaskWidth = width
			roi.MaskHeight = height
			roi.Shape = ROIShapeMask
			// Note: MaskData not populated from COM (too large to store)

		default:
			return nil, fmt.Errorf("unknown shape type: %d", shapeType)
		}

		cfg.ROIs = append(cfg.ROIs, roi)
	}

	return cfg, nil
}
