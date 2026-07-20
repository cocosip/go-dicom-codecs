package baseline

import (
	"bytes"

	"github.com/cocosip/go-dicom-codec/jpeg/standard"
)

// Encoder represents a JPEG Baseline encoder
type Encoder struct {
	width      int
	height     int
	components int
	quality    int

	qtables  [2][64]int32
	dcTables [2]*standard.HuffmanTable
	acTables [2]*standard.HuffmanTable
	dcCodes  [2][]standard.HuffmanCode
	acCodes  [2][]standard.HuffmanCode
}

// Encode encodes pixel data to JPEG Baseline format
// components: 1 for grayscale, 3 for RGB
// quality: 1-100, where 100 is best quality
func Encode(pixelData []byte, width, height, components, quality int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, standard.ErrInvalidDimensions
	}

	if components != 1 && components != 3 {
		return nil, standard.ErrInvalidComponents
	}

	if quality < 1 || quality > 100 {
		return nil, standard.ErrInvalidQuality
	}

	if len(pixelData) < width*height*components {
		return nil, standard.ErrBufferTooSmall
	}

	enc := &Encoder{
		width:      width,
		height:     height,
		components: components,
		quality:    quality,
	}

	// Initialize quantization tables
	enc.qtables[0] = standard.ScaleQuantTable(standard.DefaultLuminanceQuantTable, quality)
	enc.qtables[1] = standard.ScaleQuantTable(standard.DefaultChrominanceQuantTable, quality)

	// Initialize Huffman tables
	enc.dcTables[0] = standard.BuildStandardHuffmanTable(
		standard.StandardDCLuminanceBits,
		standard.StandardDCLuminanceValues,
	)
	enc.acTables[0] = standard.BuildStandardHuffmanTable(
		standard.StandardACLuminanceBits,
		standard.StandardACLuminanceValues,
	)
	enc.dcTables[1] = standard.BuildStandardHuffmanTable(
		standard.StandardDCChrominanceBits,
		standard.StandardDCChrominanceValues,
	)
	enc.acTables[1] = standard.BuildStandardHuffmanTable(
		standard.StandardACChrominanceBits,
		standard.StandardACChrominanceValues,
	)

	// Build Huffman codes
	enc.dcCodes[0] = standard.BuildHuffmanCodes(enc.dcTables[0])
	enc.acCodes[0] = standard.BuildHuffmanCodes(enc.acTables[0])
	enc.dcCodes[1] = standard.BuildHuffmanCodes(enc.dcTables[1])
	enc.acCodes[1] = standard.BuildHuffmanCodes(enc.acTables[1])

	if err := enc.optimizeHuffmanTables(pixelData); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := standard.NewWriter(&buf)

	// Write SOI
	if err := writer.WriteMarker(standard.MarkerSOI); err != nil {
		return nil, err
	}

	// Write DQT
	if err := enc.writeDQT(writer); err != nil {
		return nil, err
	}

	// Write SOF0.
	if err := enc.writeSOF0(writer); err != nil {
		return nil, err
	}

	// Write DHT
	if err := enc.writeDHT(writer); err != nil {
		return nil, err
	}

	// Write SOS and scan data
	if err := enc.writeSOS(writer, pixelData); err != nil {
		return nil, err
	}

	// Write EOI
	if err := writer.WriteMarker(standard.MarkerEOI); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// writeDQT writes Define Quantization Table segments
func (enc *Encoder) writeDQT(writer *standard.Writer) error {
	numTables := 1
	if enc.components == 3 {
		numTables = 2
	}

	for i := 0; i < numTables; i++ {
		data := make([]byte, 1+64)
		data[0] = byte(i) // Precision=0 (8-bit), Table ID=i

		// Write in zigzag order
		for j := 0; j < 64; j++ {
			data[1+j] = byte(enc.qtables[i][standard.ZigZag[j]])
		}

		if err := writer.WriteSegment(standard.MarkerDQT, data); err != nil {
			return err
		}
	}

	return nil
}

