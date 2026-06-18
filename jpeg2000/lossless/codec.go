// Package lossless provides JPEG 2000 Lossless codec implementations.
package lossless

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

var _ codec.Codec = (*Codec)(nil)

const j2kLosslessName = "JPEG 2000 Lossless"

// Codec implements the JPEG 2000 Lossless codec
// Transfer Syntax UID: 1.2.840.10008.1.2.4.90
type Codec struct {
	transferSyntax *transfer.Syntax
}

// NewCodec creates a new JPEG 2000 Lossless codec
func NewCodec() *Codec {
	return NewCodecWithTransferSyntax(transfer.JPEG2000Lossless)
}

// NewCodecWithTransferSyntax allows constructing the codec for alternate JPEG 2000 transfer syntaxes.
func NewCodecWithTransferSyntax(ts *transfer.Syntax) *Codec {
	return &Codec{
		transferSyntax: ts,
	}
}

// NewPart2MultiComponentLosslessCodec creates a JPEG 2000 Part 2 Multi-component Lossless codec (UID .92)
func NewPart2MultiComponentLosslessCodec() *Codec {
	return NewCodecWithTransferSyntax(transfer.JPEG2000Part2MultiComponentLosslessOnly)
}

// Name returns the codec name
func (c *Codec) Name() string {
	return j2kLosslessName
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// GetDefaultParameters returns the default codec parameters
func (c *Codec) GetDefaultParameters() codec.Parameters {
	return NewLosslessParameters()
}

// Encode encodes pixel data to JPEG 2000 Lossless format
func (c *Codec) Encode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, parameters codec.Parameters) error {
	frameInfo, err := c.validateLosslessEncodeInputs(oldPixelData, newPixelData)
	if err != nil {
		return err
	}
	losslessParams := c.extractLosslessParameters(parameters)
	if err := losslessParams.Validate(); err != nil {
		return fmt.Errorf("invalid JPEG2000 lossless parameters: %w", err)
	}
	encParams := c.configureLosslessEncodeParams(frameInfo, losslessParams)
	c.extractLosslessMCTParameters(encParams, losslessParams, parameters)
	encoder := jpeg2000.NewEncoder(encParams)
	return c.encodeLosslessAllFrames(oldPixelData, newPixelData, encoder)
}

func (c *Codec) validateLosslessEncodeInputs(oldPixelData, newPixelData imagetypes.PixelData) (*imagetypes.FrameInfo, error) {
	if oldPixelData == nil || newPixelData == nil {
		return nil, fmt.Errorf("source and destination PixelData cannot be nil")
	}
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return nil, fmt.Errorf("failed to get frame info from source pixel data")
	}
	return frameInfo, nil
}

func (c *Codec) extractLosslessParameters(parameters codec.Parameters) *JPEG2000LosslessParameters {
	if parameters == nil {
		return NewLosslessParameters()
	}
	if jp, ok := parameters.(*JPEG2000LosslessParameters); ok {
		return jp
	}
	losslessParams := NewLosslessParameters()
	c.extractBasicLosslessParams(losslessParams, parameters)
	return losslessParams
}

func (c *Codec) extractBasicLosslessParams(losslessParams *JPEG2000LosslessParameters, parameters codec.Parameters) {
	if n := parameters.GetParameter("numLevels"); n != nil {
		if nInt, ok := n.(int); ok && nInt >= 0 && nInt <= 6 {
			losslessParams.NumLevels = nInt
		}
	}
	if v := parameters.GetParameter("allowMCT"); v != nil {
		if b, ok := v.(bool); ok {
			losslessParams.AllowMCT = b
		}
	}
	if v := parameters.GetParameter("rate"); v != nil {
		if r, ok := v.(int); ok && r > 0 {
			losslessParams.Rate = r
		}
	}
	if v := parameters.GetParameter("rateLevels"); v != nil {
		if arr, ok := v.([]int); ok && len(arr) > 0 {
			losslessParams.RateLevels = arr
		}
	}
	if v := parameters.GetParameter("progressionOrder"); v != nil {
		switch x := v.(type) {
		case int:
			if x >= 0 {
				losslessParams.ProgressionOrder = uint8(x)
			}
		case uint8:
			losslessParams.ProgressionOrder = x
		}
	}
	if v := parameters.GetParameter("numLayers"); v != nil {
		if nInt, ok := v.(int); ok {
			losslessParams.NumLayers = nInt
		}
	}
	if v := parameters.GetParameter("targetRatio"); v != nil {
		switch x := v.(type) {
		case float64:
			losslessParams.TargetRatio = x
		case float32:
			losslessParams.TargetRatio = float64(x)
		case int:
			losslessParams.TargetRatio = float64(x)
		}
	}
	if v := parameters.GetParameter("usePCRDOpt"); v != nil {
		if b, ok := v.(bool); ok {
			losslessParams.UsePCRDOpt = b
		}
	}
	if v := parameters.GetParameter("appendLosslessLayer"); v != nil {
		if b, ok := v.(bool); ok {
			losslessParams.AppendLosslessLayer = b
		}
	}
}

