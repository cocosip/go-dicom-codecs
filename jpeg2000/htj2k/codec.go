package htj2k

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

var _ codec.Codec = (*Codec)(nil)

const (
	codecNameHTJ2KLossless     = "HTJ2K Lossless"
	codecNameHTJ2KLosslessRPCL = "HTJ2K Lossless RPCL"
)

// Codec implements the HTJ2K (High-Throughput JPEG 2000) codec
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019
//
// Supported Transfer Syntaxes:
// - 1.2.840.10008.1.2.4.201: HTJ2K Lossless
// - 1.2.840.10008.1.2.4.202: HTJ2K Lossless RPCL
// - 1.2.840.10008.1.2.4.203: HTJ2K (Lossy)
type Codec struct {
	transferSyntax *transfer.Syntax
	lossless       bool
	quality        int // For lossy encoding (1-100)
}

// NewLosslessCodec creates a new HTJ2K lossless codec
func NewLosslessCodec() *Codec {
	return &Codec{
		transferSyntax: transfer.HTJ2KLossless,
		lossless:       true,
	}
}

// NewLosslessRPCLCodec creates a new HTJ2K lossless RPCL codec
func NewLosslessRPCLCodec() *Codec {
	return &Codec{
		transferSyntax: transfer.HTJ2KLosslessRPCL,
		lossless:       true,
	}
}

// NewCodec creates a new HTJ2K lossy codec with specified quality
func NewCodec(quality int) *Codec {
	if quality < 1 || quality > 100 {
		quality = 80 // default
	}
	return &Codec{
		transferSyntax: transfer.HTJ2K,
		lossless:       false,
		quality:        quality,
	}
}

