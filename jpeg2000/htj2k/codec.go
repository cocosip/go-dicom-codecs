// Package htj2k implements High-Throughput JPEG 2000 codecs.
package htj2k

import (
	"fmt"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k/openjph"
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
			if progression := parameters.GetParameter(paramProgressionOrder); progression != nil {
				htj2kParams.SetParameter(paramProgressionOrder, progression)
			}
		}
	}
	if htj2kParams == nil {
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

	encParams := openJPHEncodeParams(
		frameInfo,
		htj2kParams,
		c.lossless,
		c.transferSyntax == transfer.HTJ2KLosslessRPCL,
	)

	// Create encoder with HTJ2K enabled
	encoder := openjph.NewEncoder(encParams)

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
		frameData = prepareFrameForEncode(frameData, frameInfo)
		// Encode using full JPEG 2000 pipeline (DWT + HTJ2K block coding + T2)
		encoded, err := encoder.Encode(frameData)
		if err != nil {
			return fmt.Errorf("HTJ2K encode failed for frame %d: %w", frameIndex, err)
		}
		if len(encoded) >= len(frameData) {
			return fmt.Errorf(
				"HTJ2K encode failed for frame %d: output size %d is not smaller than source size %d",
				frameIndex,
				len(encoded),
				len(frameData),
			)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(encoded); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

func openJPHEncodeParams(frameInfo *imagetypes.FrameInfo, params *Parameters, lossless, explicitProgression bool) *openjph.EncodeParams {
	encParams := openjph.DefaultEncodeParams(
		int(frameInfo.Width),
		int(frameInfo.Height),
		int(frameInfo.SamplesPerPixel),
		int(frameInfo.BitsAllocated),
		frameInfo.PixelRepresentation != 0,
	)
	encParams.EnableMCT = frameInfo.SamplesPerPixel > 1
	maxLevels := calculateMaxLevels(int(frameInfo.Width), int(frameInfo.Height))
	if params.NumLevels > maxLevels {
		encParams.NumLevels = maxLevels
	} else {
		encParams.NumLevels = params.NumLevels
	}
	encParams.CodeBlockWidth = params.BlockWidth
	encParams.CodeBlockHeight = params.BlockHeight
	encParams.ProgressionOrder = uint8(ProgressionRPCL)
	if explicitProgression {
		encParams.ProgressionOrder = uint8(params.ProgressionOrder)
	}
	encParams.Lossless = lossless
	if !lossless {
		encParams.Quality = params.Quality
	}
	return encParams
}

func prepareFrameForEncode(frameData []byte, frameInfo *imagetypes.FrameInfo) []byte {
	var (
		converted []byte
		err       error
	)
	switch frameInfo.PhotometricInterpretation {
	case "YBR_FULL":
		converted, err = convertFoDicomYBRFullToRGB(frameData)
	case "YBR_FULL_422":
		converted, err = convertFoDicomYBRFull422ToRGB(frameData, int(frameInfo.Width))
	default:
		return frameData
	}
	if err != nil {
		return frameData
	}
	return converted
}

func convertFoDicomYBRFullToRGB(data []byte) ([]byte, error) {
	if len(data)%3 != 0 {
		return nil, fmt.Errorf("invalid YBR_FULL length %d", len(data))
	}
	converted := make([]byte, len(data))
	for offset := 0; offset < len(data); offset += 3 {
		y := float64(data[offset])
		cb := float64(data[offset+1])
		cr := float64(data[offset+2])
		converted[offset] = foDicomYBRByte(y + 1.4020*(cr-128) + 0.5)
		converted[offset+1] = foDicomYBRByte(y - 0.3441*(cb-128) - 0.7141*(cr-128) + 0.5)
		converted[offset+2] = foDicomYBRByte(y + 1.7720*(cb-128) + 0.5)
	}
	return converted, nil
}

func convertFoDicomYBRFull422ToRGB(data []byte, width int) ([]byte, error) {
	if width <= 0 {
		return nil, fmt.Errorf("width must be positive")
	}
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("invalid YBR_FULL_422 length %d", len(data))
	}
	converted := make([]byte, len(data)/4*6)
	for input, output, column := 0, 0, 0; input < len(data); {
		y1 := float64(data[input])
		y2 := float64(data[input+1])
		cb := float64(data[input+2])
		cr := float64(data[input+3])
		input += 4

		converted[output] = foDicomYBRByte(y1 + 1.4020*(cr-128) + 0.5)
		converted[output+1] = foDicomYBRByte(y1 - 0.3441*(cb-128) - 0.7141*(cr-128) + 0.5)
		converted[output+2] = foDicomYBRByte(y1 + 1.7720*(cb-128) + 0.5)
		output += 3
		column++
		if column == width {
			column = 0
			continue
		}

		converted[output] = foDicomYBRByte(y2 + 1.4020*(cr-128) + 0.5)
		converted[output+1] = foDicomYBRByte(y2 - 0.3441*(cb-128) - 0.7141*(cr-128) + 0.5)
		converted[output+2] = foDicomYBRByte(y2 + 1.7720*(cb-128) + 0.5)
		output += 3
		column++
		if column == width {
			column = 0
		}
	}
	return converted, nil
}

func foDicomYBRByte(value float64) byte {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return byte(value)
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
	}
	if htj2kParams == nil {
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
		decoder := openjph.NewDecoder()

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
