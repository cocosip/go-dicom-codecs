package jpeg2000

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/colorspace"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
)

// EncodeParams contains parameters for JPEG 2000 encoding
type EncodeParams struct {
	// Image parameters
	Width      int
	Height     int
	Components int
	BitDepth   int
	IsSigned   bool

	// Tile parameters
	TileWidth  int // 0 means single tile (entire image)
	TileHeight int // 0 means single tile (entire image)

	// Coding parameters
	NumLevels       int  // Number of wavelet decomposition levels (0-6)
	Lossless        bool // true for lossless (5/3 wavelet), false for lossy (9/7 wavelet)
	CodeBlockWidth  int  // Code-block width (power of 2, typically 64)
	CodeBlockHeight int  // Code-block height (power of 2, typically 64)

	// Precinct parameters (0 = use default size of 2^15 = 32768)
	PrecinctWidth  int // Precinct width (power of 2, e.g., 128, 256, 512)
	PrecinctHeight int // Precinct height (power of 2, e.g., 128, 256, 512)

	// Lossy compression quality (1-100, only used when Lossless=false)
	// Higher values = better quality, lower compression
	// 100 = minimal quantization (near-lossless)
	// 50 = balanced quality/compression
	// 1 = maximum compression, lower quality
	Quality int // Default: 80

	// CustomQuantSteps allows caller to override quantization step sizes per subband (lossy only).
	// Length must be 3*NumLevels+1 when provided. Values are floating quant steps.
	CustomQuantSteps []float64

	// TargetRatio optionally requests a target compression ratio (orig_size / compressed_size).
	// Used by rate-distortion truncation when >0.
	TargetRatio float64

	// Progression order
	ProgressionOrder uint8 // 0=LRCP, 1=RLCP, 2=RPCL, 3=PCRL, 4=CPRL

	// Layer parameters
	NumLayers int // Number of quality layers (default 1)
	// LayerRates mirrors OpenJPEG tcp_rates before tile byte-budget conversion.
	// Values > 0 are compression rates; 0 means a final lossless/full layer.
	LayerRates []float64

	// PCRD options
	UsePCRDOpt          bool
	LayerBudgetStrategy string
	LambdaTolerance     float64

	// AppendLosslessLayer will append a final lossless layer (rate=0) after target-rate layers.
	AppendLosslessLayer bool

	// Region of Interest (ROI)
	ROI *ROIParams // Optional single-rectangle ROI with MaxShift
	// ROIConfig supports multiple ROI entries (MVP: multiple rectangles, MaxShift only)
	ROIConfig *ROIConfig

	// Custom multi-component transform (Part 2 style, experimental)
	// If provided, overrides default RCT/ICT for multi-component images.
	MCTMatrix        [][]float64
	InverseMCTMatrix [][]float64
	MCTReversible    bool
	EnableMCT        bool // when false, skip RCT/ICT and custom MCT

	// Optional per-component offsets (Part 2 offset array)
	MCTOffsets           []int32
	MCTNormScale         float64
	MCTAssocType         uint8
	MCTMatrixElementType uint8 // 0=int32, 1=float32 (default)
	MCOPrecision         uint8
	MCORecordOrder       []uint8
	MCTBindings          []MCTBindingParams

	// Block encoder factory (for HTJ2K support)
	// If nil, defaults to EBCOT T1 encoder
	BlockEncoderFactory func(width, height int) BlockEncoder

	// HTJ2KMode marks code-blocks as JPEG 2000 Part 15 HT code-blocks.
	HTJ2KMode bool
}

// BlockEncoder is an interface for T1 block encoders (EBCOT or HTJ2K)
type BlockEncoder interface {
	Encode(coeffs []int32, numPasses int, roiShift int) ([]byte, error)
}

type blockEncoderKMaxSetter interface {
	SetKMax(kmax int)
}

// MCTBindingParams describes Part 2 multi-component transform binding parameters.
// Fields map to MCT/MCC/MCO marker semantics in JPEG 2000 Part 2.
type MCTBindingParams struct {
	AssocType      uint8
	ComponentIDs   []uint16
	MCTRecordOrder []uint8
	MCOPrecision   uint8
	MCONormScale   float64
	Matrix         [][]float64
	Inverse        [][]float64
	Offsets        []int32
	ElementType    uint8
}

// DefaultEncodeParams returns default encoding parameters for lossless encoding
func DefaultEncodeParams(width, height, components, bitDepth int, isSigned bool) *EncodeParams {
	return &EncodeParams{
		Width:               width,
		Height:              height,
		Components:          components,
		BitDepth:            bitDepth,
		IsSigned:            isSigned,
		TileWidth:           0, // Single tile
		TileHeight:          0, // Single tile
		NumLevels:           5, // 5 DWT levels
		Lossless:            true,
		Quality:             80, // Default quality for lossy mode
		CodeBlockWidth:      64,
		CodeBlockHeight:     64,
		PrecinctWidth:       0, // Default (2^15)
		PrecinctHeight:      0, // Default (2^15)
		TargetRatio:         0,
		ProgressionOrder:    0, // LRCP
		NumLayers:           1,
		LayerRates:          nil,
		UsePCRDOpt:          false,
		LayerBudgetStrategy: "EXPONENTIAL",
		LambdaTolerance:     0.01,
		AppendLosslessLayer: false,
		ROI:                 nil,
		EnableMCT:           true,
	}
}

// Encoder implements JPEG 2000 encoding
type Encoder struct {
	params                  *EncodeParams
	data                    [][]int32 // [component][pixel]
	roiShifts               []int
	RoiRects                [][]RoiRect // per-component rectangles
	roiStyles               []byte      // per-component Srgn value: 0=MaxShift, 1=GeneralScaling
	roiMasks                []*roiMask  // per-component ROI mask (full-res)
	qcdReady                bool
	qcdStyle                int
	qcdGuard                int
	qcdExpn                 []int
	qcdSteps                []uint16
	openJPEGMainHeaderBytes int
	openJPEGNumTiles        int
}

// NewEncoder creates a new JPEG 2000 encoder
func NewEncoder(params *EncodeParams) *Encoder {
	return &Encoder{
		params: params,
	}
}

// Encode encodes pixel data to JPEG 2000 format
// pixelData: raw pixel data (interleaved for multi-component, planar format as [][]int32 also supported)
func (e *Encoder) Encode(pixelData []byte) ([]byte, error) {
	// Validate parameters
	if err := e.validateParams(); err != nil {
		return nil, fmt.Errorf("invalid encoding parameters: %w", err)
	}

	// Convert pixel data to component arrays
	if err := e.convertPixelData(pixelData); err != nil {
		return nil, fmt.Errorf("failed to convert pixel data: %w", err)
	}

	// Apply DC level shift BEFORE MCT (to match OpenJPEG order)
	// OpenJPEG: DC shift -> MCT -> DWT -> T1
	e.applyDCLevelShift()

	if e.params.EnableMCT {
		if len(e.params.MCTBindings) > 0 {
			e.applyMCTBindings()
		} else if e.params.MCTMatrix != nil && len(e.params.MCTMatrix) == e.params.Components {
			e.applyCustomMCT()
		} else if e.params.Components == 3 {
			if e.params.Lossless {
				y, cb, cr := colorspace.ApplyRCTToComponents(e.data[0], e.data[1], e.data[2])
				e.data[0], e.data[1], e.data[2] = y, cb, cr
			} else {
				y, cb, cr := colorspace.ApplyICTToComponents(e.data[0], e.data[1], e.data[2])
				e.data[0], e.data[1], e.data[2] = y, cb, cr
			}
		}
	}

	// Build codestream
	codestream, err := e.buildCodestream()
	if err != nil {
		return nil, fmt.Errorf("failed to build codestream: %w", err)
	}

	return codestream, nil
}

// EncodeComponents encodes component data directly (for testing)
func (e *Encoder) EncodeComponents(componentData [][]int32) ([]byte, error) {
	// Validate parameters
	if err := e.validateParams(); err != nil {
		return nil, fmt.Errorf("invalid encoding parameters: %w", err)
	}

	// Validate component data
	if len(componentData) != e.params.Components {
		return nil, fmt.Errorf("expected %d components, got %d", e.params.Components, len(componentData))
	}

	expectedSize := e.params.Width * e.params.Height
	for i, comp := range componentData {
		if len(comp) != expectedSize {
			return nil, fmt.Errorf("component %d: expected %d pixels, got %d", i, expectedSize, len(comp))
		}
	}

	// Copy component data (we need to modify it for DC level shift)
	e.data = make([][]int32, len(componentData))
	for i := range componentData {
		e.data[i] = make([]int32, len(componentData[i]))
		copy(e.data[i], componentData[i])
	}

	// Apply DC level shift BEFORE MCT (to match OpenJPEG order)
	// OpenJPEG: DC shift -> MCT -> DWT -> T1
	e.applyDCLevelShift()

	if e.params.EnableMCT {
		if len(e.params.MCTBindings) > 0 {
			e.applyMCTBindings()
		} else if e.params.MCTMatrix != nil && len(e.params.MCTMatrix) == e.params.Components {
			e.applyCustomMCT()
		} else if e.params.Components == 3 {
			if e.params.Lossless {
				y, cb, cr := colorspace.ApplyRCTToComponents(e.data[0], e.data[1], e.data[2])
				e.data[0], e.data[1], e.data[2] = y, cb, cr
			} else {
				y, cb, cr := colorspace.ApplyICTToComponents(e.data[0], e.data[1], e.data[2])
				e.data[0], e.data[1], e.data[2] = y, cb, cr
			}
		}
	}

	// Build codestream
	codestream, err := e.buildCodestream()
	if err != nil {
		return nil, fmt.Errorf("failed to build codestream: %w", err)
	}

	return codestream, nil
}

// validateParams validates encoding parameters
func (e *Encoder) validateParams() error {
	p := e.params

	if p.Width <= 0 || p.Height <= 0 {
		return fmt.Errorf("invalid dimensions: %dx%d", p.Width, p.Height)
	}

	if p.Components <= 0 || p.Components > 4 {
		return fmt.Errorf("invalid number of components: %d (must be 1-4)", p.Components)
	}

	if p.BitDepth < 1 || p.BitDepth > 16 {
		return fmt.Errorf("invalid bit depth: %d (must be 1-16)", p.BitDepth)
	}

	if p.NumLevels < 0 || p.NumLevels > 6 {
		return fmt.Errorf("invalid decomposition levels: %d (must be 0-6)", p.NumLevels)
	}

	if p.CodeBlockWidth < 4 || p.CodeBlockWidth > 1024 || !isPowerOfTwo(p.CodeBlockWidth) {
		return fmt.Errorf("invalid code-block width: %d (must be power of 2, 4-1024)", p.CodeBlockWidth)
	}

	if p.CodeBlockHeight < 4 || p.CodeBlockHeight > 1024 || !isPowerOfTwo(p.CodeBlockHeight) {
		return fmt.Errorf("invalid code-block height: %d (must be power of 2, 4-1024)", p.CodeBlockHeight)
	}

	if p.NumLayers < 1 {
		return fmt.Errorf("invalid number of layers: %d (must be > 0)", p.NumLayers)
	}

	if p.ROIConfig != nil && !p.ROIConfig.IsEmpty() {
		if err := p.ROIConfig.Validate(p.Width, p.Height); err != nil {
			return fmt.Errorf("invalid ROIConfig: %w", err)
		}
	}

	if p.ROI != nil {
		if !p.ROI.IsValid(p.Width, p.Height) {
			return fmt.Errorf("invalid ROI parameters: %+v", *p.ROI)
		}
		if p.ROI.Shift > 255 {
			return fmt.Errorf("invalid ROI shift: %d (must be <=255)", p.ROI.Shift)
		}
	}

	return nil
}

// convertPixelData converts byte array to component arrays
func (e *Encoder) convertPixelData(pixelData []byte) error {
	p := e.params
	numPixels := p.Width * p.Height
	expectedBytes := numPixels * p.Components * ((p.BitDepth + 7) / 8)

	if len(pixelData) < expectedBytes {
		return fmt.Errorf("insufficient pixel data: got %d bytes, need %d", len(pixelData), expectedBytes)
	}

	// Initialize component arrays
	e.data = make([][]int32, p.Components)
	for i := range e.data {
		e.data[i] = make([]int32, numPixels)
	}

	// Convert based on bit depth
	if p.BitDepth <= 8 {
		// 8-bit data
		for i := 0; i < numPixels; i++ {
			for c := 0; c < p.Components; c++ {
				val := int32(pixelData[i*p.Components+c])
				if p.IsSigned && val >= 128 {
					val -= 256
				}
				e.data[c][i] = val
			}
		}
	} else {
		// 16-bit data (little-endian)
		for i := 0; i < numPixels; i++ {
			for c := 0; c < p.Components; c++ {
				idx := (i*p.Components + c) * 2
				val := int32(pixelData[idx]) | (int32(pixelData[idx+1]) << 8)
				if p.IsSigned && val >= (1<<(p.BitDepth-1)) {
					val -= (1 << p.BitDepth)
				}
				e.data[c][i] = val
			}
		}
	}

	return nil
}

