// Package lossy provides JPEG 2000 Lossy codec implementations.
package lossy

import (
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

var _ codec.Codec = (*Codec)(nil)

const defaultRate = 20

// Codec implements the JPEG 2000 Lossy codec
// Transfer Syntax UID: 1.2.840.10008.1.2.4.91
type Codec struct {
	transferSyntax *transfer.Syntax
	defaultRate    int
}

// NewCodec creates a new JPEG 2000 Lossy codec with fo-dicom-aligned defaults.
func NewCodec() *Codec {
	return NewCodecWithRate(defaultRate)
}

// NewCodecWithRate creates a new JPEG 2000 Lossy codec with custom default rate.
func NewCodecWithRate(rate ...int) *Codec {
	r := defaultRate
	if len(rate) > 0 {
		r = rate[0]
	}
	return NewCodecWithTransferSyntax(transfer.JPEG2000, r)
}

// NewCodecWithTransferSyntax allows constructing the codec for alternate JPEG 2000 transfer syntaxes.
func NewCodecWithTransferSyntax(ts *transfer.Syntax, rate int) *Codec {
	if rate <= 0 {
		rate = defaultRate
	}
	return &Codec{
		transferSyntax: ts,
		defaultRate:    rate,
	}
}

// NewPart2MultiComponentCodec creates a JPEG 2000 Part 2 Multi-component codec (UID .93).
func NewPart2MultiComponentCodec() *Codec {
	return NewCodecWithTransferSyntax(transfer.JPEG2000Part2MultiComponent, defaultRate)
}

// Name returns the codec name
func (c *Codec) Name() string {
	return fmt.Sprintf("JPEG 2000 Lossy (Rate %d)", c.defaultRate)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// GetDefaultParameters returns the default codec parameters
func (c *Codec) GetDefaultParameters() codec.Parameters {
	params := NewLossyParameters()
	params.Rate = c.defaultRate
	return params
}

// Encode encodes pixel data to JPEG 2000 Lossy format
func (c *Codec) Encode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return fmt.Errorf("failed to get frame info from source pixel data")
	}

	// Get encoding parameters
	var lossyParams *JPEG2000LossyParameters
	if parameters != nil {
		// Try to use typed parameters if provided
		if jp, ok := parameters.(*JPEG2000LossyParameters); ok {
			lossyParams = jp
		} else {
			// Fallback: create from generic parameters
			lossyParams = NewLossyParameters()
			if nl := parameters.GetParameter("numLevels"); nl != nil {
				if nlInt, ok := nl.(int); ok && nlInt >= 0 && nlInt <= 6 {
					lossyParams.NumLevels = nlInt
				}
			}
			if irr := parameters.GetParameter("irreversible"); irr != nil {
				if irrBool, ok := irr.(bool); ok {
					lossyParams.Irreversible = irrBool
				}
			}
			if mct := parameters.GetParameter("allowMCT"); mct != nil {
				if mctBool, ok := mct.(bool); ok {
					lossyParams.AllowMCT = mctBool
				}
			}
			if vb := parameters.GetParameter("isVerbose"); vb != nil {
				if vbBool, ok := vb.(bool); ok {
					lossyParams.IsVerbose = vbBool
				}
			}
			if r := parameters.GetParameter("rate"); r != nil {
				if rInt, ok := r.(int); ok && rInt > 0 {
					lossyParams.Rate = rInt
				}
			}
			if rl := parameters.GetParameter("rateLevels"); rl != nil {
				if arr, ok := rl.([]int); ok && len(arr) > 0 {
					lossyParams.RateLevels = arr
				}
			}
		}
	} else {
		// Use codec defaults
		lossyParams = NewLossyParameters()
		lossyParams.Rate = c.defaultRate
	}

	// Validate parameters
	if err := lossyParams.Validate(); err != nil {
		return fmt.Errorf("invalid JPEG 2000 lossy parameters: %w", err)
	}

	// Create encoding parameters
	baseEncParams := jpeg2000.DefaultEncodeParams(
		int(frameInfo.Width),
		int(frameInfo.Height),
		int(frameInfo.SamplesPerPixel),
		int(frameInfo.BitsStored),
		frameInfo.PixelRepresentation != 0,
	)

	// Process all frames
	frameCount := oldPixelData.FrameCount()
	if frameCount == 0 {
		return fmt.Errorf("source pixel data is empty (no frames)")
	}
	for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
		// Get frame data
		frameData, err := oldPixelData.GetFrame(frameIndex)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", frameIndex, err)
		}

		if len(frameData) == 0 {
			return fmt.Errorf("frame %d pixel data is empty", frameIndex)
		}

		// Rate control: if TargetRatio > 0, adjust quality to approach target ratio.
		var encoded []byte
		var encErr error
		if lossyParams.TargetRatio > 0 {
			encoded, encErr = c.encodeFrameWithTargetRatio(frameData, frameInfo, lossyParams, baseEncParams)
		} else {
			encoded, encErr = c.encodeFrameOnce(frameData, frameInfo, lossyParams, baseEncParams)
		}
		if encErr != nil {
			return encErr
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(encoded); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG 2000 Lossy data to uncompressed pixel data
func (c *Codec) Decode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, _ codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Process all frames
	frameCount := oldPixelData.FrameCount()
	if frameCount == 0 {
		return fmt.Errorf("source pixel data is empty (no frames)")
	}
	for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
		// Get encoded frame data
		frameData, err := oldPixelData.GetFrame(frameIndex)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", frameIndex, err)
		}

		if len(frameData) == 0 {
			return fmt.Errorf("frame %d pixel data is empty", frameIndex)
		}

		// Create decoder
		decoder := jpeg2000.NewDecoder()

		// Decode (decoder automatically detects lossy vs lossless from codestream)
		if err := decoder.Decode(frameData); err != nil {
			return fmt.Errorf("JPEG 2000 decode failed for frame %d: %w", frameIndex, err)
		}

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(decoder.GetPixelData()); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterJPEG2000LossyCodec registers the JPEG 2000 Lossy codec with the global registry
func RegisterJPEG2000LossyCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewCodec()
	registry.RegisterCodec(transfer.JPEG2000Lossy, j2kCodec)
}