// writeSOF0 writes Start of Frame (Baseline DCT).
func (enc *Encoder) writeSOF0(writer *standard.Writer) error {
	data := make([]byte, 6+enc.components*3)

	data[0] = 8                     // Precision: 8 bits
	data[1] = byte(enc.height >> 8) // Height high byte
	data[2] = byte(enc.height)      // Height low byte
	data[3] = byte(enc.width >> 8)  // Width high byte
	data[4] = byte(enc.width)       // Width low byte
	data[5] = byte(enc.components)  // Number of components

	if enc.components == 1 {
		// Grayscale
		data[6] = 1    // Component ID
		data[7] = 0x11 // Sampling factors: 1x1
		data[8] = 0    // Quantization table 0
	} else {
		// YCbCr 4:4:4 sampling, matching fo-dicom's default SF444.
		// Y component
		data[6] = 1    // Component ID
		data[7] = 0x11 // Sampling factors: 1x1
		data[8] = 0    // Quantization table 0

		// Cb component
		data[9] = 2     // Component ID
		data[10] = 0x11 // Sampling factors: 1x1
		data[11] = 1    // Quantization table 1

		// Cr component
		data[12] = 3    // Component ID
		data[13] = 0x11 // Sampling factors: 1x1
		data[14] = 1    // Quantization table 1
	}

	return writer.WriteSegment(standard.MarkerSOF0, data)
}

// writeDHT writes Define Huffman Table segments
func (enc *Encoder) writeDHT(writer *standard.Writer) error {
	tables := []struct {
		class byte
		id    byte
		table *standard.HuffmanTable
	}{
		{0, 0, enc.dcTables[0]}, // DC table 0 (luminance)
		{1, 0, enc.acTables[0]}, // AC table 0 (luminance)
	}

	if enc.components == 3 {
		tables = append(tables,
			struct {
				class byte
				id    byte
				table *standard.HuffmanTable
			}{0, 1, enc.dcTables[1]}, // DC table 1 (chrominance)
			struct {
				class byte
				id    byte
				table *standard.HuffmanTable
			}{1, 1, enc.acTables[1]}, // AC table 1 (chrominance)
		)
	}

	for _, t := range tables {
		totalValues := 0
		for _, count := range t.table.Bits {
			totalValues += count
		}

		data := make([]byte, 1+16+totalValues)
		data[0] = (t.class << 4) | t.id

		for i := 0; i < 16; i++ {
			data[1+i] = byte(t.table.Bits[i])
		}

		copy(data[17:], t.table.Values)

		if err := writer.WriteSegment(standard.MarkerDHT, data); err != nil {
			return err
		}
	}

	return nil
}

// writeSOS writes Start of Scan and scan data
func (enc *Encoder) writeSOS(writer *standard.Writer, pixelData []byte) error {
	// Write SOS header
	data := make([]byte, 1+enc.components*2+3)
	data[0] = byte(enc.components)

	if enc.components == 1 {
		data[1] = 1    // Component ID
		data[2] = 0x00 // DC table 0, AC table 0
	} else {
		data[1] = 1    // Y component ID
		data[2] = 0x00 // DC table 0, AC table 0
		data[3] = 2    // Cb component ID
		data[4] = 0x11 // DC table 1, AC table 1
		data[5] = 3    // Cr component ID
		data[6] = 0x11 // DC table 1, AC table 1
	}

	// Spectral selection
	data[1+enc.components*2] = 0  // Start of spectral selection
	data[2+enc.components*2] = 63 // End of spectral selection
	data[3+enc.components*2] = 0  // Successive approximation

	if err := writer.WriteSegment(standard.MarkerSOS, data); err != nil {
		return err
	}

	// Encode scan data
	return enc.encodeScan(writer, pixelData)
}

// encodeScan encodes the scan data
func (enc *Encoder) encodeScan(writer *standard.Writer, pixelData []byte) error {
	var scanBuf bytes.Buffer
	huffEnc := standard.NewHuffmanEncoder(&scanBuf)

	if enc.components == 1 {
		// Grayscale
		if err := enc.encodeGrayscale(huffEnc, pixelData); err != nil {
			return err
		}
	} else {
		// RGB to YCbCr
		if err := enc.encodeRGB(huffEnc, pixelData); err != nil {
			return err
		}
	}

	if err := huffEnc.Flush(); err != nil {
		return err
	}

	// Write scan data
	if err := writer.WriteBytes(scanBuf.Bytes()); err != nil {
		return err
	}

	return nil
}

// encodeGrayscale encodes grayscale image
func (enc *Encoder) encodeGrayscale(huffEnc *standard.HuffmanEncoder, pixelData []byte) error {
	dcPred := 0

	blocksWide := standard.DivCeil(enc.width, 8)
	blocksHigh := standard.DivCeil(enc.height, 8)

	for by := 0; by < blocksHigh; by++ {
		for bx := 0; bx < blocksWide; bx++ {
			if err := enc.encodeBlock(huffEnc, pixelData, bx, by, 0, enc.width, &dcPred, 0); err != nil {
				return err
			}
		}
	}

	return nil
}

