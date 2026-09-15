// Package extended provides JPEG Extended codec (8/12-bit) implementations.
package extended

import (
	"context"
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*Codec)(nil)

// Codec implements the external codec.Codec interface for JPEG Extended
type Codec struct {
	quality  int
	bitDepth int // 8 or 12
}

// NewExtendedCodec creates a new JPEG Extended codec
// bitDepth: 8 or 12 bits per sample
// quality: 1-100, where 100 is best quality (default 90)
func NewExtendedCodec(bitDepth int, quality int) *Codec {
	if bitDepth != 8 && bitDepth != 12 {
		bitDepth = 12 // Default to 12-bit (main feature of Extended)
	}
	if quality < 1 || quality > 100 {
		quality = 90 // Matches fo-dicom's default DicomJpegParams quality.
	}
	return &Codec{
		quality:  quality,
		bitDepth: bitDepth,
	}
}

// Name returns the codec name
func (c *Codec) Name() string {
	return fmt.Sprintf("JPEG Extended (%d-bit, Quality %d)", c.bitDepth, c.quality)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
	return transfer.JPEGExtended12Bit
}

// DefaultParameters returns the default codec parameters.
func (c *Codec) DefaultParameters() codec.Parameters {
	params := NewExtendedParameters()
	params.Quality = c.quality
	params.BitDepth = c.bitDepth
	return params
}

// Encode encodes pixel data using JPEG Extended
func (c *Codec) Encode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.FrameInfo()
	if frameInfo.BitDepth.BitsStored > 12 {
		return fmt.Errorf("JPEG Extended only supports up to 12-bit data, got %d bits", frameInfo.BitDepth.BitsStored)
	}

	// Extract parameters
	width := int(frameInfo.Width)
	height := int(frameInfo.Height)
	components := int(frameInfo.SamplesPerPixel)

	// Get encoding parameters
	var extendedParams *JPEGExtendedParameters
	if parameters == nil {
		extendedParams = c.DefaultParameters().(*JPEGExtendedParameters)
	} else {
		var ok bool
		extendedParams, ok = parameters.(*JPEGExtendedParameters)
		if !ok || extendedParams == nil {
			return fmt.Errorf("%w: JPEG Extended requires *JPEGExtendedParameters, got %T", codec.ErrInvalidParameters, parameters)
		}
	}

	// Validate parameters
	if err := extendedParams.Validate(); err != nil {
		return fmt.Errorf("invalid JPEG Extended parameters: %w", err)
	}

	// Determine bit depth from source if not explicitly set
	bitDepth := extendedParams.BitDepth
	if frameInfo.BitDepth.BitsStored > 0 && frameInfo.BitDepth.BitsStored <= 8 {
		bitDepth = 8
	} else if frameInfo.BitDepth.BitsStored > 8 && frameInfo.BitDepth.BitsStored <= 12 {
		bitDepth = 12
	}

	quality := extendedParams.Quality

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

		// Encode
		encoded, err := Encode(frameData, width, height, components, bitDepth, quality)
		if err != nil {
			return fmt.Errorf("JPEG Extended encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(ctx, encoded); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG Extended data
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

		// Decode
		decoded, _, _, _, _, err := Decode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG Extended decode failed for frame %d: %w", frameIndex, err)
		}

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(ctx, decoded); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterExtendedCodec registers JPEG Extended codec with the global registry
// bitDepth: 8 or 12 (default 12)
// quality: 1-100 (default 90)
func RegisterExtendedCodec(bitDepth int, quality int) {
	c := NewExtendedCodec(bitDepth, quality)
	registry := codec.GlobalRegistry()
	if _, err := registry.Replace(c); err != nil {
		panic(err)
	}
}

func init() {
	RegisterExtendedCodec(12, 90)
}