// RegisterJPEG2000MultiComponentCodec registers JPEG 2000 Part 2 multi-component codec.
func RegisterJPEG2000MultiComponentCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewPart2MultiComponentCodec()
	registry.RegisterCodec(transfer.JPEG2000Part2MultiComponent, j2kCodec)
}

func init() {
	RegisterJPEG2000LossyCodec()
	RegisterJPEG2000MultiComponentCodec()
}

// encodeFrameOnce performs a single encode using the provided parameters for a single frame.
func (c *Codec) encodeFrameOnce(frameData []byte, frameInfo *imagetypes.FrameInfo, p *JPEG2000LossyParameters, baseEncParams *jpeg2000.EncodeParams) ([]byte, error) {
	encParams := *baseEncParams
	targetRatio := p.TargetRatio
	baseQuality := c.initializeBaseQuality(p, targetRatio)
	c.configureBasicEncodeParams(&encParams, frameInfo, p, baseQuality)
	c.adjustForSmallImages(&encParams, frameInfo)
	c.configureTargetRatio(&encParams, p, targetRatio)
	encParams.CustomQuantSteps = customQuantSteps(p, encParams.NumLevels)
	c.extractMCTParameters(&encParams, p)
	encoder := jpeg2000.NewEncoder(&encParams)
	encoded, err := encoder.Encode(frameData)
	if err != nil {
		return nil, fmt.Errorf("JPEG 2000 encode failed: %w", err)
	}
	return encoded, nil
}

func (c *Codec) initializeBaseQuality(p *JPEG2000LossyParameters, targetRatio float64) int {
	baseQuality := 80
	if p.Rate > 0 {
		baseQuality = clampQuality(p.Rate)
	} else if targetRatio > 0 {
		baseQuality = qualityFromRatio(targetRatio)
	}
	return baseQuality
}