// buildCodestream builds the JPEG 2000 codestream
func (e *Encoder) buildCodestream() ([]byte, error) {
	// Resolve ROI (supports legacy ROI and ROIConfig)
	if err := e.resolveROI(); err != nil {
		return nil, fmt.Errorf("failed to resolve ROI: %w", err)
	}

	buf := &bytes.Buffer{}

	// Write SOC (Start of Codestream)
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerSOC); err != nil {
		return nil, err
	}

	// Write SIZ (Image and Tile Size)
	if err := e.writeSIZ(buf); err != nil {
		return nil, fmt.Errorf("failed to write SIZ: %w", err)
	}
	if err := e.writeCAP(buf); err != nil {
		return nil, fmt.Errorf("failed to write CAP: %w", err)
	}

	// Write COD (Coding Style Default)
	if err := e.writeCOD(buf); err != nil {
		return nil, fmt.Errorf("failed to write COD: %w", err)
	}

	// Write QCD (Quantization Default)
	if err := e.writeQCD(buf); err != nil {
		return nil, fmt.Errorf("failed to write QCD: %w", err)
	}

	// Write version COM marker (similar to OpenJPEG)
	if err := e.writeVersionCOM(buf); err != nil {
		return nil, fmt.Errorf("failed to write version COM: %w", err)
	}

	// Write RGN (ROI) if present
	if err := e.writeRGN(buf); err != nil {
		return nil, fmt.Errorf("failed to write RGN: %w", err)
	}

	// Write COM (private ROI metadata) if ROI is enabled
	if err := e.writeCOM(buf); err != nil {
		return nil, fmt.Errorf("failed to write COM: %w", err)
	}

	// Write MCT/MCC (Part 2-style) if provided
	if err := e.writeMCTAndMCC(buf); err != nil {
		return nil, fmt.Errorf("failed to write MCT/MCC: %w", err)
	}

	e.openJPEGMainHeaderBytes = buf.Len()

	// Write tiles
	if err := e.writeTiles(buf); err != nil {
		return nil, fmt.Errorf("failed to write tiles: %w", err)
	}

	// Write EOC (End of Codestream)
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerEOC); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// applyCustomMCT applies a custom multi-component transform from params.MCTMatrix
func (e *Encoder) applyCustomMCT() {
	p := e.params
	if p.MCTMatrix == nil || len(p.MCTMatrix) != p.Components {
		return
	}
	width := p.Width
	height := p.Height
	n := width * height
	comps := p.Components
	out := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		out[c] = make([]int32, n)
	}
	if p.MCTOffsets != nil && len(p.MCTOffsets) == comps {
		for c := 0; c < comps; c++ {
			off := p.MCTOffsets[c]
			if off == 0 {
				continue
			}
			for i := 0; i < n; i++ {
				e.data[c][i] -= off
			}
		}
	}
	if p.MCTMatrixElementType == 0 && p.MCTReversible {
		im := make([][]int32, comps)
		for r := 0; r < comps; r++ {
			im[r] = make([]int32, comps)
			for k := 0; k < comps; k++ {
				im[r][k] = int32(p.MCTMatrix[r][k])
			}
		}
		for i := 0; i < n; i++ {
			for r := 0; r < comps; r++ {
				var sum int64
				for k := 0; k < comps; k++ {
					sum += int64(im[r][k]) * int64(e.data[k][i])
				}
				out[r][i] = int32(sum)
			}
		}
	} else {
		fixed := make([][]int32, comps)
		for r := 0; r < comps; r++ {
			fixed[r] = make([]int32, comps)
			for k := 0; k < comps; k++ {
				fixed[r][k] = int32(p.MCTMatrix[r][k] * float64(1<<13))
			}
		}
		for i := 0; i < n; i++ {
			for r := 0; r < comps; r++ {
				var sum int32
				for k := 0; k < comps; k++ {
					sum += mctFixedMul(fixed[r][k], e.data[k][i])
				}
				out[r][i] = sum
			}
		}
	}
	e.data = out
}

func (e *Encoder) applyMCTBindings() {
	p := e.params
	if len(p.MCTBindings) == 0 {
		return
	}
	order := e.determineMCTBindingOrder(p.MCTBindings, p.MCORecordOrder, p.Components)
	n := p.Width * p.Height
	for _, idx := range order {
		e.applyMCTBinding(p.MCTBindings[idx], n, p.Components)
	}
}

func (e *Encoder) determineMCTBindingOrder(bindings []MCTBindingParams, mcoOrder []byte, components int) []int {
	order := make([]int, len(bindings))
	for i := range bindings {
		order[i] = i
	}
	if len(mcoOrder) > 0 {
		allowed := mccIndicesForBindings(bindings, components)
		if validMCOOrder(mcoOrder, allowed) {
			if mapped := bindingOrderForMCO(bindings, components, mcoOrder); len(mapped) == len(bindings) {
				order = mapped
			}
		}
	}
	return order
}

func (e *Encoder) applyMCTBinding(b MCTBindingParams, n, components int) {
	compIdx := e.prepareComponentIndices(b.ComponentIDs, components)
	if len(compIdx) == 0 {
		return
	}
	e.applyMCTOffsets(b.Offsets, compIdx, n)
	matrix := e.prepareTransformMatrix(b.Matrix, len(compIdx))
	if b.ElementType == 0 {
		e.applyIntegerMatrixTransform(matrix, compIdx, n)
	} else {
		e.applyFixedPointMatrixTransform(matrix, compIdx, n)
	}
}

func (e *Encoder) prepareComponentIndices(componentIDs []uint16, components int) []int {
	compIDs := componentIDs
	if len(compIDs) == 0 && components > 0 {
		compIDs = make([]uint16, components)
		for i := range compIDs {
			compIDs[i] = uint16(i)
		}
	}
	if len(compIDs) == 0 {
		return nil
	}
	compIdx := make([]int, len(compIDs))
	for i := range compIDs {
		compIdx[i] = int(compIDs[i])
	}
	return compIdx
}

func (e *Encoder) applyMCTOffsets(offsets []int32, compIdx []int, n int) {
	if offsets == nil || len(offsets) != len(compIdx) {
		return
	}
	for idx, cid := range compIdx {
		off := offsets[idx]
		if off == 0 {
			continue
		}
		for i := 0; i < n; i++ {
			e.data[cid][i] -= off
		}
	}
}

func (e *Encoder) prepareTransformMatrix(matrix [][]float64, size int) [][]float64 {
	if matrix != nil && len(matrix) == size {
		return matrix
	}
	result := make([][]float64, size)
	for r := range result {
		result[r] = make([]float64, size)
		if r < size {
			result[r][r] = 1
		}
	}
	return result
}

func (e *Encoder) applyIntegerMatrixTransform(matrix [][]float64, compIdx []int, n int) {
	im := make([][]int32, len(compIdx))
	for r := range matrix {
		im[r] = make([]int32, len(compIdx))
		for k := range matrix[r] {
			im[r][k] = int32(matrix[r][k])
		}
	}
	for i := 0; i < n; i++ {
		out := make([]int32, len(compIdx))
		for r := range im {
			var sum int64
			for k := range im[r] {
				sum += int64(im[r][k]) * int64(e.data[compIdx[k]][i])
			}
			out[r] = int32(sum)
		}
		for r := range out {
			e.data[compIdx[r]][i] = out[r]
		}
	}
}

func (e *Encoder) applyFixedPointMatrixTransform(matrix [][]float64, compIdx []int, n int) {
	fixed := make([][]int32, len(compIdx))
	for r := range matrix {
		fixed[r] = make([]int32, len(compIdx))
		for k := range matrix[r] {
			fixed[r][k] = int32(matrix[r][k] * float64(1<<13))
		}
	}
	for i := 0; i < n; i++ {
		out := make([]int32, len(compIdx))
		for r := range fixed {
			var sum int32
			for k := range fixed[r] {
				sum += mctFixedMul(fixed[r][k], e.data[compIdx[k]][i])
			}
			out[r] = sum
		}
		for r := range out {
			e.data[compIdx[r]][i] = out[r]
		}
	}
}

func mctFixedMul(a, b int32) int32 {
	temp := int64(a)*int64(b) + 4096
	return int32(temp >> 13)
}

// writeMCTAndMCC writes Part 2 MCT/MCC/MCO markers using OpenJPEG layout.
func (e *Encoder) writeMCTAndMCC(buf *bytes.Buffer) error {
	p := e.params
	var bindings []MCTBindingParams
	if len(p.MCTBindings) > 0 {
		bindings = p.MCTBindings
	} else if p.MCTMatrix != nil && len(p.MCTMatrix) == p.Components {
		compIDs := make([]uint16, p.Components)
		for i := range compIDs {
			compIDs[i] = uint16(i)
		}
		b := MCTBindingParams{
			ComponentIDs: compIDs,
			Matrix:       p.MCTMatrix,
			Inverse:      p.InverseMCTMatrix,
			Offsets:      p.MCTOffsets,
			ElementType:  p.MCTMatrixElementType,
		}
		if p.MCTReversible {
			b.MCOPrecision = 1
		}
		bindings = []MCTBindingParams{b}
	} else {
		return nil
	}
	type mctRecord struct {
		index       uint8
		elementType codestream.MCTElementType
		arrayType   codestream.MCTArrayType
		data        []byte
	}
	type mccRecord struct {
		index       uint8
		compIDs     []uint16
		reversible  bool
		decoIndex   uint8
		offsetIndex uint8
	}
	var mctRecords []mctRecord
	var mccRecords []mccRecord
	nextID := uint8(1)
	for _, b := range bindings {
		compIDs := b.ComponentIDs
		if len(compIDs) == 0 && p.Components > 0 {
			compIDs = make([]uint16, p.Components)
			for i := range compIDs {
				compIDs[i] = uint16(i)
			}
		}
		if len(compIDs) == 0 {
			continue
		}
		et := mapMCTElementType(b.ElementType)
		inv := b.Inverse
		if inv == nil || len(inv) != len(compIDs) {
			inv = identityMatrix(len(compIDs))
		}
		data, err := encodeMCTMatrix(inv, et)
		if err != nil {
			return err
		}
		decoIndex := nextID
		nextID++
		mctRecords = append(mctRecords, mctRecord{
			index:       decoIndex,
			elementType: et,
			arrayType:   codestream.MCTArrayDecorrelate,
			data:        data,
		})
		offsetIndex := uint8(0)
		if b.Offsets != nil && len(b.Offsets) == len(compIDs) {
			offsetIndex = nextID
			nextID++
			offsetData, err := encodeMCTOffsets(b.Offsets, et)
			if err != nil {
				return err
			}
			mctRecords = append(mctRecords, mctRecord{
				index:       offsetIndex,
				elementType: et,
				arrayType:   codestream.MCTArrayOffset,
				data:        offsetData,
			})
		}
		mccIndex := nextID
		nextID++
		reversible := (b.MCOPrecision & 0x1) != 0
		mccRecords = append(mccRecords, mccRecord{
			index:       mccIndex,
			compIDs:     compIDs,
			reversible:  reversible,
			decoIndex:   decoIndex,
			offsetIndex: offsetIndex,
		})
	}
	if len(mctRecords) == 0 || len(mccRecords) == 0 {
		return nil
	}
	for _, rec := range mctRecords {
		if err := writeMCTRecord(buf, rec.index, rec.arrayType, rec.elementType, rec.data); err != nil {
			return err
		}
	}
	for _, rec := range mccRecords {
		if err := writeMCCRecord(buf, rec.index, rec.compIDs, rec.reversible, rec.decoIndex, rec.offsetIndex); err != nil {
			return err
		}
	}
	order := make([]uint8, len(mccRecords))
	for i, rec := range mccRecords {
		order[i] = rec.index
	}
	if len(p.MCORecordOrder) > 0 && validMCOOrder(p.MCORecordOrder, order) {
		order = p.MCORecordOrder
	}
	return writeMCORecord(buf, order)
}

func mapMCTElementType(t uint8) codestream.MCTElementType {
	if t == 0 {
		return codestream.MCTElementInt32
	}
	return codestream.MCTElementFloat32
}

func identityMatrix(size int) [][]float64 {
	mat := make([][]float64, size)
	for i := 0; i < size; i++ {
		mat[i] = make([]float64, size)
		mat[i][i] = 1
	}
	return mat
}

func encodeMCTMatrix(matrix [][]float64, et codestream.MCTElementType) ([]byte, error) {
	if len(matrix) == 0 {
		return nil, fmt.Errorf("empty MCT matrix")
	}
	rows := len(matrix)
	cols := len(matrix[0])
	for _, row := range matrix {
		if len(row) != cols {
			return nil, fmt.Errorf("non-rectangular MCT matrix")
		}
	}
	if rows != cols {
		return nil, fmt.Errorf("non-square MCT matrix")
	}
	elemSize := mctElementSize(et)
	if elemSize == 0 {
		return nil, fmt.Errorf("unsupported MCT element type %d", et)
	}
	buf := &bytes.Buffer{}
	buf.Grow(rows * cols * elemSize)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			switch et {
			case codestream.MCTElementInt16:
				_ = binary.Write(buf, binary.BigEndian, int16(matrix[r][c]))
			case codestream.MCTElementInt32:
				_ = binary.Write(buf, binary.BigEndian, int32(matrix[r][c]))
			case codestream.MCTElementFloat32:
				_ = binary.Write(buf, binary.BigEndian, math.Float32bits(float32(matrix[r][c])))
			case codestream.MCTElementFloat64:
				_ = binary.Write(buf, binary.BigEndian, math.Float64bits(matrix[r][c]))
			}
		}
	}
	return buf.Bytes(), nil
}

func encodeMCTOffsets(offsets []int32, et codestream.MCTElementType) ([]byte, error) {
	if len(offsets) == 0 {
		return nil, fmt.Errorf("empty MCT offsets")
	}
	elemSize := mctElementSize(et)
	if elemSize == 0 {
		return nil, fmt.Errorf("unsupported MCT element type %d", et)
	}
	buf := &bytes.Buffer{}
	buf.Grow(len(offsets) * elemSize)
	for _, off := range offsets {
		switch et {
		case codestream.MCTElementInt16:
			_ = binary.Write(buf, binary.BigEndian, int16(off))
		case codestream.MCTElementInt32:
			_ = binary.Write(buf, binary.BigEndian, off)
		case codestream.MCTElementFloat32:
			_ = binary.Write(buf, binary.BigEndian, math.Float32bits(float32(off)))
		case codestream.MCTElementFloat64:
			_ = binary.Write(buf, binary.BigEndian, math.Float64bits(float64(off)))
		}
	}
	return buf.Bytes(), nil
}

func writeMCTRecord(buf *bytes.Buffer, index uint8, arrayType codestream.MCTArrayType, elementType codestream.MCTElementType, data []byte) error {
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCT); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(8+len(data))); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(0)); err != nil {
		return err
	}
	imct := uint16(index) | (uint16(arrayType) << 8) | (uint16(elementType) << 10)
	if err := binary.Write(buf, binary.BigEndian, imct); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(0)); err != nil {
		return err
	}
	_, err := buf.Write(data)
	return err
}

func writeMCCRecord(buf *bytes.Buffer, index uint8, compIDs []uint16, reversible bool, decoIndex, offsetIndex uint8) error {
	if len(compIDs) == 0 {
		return fmt.Errorf("empty MCC component list")
	}
	compBytes := 1
	for _, id := range compIDs {
		if id > 255 {
			compBytes = 2
			break
		}
	}
	payload := &bytes.Buffer{}
	_ = binary.Write(payload, binary.BigEndian, uint16(0)) // Zmcc
	payload.WriteByte(index)                               // Imcc
	_ = binary.Write(payload, binary.BigEndian, uint16(0)) // Ymcc
	_ = binary.Write(payload, binary.BigEndian, uint16(1)) // Qmcc
	payload.WriteByte(1)                                   // Xmcci

	nmcci := uint16(len(compIDs))
	if compBytes == 2 {
		nmcci |= 0x8000
	}
	_ = binary.Write(payload, binary.BigEndian, nmcci)
	for _, id := range compIDs {
		if compBytes == 1 {
			payload.WriteByte(uint8(id))
		} else {
			_ = binary.Write(payload, binary.BigEndian, id)
		}
	}
	mmcci := uint16(len(compIDs))
	if compBytes == 2 {
		mmcci |= 0x8000
	}
	_ = binary.Write(payload, binary.BigEndian, mmcci)
	for _, id := range compIDs {
		if compBytes == 1 {
			payload.WriteByte(uint8(id))
		} else {
			_ = binary.Write(payload, binary.BigEndian, id)
		}
	}
	tmcc := uint32(decoIndex) | (uint32(offsetIndex) << 8)
	if reversible {
		tmcc |= 0x1 << 16
	}
	payload.WriteByte(byte((tmcc >> 16) & 0xFF))
	payload.WriteByte(byte((tmcc >> 8) & 0xFF))
	payload.WriteByte(byte(tmcc & 0xFF))

	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCC); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(payload.Len()+2)); err != nil {
		return err
	}
	_, err := buf.Write(payload.Bytes())
	return err
}

