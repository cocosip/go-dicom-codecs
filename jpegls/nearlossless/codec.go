// Package nearlossless implements a JPEG-LS Near-Lossless example codec.
package nearlossless

import (
	"context"
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*JPEGLSNearLosslessCodec)(nil)

// JPEGLSNearLosslessCodec implements the external codec.Codec interface for JPEG-LS Near-Lossless
type JPEGLSNearLosslessCodec struct {
	transferSyntax *transfer.Syntax
	defaultNEAR    int // Default NEAR parameter (0-255)
}

// NewJPEGLSNearLosslessCodec creates a new JPEG-LS Near-Lossless codec
// defaultNEAR: default error bound (0=lossless, 1-255=near-lossless)
func NewJPEGLSNearLosslessCodec(defaultNEAR int) *JPEGLSNearLosslessCodec {
	if defaultNEAR < 0 || defaultNEAR > 255 {
		defaultNEAR = 3 // Default near-lossless value used by fo-dicom
	}
	return &JPEGLSNearLosslessCodec{
		transferSyntax: transfer.JPEGLSNearLossless,
		defaultNEAR:    defaultNEAR,
	}
}

// Name returns the codec name
func (c *JPEGLSNearLosslessCodec) Name() string {
	return fmt.Sprintf("JPEG-LS Near-Lossless (NEAR=%d)", c.defaultNEAR)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *JPEGLSNearLosslessCodec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// DefaultParameters returns the default codec parameters.
func (c *JPEGLSNearLosslessCodec) DefaultParameters() codec.Parameters {
	params := NewNearLosslessParameters()
	params.NEAR = c.defaultNEAR
	return params
}

// Encode encodes pixel data to JPEG-LS Near-Lossless format
func (c *JPEGLSNearLosslessCodec) Encode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.FrameInfo()

	// Validate bit depth (JPEG-LS supports 2-16 bits)
	if frameInfo.BitDepth.BitsStored < 2 || frameInfo.BitDepth.BitsStored > 16 {
		return fmt.Errorf("JPEG-LS supports 2-16 bit depth, got %d bits", frameInfo.BitDepth.BitsStored)
	}

	// Get encoding parameters
	var nearLosslessParams *JPEGLSNearLosslessParameters
	if parameters == nil {
		nearLosslessParams = c.DefaultParameters().(*JPEGLSNearLosslessParameters)
	} else {
		var ok bool
		nearLosslessParams, ok = parameters.(*JPEGLSNearLosslessParameters)
		if !ok || nearLosslessParams == nil {
			return fmt.Errorf("%w: JPEG-LS Near-Lossless requires *JPEGLSNearLosslessParameters, got %T", codec.ErrInvalidParameters, parameters)
		}
	}

	// Validate parameters
	if err := nearLosslessParams.Validate(); err != nil {
		return fmt.Errorf("invalid JPEG-LS Near-Lossless parameters: %w", err)
	}
	near := nearLosslessParams.NEAR

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

		// Encode using the JPEG-LS near-lossless encoder
		jpegData, err := Encode(
			frameData,
			int(frameInfo.Width),
			int(frameInfo.Height),
			int(frameInfo.SamplesPerPixel),
			int(frameInfo.BitDepth.BitsStored),
			near,
		)
		if err != nil {
			return fmt.Errorf("JPEG-LS Near-Lossless encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(ctx, jpegData); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG-LS Near-Lossless data to uncompressed pixel data
func (c *JPEGLSNearLosslessCodec) Decode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, _ codec.Parameters) error {
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

		// Decode using the JPEG-LS near-lossless decoder
		pixelData, width, height, _, _, _, err := Decode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG-LS Near-Lossless decode failed for frame %d: %w", frameIndex, err)
		}

		// Verify dimensions match if specified
		if frameInfo.Width > 0 && width != int(frameInfo.Width) {
			return fmt.Errorf("decoded width (%d) doesn't match expected (%d)", width, frameInfo.Width)
		}
		if frameInfo.Height > 0 && height != int(frameInfo.Height) {
			return fmt.Errorf("decoded height (%d) doesn't match expected (%d)", height, frameInfo.Height)
		}

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(ctx, pixelData); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterJPEGLSNearLosslessCodec registers the JPEG-LS Near-Lossless codec with the global registry
func RegisterJPEGLSNearLosslessCodec(defaultNEAR int) {
	registry := codec.GlobalRegistry()
	jpegLSCodec := NewJPEGLSNearLosslessCodec(defaultNEAR)
	if _, err := registry.Replace(jpegLSCodec); err != nil {
		panic(err)
	}
}

func init() {
	RegisterJPEGLSNearLosslessCodec(3)
}
