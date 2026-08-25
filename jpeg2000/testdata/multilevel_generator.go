package testdata

import (
	"bytes"
	"encoding/binary"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/openjpeg/wavelet"
)

// GenerateMultilevelJ2K generates a JPEG 2000 codestream with wavelet decomposition
// This creates a more complete JPEG 2000 with:
// - Single tile
// - Grayscale (1 component)
// - Configurable wavelet decomposition levels (1-6)
// - Simple test pattern data
//
// Parameters:
//   - width, height: Image dimensions (should be power of 2 for clean wavelet transform)
//   - bitDepth: Bits per pixel (8, 12, or 16)
//   - numLevels: Number of wavelet decomposition levels (0-6)
func GenerateMultilevelJ2K(width, height, bitDepth, numLevels int) []byte {
	buf := &bytes.Buffer{}

	// SOC - Start of Codestream
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF4F))

	// SIZ - Image and Tile Size
	writeSIZMultilevel(buf, width, height, bitDepth)

	// COD - Coding Style Default (with specified decomposition levels)
	writeCODMultilevel(buf, numLevels)

	// QCD - Quantization Default
	writeQCDMultilevel(buf, bitDepth, numLevels)

	// SOT - Start of Tile
	writeSOTMultilevel(buf, 0, calculateTileLengthMultilevel(width, height, numLevels))

	// SOD - Start of Data
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF93))

	// Tile data - generate simple test pattern and apply forward wavelet
	writeTileDataMultilevel(buf, width, height, bitDepth, numLevels)

	// EOC - End of Codestream
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFFD9))

	return buf.Bytes()
}

func writeSIZMultilevel(buf *bytes.Buffer, width, height, bitDepth int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF51)) // SIZ marker

	sizData := bytes.Buffer{}

	_ = binary.Write(&sizData, binary.BigEndian, uint16(0))       // Rsiz
	_ = binary.Write(&sizData, binary.BigEndian, uint32(width))   // Xsiz
	_ = binary.Write(&sizData, binary.BigEndian, uint32(height))  // Ysiz
	_ = binary.Write(&sizData, binary.BigEndian, uint32(0))       // XOsiz
	_ = binary.Write(&sizData, binary.BigEndian, uint32(0))       // YOsiz
	_ = binary.Write(&sizData, binary.BigEndian, uint32(width))   // XTsiz (tile width)
	_ = binary.Write(&sizData, binary.BigEndian, uint32(height))  // YTsiz (tile height)
	_ = binary.Write(&sizData, binary.BigEndian, uint32(0))       // XTOsiz
	_ = binary.Write(&sizData, binary.BigEndian, uint32(0))       // YTOsiz
	_ = binary.Write(&sizData, binary.BigEndian, uint16(1))       // Csiz (1 component)

	// Component 0
	ssiz := uint8(bitDepth - 1)
	_ = binary.Write(&sizData, binary.BigEndian, ssiz)
	_ = binary.Write(&sizData, binary.BigEndian, uint8(1)) // XRsiz
	_ = binary.Write(&sizData, binary.BigEndian, uint8(1)) // YRsiz

	length := uint16(sizData.Len() + 2)
	_ = binary.Write(buf, binary.BigEndian, length)
	buf.Write(sizData.Bytes())
}

func writeCODMultilevel(buf *bytes.Buffer, numLevels int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF52)) // COD marker

	codData := bytes.Buffer{}

	// Scod - Coding style parameters
	_ = binary.Write(&codData, binary.BigEndian, uint8(0))

	// SGcod - Progression order, number of layers, MCT
	_ = binary.Write(&codData, binary.BigEndian, uint8(0))    // LRCP progression order
	_ = binary.Write(&codData, binary.BigEndian, uint16(1))   // 1 quality layer
	_ = binary.Write(&codData, binary.BigEndian, uint8(0))    // No MCT

	// SPcod - Decomposition levels and code-block size
	_ = binary.Write(&codData, binary.BigEndian, uint8(numLevels))  // Number of decomposition levels
	_ = binary.Write(&codData, binary.BigEndian, uint8(4))          // Code-block width exponent (2^4 = 16)
	_ = binary.Write(&codData, binary.BigEndian, uint8(4))          // Code-block height exponent (2^4 = 16)
	_ = binary.Write(&codData, binary.BigEndian, uint8(0))          // Code-block style
	_ = binary.Write(&codData, binary.BigEndian, uint8(1))          // Transformation: 5-3 reversible

	// Precinct size (optional, not included for simplicity)

	length := uint16(codData.Len() + 2)
	_ = binary.Write(buf, binary.BigEndian, length)
	buf.Write(codData.Bytes())
}

func writeQCDMultilevel(buf *bytes.Buffer, bitDepth, numLevels int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF5C)) // QCD marker

	qcdData := bytes.Buffer{}

	// Sqcd - Quantization style (no quantization for lossless)
	_ = binary.Write(&qcdData, binary.BigEndian, uint8(0))

	// SPqcd - Quantization step size for each subband
	// For reversible (5-3) transform with no quantization
	// We need (3 * numLevels + 1) step values
	// LL band + 3 bands (HL, LH, HH) per level

	numSubbands := 3*numLevels + 1
	for i := 0; i < numSubbands; i++ {
		// Reversible quantization: exponent only (8 bits)
		// exp = bitDepth + gain[band]
		// For simplicity, use bitDepth for all bands
		_ = binary.Write(&qcdData, binary.BigEndian, uint8(bitDepth<<3))
	}

	length := uint16(qcdData.Len() + 2)
	_ = binary.Write(buf, binary.BigEndian, length)
	buf.Write(qcdData.Bytes())
}

func writeSOTMultilevel(buf *bytes.Buffer, tileIdx int, tileLength int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF90)) // SOT marker
	_ = binary.Write(buf, binary.BigEndian, uint16(10))     // Lsot (length)

	_ = binary.Write(buf, binary.BigEndian, uint16(tileIdx))    // Isot
	_ = binary.Write(buf, binary.BigEndian, uint32(tileLength)) // Psot
	_ = binary.Write(buf, binary.BigEndian, uint8(0))           // TPsot
	_ = binary.Write(buf, binary.BigEndian, uint8(1))           // TNsot (1 tile)
}

func calculateTileLengthMultilevel(width, height, numLevels int) int {
	// Approximate tile length
	// In real implementation, this would be calculated precisely
	baseSize := (width * height * 2) / 8 // Rough estimate
	return baseSize + 100                 // Add margin for headers
}

func writeTileDataMultilevel(buf *bytes.Buffer, width, height, bitDepth, numLevels int) {
	// Generate a simple test pattern
	// We'll create a gradient pattern for testing
	imageData := make([]int32, width*height)

	// Create gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Simple diagonal gradient
			value := (x + y) % (1 << bitDepth)
			imageData[y*width+x] = int32(value)
		}
	}

	// Apply forward wavelet transform if numLevels > 0
	if numLevels > 0 {
		// Make a copy for the transform
		coeffs := make([]int32, len(imageData))
		copy(coeffs, imageData)

		// Apply forward multilevel wavelet transform
		wavelet.ForwardMultilevel(coeffs, width, height, numLevels)

		// For MVP, we'll write a minimal placeholder
		// In a complete implementation, we would:
		// 1. Organize coefficients into subbands
		// 2. Split into code-blocks
		// 3. Apply EBCOT Tier-1 encoding
		// 4. Create packet headers (Tier-2)
		// 5. Write compressed bitstream
	}

	// Write minimal placeholder data (empty packets)
	// Real implementation would write encoded packets here
	placeholderData := make([]byte, 100)
	buf.Write(placeholderData)
}