func writeMCORecord(buf *bytes.Buffer, order []uint8) error {
	if len(order) == 0 {
		return nil
	}
	if len(order) > 255 {
		return fmt.Errorf("too many MCO stages: %d", len(order))
	}
	payload := &bytes.Buffer{}
	payload.WriteByte(uint8(len(order)))
	for _, id := range order {
		payload.WriteByte(id)
	}
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCO); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(payload.Len()+2)); err != nil {
		return err
	}
	_, err := buf.Write(payload.Bytes())
	return err
}

func validMCOOrder(order []uint8, allowed []uint8) bool {
	if len(order) != len(allowed) {
		return false
	}
	allow := make(map[uint8]struct{}, len(allowed))
	for _, id := range allowed {
		allow[id] = struct{}{}
	}
	for _, id := range order {
		if _, ok := allow[id]; !ok {
			return false
		}
	}
	return true
}

func mccIndicesForBindings(bindings []MCTBindingParams, components int) []uint8 {
	if len(bindings) == 0 {
		return nil
	}
	nextID := uint8(1)
	indices := make([]uint8, len(bindings))
	for i, b := range bindings {
		compIDs := b.ComponentIDs
		if len(compIDs) == 0 && components > 0 {
			compIDs = make([]uint16, components)
			for j := range compIDs {
				compIDs[j] = uint16(j)
			}
		}
		nextID++ // decorrelation index
		if b.Offsets != nil && len(b.Offsets) == len(compIDs) {
			nextID++ // offset index
		}
		indices[i] = nextID
		nextID++ // MCC index
	}
	return indices
}

func bindingOrderForMCO(bindings []MCTBindingParams, components int, order []uint8) []int {
	if len(bindings) == 0 || len(order) == 0 {
		return nil
	}
	nextID := uint8(1)
	mccIdx := make([]uint8, len(bindings))
	for i, b := range bindings {
		compIDs := b.ComponentIDs
		if len(compIDs) == 0 && components > 0 {
			compIDs = make([]uint16, components)
			for j := range compIDs {
				compIDs[j] = uint16(j)
			}
		}
		nextID++ // decorrelation index
		if b.Offsets != nil && len(b.Offsets) == len(compIDs) {
			nextID++ // offset index
		}
		mccIdx[i] = nextID
		nextID++ // MCC index
	}
	result := make([]int, 0, len(bindings))
	used := make([]bool, len(bindings))
	for _, id := range order {
		for i, idx := range mccIdx {
			if idx == id && !used[i] {
				result = append(result, i)
				used[i] = true
				break
			}
		}
	}
	for i := range bindings {
		if !used[i] {
			result = append(result, i)
		}
	}
	return result
}

// resolveROI normalizes ROI inputs (legacy ROI or ROIConfig) into internal slices.
func (e *Encoder) resolveROI() error {
	e.roiShifts = nil
	e.RoiRects = nil
	e.roiStyles = nil
	e.roiMasks = nil

	// ROIConfig takes priority when present
	if e.params.ROIConfig != nil && !e.params.ROIConfig.IsEmpty() {
		style, shifts, rectsByComp, err := e.params.ROIConfig.ResolveRectangles(e.params.Width, e.params.Height, e.params.Components)
		if err != nil {
			return err
		}
		e.roiShifts = shifts
		e.RoiRects = rectsByComp
		e.roiMasks = buildMasksFromConfig(e.params.Width, e.params.Height, e.params.Components, rectsByComp, e.params.ROIConfig)
		if len(shifts) > 0 {
			e.roiStyles = make([]byte, len(shifts))
			for i := range e.roiStyles {
				e.roiStyles[i] = style
			}
		}
		return nil
	}

	// Legacy single-rectangle ROI
	if e.params.ROI != nil {
		if !e.params.ROI.IsValid(e.params.Width, e.params.Height) {
			return fmt.Errorf("invalid ROI parameters: %+v", *e.params.ROI)
		}
		e.roiShifts = make([]int, e.params.Components)
		e.RoiRects = make([][]RoiRect, e.params.Components)
		e.roiStyles = make([]byte, e.params.Components)
		e.roiMasks = make([]*roiMask, e.params.Components)
		for c := 0; c < e.params.Components; c++ {
			e.roiShifts[c] = e.params.ROI.Shift
			e.roiStyles[c] = 0
			e.RoiRects[c] = []RoiRect{{
				x0: e.params.ROI.X0,
				y0: e.params.ROI.Y0,
				x1: e.params.ROI.X0 + e.params.ROI.Width,
				y1: e.params.ROI.Y0 + e.params.ROI.Height,
			}}
			e.roiMasks[c] = newROIMask(e.params.Width, e.params.Height)
			e.roiMasks[c].setRect(e.params.ROI.X0, e.params.ROI.Y0, e.params.ROI.X0+e.params.ROI.Width, e.params.ROI.Y0+e.params.ROI.Height)
		}
	}

	return nil
}

// writeSIZ writes the SIZ (Image and Tile Size) segment
func (e *Encoder) writeSIZ(buf *bytes.Buffer) error {
	p := e.params

	sizData := &bytes.Buffer{}

	rsiz := uint16(0)
	if p.HTJ2KMode {
		rsiz = 0x4000
	}
	if err := binary.Write(sizData, binary.BigEndian, rsiz); err != nil {
		return err
	}

	// Xsiz, Ysiz - Image size
	if err := binary.Write(sizData, binary.BigEndian, uint32(p.Width)); err != nil {
		return err
	}
	if err := binary.Write(sizData, binary.BigEndian, uint32(p.Height)); err != nil {
		return err
	}

	// XOsiz, YOsiz - Image offset
	if err := binary.Write(sizData, binary.BigEndian, uint32(0)); err != nil {
		return err
	}
	if err := binary.Write(sizData, binary.BigEndian, uint32(0)); err != nil {
		return err
	}

	// XTsiz, YTsiz - Tile size
	tileWidth := p.TileWidth
	tileHeight := p.TileHeight
	if tileWidth == 0 {
		tileWidth = p.Width
	}
	if tileHeight == 0 {
		tileHeight = p.Height
	}
	if err := binary.Write(sizData, binary.BigEndian, uint32(tileWidth)); err != nil {
		return err
	}
	if err := binary.Write(sizData, binary.BigEndian, uint32(tileHeight)); err != nil {
		return err
	}

	// XTOsiz, YTOsiz - Tile offset
	if err := binary.Write(sizData, binary.BigEndian, uint32(0)); err != nil {
		return err
	}
	if err := binary.Write(sizData, binary.BigEndian, uint32(0)); err != nil {
		return err
	}

	// Csiz - Number of components
	if err := binary.Write(sizData, binary.BigEndian, uint16(p.Components)); err != nil {
		return err
	}

	// Component information
	ssiz := uint8(p.BitDepth - 1)
	if p.IsSigned {
		ssiz |= 0x80
	}
	for i := 0; i < p.Components; i++ {
		if err := binary.Write(sizData, binary.BigEndian, ssiz); err != nil {
			return err
		}
		if err := binary.Write(sizData, binary.BigEndian, uint8(1)); err != nil {
			return err
		}
		if err := binary.Write(sizData, binary.BigEndian, uint8(1)); err != nil {
			return err
		}
	}

	// Write marker and length
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerSIZ); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(sizData.Len()+2)); err != nil {
		return err
	}
	if _, err := buf.Write(sizData.Bytes()); err != nil {
		return err
	}

	return nil
}

func (e *Encoder) writeCAP(buf *bytes.Buffer) error {
	if !e.params.HTJ2KMode {
		return nil
	}

	capData := &bytes.Buffer{}
	if err := binary.Write(capData, binary.BigEndian, uint32(0x00020000)); err != nil {
		return err
	}
	if err := binary.Write(capData, binary.BigEndian, uint16(0)); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerCAP); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(capData.Len()+2)); err != nil {
		return err
	}
	_, err := buf.Write(capData.Bytes())
	return err
}

// writeCOD writes the COD (Coding Style Default) segment
func (e *Encoder) writeCOD(buf *bytes.Buffer) error {
	p := e.params

	codData := &bytes.Buffer{}

	// Scod - Coding style parameters
	// Bit 0: Precinct defined (1 if custom precinct sizes are used)
	scod := uint8(0)
	if p.PrecinctWidth > 0 || p.PrecinctHeight > 0 {
		scod |= 0x01 // Enable precinct sizes
	}
	if err := binary.Write(codData, binary.BigEndian, scod); err != nil {
		return err
	}

	// SGcod - Progression order and layers
	if err := binary.Write(codData, binary.BigEndian, p.ProgressionOrder); err != nil {
		return err
	}
	if err := binary.Write(codData, binary.BigEndian, uint16(p.NumLayers)); err != nil {
		return err
	}

	// MCT - Multiple component transformation (1 for RGB, 0 for grayscale)
	mct := uint8(0)
	if p.EnableMCT && (len(p.MCTBindings) > 0 || (p.MCTMatrix != nil && len(p.MCTMatrix) == p.Components) || p.Components >= 3) {
		mct = 1
	}
	if err := binary.Write(codData, binary.BigEndian, mct); err != nil {
		return err
	}

	// SPcod - Decomposition levels and code-block size
	if err := binary.Write(codData, binary.BigEndian, uint8(p.NumLevels)); err != nil {
		return err
	}

	// Code-block size (log2(width) - 2, log2(height) - 2)
	cbWidthExp := uint8(log2(p.CodeBlockWidth) - 2)
	cbHeightExp := uint8(log2(p.CodeBlockHeight) - 2)
	if err := binary.Write(codData, binary.BigEndian, cbWidthExp); err != nil {
		return err
	}
	if err := binary.Write(codData, binary.BigEndian, cbHeightExp); err != nil {
		return err
	}

	codeBlockStyle := uint8(0)
	if p.HTJ2KMode {
		codeBlockStyle |= 0x40
	}
	if err := binary.Write(codData, binary.BigEndian, codeBlockStyle); err != nil {
		return err
	}

	// Transformation (0 = 9/7 irreversible, 1 = 5/3 reversible)
	transform := uint8(1)
	if !p.Lossless {
		transform = 0
	}
	if err := binary.Write(codData, binary.BigEndian, transform); err != nil {
		return err
	}

	// Write precinct sizes if enabled (Scod bit 0 = 1)
	if scod&0x01 != 0 {
		// One precinct size per resolution level (numLevels + 1)
		numResolutions := p.NumLevels + 1
		for r := 0; r < numResolutions; r++ {
			// Calculate precinct size for this resolution level
			// Default precinct size is 2^15 (32768) if not specified
			ppx, ppy := e.getPrecinctSizeExponents(r)

			// Pack PPx and PPy into single byte: PPy (high 4 bits) | PPx (low 4 bits)
			ppxppy := (ppy << 4) | ppx
			if err := binary.Write(codData, binary.BigEndian, ppxppy); err != nil {
				return err
			}
		}
	}

	// Write marker and length
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerCOD); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(codData.Len()+2)); err != nil {
		return err
	}
	if _, err := buf.Write(codData.Bytes()); err != nil {
		return err
	}

	return nil
}

// getPrecinctSize returns the actual precinct dimensions (in pixels) for a given resolution level
func (e *Encoder) getPrecinctSize(resolutionLevel int) (width, height int) {
	ppx, ppy := e.getPrecinctSizeExponents(resolutionLevel)
	return 1 << ppx, 1 << ppy
}

// calculatePrecinctIndex calculates the precinct index for a code-block
// based on its position within the resolution level
// In JPEG 2000, precincts are defined at the resolution level, and all subbands
// at the same resolution share the same precinct partitioning
// cbX0, cbY0 are in the **resolution reference grid** (not global wavelet space)
func (e *Encoder) calculatePrecinctIndex(cbX0, cbY0, resolutionLevel int) int {
	// Get precinct dimensions for this resolution
	precinctWidth, precinctHeight := e.getPrecinctSize(resolutionLevel)

	// Get resolution dimensions
	resWidth, _ := e.getResolutionDimensions(resolutionLevel)

	// Calculate precinct grid position
	px := cbX0 / precinctWidth
	py := cbY0 / precinctHeight

	// Calculate number of precincts in X direction based on resolution dimensions
	numPrecinctX := (resWidth + precinctWidth - 1) / precinctWidth

	// Linear precinct index
	return py*numPrecinctX + px
}

// toResolutionCoordinates converts global wavelet coordinates to resolution reference grid coordinates
// For resolution 0 (LL subband), the coordinates are already in the correct space
// For resolution > 0, we need to map based on the subband type:
//
//	HL (band=1): coordinates are at offset (subbandWidth, 0)
//	LH (band=2): coordinates are at offset (0, subbandHeight)
//	HH (band=3): coordinates are at offset (subbandWidth, subbandHeight)
func (e *Encoder) toResolutionCoordinates(globalX, globalY, resolutionLevel, band int) (int, int) {
	if resolutionLevel == 0 {
		// LL subband - coordinates are already correct
		return globalX, globalY
	}

	// For resolution > 0, get subband dimensions
	subbandWidth, subbandHeight := e.getSubbandDimensions(resolutionLevel)

	// Map coordinates based on subband type
	// In the wavelet transform, subbands are laid out as:
	// +----+----+
	// | LL | HL |
	// +----+----+
	// | LH | HH |
	// +----+----+
	// So we need to subtract the subband offset to get resolution-local coordinates
	resX := globalX
	resY := globalY

	switch band {
	case 0: // LL (shouldn't happen for res > 0)
		// Already correct
	case 1: // HL (high-low) - right of LL
		resX = globalX - subbandWidth
	case 2: // LH (low-high) - below LL
		resY = globalY - subbandHeight
	case 3: // HH (high-high) - diagonal from LL
		resX = globalX - subbandWidth
		resY = globalY - subbandHeight
	}

	return resX, resY
}