func (c *Codec) configureLosslessEncodeParams(frameInfo *imagetypes.FrameInfo, losslessParams *JPEG2000LosslessParameters) *jpeg2000.EncodeParams {
	encParams := jpeg2000.DefaultEncodeParams(
		int(frameInfo.Width),
		int(frameInfo.Height),
		int(frameInfo.SamplesPerPixel),
		int(frameInfo.BitsStored),
		frameInfo.PixelRepresentation != 0,
	)
	encParams.NumLevels = losslessParams.NumLevels
	encParams.ProgressionOrder = losslessParams.ProgressionOrder
	encParams.NumLayers = losslessParams.NumLayers
	targetRatio := losslessParams.TargetRatio
	if targetRatio <= 0 && losslessParams.Rate > 0 {
		targetRatio = rateToTargetRatio(losslessParams.Rate, int(frameInfo.BitsStored), int(frameInfo.BitsAllocated))
	}
	encParams.TargetRatio = targetRatio
	encParams.UsePCRDOpt = losslessParams.UsePCRDOpt || targetRatio > 0
	encParams.EnableMCT = losslessParams.AllowMCT
	encParams.AppendLosslessLayer = losslessParams.AppendLosslessLayer
	if targetRatio > 0 && encParams.NumLayers <= 1 {
		encParams.NumLayers = layersFromRateLevels(losslessParams.Rate, losslessParams.RateLevels)
	}
	if targetRatio > 0 && losslessParams.AppendLosslessLayer {
		encParams.NumLayers++
	}
	encParams.LayerRates = openJPEGLayerRates(
		losslessParams.Rate,
		losslessParams.RateLevels,
		int(frameInfo.BitsStored),
		int(frameInfo.BitsAllocated),
		losslessParams.AppendLosslessLayer,
	)
	return encParams
}

func (c *Codec) extractLosslessMCTParameters(encParams *jpeg2000.EncodeParams, losslessParams *JPEG2000LosslessParameters, parameters codec.Parameters) {
	if !losslessParams.AllowMCT || parameters == nil {
		return
	}
	if v := parameters.GetParameter("mctMatrix"); v != nil {
		if m, ok := v.([][]float64); ok {
			encParams.MCTMatrix = m
		}
	}
	if v := parameters.GetParameter("inverseMctMatrix"); v != nil {
		if m, ok := v.([][]float64); ok {
			encParams.InverseMCTMatrix = m
		}
	}
	if v := parameters.GetParameter("mctOffsets"); v != nil {
		if m, ok := v.([]int32); ok {
			encParams.MCTOffsets = m
		}
	}
	if v := parameters.GetParameter("mctNormScale"); v != nil {
		switch x := v.(type) {
		case float64:
			encParams.MCTNormScale = x
		case float32:
			encParams.MCTNormScale = float64(x)
		}
	}
	if v := parameters.GetParameter("mctAssocType"); v != nil {
		if t, ok := v.(uint8); ok {
			encParams.MCTAssocType = t
		}
	}
	if v := parameters.GetParameter("mctMatrixElementType"); v != nil {
		if t, ok := v.(uint8); ok {
			encParams.MCTMatrixElementType = t
		}
	}
	if v := parameters.GetParameter("mcoPrecision"); v != nil {
		if t, ok := v.(uint8); ok {
			encParams.MCOPrecision = t
		}
	}
	if v := parameters.GetParameter("mcoRecordOrder"); v != nil {
		if arr, ok := v.([]uint8); ok {
			encParams.MCORecordOrder = arr
		}
	}
	if v := parameters.GetParameter("mctBindings"); v != nil {
		if arr, ok := v.([]jpeg2000.MCTBindingParams); ok {
			encParams.MCTBindings = arr
		}
	}
}

func (c *Codec) encodeLosslessAllFrames(oldPixelData, newPixelData imagetypes.PixelData, encoder *jpeg2000.Encoder) error {
	frameCount := oldPixelData.FrameCount()
	if frameCount == 0 {
		return fmt.Errorf("source pixel data is empty (no frames)")
	}
	for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
		frameData, err := oldPixelData.GetFrame(frameIndex)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", frameIndex, err)
		}
		if len(frameData) == 0 {
			return fmt.Errorf("frame %d pixel data is empty", frameIndex)
		}
		encoded, err := encoder.Encode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG 2000 encode failed for frame %d: %w", frameIndex, err)
		}
		if err := newPixelData.AddFrame(encoded); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}
	return nil
}

// Decode decodes JPEG 2000 Lossless data to uncompressed pixel data
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

		// Decode
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

// RegisterJPEG2000LosslessCodec registers the JPEG 2000 Lossless codec with the global registry
func RegisterJPEG2000LosslessCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewCodec()
	registry.RegisterCodec(transfer.JPEG2000Lossless, j2kCodec)
}

// RegisterJPEG2000MCLosslessCodec registers JPEG 2000 Part 2 Multi-component lossless codec.
func RegisterJPEG2000MCLosslessCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewPart2MultiComponentLosslessCodec()
	registry.RegisterCodec(transfer.JPEG2000Part2MultiComponentLosslessOnly, j2kCodec)
}

func init() {
	RegisterJPEG2000LosslessCodec()
	RegisterJPEG2000MCLosslessCodec()
}

func rateToTargetRatio(rate, bitsStored, bitsAllocated int) float64 {
	if rate <= 0 {
		return 0
	}
	if bitsAllocated <= 0 {
		bitsAllocated = bitsStored
	}
	if bitsStored <= 0 || bitsAllocated <= 0 {
		return float64(rate)
	}
	return float64(rate) * float64(bitsStored) / float64(bitsAllocated)
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

func openJPEGLayerRates(rate int, levels []int, bitsStored, bitsAllocated int, appendLossless bool) []float64 {
	if rate <= 0 {
		return nil
	}
	rates := make([]float64, 0, len(levels)+2)
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
	} else {
		rates = append(rates, float64(rate)*float64(bitsStored)/float64(bitsAllocated))
	}
	if appendLossless {
		rates = append(rates, 0)
	}
	return rates
}
