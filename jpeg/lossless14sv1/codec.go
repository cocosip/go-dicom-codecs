// Package lossless14sv1 provides JPEG Lossless (SV1) codec implementations.
package lossless14sv1

import (
	"context"
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*LosslessSV1Codec)(nil)

// LosslessSV1Codec implements the external codec.Codec interface for JPEG Lossless SV1
// SV1 (Selection Value 1) means it only uses predictor 1 (left pixel)
type LosslessSV1Codec struct {
	transferSyntax *transfer.Syntax
}

// NewLosslessSV1Codec creates a new JPEG Lossless SV1 codec
func NewLosslessSV1Codec() *LosslessSV1Codec {
	return &LosslessSV1Codec{
		transferSyntax: transfer.JPEGLosslessSV1,
	}
}

// Name returns the codec name
func (c *LosslessSV1Codec) Name() string {
	return "JPEG Lossless SV1 (Predictor 1)"
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *LosslessSV1Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// DefaultParameters returns the default codec parameters.
func (c *LosslessSV1Codec) DefaultParameters() codec.Parameters {
	return codec.NoParameters{}
}

// Encode encodes pixel data to JPEG Lossless SV1 format
func (c *LosslessSV1Codec) Encode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, _ codec.Parameters) error {
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

		// Encode using the lossless SV1 encoder
		jpegData, err := Encode(
			frameData,
			int(frameInfo.Width),
			int(frameInfo.Height),
			int(frameInfo.SamplesPerPixel),
			int(frameInfo.BitDepth.BitsStored),
		)
		if err != nil {
			return fmt.Errorf("JPEG Lossless SV1 encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(ctx, jpegData); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG Lossless SV1 data to uncompressed pixel data
func (c *LosslessSV1Codec) Decode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, _ codec.Parameters) error {
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

		// Decode using the lossless SV1 decoder
		pixelData, width, height, components, _, err := Decode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG Lossless SV1 decode failed for frame %d: %w", frameIndex, err)
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

		// JPEG Lossless decoder outputs raw pixel values as encoded.
		// No reverse shifting needed - pixel representation is preserved in raw bytes.

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(ctx, pixelData); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterLosslessSV1Codec registers the JPEG Lossless SV1 codec with the global registry
func RegisterLosslessSV1Codec() {
	registry := codec.GlobalRegistry()
	losslessSV1Codec := NewLosslessSV1Codec()
	if _, err := registry.Replace(losslessSV1Codec); err != nil {
		panic(err)
	}
}

func init() {
	RegisterLosslessSV1Codec()
}