// getSubbandDimensions returns the dimensions of a subband at a resolution level
func (e *Encoder) getSubbandDimensions(resolutionLevel int) (width, height int) {
	// For resolution level r:
	// - r=0: LL subband dimensions = image / (2^numLevels)
	// - r>0: HL/LH/HH subband dimensions = image / (2^(numLevels - r + 1))
	//
	// Simplified: All subbands at any resolution have the same calculation
	// subbandWidth = imageWidth >> (numLevels - resolutionLevel + 1) for res > 0
	// subbandWidth = imageWidth >> numLevels for res == 0

	if resolutionLevel == 0 {
		// LL subband
		width = ceilDivPow2(e.params.Width, e.params.NumLevels)
		height = ceilDivPow2(e.params.Height, e.params.NumLevels)
	} else {
		// HL/LH/HH subbands
		// These come from decomposition level (numLevels - resolutionLevel + 1)
		level := e.params.NumLevels - resolutionLevel + 1
		if level < 0 {
			level = 0
		}
		width = ceilDivPow2(e.params.Width, level)
		height = ceilDivPow2(e.params.Height, level)
	}

	// Ensure minimum size of 1
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	return width, height
}

// getResolutionDimensions returns the dimensions of a resolution level
func (e *Encoder) getResolutionDimensions(resolutionLevel int) (width, height int) {
	// For resolution level r:
	// - r=0: LL subband (lowest resolution) = image / (2^numLevels)
	// - r=1: includes HL/LH/HH at decomposition level (numLevels-1) = image / (2^(numLevels-1))
	// - r=numLevels: highest resolution = full image
	//
	// Formula: width = imageWidth / (2^(numLevels - resolutionLevel))

	divisor := e.params.NumLevels - resolutionLevel
	if divisor < 0 {
		divisor = 0
	}

	width = ceilDivPow2(e.params.Width, divisor)
	height = ceilDivPow2(e.params.Height, divisor)

	// Ensure minimum size of 1
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	return width, height
}

// ceilDivPow2 computes ceil(n / 2^pow) for pow >= 0.
func ceilDivPow2(n, pow int) int {
	if pow <= 0 {
		return n
	}
	divisor := 1 << pow
	return (n + divisor - 1) / divisor
}

// getPrecinctSizeExponents returns the precinct size exponents (PPx, PPy) for a given resolution level
// PPx and PPy are stored as log2 values (e.g., PPx=7 means width=2^7=128)
// ISO/IEC 15444-1 specifies that precinct sizes should be adjusted per resolution level
func (e *Encoder) getPrecinctSizeExponents(resolutionLevel int) (ppx, ppy uint8) {
	p := e.params

	// Default precinct size is 2^15 (32768) if not specified
	precinctWidth := p.PrecinctWidth
	precinctHeight := p.PrecinctHeight

	if precinctWidth == 0 {
		precinctWidth = 1 << 15 // 32768
	}
	if precinctHeight == 0 {
		precinctHeight = 1 << 15 // 32768
	}

	// Calculate exponents (log2)
	ppx = uint8(log2(precinctWidth))
	ppy = uint8(log2(precinctHeight))

	customPrecincts := p.PrecinctWidth > 0 || p.PrecinctHeight > 0
	if customPrecincts {
		// OpenJPEG scales precinct sizes down by 2 for each lower resolution.
		shift := p.NumLevels - resolutionLevel
		if shift < 0 {
			shift = 0
		}
		if shift > 0 {
			ppxInt := int(ppx) - shift
			if ppxInt < 0 {
				ppxInt = 0
			}
			ppyInt := int(ppy) - shift
			if ppyInt < 0 {
				ppyInt = 0
			}
			ppx = uint8(ppxInt)
			ppy = uint8(ppyInt)
		}
	}

	// PPx and PPy must be at least 0 (meaning 2^0 = 1 pixel minimum)
	// and at most 15 (meaning 2^15 = 32768 pixels maximum)
	if ppx > 15 {
		ppx = 15
	}
	if ppy > 15 {
		ppy = 15
	}

	return ppx, ppy
}

type quantizationInfo struct {
	style     int
	guardBits int
	expn      []int
	steps     []uint16
}

func (e *Encoder) quantizationInfo() quantizationInfo {
	if e.qcdReady {
		return quantizationInfo{
			style:     e.qcdStyle,
			guardBits: e.qcdGuard,
			expn:      e.qcdExpn,
			steps:     e.qcdSteps,
		}
	}

	p := e.params
	info := quantizationInfo{}

	if p.Lossless {
		info.style = 0
		info.guardBits = 2
		expn := make([]int, 0, 3*p.NumLevels+1)
		for res := 0; res <= p.NumLevels; res++ {
			if res == 0 {
				expn = append(expn, p.BitDepth+losslessLog2Gain(res, 0))
			} else {
				expn = append(expn, p.BitDepth+losslessLog2Gain(res, 1))
				expn = append(expn, p.BitDepth+losslessLog2Gain(res, 2))
				expn = append(expn, p.BitDepth+losslessLog2Gain(res, 3))
			}
		}
		info.expn = expn
	} else {
		info.style = 2
		guardBits := 2
		quantParams := e.lossyQuantizationParams()
		guardBits = quantParams.GuardBits
		steps := quantParams.EncodedSteps
		expn := make([]int, len(steps))
		for i, step := range steps {
			expn[i] = int((step >> 11) & 0x1F)
		}
		info.guardBits = guardBits
		info.expn = expn
		info.steps = steps
	}

	e.qcdReady = true
	e.qcdStyle = info.style
	e.qcdGuard = info.guardBits
	e.qcdExpn = info.expn
	e.qcdSteps = info.steps

	return info
}

func (e *Encoder) lossyQuantizationParams() *QuantizationParams {
	if len(e.params.CustomQuantSteps) > 0 {
		steps := encodeQuantStepsFromFloats(e.params.CustomQuantSteps, e.params.BitDepth)
		return &QuantizationParams{
			Style:        2,
			GuardBits:    2,
			StepSizes:    append([]float64(nil), e.params.CustomQuantSteps...),
			EncodedSteps: steps,
		}
	}
	if len(e.params.LayerRates) > 0 {
		return CalculateOpenJPEGQuantizationParams(e.params.NumLevels, e.params.BitDepth)
	}
	return CalculateQuantizationParams(e.params.Quality, e.params.NumLevels, e.params.BitDepth)
}

func losslessLog2Gain(res, band int) int {
	if res == 0 {
		return 0
	}
	if band == 3 {
		return 2
	}
	return 1
}

func subbandIndex(numLevels, res, band int) int {
	if res < 0 || res > numLevels {
		return -1
	}
	if res == 0 {
		if band != 0 {
			return -1
		}
		return 0
	}
	if band < 1 || band > 3 {
		return -1
	}
	return 1 + (res-1)*3 + (band - 1)
}

func (e *Encoder) bandNumbps(res, band int) int {
	info := e.quantizationInfo()
	idx := subbandIndex(e.params.NumLevels, res, band)
	if idx < 0 || idx >= len(info.expn) {
		return 0
	}
	return info.expn[idx] + info.guardBits - 1
}

// writeQCD writes the QCD (Quantization Default) segment
func (e *Encoder) writeQCD(buf *bytes.Buffer) error {
	info := e.quantizationInfo()

	qcdData := &bytes.Buffer{}

	if e.params.Lossless {
		// Lossless mode: no quantization (style 0)
		// Sqcd - bits 0-4: quantization type, bits 5-7: guard bits
		// Match OpenJPEG: qntsty + (numgbits << 5)
		sqcd := uint8(info.guardBits << 5)
		if err := binary.Write(qcdData, binary.BigEndian, sqcd); err != nil {
			return err
		}

		// SPqcd - Quantization step size for each subband
		// For lossless: exponent only (8 bits), no mantissa
		// Values are shifted left by 3 bits when encoded
		for _, expn := range info.expn {
			if err := binary.Write(qcdData, binary.BigEndian, uint8(expn<<3)); err != nil {
				return err
			}
		}
	} else {
		// Lossy mode: scalar expounded quantization (style 2)
		// Sqcd - bits 0-4: quantization type (2 = scalar expounded), bits 5-7: guard bits
		sqcd := uint8((info.guardBits << 5) | (info.style & 0x1F))
		if err := binary.Write(qcdData, binary.BigEndian, sqcd); err != nil {
			return err
		}

		// SPqcd - Quantization step sizes for each subband
		// For scalar expounded: 16-bit value per subband (5-bit exponent, 11-bit mantissa)
		for _, encodedStep := range info.steps {
			_ = binary.Write(qcdData, binary.BigEndian, encodedStep)
		}
	}

	// Write marker and length
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerQCD); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(qcdData.Len()+2)); err != nil {
		return err
	}
	if _, err := buf.Write(qcdData.Bytes()); err != nil {
		return err
	}

	return nil
}

// writeRGN writes ROI (Region of Interest) marker segments (main header) if ROI is enabled.
// Baseline: MaxShift, one RGN per component, Crgn fits in 1 byte.
func (e *Encoder) writeRGN(buf *bytes.Buffer) error {
	if len(e.roiShifts) == 0 {
		return nil
	}

	for comp := 0; comp < e.params.Components; comp++ {
		shift := 0
		if comp < len(e.roiShifts) {
			shift = e.roiShifts[comp]
		}
		if shift <= 0 {
			continue
		}

		style := byte(0)
		if comp < len(e.roiStyles) {
			style = e.roiStyles[comp]
		}

		segment := &bytes.Buffer{}
		// Lrgn = 5 (length includes itself)
		_ = binary.Write(segment, binary.BigEndian, uint16(5))
		segment.WriteByte(byte(comp))  // Crgn
		segment.WriteByte(style)       // Srgn: 0 = MaxShift, 1 = General Scaling
		segment.WriteByte(byte(shift)) // SPrgn

		if err := binary.Write(buf, binary.BigEndian, codestream.MarkerRGN); err != nil {
			return err
		}
		if _, err := buf.Write(segment.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

// writeVersionCOM writes a COM (Comment) marker with version information.
// This matches OpenJPEG's behavior of including a version string.
func (e *Encoder) writeVersionCOM(buf *bytes.Buffer) error {
	const version = "Created by OpenJPEG version 2.5.4"

	data := &bytes.Buffer{}

	// Rcom = 0x0001 (Binary data, Latin alphabet)
	if err := binary.Write(data, binary.BigEndian, uint16(0x0001)); err != nil {
		return err
	}

	// Comment text
	data.WriteString(version)

	// Write marker and length
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerCOM); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(data.Len()+2)); err != nil {
		return err
	}
	if _, err := buf.Write(data.Bytes()); err != nil {
		return err
	}

	return nil
}

// writeCOM writes a COM (Comment) marker with private ROI metadata.
// This allows the decoder to reconstruct ROI geometry without external parameters.
// Format: Magic("JP2ROI") + Version(1) + ROI count + ROI geometries
func (e *Encoder) writeCOM(buf *bytes.Buffer) error {
	// Only write COM if we have ROI configuration
	if e.params.ROIConfig == nil || e.params.ROIConfig.IsEmpty() {
		return nil
	}

	data := &bytes.Buffer{}

	// Magic string: "JP2ROI" (6 bytes)
	data.WriteString("JP2ROI")

	// Version: 1 (1 byte)
	data.WriteByte(1)

	// Number of ROI regions (2 bytes)
	_ = binary.Write(data, binary.BigEndian, uint16(len(e.params.ROIConfig.ROIs)))

	// Encode each ROI region
	for _, roi := range e.params.ROIConfig.ROIs {
		// Shape type: 0=Rectangle, 1=Polygon, 2=Mask (1 byte)
		var shapeType byte
		if roi.Rect != nil {
			shapeType = 0
		} else if len(roi.Polygon) > 0 {
			shapeType = 1
		} else if roi.MaskData != nil {
			shapeType = 2
		}
		data.WriteByte(shapeType)

		// Number of components (1 byte)
		numComps := len(roi.Components)
		if numComps == 0 {
			numComps = e.params.Components // All components
		}
		data.WriteByte(byte(numComps))

		// Component indices
		if len(roi.Components) > 0 {
			for _, comp := range roi.Components {
				data.WriteByte(byte(comp))
			}
		} else {
			// All components
			for c := 0; c < e.params.Components; c++ {
				data.WriteByte(byte(c))
			}
		}

		// Geometry data based on shape type
		switch shapeType {
		case 0: // Rectangle
			_ = binary.Write(data, binary.BigEndian, uint32(roi.Rect.X0))
			_ = binary.Write(data, binary.BigEndian, uint32(roi.Rect.Y0))
			_ = binary.Write(data, binary.BigEndian, uint32(roi.Rect.X0+roi.Rect.Width))
			_ = binary.Write(data, binary.BigEndian, uint32(roi.Rect.Y0+roi.Rect.Height))
		case 1: // Polygon
			_ = binary.Write(data, binary.BigEndian, uint16(len(roi.Polygon)))
			for _, pt := range roi.Polygon {
				_ = binary.Write(data, binary.BigEndian, uint32(pt.X))
				_ = binary.Write(data, binary.BigEndian, uint32(pt.Y))
			}
		case 2: // Mask (don't store raw mask, too large - store dimensions only as placeholder)
			_ = binary.Write(data, binary.BigEndian, uint32(roi.MaskWidth))
			_ = binary.Write(data, binary.BigEndian, uint32(roi.MaskHeight))
			// Note: Actual mask data not stored in COM (too large)
			// Decoder needs external mask or should use rectangle/polygon instead
		}
	}

	// Write COM marker
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerCOM); err != nil {
		return err
	}

	// Write length (2 bytes for length itself + 2 bytes for Rcom + data)
	length := uint16(2 + 2 + data.Len())
	if err := binary.Write(buf, binary.BigEndian, length); err != nil {
		return err
	}

	// Write Rcom (Registration value): 0x0001 for binary data (ISO/IEC 8859-15)
	// We use 0x0000 for private binary format
	if err := binary.Write(buf, binary.BigEndian, uint16(0x0000)); err != nil {
		return err
	}

	// Write data
	if _, err := buf.Write(data.Bytes()); err != nil {
		return err
	}

	return nil
}