// Name returns the codec name
func (c *Codec) Name() string {
	if c.lossless {
		if c.transferSyntax == transfer.HTJ2KLosslessRPCL {
			return codecNameHTJ2KLosslessRPCL
		}
		return codecNameHTJ2KLossless
	}
	return fmt.Sprintf("HTJ2K (Quality %d)", c.quality)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// GetDefaultParameters returns the default codec parameters
func (c *Codec) GetDefaultParameters() codec.Parameters {
	if c.lossless {
		return NewHTJ2KLosslessParameters()
	}
	params := NewHTJ2KParameters()
	params.Quality = c.quality
	return params
}

// Encode encodes pixel data to HTJ2K format
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
	var htj2kParams *Parameters
	if parameters != nil {
		// Try to use typed parameters if provided
		if hp, ok := parameters.(*Parameters); ok {
			htj2kParams = hp
		} else {
			// Fallback: create from generic parameters
			htj2kParams = NewHTJ2KParameters()
			if q := parameters.GetParameter(paramQuality); q != nil {
				if qInt, ok := q.(int); ok {
					htj2kParams.Quality = qInt
				}
			}
			if bw := parameters.GetParameter(paramBlockWidth); bw != nil {
				if bwInt, ok := bw.(int); ok {
					htj2kParams.BlockWidth = bwInt
				}
			}
			if bh := parameters.GetParameter(paramBlockHeight); bh != nil {
				if bhInt, ok := bh.(int); ok {
					htj2kParams.BlockHeight = bhInt
				}
			}
			if nl := parameters.GetParameter(paramNumLevels); nl != nil {
				if nlInt, ok := nl.(int); ok {
					htj2kParams.NumLevels = nlInt
				}
			}
		}
	} else {
		// Use defaults
		if c.lossless {
			htj2kParams = NewHTJ2KLosslessParameters()
		} else {
			htj2kParams = NewHTJ2KParameters()
			htj2kParams.Quality = c.quality
		}
	}

	// Validate parameters
	if err := htj2kParams.Validate(); err != nil {
		return fmt.Errorf("invalid HTJ2K parameters: %w", err)
	}

	// Create JPEG 2000 encoding parameters with HTJ2K enabled
	encParams := jpeg2000.DefaultEncodeParams(
		int(frameInfo.Width),
		int(frameInfo.Height),
		int(frameInfo.SamplesPerPixel),
		int(frameInfo.BitsStored),
		frameInfo.PixelRepresentation != 0,
	)

	// Configure HTJ2K-specific settings
	// Adjust NumLevels based on image size to ensure minimum subband size >= 1
	maxLevels := calculateMaxLevels(int(frameInfo.Width), int(frameInfo.Height))
	if htj2kParams.NumLevels > maxLevels {
		encParams.NumLevels = maxLevels
	} else {
		encParams.NumLevels = htj2kParams.NumLevels
	}
	encParams.CodeBlockWidth = htj2kParams.BlockWidth
	encParams.CodeBlockHeight = htj2kParams.BlockHeight
	encParams.ProgressionOrder = 2 // OpenJPH default is RPCL.
	encParams.HTJ2KMode = true

	// Set HTJ2K block encoder factory
	encParams.BlockEncoderFactory = func(width, height int) jpeg2000.BlockEncoder {
		return NewHTEncoder(width, height)
	}

	// Configure lossless vs lossy mode
	if c.lossless {
		encParams.Lossless = true
	} else {
		encParams.Lossless = false
		encParams.Quality = htj2kParams.Quality
	}

	// Create encoder with HTJ2K enabled
	encoder := jpeg2000.NewEncoder(encParams)

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

		// Encode using full JPEG 2000 pipeline (DWT + HTJ2K block coding + T2)
		encoded, err := encoder.Encode(frameData)
		if err != nil {
			return fmt.Errorf("HTJ2K encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(encoded); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes HTJ2K data to uncompressed pixel data
func (c *Codec) Decode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get decoding parameters
	var htj2kParams *Parameters
	if parameters != nil {
		// Try to use typed parameters if provided
		if hp, ok := parameters.(*Parameters); ok {
			htj2kParams = hp
		} else {
			// Fallback: create from generic parameters
			htj2kParams = NewHTJ2KParameters()
			if bw := parameters.GetParameter(paramBlockWidth); bw != nil {
				if bwInt, ok := bw.(int); ok {
					htj2kParams.BlockWidth = bwInt
				}
			}
			if bh := parameters.GetParameter(paramBlockHeight); bh != nil {
				if bhInt, ok := bh.(int); ok {
					htj2kParams.BlockHeight = bhInt
				}
			}
		}
	} else {
		// Use defaults
		htj2kParams = NewHTJ2KParameters()
	}

	// Validate parameters
	if err := htj2kParams.Validate(); err != nil {
		return fmt.Errorf("invalid HTJ2K parameters: %w", err)
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

		// Create JPEG 2000 decoder
		decoder := jpeg2000.NewDecoder()

		// Set HTJ2K block decoder factory
		// The decoder will use this factory to create HTJ2K block decoders instead of EBCOT T1 decoders
		decoder.SetBlockDecoderFactory(func(width, height int, _ int) t2.BlockDecoder {
			return NewHTDecoder(width, height)
		})

		// Decode using full JPEG 2000 pipeline (T2 + HTJ2K block decoding + Inverse DWT)
		if err := decoder.Decode(frameData); err != nil {
			return fmt.Errorf("HTJ2K decode failed for frame %d: %w", frameIndex, err)
		}

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(decoder.GetPixelData()); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterHTJ2KCodecs registers all HTJ2K codecs with the global registry
func RegisterHTJ2KCodecs() {
	registry := codec.GetGlobalRegistry()

	// Register HTJ2K Lossless
	losslessCodec := NewLosslessCodec()
	registry.RegisterCodec(transfer.HTJ2KLossless, losslessCodec)

	// Register HTJ2K Lossless RPCL
	losslessRPCLCodec := NewLosslessRPCLCodec()
	registry.RegisterCodec(transfer.HTJ2KLosslessRPCL, losslessRPCLCodec)

	// Register HTJ2K Lossy
	lossyCodec := NewCodec(80) // Default quality: 80
	registry.RegisterCodec(transfer.HTJ2K, lossyCodec)
}

func init() {
	RegisterHTJ2KCodecs()
}

// calculateMaxLevels calculates the maximum number of wavelet decomposition levels
// that can be applied to an image of given dimensions.
// Each level divides dimensions by 2, so max levels = floor(log2(min(width, height)))
func calculateMaxLevels(width, height int) int {
	minDim := width
	if height < minDim {
		minDim = height
	}

	if minDim <= 0 {
		return 0
	}

	// Calculate floor(log2(minDim))
	maxLevels := 0
	for (1 << maxLevels) < minDim {
		maxLevels++
	}

	// Cap at 6 levels (JPEG2000 standard limit)
	if maxLevels > 6 {
		maxLevels = 6
	}

	return maxLevels
}