func (c *Codec) configureBasicEncodeParams(encParams *jpeg2000.EncodeParams, frameInfo *imagetypes.FrameInfo, p *JPEG2000LossyParameters, baseQuality int) {
	encParams.Lossless = !p.Irreversible
	encParams.NumLevels = clampNumLevels(p.NumLevels, int(frameInfo.Width), int(frameInfo.Height))
	encParams.NumLayers = p.NumLayers
	encParams.EnableMCT = p.AllowMCT
	encParams.Quality = effectiveQuality(baseQuality, p.QuantStepScale)
	if p.Irreversible && p.Rate > 0 && p.TargetRatio <= 0 {
		encParams.LayerRates = openJPEGLayerRates(p.Rate, p.RateLevels, int(frameInfo.BitsStored), int(frameInfo.BitsAllocated))
		if len(encParams.LayerRates) > 0 {
			encParams.NumLayers = len(encParams.LayerRates)
			encParams.UsePCRDOpt = true
		}
	}
	if !encParams.Lossless && int(frameInfo.SamplesPerPixel) >= 3 && encParams.Quality < 100 {
		bump := 10
		q := encParams.Quality + bump
		if q > 100 {
			q = 100
		}
		encParams.Quality = q
	}
}

func (c *Codec) adjustForSmallImages(encParams *jpeg2000.EncodeParams, frameInfo *imagetypes.FrameInfo) {
	minDim := int(frameInfo.Width)
	if int(frameInfo.Height) < minDim {
		minDim = int(frameInfo.Height)
	}
	if int(frameInfo.SamplesPerPixel) >= 3 && minDim <= 32 {
		encParams.Lossless = true
		if encParams.NumLevels > 1 {
			encParams.NumLevels = 1
		}
	}
	if !encParams.Lossless && minDim <= 48 {
		if encParams.NumLevels > 1 {
			encParams.NumLevels = 1
		}
		if encParams.NumLayers > 2 {
			encParams.NumLayers = 2
		}
		if encParams.Quality < 92 {
			encParams.Quality = 92
		}
	}
}

func (c *Codec) configureTargetRatio(encParams *jpeg2000.EncodeParams, p *JPEG2000LossyParameters, targetRatio float64) {
	if targetRatio > 0 && !encParams.Lossless {
		encParams.TargetRatio = targetRatio
		encParams.UsePCRDOpt = true
		if encParams.NumLayers <= 1 {
			encParams.NumLayers = layersFromRateLevels(p.Rate, p.RateLevels)
		}
	}
}

func (c *Codec) extractMCTParameters(encParams *jpeg2000.EncodeParams, p *JPEG2000LossyParameters) {
	if p == nil {
		return
	}
	if v := p.GetParameter("mctMatrix"); v != nil {
		if m, ok := v.([][]float64); ok {
			encParams.MCTMatrix = m
		}
	}
	if v := p.GetParameter("inverseMctMatrix"); v != nil {
		if m, ok := v.([][]float64); ok {
			encParams.InverseMCTMatrix = m
		}
	}
	if v := p.GetParameter("mctOffsets"); v != nil {
		if m, ok := v.([]int32); ok {
			encParams.MCTOffsets = m
		}
	}
	if v := p.GetParameter("mctNormScale"); v != nil {
		switch x := v.(type) {
		case float64:
			encParams.MCTNormScale = x
		case float32:
			encParams.MCTNormScale = float64(x)
		}
	}
	if v := p.GetParameter("mctAssocType"); v != nil {
		if t, ok := v.(uint8); ok {
			encParams.MCTAssocType = t
		}
	}
	if v := p.GetParameter("mctMatrixElementType"); v != nil {
		if t, ok := v.(uint8); ok {
			encParams.MCTMatrixElementType = t
		}
	}
	if v := p.GetParameter("mcoPrecision"); v != nil {
		if t, ok := v.(uint8); ok {
			encParams.MCOPrecision = t
		}
	}
	if v := p.GetParameter("mcoRecordOrder"); v != nil {
		if arr, ok := v.([]uint8); ok {
			encParams.MCORecordOrder = arr
		}
	}
	if v := p.GetParameter("mctBindings"); v != nil {
		if arr, ok := v.([]jpeg2000.MCTBindingParams); ok {
			encParams.MCTBindings = arr
		}
	}
}