// writeTileRGN writes ROI marker segments in tile-part header.
// This allows tile-specific ROI information (optional enhancement).
func (e *Encoder) writeTileRGN(buf *bytes.Buffer) error {
	// For now, write the same RGN as main header
	// In the future, this could support tile-specific ROI regions
	if len(e.roiShifts) == 0 {
		return nil
	}

	for comp := 0; comp < e.params.Components; comp++ {
		shift := 0
		if comp < len(e.roiShifts) {
			shift = e.roiShifts[comp]
		}
		if shift <= 0 {
			continue
		}

		style := byte(0)
		if comp < len(e.roiStyles) {
			style = e.roiStyles[comp]
		}

		segment := &bytes.Buffer{}
		// Lrgn = 5 (length includes itself)
		_ = binary.Write(segment, binary.BigEndian, uint16(5))
		segment.WriteByte(byte(comp))  // Crgn
		segment.WriteByte(style)       // Srgn: 0 = MaxShift, 1 = General Scaling
		segment.WriteByte(byte(shift)) // SPrgn

		if err := binary.Write(buf, binary.BigEndian, codestream.MarkerRGN); err != nil {
			return err
		}
		if _, err := buf.Write(segment.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

type tileEncoding struct {
	idx       int
	width     int
	height    int
	packetEnc *t2.PacketEncoder
	blocks    []*t2.PrecinctCodeBlock
}

func (e *Encoder) tileBounds(tileIdx, tileWidth, tileHeight, numTilesX int) (int, int, int, int) {
	tileX := tileIdx % numTilesX
	tileY := tileIdx / numTilesX

	x0 := tileX * tileWidth
	y0 := tileY * tileHeight
	x1 := x0 + tileWidth
	y1 := y0 + tileHeight

	if x1 > e.params.Width {
		x1 = e.params.Width
	}
	if y1 > e.params.Height {
		y1 = e.params.Height
	}

	return x0, y0, x1, y1
}

// writeTiles writes all tile data
func (e *Encoder) writeTiles(buf *bytes.Buffer) error {
	p := e.params

	// Calculate tile dimensions
	tileWidth := p.TileWidth
	tileHeight := p.TileHeight
	if tileWidth == 0 {
		tileWidth = p.Width
	}
	if tileHeight == 0 {
		tileHeight = p.Height
	}

	numTilesX := (p.Width + tileWidth - 1) / tileWidth
	numTilesY := (p.Height + tileHeight - 1) / tileHeight
	numTiles := numTilesX * numTilesY
	e.openJPEGNumTiles = numTiles

	useGlobalPCRD := numTiles > 1 && (e.params.NumLayers > 1 || e.params.TargetRatio > 0)
	if useGlobalPCRD {
		return e.writeTilesWithGlobalRateDistortion(buf, tileWidth, tileHeight, numTilesX, numTiles)
	}

	// Write each tile
	for tileIdx := 0; tileIdx < numTiles; tileIdx++ {
		if err := e.writeTile(buf, tileIdx, tileWidth, tileHeight, numTilesX); err != nil {
			return fmt.Errorf("failed to write tile %d: %w", tileIdx, err)
		}
	}

	return nil
}

// writeTilesWithGlobalRateDistortion performs global PCRD allocation across tiles.
func (e *Encoder) writeTilesWithGlobalRateDistortion(buf *bytes.Buffer, tileWidth, tileHeight, numTilesX, numTiles int) error {
	tileEncodings := make([]tileEncoding, 0, numTiles)
	allBlocks := make([]*t2.PrecinctCodeBlock, 0)
	packetEncs := make([]*t2.PacketEncoder, 0, numTiles)

	for tileIdx := 0; tileIdx < numTiles; tileIdx++ {
		x0, y0, x1, y1 := e.tileBounds(tileIdx, tileWidth, tileHeight, numTilesX)
		actualWidth := x1 - x0
		actualHeight := y1 - y0

		// Extract tile data
		tileData := make([][]int32, e.params.Components)
		for c := 0; c < e.params.Components; c++ {
			tileData[c] = make([]int32, actualWidth*actualHeight)
			for ty := 0; ty < actualHeight; ty++ {
				srcIdx := (y0+ty)*e.params.Width + x0
				dstIdx := ty * actualWidth
				copy(tileData[c][dstIdx:dstIdx+actualWidth], e.data[c][srcIdx:srcIdx+actualWidth])
			}
		}

		// Apply wavelet transform
		transformedData := e.applyWaveletTransform(tileData, actualWidth, actualHeight, x0, y0)

		packetEnc, blocks := e.buildTilePacketEncoder(transformedData, actualWidth, actualHeight)
		tileEncodings = append(tileEncodings, tileEncoding{
			idx:       tileIdx,
			width:     actualWidth,
			height:    actualHeight,
			packetEnc: packetEnc,
			blocks:    blocks,
		})
		allBlocks = append(allBlocks, blocks...)
		packetEncs = append(packetEncs, packetEnc)
	}

	if e.params.NumLayers > 1 || e.params.TargetRatio > 0 {
		origBytes := e.params.Width * e.params.Height * e.params.Components * ((e.params.BitDepth + 7) / 8)
		e.applyRateDistortionGlobal(allBlocks, packetEncs, origBytes, numTiles)
	}

	for _, tile := range tileEncodings {
		tileBytes := []byte{0x00}
		if tile.packetEnc != nil {
			tile.packetEnc.ResetState()
			packets, err := tile.packetEnc.EncodePackets()
			if err == nil {
				tileBytes = e.packetsToBytes(packets)
			}
		}

		tileHeader := &bytes.Buffer{}
		if err := e.writeTileRGN(tileHeader); err != nil {
			return fmt.Errorf("failed to write tile-part RGN: %w", err)
		}

		if err := binary.Write(buf, binary.BigEndian, codestream.MarkerSOT); err != nil {
			return err
		}
		_ = binary.Write(buf, binary.BigEndian, uint16(10)) // Lsot

		_ = binary.Write(buf, binary.BigEndian, uint16(tile.idx)) // Isot
		tilePartLength := len(tileBytes) + tileHeader.Len() + 14  // SOT(12) + header + SOD(2) + data
		_ = binary.Write(buf, binary.BigEndian, uint32(tilePartLength))
		_ = binary.Write(buf, binary.BigEndian, uint8(0)) // TPsot
		_ = binary.Write(buf, binary.BigEndian, uint8(1)) // TNsot

		if _, err := buf.Write(tileHeader.Bytes()); err != nil {
			return err
		}

		if err := binary.Write(buf, binary.BigEndian, codestream.MarkerSOD); err != nil {
			return err
		}

		if _, err := buf.Write(tileBytes); err != nil {
			return err
		}
	}

	return nil
}

// writeTile writes a single tile
func (e *Encoder) writeTile(buf *bytes.Buffer, tileIdx, tileWidth, tileHeight, numTilesX int) error {
	// Calculate tile bounds
	x0, y0, x1, y1 := e.tileBounds(tileIdx, tileWidth, tileHeight, numTilesX)

	actualWidth := x1 - x0
	actualHeight := y1 - y0

	// Extract tile data
	tileData := make([][]int32, e.params.Components)
	for c := 0; c < e.params.Components; c++ {
		tileData[c] = make([]int32, actualWidth*actualHeight)
		for ty := 0; ty < actualHeight; ty++ {
			srcIdx := (y0+ty)*e.params.Width + x0
			dstIdx := ty * actualWidth
			copy(tileData[c][dstIdx:dstIdx+actualWidth], e.data[c][srcIdx:srcIdx+actualWidth])
		}
	}

	// Apply wavelet transform
	transformedData := e.applyWaveletTransform(tileData, actualWidth, actualHeight, x0, y0)

	// Encode tile data
	tileBytes := e.encodeTileData(transformedData, actualWidth, actualHeight)

	// Build tile-part header (e.g., RGN) to compute Psot correctly
	tileHeader := &bytes.Buffer{}
	if err := e.writeTileRGN(tileHeader); err != nil {
		return fmt.Errorf("failed to write tile-part RGN: %w", err)
	}

	// Write SOT (Start of Tile)
	_ = binary.Write(buf, binary.BigEndian, codestream.MarkerSOT)
	_ = binary.Write(buf, binary.BigEndian, uint16(10)) // Lsot

	_ = binary.Write(buf, binary.BigEndian, uint16(tileIdx)) // Isot
	tilePartLength := len(tileBytes) + tileHeader.Len() + 14 // SOT(12) + header + SOD(2) + data
	_ = binary.Write(buf, binary.BigEndian, uint32(tilePartLength))
	_ = binary.Write(buf, binary.BigEndian, uint8(0)) // TPsot
	_ = binary.Write(buf, binary.BigEndian, uint8(1)) // TNsot

	// Write tile-part header (e.g., RGN)
	if _, err := buf.Write(tileHeader.Bytes()); err != nil {
		return err
	}

	// Write SOD (Start of Data)
	_ = binary.Write(buf, binary.BigEndian, codestream.MarkerSOD)

	// Write tile data
	buf.Write(tileBytes)

	return nil
}

// applyWaveletTransform applies wavelet transform to tile data
func (e *Encoder) applyWaveletTransform(tileData [][]int32, width, height, x0, y0 int) [][]int32 {
	if e.params.NumLevels == 0 {
		// No transform
		return tileData
	}

	if e.params.Lossless {
		// Apply 5/3 reversible wavelet transform (lossless)
		transformed := make([][]int32, len(tileData))
		for c := 0; c < len(tileData); c++ {
			// Copy component data
			transformed[c] = make([]int32, len(tileData[c]))
			copy(transformed[c], tileData[c])

			// Apply forward multilevel DWT
			wavelet.ForwardMultilevelWithParity(transformed[c], width, height, e.params.NumLevels, x0, y0)
		}
		return transformed
	}
	// Apply 9/7 irreversible wavelet transform (lossy)
	transformed := make([][]int32, len(tileData))
	quantParams := e.lossyQuantizationParams()
	stepSizes := OpenJPEGRuntimeQuantizationSteps(quantParams.EncodedSteps, e.params.NumLevels, e.params.BitDepth)
	for c := 0; c < len(tileData); c++ {
		// OpenJPEG stores irreversible tile samples as OPJ_FLOAT32 through DWT and T1 input.
		floatData := wavelet.ConvertInt32ToFloat32(tileData[c])
		// Apply forward multilevel 9/7 DWT
		wavelet.ForwardMultilevel97Float32WithParity(floatData, width, height, e.params.NumLevels, x0, y0)
		// Apply quantization per subband using float coefficients
		transformed[c] = e.applyQuantizationBySubbandFloat(floatData, width, height, x0, y0, stepSizes)
	}
	return transformed
}

// applyQuantizationBySubbandFloat applies quantization to each subband separately.
// coeffs: wavelet coefficients in subband layout (float domain)
// width, height: dimensions of the full image
// stepSizes: quantization step sizes for each subband (LL, HL1, LH1, HH1, HL2, ...)
func (e *Encoder) applyQuantizationBySubbandFloat(coeffs []float32, width, height, x0, y0 int, stepSizes []float64) []int32 {
	if len(stepSizes) == 0 || e.params.NumLevels == 0 {
		// No quantization
		out := make([]int32, len(coeffs))
		for i, v := range coeffs {
			out[i] = int32(math.RoundToEven(float64(v)))
		}
		return out
	}

	quantized := make([]int32, len(coeffs))

	numLevels := e.params.NumLevels
	subbandIdx := 0

	// LL subband (resolution 0)
	bands := bandInfosForResolution(width, height, x0, y0, numLevels, 0)
	if len(bands) > 0 && subbandIdx < len(stepSizes) {
		b := bands[0]
		if b.width > 0 && b.height > 0 {
			e.quantizeSubbandFloat(coeffs, quantized, b.offsetX, b.offsetY, b.width, b.height, width, stepSizes[subbandIdx])
		}
	}
	subbandIdx++

	// HL/LH/HH subbands from low to high resolution
	for res := 1; res <= numLevels; res++ {
		bands = bandInfosForResolution(width, height, x0, y0, numLevels, res)
		for _, b := range bands {
			if subbandIdx < len(stepSizes) && b.width > 0 && b.height > 0 {
				e.quantizeSubbandFloat(coeffs, quantized, b.offsetX, b.offsetY, b.width, b.height, width, stepSizes[subbandIdx])
			}
			subbandIdx++
		}
	}

	return quantized
}

// quantizeSubbandFloat quantizes a single subband.
// coeffs: full coefficient array (float domain)
// out: quantized output
// x0, y0: top-left corner of subband
// w, h: dimensions of subband
// stride: row stride (width of full image)
// stepSize: quantization step size
func (e *Encoder) quantizeSubbandFloat(coeffs []float32, out []int32, x0, y0, w, h, stride int, stepSize float64) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := (y0+y)*stride + (x0 + x)
			if idx < len(out) {
				if stepSize <= 0 {
					out[idx] = int32(math.RoundToEven(float64(coeffs[idx])))
				} else {
					quantized := (coeffs[idx] / float32(stepSize)) * float32(1<<t1NMSEDecFracBits)
					out[idx] = int32(math.RoundToEven(float64(quantized)))
				}
			}
		}
	}
}

type bandInfo struct {
	band             int
	width, height    int
	offsetX, offsetY int
}

func splitLengths(n int, even bool) (low int) {
	if even {
		return (n + 1) / 2
	}
	return n / 2
}

func isEven(value int) bool {
	return value&1 == 0
}

func nextCoord(value int) int {
	return (value + 1) >> 1
}

func resolutionDimsWithOrigin(width, height, x0, y0, numLevels, res int) (resW, resH int) {
	levelNo := numLevels - res
	if levelNo < 0 {
		levelNo = 0
	}
	resW = width
	resH = height
	resX0 := x0
	resY0 := y0
	for i := 0; i < levelNo; i++ {
		lowW := splitLengths(resW, isEven(resX0))
		lowH := splitLengths(resH, isEven(resY0))
		resW = lowW
		resH = lowH
		resX0 = nextCoord(resX0)
		resY0 = nextCoord(resY0)
	}
	return
}

func bandInfosForResolution(width, height, x0, y0, numLevels, res int) []bandInfo {
	resW, resH := resolutionDimsWithOrigin(width, height, x0, y0, numLevels, res)
	if res == 0 {
		return []bandInfo{{
			band:   0,
			width:  resW,
			height: resH,
		}}
	}
	lowW, lowH := resolutionDimsWithOrigin(width, height, x0, y0, numLevels, res-1)
	highW := resW - lowW
	highH := resH - lowH
	return []bandInfo{
		{band: 1, width: highW, height: lowH, offsetX: lowW, offsetY: 0},
		{band: 2, width: lowW, height: highH, offsetX: 0, offsetY: lowH},
		{band: 3, width: highW, height: highH, offsetX: lowW, offsetY: lowH},
	}
}

