// Package lossless provides JPEG 2000 Lossless codec implementations.
package lossless

import (
	"context"
	"fmt"

	"github.com/cocosip/go-dicom-codecs/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
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

// DefaultParameters returns the default codec parameters.
func (c *Codec) DefaultParameters() codec.Parameters {
	return NewLosslessParameters()
}

// Encode encodes pixel data to JPEG 2000 Lossless format
func (c *Codec) Encode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, parameters codec.Parameters) error {
	frameInfo, err := c.validateLosslessEncodeInputs(oldPixelData, newPixelData)
	if err != nil {
		return err
	}
	losslessParams, err := c.extractLosslessParameters(parameters)
	if err != nil {
		return err
	}
	if err := losslessParams.Validate(); err != nil {
		return fmt.Errorf("invalid JPEG2000 lossless parameters: %w", err)
	}
	encParams := c.configureLosslessEncodeParams(frameInfo, losslessParams)
	c.extractLosslessMCTParameters(encParams, losslessParams)
	encoder := jpeg2000.NewEncoder(encParams)
	return c.encodeLosslessAllFrames(ctx, oldPixelData, newPixelData, encoder)
}

func (c *Codec) validateLosslessEncodeInputs(oldPixelData codec.FrameSource, newPixelData codec.FrameSink) (codec.FrameInfo, error) {
	if oldPixelData == nil || newPixelData == nil {
		return codec.FrameInfo{}, fmt.Errorf("source and destination PixelData cannot be nil")
	}
	return oldPixelData.FrameInfo(), nil
}

func (c *Codec) extractLosslessParameters(parameters codec.Parameters) (*JPEG2000LosslessParameters, error) {
	if parameters == nil {
		return NewLosslessParameters(), nil
	}
	losslessParams, ok := parameters.(*JPEG2000LosslessParameters)
	if !ok || losslessParams == nil {
		return nil, fmt.Errorf("%w: JPEG 2000 Lossless requires *JPEG2000LosslessParameters, got %T", codec.ErrInvalidParameters, parameters)
	}
	return losslessParams, nil
}

func (c *Codec) configureLosslessEncodeParams(frameInfo codec.FrameInfo, losslessParams *JPEG2000LosslessParameters) *jpeg2000.EncodeParams {
	encParams := jpeg2000.DefaultEncodeParams(
		int(frameInfo.Width),
		int(frameInfo.Height),
		int(frameInfo.SamplesPerPixel),
		int(frameInfo.BitDepth.BitsStored),
		frameInfo.PixelRepresentation.IsSigned(),
	)
	encParams.NumLevels = losslessParams.NumLevels
	encParams.ProgressionOrder = losslessParams.ProgressionOrder
	encParams.NumLayers = losslessParams.NumLayers
	targetRatio := losslessParams.TargetRatio
	if targetRatio <= 0 && losslessParams.Rate > 0 {
		targetRatio = rateToTargetRatio(losslessParams.Rate, int(frameInfo.BitDepth.BitsStored), int(frameInfo.BitDepth.BitsAllocated))
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
		int(frameInfo.BitDepth.BitsStored),
		int(frameInfo.BitDepth.BitsAllocated),
		losslessParams.AppendLosslessLayer,
	)
	return encParams
}

func (c *Codec) extractLosslessMCTParameters(encParams *jpeg2000.EncodeParams, parameters *JPEG2000LosslessParameters) {
	if !parameters.AllowMCT {
		return
	}
	getter := parameters
	if v := getter.GetParameter("mctMatrix"); v != nil {
		if m, ok := v.([][]float64); ok {
			encParams.MCTMatrix = m
		}
	}
	if v := getter.GetParameter("inverseMctMatrix"); v != nil {
		if m, ok := v.([][]float64); ok {
			encParams.InverseMCTMatrix = m
		}
	}
	if v := getter.GetParameter("mctOffsets"); v != nil {
		if m, ok := v.([]int32); ok {
			encParams.MCTOffsets = m
		}
	}
	if v := getter.GetParameter("mctNormScale"); v != nil {
		switch x := v.(type) {
		case float64:
			encParams.MCTNormScale = x
		case float32:
			encParams.MCTNormScale = float64(x)
		}
	}
	if v := getter.GetParameter("mctAssocType"); v != nil {
		if t, ok := v.(uint8); ok {
			encParams.MCTAssocType = t
		}
	}
	if v := getter.GetParameter("mctMatrixElementType"); v != nil {
		if t, ok := v.(uint8); ok {
			encParams.MCTMatrixElementType = t
		}
	}
	if v := getter.GetParameter("mcoPrecision"); v != nil {
		if t, ok := v.(uint8); ok {
			encParams.MCOPrecision = t
		}
	}
	if v := getter.GetParameter("mcoRecordOrder"); v != nil {
		if arr, ok := v.([]uint8); ok {
			encParams.MCORecordOrder = arr
		}
	}
	if v := getter.GetParameter("mctBindings"); v != nil {
		if arr, ok := v.([]jpeg2000.MCTBindingParams); ok {
			encParams.MCTBindings = arr
		}
	}
}

func (c *Codec) encodeLosslessAllFrames(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, encoder *jpeg2000.Encoder) error {
	frameCount := oldPixelData.FrameCount()
	if frameCount == 0 {
		return fmt.Errorf("source pixel data is empty (no frames)")
	}
	for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
		frameData, err := oldPixelData.Frame(ctx, frameIndex)
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
		if err := newPixelData.AddFrame(ctx, encoded); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}
	return nil
}

// Decode decodes JPEG 2000 Lossless data to uncompressed pixel data
func (c *Codec) Decode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, _ codec.Parameters) error {
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
		frameData, err := oldPixelData.Frame(ctx, frameIndex)
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
		if err := newPixelData.AddFrame(ctx, decoder.GetPixelData()); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterJPEG2000LosslessCodec registers the JPEG 2000 Lossless codec with the global registry
func RegisterJPEG2000LosslessCodec() {
	registry := codec.GlobalRegistry()
	j2kCodec := NewCodec()
	if _, err := registry.Replace(j2kCodec); err != nil {
		panic(err)
	}
}

// RegisterJPEG2000MCLosslessCodec registers JPEG 2000 Part 2 Multi-component lossless codec.
func RegisterJPEG2000MCLosslessCodec() {
	registry := codec.GlobalRegistry()
	j2kCodec := NewPart2MultiComponentLosslessCodec()
	if _, err := registry.Replace(j2kCodec); err != nil {
		panic(err)
	}
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
