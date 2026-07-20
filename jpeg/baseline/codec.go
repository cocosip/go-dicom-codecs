package baseline

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

var _ codec.Codec = (*Codec)(nil)

// Codec implements the external codec.Codec interface for JPEG Baseline
type Codec struct {
	transferSyntax *transfer.Syntax
	quality        int // Default quality (1-100)
}

// NewBaselineCodec creates a new JPEG Baseline codec
// quality: 1-100, where 100 is best quality (default: 90)
func NewBaselineCodec(quality int) *Codec {
	if quality < 1 || quality > 100 {
		quality = 90 // default
	}
	return &Codec{
		transferSyntax: transfer.JPEGBaseline8Bit,
		quality:        quality,
	}
}

// Name returns the codec name
func (c *Codec) Name() string {
	return fmt.Sprintf("JPEG Baseline (Quality %d)", c.quality)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// GetDefaultParameters returns the default codec parameters
func (c *Codec) GetDefaultParameters() codec.Parameters {
	params := NewBaselineParameters()
	params.Quality = c.quality
	return params
}

// Encode encodes pixel data to JPEG Baseline format
func (c *Codec) Encode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return fmt.Errorf("failed to get frame info from source pixel data")
	}

	// JPEG Baseline only supports 8-bit data
	if frameInfo.BitsStored > 8 {
		return fmt.Errorf("JPEG Baseline only supports 8-bit data, got %d bits", frameInfo.BitsStored)
	}

	// Get encoding parameters
	var baselineParams *JPEGBaselineParameters
	if parameters != nil {
		// Try to use typed parameters if provided
		if jp, ok := parameters.(*JPEGBaselineParameters); ok {
			baselineParams = jp
		} else {
			// Fallback: create from generic parameters
			baselineParams = NewBaselineParameters()
			if q := parameters.GetParameter("quality"); q != nil {
				if qInt, ok := q.(int); ok && qInt >= 1 && qInt <= 100 {
					baselineParams.Quality = qInt
				}
			}
		}
	} else {
		// Use codec defaults
		baselineParams = NewBaselineParameters()
		baselineParams.Quality = c.quality
	}

	// Validate parameters
	if err := baselineParams.Validate(); err != nil {
		return fmt.Errorf("invalid JPEG Baseline parameters: %w", err)
	}
	quality := baselineParams.Quality

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

		// Encode using the baseline encoder
		jpegData, err := Encode(
			frameData,
			int(frameInfo.Width),
			int(frameInfo.Height),
			int(frameInfo.SamplesPerPixel),
			quality,
		)
		if err != nil {
			return fmt.Errorf("JPEG Baseline encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(jpegData); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG Baseline data to uncompressed pixel data
func (c *Codec) Decode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, _ codec.Parameters) error {
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

		// Decode using the baseline decoder
		pixelData, width, height, _, err := Decode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG Baseline decode failed for frame %d: %w", frameIndex, err)
		}

		// Verify dimensions match if specified
		if frameInfo.Width > 0 && width != int(frameInfo.Width) {
			return fmt.Errorf("decoded width (%d) doesn't match expected (%d)", width, frameInfo.Width)
		}
		if frameInfo.Height > 0 && height != int(frameInfo.Height) {
			return fmt.Errorf("decoded height (%d) doesn't match expected (%d)", height, frameInfo.Height)
		}

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(pixelData); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterBaselineCodec registers the JPEG Baseline codec with the global registry
func RegisterBaselineCodec(quality int) {
	registry := codec.GetGlobalRegistry()
	baselineCodec := NewBaselineCodec(quality)
	registry.RegisterCodec(transfer.JPEGBaseline8Bit, baselineCodec)
}

func init() {
	RegisterBaselineCodec(90)
}