func (e *Encoder) buildTilePacketEncoder(tileData [][]int32, width, height int) (*t2.PacketEncoder, []*t2.PrecinctCodeBlock) {
	packetEnc := t2.NewPacketEncoder(
		e.params.Components,
		e.params.NumLayers,
		e.params.NumLevels+1,                           // numResolutions = numLevels + 1
		t2.ProgressionOrder(e.params.ProgressionOrder), // Cast uint8 to ProgressionOrder
	)
	packetEnc.SetImageDimensions(width, height)
	precinctWidths := make([]int, e.params.NumLevels+1)
	precinctHeights := make([]int, e.params.NumLevels+1)
	for res := 0; res <= e.params.NumLevels; res++ {
		pw, ph := e.getPrecinctSize(res)
		precinctWidths[res] = pw
		precinctHeights[res] = ph
	}
	packetEnc.SetPrecinctSizes(precinctWidths, precinctHeights)
	for comp := 0; comp < e.params.Components; comp++ {
		packetEnc.SetComponentSampling(comp, 1, 1)
		packetEnc.SetComponentBounds(comp, 0, 0, width, height)
	}
	allBlocks := make([]*t2.PrecinctCodeBlock, 0)

	// Process each component
	for comp := 0; comp < e.params.Components; comp++ {
		// Global code-block index across all resolutions
		globalCBIdx := 0

		// Process each resolution level
		// Resolution 0 = LL subband (lowest frequency)
		// Resolution 1+ = HL, LH, HH subbands
		for res := 0; res <= e.params.NumLevels; res++ {
			// Get subband dimensions for this resolution
			subbands := e.getSubbandsForResolution(tileData[comp], width, height, res)

			// Process each subband
			for _, subband := range subbands {
				// Partition subband into code-blocks
				codeBlocks := e.partitionIntoCodeBlocks(subband, comp)

				// Encode each code-block with T1
				for _, cb := range codeBlocks {
					encodedCB := e.encodeCodeBlock(cb, globalCBIdx)

					// Set the code-block index correctly
					encodedCB.Index = globalCBIdx
					// Calculate precinct index based on code-block position
					// Convert from global wavelet space to resolution reference grid
					resX0, resY0 := e.toResolutionCoordinates(encodedCB.X0, encodedCB.Y0, res, subband.band)
					precinctIdx := e.calculatePrecinctIndex(resX0, resY0, res)
					precinctWidth, precinctHeight := e.getPrecinctSize(res)
					px := resX0 / precinctWidth
					py := resY0 / precinctHeight
					localX := resX0 - px*precinctWidth
					localY := resY0 - py*precinctHeight
					encodedCB.CBX = localX / e.params.CodeBlockWidth
					encodedCB.CBY = localY / e.params.CodeBlockHeight

					// Add to T2 packet encoder
					packetEnc.AddCodeBlock(comp, res, precinctIdx, encodedCB)
					allBlocks = append(allBlocks, encodedCB)
					globalCBIdx++
				}
			}
		}
	}

	return packetEnc, allBlocks
}

// encodeTileData encodes tile data using T1 and T2 encoding.
func (e *Encoder) encodeTileData(tileData [][]int32, width, height int) []byte {
	packetEnc, allBlocks := e.buildTilePacketEncoder(tileData, width, height)

	// Apply rate-distortion optimized allocation (PCRD) if layered or TargetRatio is requested.
	if e.params.NumLayers > 1 || e.params.TargetRatio > 0 {
		origBytes := e.params.Width * e.params.Height * e.params.Components * ((e.params.BitDepth + 7) / 8)
		e.applyRateDistortionGlobal(allBlocks, []*t2.PacketEncoder{packetEnc}, origBytes, 1)
	}

	// Generate packets
	packetEnc.ResetState()
	packets, err := packetEnc.EncodePackets()
	if err != nil {
		// Fallback to empty packet on error
		return []byte{0x00}
	}

	return e.packetsToBytes(packets)
}

func (e *Encoder) packetsToBytes(packets []t2.Packet) []byte {
	// OpenJPEG applies bit-stuffing only to packet headers (handled during header encoding).
	buf := &bytes.Buffer{}
	for _, packet := range packets {
		// Header already contains OpenJPEG-style bit stuffing.
		buf.Write(packet.Header)
		// Body is raw code-block data (no byte stuffing).
		buf.Write(packet.Body)
	}
	return buf.Bytes()
}

// estimateFixedOverhead builds main header segments to estimate constant bytes (excluding tile packet data).
func (e *Encoder) estimateFixedOverhead() int {
	buf := &bytes.Buffer{}
	// SOC
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerSOC); err != nil {
		return buf.Len()
	}
	// SIZ, COD, QCD, RGN, COM
	if err := e.writeSIZ(buf); err != nil {
		return buf.Len()
	}
	if err := e.writeCOD(buf); err != nil {
		return buf.Len()
	}
	if err := e.writeQCD(buf); err != nil {
		return buf.Len()
	}
	if err := e.writeRGN(buf); err != nil {
		return buf.Len()
	}
	if err := e.writeCOM(buf); err != nil {
		return buf.Len()
	}
	// Assume single tile overhead: SOT(12) + SOD(2) without data
	// We still include tile-part RGN if ROI present
	tile := &bytes.Buffer{}
	if err := binary.Write(tile, binary.BigEndian, codestream.MarkerSOT); err != nil {
		return buf.Len()
	}
	_ = binary.Write(tile, binary.BigEndian, uint16(10))
	_ = binary.Write(tile, binary.BigEndian, uint16(0))
	_ = binary.Write(tile, binary.BigEndian, uint32(14))
	_ = binary.Write(tile, binary.BigEndian, uint8(0))
	_ = binary.Write(tile, binary.BigEndian, uint8(1))
	if err := e.writeTileRGN(tile); err != nil {
		return buf.Len()
	}
	if err := binary.Write(tile, binary.BigEndian, codestream.MarkerSOD); err != nil {
		return buf.Len()
	}
	// Append tile overhead
	buf.Write(tile.Bytes())
	// EOC
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerEOC); err != nil {
		return buf.Len()
	}
	// Byte-stuffing applies to packet data; headers here are marker-coded, do not stuff
	return buf.Len()
}

func (e *Encoder) estimateFixedOverheadForTiles(numTiles int) int {
	if numTiles <= 1 {
		return e.estimateFixedOverhead()
	}
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerSOC); err != nil {
		return buf.Len()
	}
	if err := e.writeSIZ(buf); err != nil {
		return buf.Len()
	}
	if err := e.writeCOD(buf); err != nil {
		return buf.Len()
	}
	if err := e.writeQCD(buf); err != nil {
		return buf.Len()
	}
	if err := e.writeRGN(buf); err != nil {
		return buf.Len()
	}
	if err := e.writeCOM(buf); err != nil {
		return buf.Len()
	}
	for tile := 0; tile < numTiles; tile++ {
		tileHeader := &bytes.Buffer{}
		if err := e.writeTileRGN(tileHeader); err != nil {
			return buf.Len()
		}
		_ = binary.Write(buf, binary.BigEndian, codestream.MarkerSOT)
		_ = binary.Write(buf, binary.BigEndian, uint16(10))
		_ = binary.Write(buf, binary.BigEndian, uint16(tile))
		_ = binary.Write(buf, binary.BigEndian, uint32(14+tileHeader.Len()))
		_ = binary.Write(buf, binary.BigEndian, uint8(0))
		_ = binary.Write(buf, binary.BigEndian, uint8(1))
		buf.Write(tileHeader.Bytes())
		if err := binary.Write(buf, binary.BigEndian, codestream.MarkerSOD); err != nil {
			return buf.Len()
		}
	}
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerEOC); err != nil {
		return buf.Len()
	}
	return buf.Len()
}

func (e *Encoder) applyRateDistortionGlobal(blocks []*t2.PrecinctCodeBlock, packetEncs []*t2.PacketEncoder, origBytes int, numTiles int) {
	if len(blocks) == 0 {
		return
	}
	if e.params.NumLayers <= 0 {
		e.params.NumLayers = 1
	}

	if len(e.params.LayerRates) > 0 {
		e.applyRateDistortionWithBudget(blocks, packetEncs, 0)
		return
	}

	if e.params.UsePCRDOpt && e.params.TargetRatio > 0 {
		midRefine := e.params.TargetRatio >= 7.5 && e.params.TargetRatio <= 8.5
		targetTotal := float64(origBytes) / e.params.TargetRatio
		fixed := float64(e.estimateFixedOverheadForTiles(numTiles))
		if targetTotal <= fixed {
			e.applyRateDistortion(blocks, origBytes)
			return
		}
		targetData := targetTotal - fixed
		budget := targetData
		maxIter := 6
		minScale := 0.5
		maxScale := 1.5
		if midRefine {
			maxIter = 12
			minScale = 0.8
			maxScale = 1.2
		}
		for iter := 0; iter < maxIter; iter++ {
			e.applyRateDistortionWithBudget(blocks, nil, budget)
			pktBytes := 0
			for _, pe := range packetEncs {
				if pe == nil {
					continue
				}
				pe.ResetState()
				packets, err := pe.EncodePackets()
				if err != nil {
					pktBytes = 0
					break
				}
				for _, p := range packets {
					pktBytes += PacketPayloadLen(p.Header) + PacketPayloadLen(p.Body)
				}
			}
			if pktBytes == 0 {
				break
			}
			errPct := math.Abs(float64(pktBytes)-targetData) / targetData
			if errPct <= 0.05 {
				break
			}
			scale := targetData / float64(pktBytes)
			if scale < minScale {
				scale = minScale
			}
			if scale > maxScale {
				scale = maxScale
			}
			budget = budget * scale
		}
		return
	}

	e.applyRateDistortion(blocks, origBytes)
}

// applyRateDistortionWithBudget performs allocation using a target total packet-data budget in bytes.
func (e *Encoder) applyRateDistortionWithBudget(blocks []*t2.PrecinctCodeBlock, packetEncs []*t2.PacketEncoder, targetBudget float64) {
	numLayers, appendLossless := e.initRDLayerConfig()
	passesPerBlock, totalRate := e.collectRDPassesAndRate(blocks)
	budget := e.computeRDBudget(targetBudget, totalRate)
	alloc := e.computeRDLayerAllocation(passesPerBlock, blocks, packetEncs, numLayers, budget)
	for idx, cb := range blocks {
		e.finalizeRDCodeBlockLayers(cb, idx, numLayers, alloc, appendLossless)
	}
}

func (e *Encoder) initRDLayerConfig() (int, bool) {
	numLayers := e.params.NumLayers
	if numLayers <= 0 {
		numLayers = 1
	}
	appendLossless := e.params.AppendLosslessLayer && numLayers > 1
	if e.params.Lossless && numLayers > 1 {
		appendLossless = true
	}
	return numLayers, appendLossless
}

func (e *Encoder) collectRDPassesAndRate(blocks []*t2.PrecinctCodeBlock) ([][]t1.PassData, float64) {
	passesPerBlock := make([][]t1.PassData, 0, len(blocks))
	totalRate := 0.0
	for _, cb := range blocks {
		passesPerBlock = append(passesPerBlock, cb.Passes)
		if len(cb.Passes) > 0 {
			last := cb.Passes[len(cb.Passes)-1]
			bytes := last.Rate
			if bytes == 0 {
				bytes = last.ActualBytes
			}
			totalRate += float64(bytes)
		}
	}
	return passesPerBlock, totalRate
}

func (e *Encoder) computeRDBudget(targetBudget, totalRate float64) float64 {
	budget := targetBudget
	if budget <= 0 || budget > totalRate {
		budget = totalRate
	}
	return budget
}

func (e *Encoder) computeRDLayerAllocation(passesPerBlock [][]t1.PassData, blocks []*t2.PrecinctCodeBlock, packetEncs []*t2.PacketEncoder, numLayers int, budget float64) *LayerAllocation {
	if len(e.params.LayerRates) > 0 {
		layerBudgets := e.openJPEGLayerBudgets(budget)
		var measurer PacketRateMeasurer
		if len(packetEncs) > 0 {
			measurer = func(layer int, selected []int, committed [][]int) (int, error) {
				return MeasureOpenJPEGLayerSelectionBytes(packetEncs, blocks, numLayers, layer, selected, committed)
			}
		}
		return AllocateLayersOpenJPEGThresholdMeasured(passesPerBlock, layerBudgets, measurer)
	}
	if e.params.UsePCRDOpt && e.params.TargetRatio > 8.0 {
		layerBudgets := ComputeLayerBudgets(budget, numLayers, e.params.LayerBudgetStrategy)
		return AllocateLayersWithLambda(passesPerBlock, numLayers, layerBudgets, e.params.LambdaTolerance)
	}
	return AllocateLayersRateDistortionPasses(passesPerBlock, numLayers, budget)
}

func (e *Encoder) openJPEGLayerBudgets(fullBudget float64) []float64 {
	numLayers := e.params.NumLayers
	if numLayers <= 0 {
		numLayers = 1
	}
	budgets := make([]float64, numLayers)
	sizePixel := float64(e.params.Components * e.params.BitDepth)
	bitsEmpty := 8.0
	pixels := float64(e.params.Width * e.params.Height)
	if e.params.Components <= 0 || e.params.BitDepth <= 0 || pixels <= 0 {
		for i := range budgets {
			budgets[i] = fullBudget
		}
		return budgets
	}
	for i := range budgets {
		rate := 0.0
		if i < len(e.params.LayerRates) {
			rate = e.params.LayerRates[i]
		}
		if rate <= 0 {
			budgets[i] = fullBudget
			continue
		}
		budget := (sizePixel * pixels) / (rate * bitsEmpty)
		if e.openJPEGMainHeaderBytes > 0 {
			numTiles := e.openJPEGNumTiles
			if numTiles <= 0 {
				numTiles = 1
			}
			budget -= float64(e.openJPEGMainHeaderBytes) / float64(numTiles)
		}
		if budget < 30 {
			budget = 30
		}
		budget = math.Ceil(budget)
		if budget > fullBudget {
			budget = fullBudget
		}
		budgets[i] = budget
	}
	return budgets
}

func (e *Encoder) finalizeRDCodeBlockLayers(cb *t2.PrecinctCodeBlock, idx, numLayers int, alloc *LayerAllocation, appendLossless bool) {
	if len(cb.Passes) == 0 || cb.CompleteData == nil {
		return
	}
	e.initRDPassLengths(cb)
	cb.LayerPasses = make([]int, numLayers)
	cb.LayerData = make([][]byte, numLayers)
	e.allocateRDLayerData(cb, idx, numLayers, alloc)
	if appendLossless && len(cb.Passes) > 0 {
		e.appendRDLosslessLayer(cb, numLayers)
	}
	cb.Data = cb.CompleteData
	cb.UseTERMALL = e.classicCodeBlockStyle()&0x04 != 0
}

func (e *Encoder) initRDPassLengths(cb *t2.PrecinctCodeBlock) {
	if len(cb.PassLengths) == 0 {
		cb.PassLengths = make([]int, len(cb.Passes))
		for i, p := range cb.Passes {
			cb.PassLengths[i] = p.Rate
		}
	}
}

