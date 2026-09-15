// Package lossless provides the JPEG Lossless (SV1) codec implementation.
package lossless

import (
	"context"
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*Codec)(nil)

// Codec implements the external codec.Codec interface for JPEG Lossless (Process 14)
type Codec struct {
	transferSyntax *transfer.Syntax
	predictor      int // 0 for auto-select, 1-7 for specific predictor
}

// NewLosslessCodec creates a new JPEG Lossless codec
// predictor: 0 for auto-select, 1-7 for specific predictor
func NewLosslessCodec(predictor int) *Codec {
	return &Codec{
		transferSyntax: transfer.JPEGLossless,
		predictor:      predictor,
	}
}

// Name returns the codec name
func (c *Codec) Name() string {
	if c.predictor == 0 {
		return "JPEG Lossless (Auto Predictor)"
	}
	return fmt.Sprintf("JPEG Lossless (Predictor %d - %s)", c.predictor, PredictorName(c.predictor))
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// DefaultParameters returns the default codec parameters.
func (c *Codec) DefaultParameters() codec.Parameters {
	params := NewLosslessParameters()
	params.Predictor = c.predictor
	return params
}

// Encode encodes pixel data to JPEG Lossless format
func (c *Codec) Encode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.FrameInfo()

	// Get encoding parameters
	var losslessParams *JPEGLosslessParameters
	if parameters == nil {
		losslessParams = c.DefaultParameters().(*JPEGLosslessParameters)
	} else {
		var ok bool
		losslessParams, ok = parameters.(*JPEGLosslessParameters)
		if !ok || losslessParams == nil {
			return fmt.Errorf("%w: JPEG Lossless requires *JPEGLosslessParameters, got %T", codec.ErrInvalidParameters, parameters)
		}
	}

	// Validate parameters
	if err := losslessParams.Validate(); err != nil {
		return fmt.Errorf("invalid JPEG Lossless parameters: %w", err)
	}
	predictor := losslessParams.Predictor
	// DICOM UID 1.2.840.10008.1.2.4.57 (transfer.JPEGLossless) is Selection Value 1 only.
	// Enforce predictor=1 for this TS to match reader expectations.
	if c.transferSyntax == transfer.JPEGLossless || predictor == 0 {
		predictor = 1
	}

	// Process all frames
	frameCount := oldPixelData.FrameCount()
	if frameCount == 0 {
		return fmt.Errorf("source pixel data is empty (no frames)")
	}
	for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
		// Get frame data
		frameData, err := oldPixelData.Frame(ctx, frameIndex)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", frameIndex, err)
		}

		if len(frameData) == 0 {
			return fmt.Errorf("frame %d pixel data is empty", frameIndex)
		}

		// JPEG Lossless uses predictive coding with differences, which naturally handles
		// both signed and unsigned data without needing pixel value shifting.
		// The predictor works with raw byte values regardless of pixel representation.
		// DO NOT shift pixel data for lossless JPEG encoding.

		// Encode using the lossless encoder
		jpegData, err := Encode(
			frameData, // No adjustment needed
			int(frameInfo.Width),
			int(frameInfo.Height),
			int(frameInfo.SamplesPerPixel),
			int(frameInfo.BitDepth.BitsStored),
			predictor,
		)
		if err != nil {
			return fmt.Errorf("JPEG Lossless encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(ctx, jpegData); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG Lossless data to uncompressed pixel data
func (c *Codec) Decode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, _ codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.FrameInfo()

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

		// Decode using the lossless decoder
		pixelData, width, height, components, _, err := Decode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG Lossless decode failed for frame %d: %w", frameIndex, err)
		}

		// Verify dimensions match
		if width != int(frameInfo.Width) || height != int(frameInfo.Height) {
			return fmt.Errorf("decoded dimensions (%dx%d) don't match expected (%dx%d)",
				width, height, frameInfo.Width, frameInfo.Height)
		}

		if components != int(frameInfo.SamplesPerPixel) {
			return fmt.Errorf("decoded components (%d) don't match expected (%d)",
				components, frameInfo.SamplesPerPixel)
		}

		// JPEG Lossless decodes directly to the original pixel representation.
		// No pixel value shifting needed - the codec preserves the original two's complement encoding.

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(ctx, pixelData); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterLosslessCodec registers the JPEG Lossless codec with the global registry
func RegisterLosslessCodec(predictor int) {
	registry := codec.GlobalRegistry()
	losslessCodec := NewLosslessCodec(predictor)
	if _, err := registry.Replace(losslessCodec); err != nil {
		panic(err)
	}
}

func init() {
	RegisterLosslessCodec(0)
}
