// Package extended provides JPEG Extended codec (8/12-bit) implementations.
package extended

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
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
	return transfer.JPEGProcess2_4
}

// GetDefaultParameters returns the default codec parameters
func (c *Codec) GetDefaultParameters() codec.Parameters {
	params := NewExtendedParameters()
	params.Quality = c.quality
	params.BitDepth = c.bitDepth
	return params
}

// Encode encodes pixel data using JPEG Extended
func (c *Codec) Encode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return fmt.Errorf("failed to get frame info from source pixel data")
	}
	if frameInfo.BitsStored > 12 {
		return fmt.Errorf("JPEG Extended only supports up to 12-bit data, got %d bits", frameInfo.BitsStored)
	}

	// Extract parameters
	width := int(frameInfo.Width)
	height := int(frameInfo.Height)
	components := int(frameInfo.SamplesPerPixel)

	// Get encoding parameters
	var extendedParams *JPEGExtendedParameters
	if parameters != nil {
		// Try to use typed parameters if provided
		if jp, ok := parameters.(*JPEGExtendedParameters); ok {
			extendedParams = jp
		} else {
			// Fallback: create from generic parameters
			extendedParams = NewExtendedParameters()
			if q := parameters.GetParameter("quality"); q != nil {
				if qInt, ok := q.(int); ok && qInt >= 1 && qInt <= 100 {
					extendedParams.Quality = qInt
				}
			}
			if bd := parameters.GetParameter("bitDepth"); bd != nil {
				if bdInt, ok := bd.(int); ok && (bdInt == 8 || bdInt == 12) {
					extendedParams.BitDepth = bdInt
				}
			}
		}
	} else {
		// Use codec defaults
		extendedParams = NewExtendedParameters()
		extendedParams.Quality = c.quality
		extendedParams.BitDepth = c.bitDepth
	}

	// Validate parameters
	if err := extendedParams.Validate(); err != nil {
		return fmt.Errorf("invalid JPEG Extended parameters: %w", err)
	}

	// Determine bit depth from source if not explicitly set
	bitDepth := extendedParams.BitDepth
	if frameInfo.BitsStored > 0 && frameInfo.BitsStored <= 8 {
		bitDepth = 8
	} else if frameInfo.BitsStored > 8 && frameInfo.BitsStored <= 12 {
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
		frameData, err := oldPixelData.GetFrame(frameIndex)
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
		if err := newPixelData.AddFrame(encoded); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG Extended data
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

		// Decode
		decoded, _, _, _, _, err := Decode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG Extended decode failed for frame %d: %w", frameIndex, err)
		}

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(decoded); err != nil {
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
	registry := codec.GetGlobalRegistry()
	registry.RegisterCodec(transfer.JPEGProcess2_4, c)
}

func init() {
	RegisterExtendedCodec(12, 90)
}
