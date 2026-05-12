// Package lossless provides JPEG-LS lossless codec implementations.
package lossless

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

var _ codec.Codec = (*JPEGLSLosslessCodec)(nil)

// JPEGLSLosslessCodec implements the external codec.Codec interface for JPEG-LS Lossless
type JPEGLSLosslessCodec struct {
	transferSyntax *transfer.Syntax
}

// NewJPEGLSLosslessCodec creates a new JPEG-LS Lossless codec
func NewJPEGLSLosslessCodec() *JPEGLSLosslessCodec {
	return &JPEGLSLosslessCodec{
		transferSyntax: transfer.JPEGLSLossless,
	}
}

// Name returns the codec name
func (c *JPEGLSLosslessCodec) Name() string {
	return "JPEG-LS Lossless"
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *JPEGLSLosslessCodec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// GetDefaultParameters returns the default codec parameters
func (c *JPEGLSLosslessCodec) GetDefaultParameters() codec.Parameters {
	return codec.NewBaseParameters()
}

// Encode encodes pixel data to JPEG-LS Lossless format
func (c *JPEGLSLosslessCodec) Encode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, _ codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return fmt.Errorf("failed to get frame info from source pixel data")
	}

	// Validate bit depth (JPEG-LS supports 2-16 bits)
	if frameInfo.BitsStored < 2 || frameInfo.BitsStored > 16 {
		return fmt.Errorf("JPEG-LS supports 2-16 bit depth, got %d bits", frameInfo.BitsStored)
	}

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

		// JPEG-LS uses predictive coding with differences, which naturally handles
		// both signed and unsigned data without needing pixel value shifting.
		// DO NOT shift pixel data for lossless encoding.

		// Encode using the JPEG-LS encoder
		jpegData, err := Encode(
			frameData, // No adjustment needed
			int(frameInfo.Width),
			int(frameInfo.Height),
			int(frameInfo.SamplesPerPixel),
			int(frameInfo.BitsStored),
		)
		if err != nil {
			return fmt.Errorf("JPEG-LS Lossless encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(jpegData); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG-LS Lossless data to uncompressed pixel data
func (c *JPEGLSLosslessCodec) Decode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, _ codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return fmt.Errorf("failed to get frame info from source pixel data")
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

		// Decode using the JPEG-LS decoder
		pixelData, width, height, _, _, err := Decode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG-LS Lossless decode failed for frame %d: %w", frameIndex, err)
		}

		// Verify dimensions match if specified
		if frameInfo.Width > 0 && width != int(frameInfo.Width) {
			return fmt.Errorf("decoded width (%d) doesn't match expected (%d)", width, frameInfo.Width)
		}
		if frameInfo.Height > 0 && height != int(frameInfo.Height) {
			return fmt.Errorf("decoded height (%d) doesn't match expected (%d)", height, frameInfo.Height)
		}

		// JPEG-LS decodes directly to the original pixel representation.
		// No pixel value shifting needed - the codec preserves the original two's complement encoding.

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(pixelData); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterJPEGLSLosslessCodec registers the JPEG-LS Lossless codec with the global registry
func RegisterJPEGLSLosslessCodec() {
	registry := codec.GetGlobalRegistry()
	jpegLSCodec := NewJPEGLSLosslessCodec()
	registry.RegisterCodec(transfer.JPEGLSLossless, jpegLSCodec)
}

func init() {
	RegisterJPEGLSLosslessCodec()
}