func (e *Encoder) allocateRDLayerData(cb *t2.PrecinctCodeBlock, idx, numLayers int, alloc *LayerAllocation) {
	prevEnd := 0
	for layer := 0; layer < numLayers; layer++ {
		passCount := alloc.GetPassesForLayer(idx, layer)
		if passCount > len(cb.Passes) {
			passCount = len(cb.Passes)
		}
		cb.LayerPasses[layer] = passCount
		end := prevEnd
		if passCount > 0 {
			end = cb.Passes[passCount-1].Rate
			if end == 0 {
				end = cb.Passes[passCount-1].ActualBytes
			}
		}
		if end < prevEnd {
			end = prevEnd
		}
		if end > len(cb.CompleteData) {
			end = len(cb.CompleteData)
		}
		cb.LayerData[layer] = cb.CompleteData[prevEnd:end]
		prevEnd = end
	}
}

func (e *Encoder) appendRDLosslessLayer(cb *t2.PrecinctCodeBlock, numLayers int) {
	last := numLayers - 1
	prevPasses := 0
	if last > 0 && (last-1) < len(cb.LayerPasses) {
		prevPasses = cb.LayerPasses[last-1]
	}
	if prevPasses < 0 {
		prevPasses = 0
	}
	if prevPasses > len(cb.Passes) {
		prevPasses = len(cb.Passes)
	}
	fullPasses := len(cb.Passes)
	cb.LayerPasses[last] = fullPasses
	start := 0
	if prevPasses > 0 {
		start = cb.Passes[prevPasses-1].Rate
		if start == 0 {
			start = cb.Passes[prevPasses-1].ActualBytes
		}
	}
	end := cb.Passes[fullPasses-1].Rate
	if end == 0 {
		end = cb.Passes[fullPasses-1].ActualBytes
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(cb.CompleteData) {
		end = len(cb.CompleteData)
	}
	cb.LayerData[last] = cb.CompleteData[start:end]
}

// applyRateDistortion truncates/allocates passes across layers using PCRD-style allocation.
func (e *Encoder) applyRateDistortion(blocks []*t2.PrecinctCodeBlock, origBytes int) {
	numLayers := e.params.NumLayers
	if numLayers <= 0 {
		numLayers = 1
	}
	appendLossless := e.params.AppendLosslessLayer && numLayers > 1
	if e.params.Lossless && numLayers > 1 {
		appendLossless = true
	}
	if len(blocks) == 0 {
		return
	}
	passesPerBlock, totalRate := e.collectPassesAndRate(blocks)
	budget, alloc := e.computeBudgetAndAlloc(passesPerBlock, numLayers, totalRate, origBytes)
	if budget > 0 && budget < totalRate {
		e.enforceGlobalBudget(alloc, passesPerBlock, numLayers, totalRate, budget)
	}
	for idx, cb := range blocks {
		e.finalizeBlock(cb, numLayers, alloc, idx, appendLossless)
	}
}

func passBytes(passes []t1.PassData, count int) int {
	if count <= 0 {
		return 0
	}
	if count > len(passes) {
		count = len(passes)
	}
	b := passes[count-1].Rate
	if b == 0 {
		b = passes[count-1].ActualBytes
	}
	return b
}

func (e *Encoder) collectPassesAndRate(blocks []*t2.PrecinctCodeBlock) ([][]t1.PassData, float64) {
	passesPerBlock := make([][]t1.PassData, 0, len(blocks))
	totalRate := 0.0
	for _, cb := range blocks {
		passesPerBlock = append(passesPerBlock, cb.Passes)
		if len(cb.Passes) > 0 {
			last := cb.Passes[len(cb.Passes)-1]
			bytes := last.Rate
			if bytes == 0 {
				bytes = last.ActualBytes
			}
			totalRate += float64(bytes)
		}
	}
	return passesPerBlock, totalRate
}

func (e *Encoder) computeBudgetAndAlloc(passesPerBlock [][]t1.PassData, numLayers int, totalRate float64, origBytes int) (float64, *LayerAllocation) {
	budget := totalRate
	if e.params.TargetRatio > 0 && origBytes > 0 {
		effectiveTargetRatio := e.params.TargetRatio
		if !e.params.UsePCRDOpt && effectiveTargetRatio > 6.0 {
			effectiveTargetRatio = 6.0
		}
		target := float64(origBytes) / effectiveTargetRatio
		if target < budget {
			budget = target
		}
	}
	var alloc *LayerAllocation
	if e.params.UsePCRDOpt {
		layerBudgets := ComputeLayerBudgets(budget, numLayers, e.params.LayerBudgetStrategy)
		alloc = AllocateLayersWithLambda(passesPerBlock, numLayers, layerBudgets, e.params.LambdaTolerance)
	} else {
		alloc = AllocateLayersRateDistortionPasses(passesPerBlock, numLayers, budget)
	}
	return budget, alloc
}

func (e *Encoder) enforceGlobalBudget(alloc *LayerAllocation, passesPerBlock [][]t1.PassData, numLayers int, totalRate, budget float64) {
	for idx, passes := range passesPerBlock {
		fullBytes := passBytes(passes, len(passes))
		if fullBytes == 0 {
			continue
		}
		allowed := int(math.Floor(budget * float64(fullBytes) / totalRate))
		if allowed <= 0 && len(passes) > 0 {
			allowed = passBytes(passes, 1)
		}
		passCount := 0
		for i := 0; i < len(passes); i++ {
			if passBytes(passes, i+1) <= allowed {
				passCount = i + 1
			} else {
				break
			}
		}
		if passCount == 0 && len(passes) > 0 {
			passCount = 1
		}
		for layer := 0; layer < numLayers; layer++ {
			frac := float64(layer+1) / float64(numLayers)
			layerPass := int(math.Ceil(frac * float64(passCount)))
			if layerPass > passCount {
				layerPass = passCount
			}
			if layer > 0 && layerPass < alloc.CodeBlockPasses[idx][layer-1] {
				layerPass = alloc.CodeBlockPasses[idx][layer-1]
			}
			alloc.CodeBlockPasses[idx][layer] = layerPass
		}
	}
}

func (e *Encoder) finalizeBlock(cb *t2.PrecinctCodeBlock, numLayers int, alloc *LayerAllocation, idx int, appendLossless bool) {
	if len(cb.Passes) == 0 || cb.CompleteData == nil {
		return
	}
	if len(cb.PassLengths) == 0 {
		cb.PassLengths = make([]int, len(cb.Passes))
		for i, p := range cb.Passes {
			cb.PassLengths[i] = p.Rate
		}
	}
	cb.LayerPasses = make([]int, numLayers)
	cb.LayerData = make([][]byte, numLayers)
	prevEnd := 0
	for layer := 0; layer < numLayers; layer++ {
		passCount := alloc.GetPassesForLayer(idx, layer)
		if passCount > len(cb.Passes) {
			passCount = len(cb.Passes)
		}
		cb.LayerPasses[layer] = passCount
		end := prevEnd
		if passCount > 0 {
			end = cb.Passes[passCount-1].Rate
			if end == 0 {
				end = cb.Passes[passCount-1].ActualBytes
			}
		}
		if end < prevEnd {
			end = prevEnd
		}
		if end > len(cb.CompleteData) {
			end = len(cb.CompleteData)
		}
		cb.LayerData[layer] = cb.CompleteData[prevEnd:end]
		prevEnd = end
	}
	if appendLossless && len(cb.Passes) > 0 {
		last := numLayers - 1
		prevPasses := 0
		if last > 0 && (last-1) < len(cb.LayerPasses) {
			prevPasses = cb.LayerPasses[last-1]
		}
		if prevPasses < 0 {
			prevPasses = 0
		}
		if prevPasses > len(cb.Passes) {
			prevPasses = len(cb.Passes)
		}
		fullPasses := len(cb.Passes)
		cb.LayerPasses[last] = fullPasses
		start := 0
		if prevPasses > 0 {
			start = cb.Passes[prevPasses-1].Rate
			if start == 0 {
				start = cb.Passes[prevPasses-1].ActualBytes
			}
		}
		end := cb.Passes[fullPasses-1].Rate
		if end == 0 {
			end = cb.Passes[fullPasses-1].ActualBytes
		}
		if start < 0 {
			start = 0
		}
		if end < start {
			end = start
		}
		if end > len(cb.CompleteData) {
			end = len(cb.CompleteData)
		}
		cb.LayerData[last] = cb.CompleteData[start:end]
	}
	cb.Data = cb.CompleteData
	cb.UseTERMALL = false
}

// subbandInfo represents a wavelet subband
type subbandInfo struct {
	data   []int32 // Coefficient data
	x0, y0 int     // Subband origin
	width  int     // Subband width
	height int     // Subband height
	band   int     // Band type: 0=LL, 1=HL, 2=LH, 3=HH
	res    int     // Resolution level (0=LL)
	scale  int     // Scale factor to map subband coords to full resolution (power of two)
}

// getSubbandsForResolution extracts subbands for a specific resolution level
func (e *Encoder) getSubbandsForResolution(data []int32, width, height, resolution int) []subbandInfo {
	// Resolution 0 contains only LL subband (approximation)
	// Resolution r > 0 contains HL, LH, HH subbands from decomposition level r

	var subbands []subbandInfo

	if resolution == 0 {
		// LL subband (top-left quadrant after all decompositions)
		// Use ceiling division to match JPEG2000 standard and OpenJPEG
		divisor := 1 << e.params.NumLevels
		llWidth := (width + divisor - 1) / divisor   // Ceiling division
		llHeight := (height + divisor - 1) / divisor // Ceiling division
		scale := divisor

		llData := make([]int32, llWidth*llHeight)
		for y := 0; y < llHeight; y++ {
			for x := 0; x < llWidth; x++ {
				srcIdx := y*width + x
				if srcIdx < len(data) && y < height && x < width {
					llData[y*llWidth+x] = data[srcIdx]
				}
				// else: pad with zero (already initialized to 0)
			}
		}

		sb := subbandInfo{
			data:   llData,
			x0:     0,
			y0:     0,
			width:  llWidth,
			height: llHeight,
			band:   0, // LL
			res:    0,
			scale:  scale,
		}
		subbands = append(subbands, sb)
	} else {
		// For resolution r, extract HL, LH, HH subbands
		// OpenJPEG uses: levelDiff = numResolutions - 1 - resno
		// Since numResolutions = NumLevels + 1:
		// levelDiff = (NumLevels + 1) - 1 - resolution = NumLevels - resolution
		level := e.params.NumLevels - resolution
		if level < 0 {
			level = 0
		}

		// Helper function for ceiling division by power of 2 (matching OpenJPEG)
		ceildivpow2 := func(a, b int) int {
			return (a + (1 << b) - 1) >> b
		}

		scale := 1 << (level + 1)

		// OpenJPEG band offset calculation (see tcd.c lines 1043-1056):
		// bandno: 1=HL, 2=LH, 3=HH
		// x0b = bandno & 1 (1 for HL and HH, 0 for LH)
		// y0b = bandno >> 1 (1 for LH and HH, 0 for HL)
		// width = ceildivpow2(image_width - (x0b << level), level+1)
		// height = ceildivpow2(image_height - (y0b << level), level+1)

		// LL subband dimensions (needed for offsets)
		llWidth := ceildivpow2(width-(0<<level), level+1)
		llHeight := ceildivpow2(height-(0<<level), level+1)

		// HL (high-low): x0b=1, y0b=0
		hlWidth := ceildivpow2(width-(1<<level), level+1)
		hlHeight := ceildivpow2(height-(0<<level), level+1)
		hlData := make([]int32, hlWidth*hlHeight)
		for y := 0; y < hlHeight; y++ {
			for x := 0; x < hlWidth; x++ {
				srcIdx := y*width + (llWidth + x)
				if srcIdx < len(data) && llWidth+x < width {
					hlData[y*hlWidth+x] = data[srcIdx]
				}
				// else: pad with zero (already initialized to 0)
			}
		}
		subbands = append(subbands, subbandInfo{
			data:   hlData,
			x0:     llWidth,
			y0:     0,
			width:  hlWidth,
			height: hlHeight,
			band:   1, // HL
			res:    resolution,
			scale:  scale,
		})

		// LH (low-high): x0b=0, y0b=1
		lhWidth := ceildivpow2(width-(0<<level), level+1)
		lhHeight := ceildivpow2(height-(1<<level), level+1)
		lhData := make([]int32, lhWidth*lhHeight)
		for y := 0; y < lhHeight; y++ {
			for x := 0; x < lhWidth; x++ {
				srcIdx := (llHeight+y)*width + x
				if srcIdx < len(data) && llHeight+y < height {
					lhData[y*lhWidth+x] = data[srcIdx]
				}
				// else: pad with zero (already initialized to 0)
			}
		}
		subbands = append(subbands, subbandInfo{
			data:   lhData,
			x0:     0,
			y0:     llHeight,
			width:  lhWidth,
			height: lhHeight,
			band:   2, // LH
			res:    resolution,
			scale:  scale,
		})

		// HH (high-high): x0b=1, y0b=1
		hhWidth := ceildivpow2(width-(1<<level), level+1)
		hhHeight := ceildivpow2(height-(1<<level), level+1)
		hhData := make([]int32, hhWidth*hhHeight)
		for y := 0; y < hhHeight; y++ {
			for x := 0; x < hhWidth; x++ {
				srcIdx := (llHeight+y)*width + (llWidth + x)
				if srcIdx < len(data) && llHeight+y < height && llWidth+x < width {
					hhData[y*hhWidth+x] = data[srcIdx]
				}
				// else: pad with zero (already initialized to 0)
			}
		}
		subbands = append(subbands, subbandInfo{
			data:   hhData,
			x0:     llWidth,
			y0:     llHeight,
			width:  hhWidth,
			height: hhHeight,
			band:   3, // HH
			res:    resolution,
			scale:  scale,
		})
	}

	return subbands
}

type codeBlockInfo struct {
	compIdx  int
	data     []int32
	width    int
	height   int
	globalX0 int // Global X position in coefficient array
	globalY0 int // Global Y position in coefficient array
	cbx      int // Code-block X index within subband
	cby      int // Code-block Y index within subband
	scale    int // Downsampling factor from full resolution (reserved)
	resLevel int // Resolution level (0=LL)
	band     int // Subband identifier (0=LL,1=HL,2=LH,3=HH)
	mask     [][]bool
}

// partitionIntoCodeBlocks partitions a subband into code-blocks
func (e *Encoder) partitionIntoCodeBlocks(subband subbandInfo, compIdx int) []codeBlockInfo {
	cbWidth := e.params.CodeBlockWidth
	cbHeight := e.params.CodeBlockHeight

	numCBX := (subband.width + cbWidth - 1) / cbWidth
	numCBY := (subband.height + cbHeight - 1) / cbHeight

	codeBlocks := make([]codeBlockInfo, 0, numCBX*numCBY)

	for cby := 0; cby < numCBY; cby++ {
		for cbx := 0; cbx < numCBX; cbx++ {
			// Calculate code-block bounds
			x0 := cbx * cbWidth
			y0 := cby * cbHeight
			x1 := x0 + cbWidth
			y1 := y0 + cbHeight

			if x1 > subband.width {
				x1 = subband.width
			}
			if y1 > subband.height {
				y1 = subband.height
			}

			actualWidth := x1 - x0
			actualHeight := y1 - y0

			// Extract code-block data
			cbData := make([]int32, actualWidth*actualHeight)
			for y := 0; y < actualHeight; y++ {
				for x := 0; x < actualWidth; x++ {
					srcIdx := (y0+y)*subband.width + (x0 + x)
					dstIdx := y*actualWidth + x
					cbData[dstIdx] = subband.data[srcIdx]
				}
			}

			// Store code-block with its dimensions and global position
			globalX0 := subband.x0 + x0
			globalY0 := subband.y0 + y0

			var mask [][]bool
			if compIdx < len(e.roiMasks) && e.roiMasks[compIdx] != nil {
				// Use subband.scale to map subband coords to full resolution
				step := max(1, subband.scale)
				fullX0 := (subband.x0 + x0) * step
				fullY0 := (subband.y0 + y0) * step
				fullX1 := (subband.x0 + x1) * step
				fullY1 := (subband.y0 + y1) * step
				mask = e.roiMasks[compIdx].downsample(fullX0, fullY0, fullX1, fullY1, step)
			}

			codeBlocks = append(codeBlocks, codeBlockInfo{
				compIdx:  compIdx,
				data:     cbData,
				width:    actualWidth,
				height:   actualHeight,
				globalX0: globalX0,
				globalY0: globalY0,
				cbx:      cbx,
				cby:      cby,
				scale:    subband.scale,
				resLevel: subband.res,
				band:     subband.band,
				mask:     mask,
			})
		}
	}

	return codeBlocks
}

// encodeCodeBlock encodes a single code-block using T1 EBCOT encoder
func (e *Encoder) encodeCodeBlock(cb codeBlockInfo, _ int) *t2.PrecinctCodeBlock {
	// Use provided dimensions
	actualWidth := cb.width
	actualHeight := cb.height
	cbData := cb.data

	if !e.params.HTJ2KMode && e.params.Lossless {
		// Apply T1 NMSEDEC FRACBITS scaling (left shift 6 bits).
		// This matches OpenJPEG's representation for classic EBCOT T1 coding.
		for i := range cbData {
			cbData[i] <<= t1NMSEDecFracBits
		}
	}

	cblkNumbps := e.codeBlockNumBps(cbData)
	bandNumbps := e.bandNumbps(cb.resLevel, cb.band)
	if bandNumbps <= 0 {
		bandNumbps = cblkNumbps
	}
	numPasses, zeroBitPlanes := e.codeBlockPassLayout(cblkNumbps, bandNumbps)

	// Create block encoder (EBCOT T1 or HTJ2K)
	blockEnc := e.newCodeBlockEncoder(actualWidth, actualHeight, cb.resLevel, cb.band, bandNumbps)

	// ROI handling: determine style/shift/inside and apply scaling/roishift
	_, roiShift, inside := e.roiContext(cb)
	roishift := 0
	if roiShift > 0 && inside {
		// MaxShift/General Scaling: scale ROI coefficients before coding.
		if len(cb.mask) > 0 && len(cb.mask[0]) > 0 {
			applyGeneralScalingMasked(cbData, cb.mask, roiShift)
		} else {
			applyGeneralScaling(cbData, roiShift)
		}
	}

	// Create PrecinctCodeBlock structure
	pcb := &t2.PrecinctCodeBlock{
		Index:          0, // Will be set by caller if needed
		X0:             cb.globalX0,
		Y0:             cb.globalY0,
		X1:             cb.globalX0 + actualWidth,
		Y1:             cb.globalY0 + actualHeight,
		CBX:            cb.cbx,
		CBY:            cb.cby,
		Band:           cb.band,
		Included:       false, // First inclusion in packet
		NumPassesTotal: numPasses,
		ZeroBitPlanes:  zeroBitPlanes,
	}

	useLayered := e.params.NumLayers > 1 || e.params.TargetRatio > 0

	if useLayered {
		return e.encodeLayeredCodeBlock(pcb, blockEnc, cbData, numPasses, roishift)
	}
	return e.encodeSingleLayerCodeBlock(pcb, blockEnc, cbData, numPasses, roishift, bandNumbps, zeroBitPlanes)
}

const t1NMSEDecFracBits = 6

func (e *Encoder) codeBlockNumBps(cbData []int32) int {
	rawMaxBitplane := calculateMaxBitplane(cbData)
	if rawMaxBitplane < 0 {
		return 0
	}
	cblkNumbps := rawMaxBitplane + 1
	if !e.params.HTJ2KMode {
		cblkNumbps -= t1NMSEDecFracBits
	}
	if cblkNumbps < 0 {
		return 0
	}
	return cblkNumbps
}

func (e *Encoder) codeBlockPassLayout(cblkNumbps, bandNumbps int) (numPasses, zeroBitPlanes int) {
	numPasses = 1
	if cblkNumbps > 0 {
		numPasses = (cblkNumbps * 3) - 2
	}
	zeroBitPlanes = bandNumbps - cblkNumbps
	if zeroBitPlanes < 0 {
		zeroBitPlanes = 0
	}
	if e.params.HTJ2KMode {
		numPasses = 1
		zeroBitPlanes = bandNumbps - 1
		if zeroBitPlanes < 0 {
			zeroBitPlanes = 0
		}
		if cblkNumbps == 0 {
			numPasses = 0
		}
	}
	return numPasses, zeroBitPlanes
}

func (e *Encoder) newCodeBlockEncoder(width, height, res, band, bandNumbps int) BlockEncoder {
	var blockEnc BlockEncoder
	if e.params.BlockEncoderFactory != nil {
		blockEnc = e.params.BlockEncoderFactory(width, height)
	} else {
		t1Enc := t1.NewT1Encoder(width, height, 0)
		t1Enc.SetOrientation(band)
		if !e.params.HTJ2KMode {
			t1Enc.SetNMSEDecFractionalBits(t1NMSEDecFracBits)
			if e.params.Lossless {
				level := e.params.NumLevels - res
				norm := dwtNorm53(level, band)
				weight := norm * norm / 8192.0
				t1Enc.SetDistortionWeight(weight)
			} else {
				step := e.openJPEGRuntimeStepForBand(res, band)
				level := e.params.NumLevels - res
				t1Enc.SetDistortionWeight(openJPEGDistortionWeight(false, level, band, step))
			}
		}
		blockEnc = t1Enc
	}
	if setter, ok := blockEnc.(blockEncoderKMaxSetter); ok {
		setter.SetKMax(bandNumbps)
	}
	return blockEnc
}

func (e *Encoder) openJPEGRuntimeStepForBand(res, band int) float64 {
	quantParams := e.lossyQuantizationParams()
	steps := OpenJPEGRuntimeQuantizationSteps(quantParams.EncodedSteps, e.params.NumLevels, e.params.BitDepth)
	idx := subbandIndexForResolutionBand(e.params.NumLevels, res, band)
	if idx < 0 || idx >= len(steps) {
		return 1.0
	}
	return steps[idx]
}

func subbandIndexForResolutionBand(numLevels, res, band int) int {
	if res == 0 {
		return 0
	}
	if res < 0 || res > numLevels || band < 1 || band > 3 {
		return -1
	}
	return 1 + (res-1)*3 + (band - 1)
}

func openJPEGDistortionWeight(lossless bool, level, orient int, stepSize float64) float64 {
	if lossless {
		norm := dwtNorm53(level, orient)
		return norm * norm / 8192.0
	}
	if stepSize <= 0 {
		stepSize = 1.0
	}
	log2Gain := 0
	if orient == 3 {
		log2Gain = 2
	} else if orient != 0 {
		log2Gain = 1
	}
	gain := 1 << log2Gain
	adjustedStep := stepSize / float64(gain)
	weight := dwtNorm97(level, orient) * adjustedStep
	return weight * weight / 8192.0
}

func (e *Encoder) layerBoundaries(numPasses int) []int {
	if e.params.NumLayers <= 1 {
		return []int{1, numPasses}
	}
	layerAlloc := AllocateLayersSimple(numPasses, e.params.NumLayers, 1)
	layerBoundaries := make([]int, e.params.NumLayers)
	for layer := 0; layer < e.params.NumLayers; layer++ {
		layerBoundaries[layer] = layerAlloc.GetPassesForLayer(0, layer)
	}
	return layerBoundaries
}

func (e *Encoder) encodeLayeredCodeBlock(pcb *t2.PrecinctCodeBlock, blockEnc BlockEncoder, cbData []int32, numPasses, roishift int) *t2.PrecinctCodeBlock {
	var passes []t1.PassData
	var completeData []byte
	var err error

	if t1Enc, ok := blockEnc.(*t1.Encoder); ok {
		passes, completeData, err = t1Enc.EncodeLayered(cbData, numPasses, roishift, e.layerBoundaries(numPasses), e.classicCodeBlockStyle())
	} else {
		completeData, err = blockEnc.Encode(cbData, numPasses, roishift)
		if err == nil {
			passes = []t1.PassData{{ActualBytes: len(completeData)}}
		}
	}

	if err != nil {
		encodedData := []byte{0x00}
		pcb.Data = encodedData
		pcb.LayerData = [][]byte{encodedData}
		pcb.LayerPasses = []int{1}
		return pcb
	}

	if len(passes) == 0 {
		pcb.NumPassesTotal = 0
		pcb.Data = nil
		pcb.CompleteData = completeData
		return pcb
	}

	pcb.PassLengths = make([]int, len(passes))
	for i, pass := range passes {
		pcb.PassLengths[i] = pass.Rate
	}
	pcb.Passes = passes
	pcb.CompleteData = completeData
	pcb.Data = completeData
	pcb.UseTERMALL = e.classicCodeBlockStyle()&0x04 != 0
	return pcb
}

func (e *Encoder) classicCodeBlockStyle() uint8 {
	if e.params.HTJ2KMode {
		return 0x40
	}
	return 0
}

func (e *Encoder) encodeSingleLayerCodeBlock(pcb *t2.PrecinctCodeBlock, blockEnc BlockEncoder, cbData []int32, numPasses, roishift, bandNumbps, zeroBitPlanes int) *t2.PrecinctCodeBlock {
	if e.params.HTJ2KMode && numPasses == 0 {
		pcb.NumPassesTotal = 0
		pcb.ZeroBitPlanes = zeroBitPlanes
		pcb.Data = nil
		return pcb
	}
	encodedData, err := blockEnc.Encode(cbData, numPasses, roishift)
	if err != nil {
		// Return minimal code-block on error
		encodedData = []byte{0x00}
		numPasses = 1
		zeroBitPlanes = bandNumbps
		pcb.NumPassesTotal = numPasses
		pcb.ZeroBitPlanes = zeroBitPlanes
	}
	pcb.Data = encodedData

	return pcb
}

// roiShiftForCodeBlock returns the MaxShift value for ROI blocks.
func (e *Encoder) roiShiftForCodeBlock(cb codeBlockInfo) int {
	style, shift, inside := e.roiContext(cb)
	if shift <= 0 {
		return 0
	}
	if style != 0 || !inside {
		return 0
	}
	return shift
}

// roiContext returns ROI style, shift, and whether the block intersects ROI.
func (e *Encoder) roiContext(cb codeBlockInfo) (byte, int, bool) {
	if cb.compIdx < 0 || cb.compIdx >= len(e.roiShifts) {
		return 0, 0, false
	}
	style := byte(0)
	if cb.compIdx < len(e.roiStyles) {
		style = e.roiStyles[cb.compIdx]
	}
	shift := e.roiShifts[cb.compIdx]
	if shift <= 0 {
		return style, 0, false
	}
	x0 := cb.globalX0
	y0 := cb.globalY0
	x1 := cb.globalX0 + cb.width
	y1 := cb.globalY0 + cb.height

	inside := false
	hasMask := len(cb.mask) > 0 && len(cb.mask[0]) > 0
	if hasMask {
		inside = maskAnyTrue(cb.mask)
	} else {
		rects := e.RoiRects[cb.compIdx]
		for _, rect := range rects {
			if rect.intersects(x0, y0, x1, y1) {
				inside = true
				break
			}
		}
	}
	return style, shift, inside
}

// applyGeneralScaling multiplies coefficients in-place by 2^shift.
func applyGeneralScaling(data []int32, shift int) {
	if shift <= 0 {
		return
	}
	factor := int32(1 << shift)
	for i := range data {
		data[i] *= factor
	}
}

// applyGeneralScalingMasked multiplies only coefficients covered by mask by 2^shift.
func applyGeneralScalingMasked(data []int32, mask [][]bool, shift int) {
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
					data[idx] *= factor
				}
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

// calculateMaxBitplane finds the highest bit-plane that contains a '1' bit
func calculateMaxBitplane(data []int32) int {
	maxAbs := int32(0)
	for _, val := range data {
		absVal := val
		if absVal < 0 {
			absVal = -absVal
		}
		if absVal > maxAbs {
			maxAbs = absVal
		}
	}

	if maxAbs == 0 {
		return -1
	}

	// Find highest bit set
	bitplane := 0
	for maxAbs > 0 {
		maxAbs >>= 1
		bitplane++
	}

	return bitplane - 1
}

// Helper functions

func isPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

func log2(n int) int {
	result := 0
	for n > 1 {
		n >>= 1
		result++
	}
	return result
}

// encodeQuantStepsFromFloats converts floating quantization steps to OpenJPEG-encoded form.
func encodeQuantStepsFromFloats(steps []float64, bitDepth int) []uint16 {
	if len(steps) == 0 {
		return nil
	}
	encoded := make([]uint16, len(steps))
	for i, stepSize := range steps {
		encoded[i] = encodeQuantizationStep(stepSize, bitDepth)
	}
	return encoded
}

// applyDCLevelShift applies DC level shift for unsigned data
// For unsigned data: subtract 2^(bitDepth-1) to convert to signed range
func (e *Encoder) applyDCLevelShift() {
	if e.params.IsSigned {
		// Signed data - no level shift needed
		return
	}

	// Unsigned data - subtract 2^(bitDepth-1)
	shift := int32(1 << (e.params.BitDepth - 1))
	for comp := range e.data {
		for i := range e.data[comp] {
			e.data[comp][i] -= shift
		}
	}
}