// encodeRGB encodes RGB image with full-resolution YCbCr components.
func (enc *Encoder) encodeRGB(huffEnc *standard.HuffmanEncoder, pixelData []byte) error {
	// Convert RGB to YCbCr and encode with 4:4:4 sampling.
	ycbcr := enc.rgbToYCbCr(pixelData)

	dcPred := [3]int{0, 0, 0}

	// Process in 8x8 MCUs with one block for each component.
	blocksWide := standard.DivCeil(enc.width, 8)
	blocksHigh := standard.DivCeil(enc.height, 8)
	stride := standard.DivCeil(enc.width, 8) * 8

	for blockY := 0; blockY < blocksHigh; blockY++ {
		for blockX := 0; blockX < blocksWide; blockX++ {
			if err := enc.encodeBlock(huffEnc, ycbcr.Y, blockX, blockY, 0, stride, &dcPred[0], 0); err != nil {
				return err
			}
			if err := enc.encodeBlock(huffEnc, ycbcr.Cb, blockX, blockY, 0, stride, &dcPred[1], 1); err != nil {
				return err
			}
			if err := enc.encodeBlock(huffEnc, ycbcr.Cr, blockX, blockY, 0, stride, &dcPred[2], 1); err != nil {
				return err
			}
		}
	}

	return nil
}

// YCbCrData holds YCbCr image data
type YCbCrData struct {
	Y  []byte
	Cb []byte
	Cr []byte
}

// rgbToYCbCr converts RGB to full-resolution YCbCr for 4:4:4 sampling.
func (enc *Encoder) rgbToYCbCr(rgb []byte) *YCbCrData {
	stride := standard.DivCeil(enc.width, 8) * 8
	height := standard.DivCeil(enc.height, 8) * 8

	y := make([]byte, stride*height)
	cb := make([]byte, stride*height)
	cr := make([]byte, stride*height)

	for row := 0; row < enc.height; row++ {
		for col := 0; col < enc.width; col++ {
			offset := (row*enc.width + col) * 3
			r := int(rgb[offset+0])
			g := int(rgb[offset+1])
			b := int(rgb[offset+2])

			// RGB to YCbCr conversion
			yy := (19595*r + 38470*g + 7471*b + 32768) >> 16
			cbVal := ((-11056*r - 21712*g + 32768*b + 8421376) >> 16)
			crVal := ((32768*r - 27440*g - 5328*b + 8421376) >> 16)

			index := row*stride + col
			y[index] = byte(standard.Clamp(yy, 0, 255))
			cb[index] = byte(standard.Clamp(cbVal, 0, 255))
			cr[index] = byte(standard.Clamp(crVal, 0, 255))
		}
	}

	return &YCbCrData{Y: y, Cb: cb, Cr: cr}
}

// encodeBlock encodes a single 8x8 block
func (enc *Encoder) encodeBlock(huffEnc *standard.HuffmanEncoder, data []byte, blockX, blockY, _ int, stride int, dcPred *int, tableIdx int) error {
	coef := enc.quantizeBlock(data, blockX, blockY, stride, tableIdx)

	// Encode DC coefficient
	dcDiff := int(coef[0]) - *dcPred
	*dcPred = int(coef[0])

	cat, bits := huffEnc.EncodeCategory(dcDiff)
	dcCode := enc.dcCodes[tableIdx][cat]
	if err := huffEnc.WriteBits(uint32(dcCode.Code), dcCode.Len); err != nil {
		return err
	}
	if cat > 0 {
		if err := huffEnc.WriteBits(bits, cat); err != nil {
			return err
		}
	}

	// Encode AC coefficients
	acCode := enc.acCodes[tableIdx]
	zeroRun := 0

	for k := 1; k < 64; k++ {
		val := int(coef[standard.ZigZag[k]])

		if val == 0 {
			zeroRun++
			continue
		}

		// Emit any pending zero runs
		for zeroRun >= 16 {
			// ZRL: 16 zeros
			code := acCode[0xF0]
			if err := huffEnc.WriteBits(uint32(code.Code), code.Len); err != nil {
				return err
			}
			zeroRun -= 16
		}

		cat, bits := huffEnc.EncodeCategory(val)
		rs := byte((zeroRun << 4) | cat)
		code := acCode[rs]
		if err := huffEnc.WriteBits(uint32(code.Code), code.Len); err != nil {
			return err
		}
		if err := huffEnc.WriteBits(bits, cat); err != nil {
			return err
		}

		zeroRun = 0
	}

	// EOB if there are trailing zeros
	if zeroRun > 0 {
		code := acCode[0x00]
		if err := huffEnc.WriteBits(uint32(code.Code), code.Len); err != nil {
			return err
		}
	}

	return nil
}