// encodeFrameWithTargetRatio performs rate control on quality to reach target ratio for a single frame.
func (c *Codec) encodeFrameWithTargetRatio(frameData []byte, frameInfo *imagetypes.FrameInfo, base *JPEG2000LossyParameters, baseEncParams *jpeg2000.EncodeParams) ([]byte, error) {
	target := base.TargetRatio
	if target <= 0 {
		return c.encodeFrameOnce(frameData, frameInfo, base, baseEncParams)
	}

	pcopy := *base
	pcopy.TargetRatio = target
	return c.encodeFrameOnce(frameData, frameInfo, &pcopy, baseEncParams)
}

// clampNumLevels limits decomposition levels so the LL band remains at least 2x2.
// This avoids excessive quantization on very small images (e.g., 16x16).
func clampNumLevels(requested, width, height int) int {
	minDim := width
	if height < minDim {
		minDim = height
	}

	maxLevels := 0
	// Each level halves dimensions; keep LL >= 2 pixels on the shortest side.
	for maxLevels < 6 && (minDim>>(maxLevels+1)) >= 1 {
		maxLevels++
	}
	if maxLevels < 0 {
		maxLevels = 0
	}
	if requested > maxLevels {
		return maxLevels
	}
	return requested
}

// effectiveQuality derives the quantization quality baseline from a base quality
// and quantization step scaling.
func effectiveQuality(baseQuality int, quantStepScale float64) int {
	q := clampQuality(baseQuality)

	if quantStepScale > 0 && quantStepScale != 1.0 {
		// baseStep = 2^((100-quality)/12.5); scaling step by S is equivalent to reducing quality by 12.5*log2(S).
		adjust := int(math.Round(12.5 * math.Log2(quantStepScale)))
		q = clampQuality(q - adjust)
	}

	return clampQuality(q)
}

func qualityFromRatio(ratio float64) int {
	if ratio <= 0 {
		return 80
	}
	// Heuristic: quality drops logarithmically with target ratio.
	// ratio=2 -> ~85, ratio=3 -> ~76, ratio=5 -> ~65, ratio=10 -> ~50
	q := int(math.Round(100 - 15*math.Log2(ratio)))
	return clampQuality(q)
}

func layersFromRateLevels(rate int, levels []int) int {
	if rate <= 0 || len(levels) == 0 {
		return 1
	}
	layers := 1
	for _, v := range levels {
		if v > rate {
			layers++
		}
	}
	if layers < 1 {
		return 1
	}
	return layers
}

func openJPEGLayerRates(rate int, levels []int, bitsStored, bitsAllocated int) []float64 {
	if rate <= 0 {
		return nil
	}
	rates := make([]float64, 0, len(levels)+1)
	for _, v := range levels {
		if v > rate {
			rates = append(rates, float64(v))
			continue
		}
		break
	}
	if bitsAllocated <= 0 {
		bitsAllocated = bitsStored
	}
	if bitsStored <= 0 || bitsAllocated <= 0 {
		rates = append(rates, float64(rate))
		return rates
	}
	rates = append(rates, float64(rate)*float64(bitsStored)/float64(bitsAllocated))
	return rates
}

func clampQuality(q int) int {
	if q < 1 {
		return 1
	}
	if q > 100 {
		return 100
	}
	return q
}

// customQuantSteps returns per-subbands quant steps if provided and sized correctly, applying QuantStepScale.
func customQuantSteps(p *JPEG2000LossyParameters, numLevels int) []float64 {
	if len(p.SubbandSteps) == 0 {
		return nil
	}
	expected := 3*numLevels + 1
	if len(p.SubbandSteps) != expected {
		return nil
	}
	if p.QuantStepScale == 1.0 {
		return p.SubbandSteps
	}
	out := make([]float64, len(p.SubbandSteps))
	for i, v := range p.SubbandSteps {
		out[i] = v * p.QuantStepScale
	}
	return out
}