func (enc *Encoder) quantizeBlock(data []byte, blockX, blockY, stride, tableIdx int) [64]int32 {
	// Extract 8x8 block
	var block [64]byte
	for y := 0; y < 8; y++ {
		srcY := blockY*8 + y
		if srcY >= len(data)/stride {
			break
		}
		for x := 0; x < 8; x++ {
			srcX := blockX*8 + x
			if srcX < stride && srcY*stride+srcX < len(data) {
				block[y*8+x] = data[srcY*stride+srcX]
			} else {
				block[y*8+x] = 0
			}
		}
	}

	// IJG's integer DCT retains an eightfold scale that its quantizer removes.
	var coef [64]int32
	standard.DCTISlow(block[:], 8, coef[:])

	// Quantize
	qtable := &enc.qtables[tableIdx]
	for i := 0; i < 64; i++ {
		divisor := qtable[i] * 8
		if coef[i] < 0 {
			coef[i] = -((-coef[i] + divisor/2) / divisor)
		} else {
			coef[i] = (coef[i] + divisor/2) / divisor
		}
	}

	return coef
}

type huffmanFrequencies struct {
	dc [2][256]uint64
	ac [2][256]uint64
}

func (enc *Encoder) optimizeHuffmanTables(pixelData []byte) error {
	frequencies := &huffmanFrequencies{}

	if enc.components == 1 {
		if err := enc.countGrayscale(frequencies, pixelData); err != nil {
			return err
		}
	} else if err := enc.countRGB(frequencies, pixelData); err != nil {
		return err
	}

	tableCount := 1
	if enc.components == 3 {
		tableCount = 2
	}
	for tableIdx := 0; tableIdx < tableCount; tableIdx++ {
		enc.dcTables[tableIdx] = standard.BuildOptimalHuffmanTable(frequencies.dc[tableIdx])
		enc.acTables[tableIdx] = standard.BuildOptimalHuffmanTable(frequencies.ac[tableIdx])
		enc.dcCodes[tableIdx] = standard.BuildHuffmanCodes(enc.dcTables[tableIdx])
		enc.acCodes[tableIdx] = standard.BuildHuffmanCodes(enc.acTables[tableIdx])
	}

	return nil
}

func (enc *Encoder) countGrayscale(frequencies *huffmanFrequencies, pixelData []byte) error {
	dcPred := 0
	for blockY := 0; blockY < standard.DivCeil(enc.height, 8); blockY++ {
		for blockX := 0; blockX < standard.DivCeil(enc.width, 8); blockX++ {
			enc.countBlock(frequencies, pixelData, blockX, blockY, enc.width, &dcPred, 0)
		}
	}
	return nil
}

func (enc *Encoder) countRGB(frequencies *huffmanFrequencies, pixelData []byte) error {
	ycbcr := enc.rgbToYCbCr(pixelData)
	dcPred := [3]int{}
	blocksWide := standard.DivCeil(enc.width, 8)
	blocksHigh := standard.DivCeil(enc.height, 8)
	stride := standard.DivCeil(enc.width, 8) * 8

	for blockY := 0; blockY < blocksHigh; blockY++ {
		for blockX := 0; blockX < blocksWide; blockX++ {
			enc.countBlock(frequencies, ycbcr.Y, blockX, blockY, stride, &dcPred[0], 0)
			enc.countBlock(frequencies, ycbcr.Cb, blockX, blockY, stride, &dcPred[1], 1)
			enc.countBlock(frequencies, ycbcr.Cr, blockX, blockY, stride, &dcPred[2], 1)
		}
	}
	return nil
}

func (enc *Encoder) countBlock(frequencies *huffmanFrequencies, data []byte, blockX, blockY, stride int, dcPred *int, tableIdx int) {
	coef := enc.quantizeBlock(data, blockX, blockY, stride, tableIdx)
	dcDiff := int(coef[0]) - *dcPred
	*dcPred = int(coef[0])
	frequencies.dc[tableIdx][huffmanCategory(dcDiff)]++

	zeroRun := 0
	for k := 1; k < 64; k++ {
		value := int(coef[standard.ZigZag[k]])
		if value == 0 {
			zeroRun++
			continue
		}

		for zeroRun >= 16 {
			frequencies.ac[tableIdx][0xF0]++
			zeroRun -= 16
		}
		frequencies.ac[tableIdx][byte((zeroRun<<4)|huffmanCategory(value))]++
		zeroRun = 0
	}
	if zeroRun > 0 {
		frequencies.ac[tableIdx][0]++
	}
}

func huffmanCategory(value int) int {
	if value < 0 {
		value = -value
	}

	category := 0
	for value > 0 {
		category++
		value >>= 1
	}
	return category
}
