package baseline

import (
	"bytes"
	"math/bits"

	"github.com/cocosip/go-dicom-codecs/jpeg/standard"
)

// Encoder represents a JPEG Baseline encoder
type Encoder struct {
	width      int
	height     int
	components int
	quality    int

	qtables   [2][64]int32
	qdivisors [2][64]uint32 // qtables scaled by 8, the IJG quantization step
	qmagic    [2][64]uint64 // reciprocal magic numbers for qdivisors
	dcTables  [2]*standard.HuffmanTable
	acTables  [2]*standard.HuffmanTable
	dcCodes   [2][]standard.HuffmanCode
	acCodes   [2][]standard.HuffmanCode

	// coefs caches the quantized coefficients of every block in scan order,
	// so the DCT runs once instead of once per Huffman pass.
	coefs []int32
	ycbcr *YCbCrData
	// freq accumulates the symbol frequencies while the blocks are quantized.
	freq huffmanFrequencies
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
	for t := 0; t < 2; t++ {
		enc.prepareQuantTable(t)
	}

	// The Huffman tables are optimized below, so no standard table is built.
	if err := enc.computeCoefficients(pixelData); err != nil {
		return nil, err
	}
	if err := enc.optimizeHuffmanTables(); err != nil {
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
		data[6] = 0    // Component ID used by fo-dicom Native for grayscale
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
		data[1] = 0    // Component ID used by fo-dicom Native for grayscale
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

// computeCoefficients runs the DCT and quantization once per block and keeps
// the result for the Huffman passes.
func (enc *Encoder) computeCoefficients(pixelData []byte) error {
	blocksWide := standard.DivCeil(enc.width, 8)
	blocksHigh := standard.DivCeil(enc.height, 8)
	blockCount := blocksWide * blocksHigh

	if enc.components == 1 {
		enc.coefs = make([]int32, blockCount*64)
		index := 0
		var dcPred [3]int
		for blockY := 0; blockY < blocksHigh; blockY++ {
			for blockX := 0; blockX < blocksWide; blockX++ {
				block := enc.coefs[index*64 : index*64+64]
				enc.quantizeBlock(block, pixelData, blockX, blockY, enc.width, 0)
				countCoefficients(&enc.freq, block, 0, &dcPred[0])
				index++
			}
		}
		return nil
	}

	enc.ycbcr = enc.rgbToYCbCr(pixelData)
	stride := blocksWide * 8
	planes := [3][]byte{enc.ycbcr.Y, enc.ycbcr.Cb, enc.ycbcr.Cr}
	tables := [3]int{0, 1, 1}
	enc.coefs = make([]int32, blockCount*3*64)
	index := 0
	var dcPred [3]int
	for blockY := 0; blockY < blocksHigh; blockY++ {
		for blockX := 0; blockX < blocksWide; blockX++ {
			for comp := 0; comp < 3; comp++ {
				block := enc.coefs[index*64 : index*64+64]
				enc.quantizeBlock(block, planes[comp], blockX, blockY, stride, tables[comp])
				countCoefficients(&enc.freq, block, tables[comp], &dcPred[comp])
				index++
			}
		}
	}
	return nil
}

// encodeScan encodes the scan data
func (enc *Encoder) encodeScan(writer *standard.Writer, _ []byte) error {
	var scanBuf bytes.Buffer
	huffEnc := standard.NewHuffmanEncoder(&scanBuf)

	tables := [3]int{0, 1, 1}
	var dcPred [3]int
	index := 0
	for i := 0; i < len(enc.coefs)/(64*enc.components); i++ {
		for comp := 0; comp < enc.components; comp++ {
			coef := enc.coefs[index*64 : index*64+64]
			tableIdx := 0
			if enc.components == 3 {
				tableIdx = tables[comp]
			}
			if err := enc.encodeBlock(huffEnc, coef, tableIdx, &dcPred[comp]); err != nil {
				return err
			}
			index++
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
		src := rgb[row*enc.width*3:]
		index := row * stride
		var lastY, lastCb, lastCr byte
		for col := 0; col < enc.width; col++ {
			r := int(src[col*3])
			g := int(src[col*3+1])
			b := int(src[col*3+2])

			yy := (19595*r + 38470*g + 7471*b + 32768) >> 16
			cbVal := (-11056*r - 21712*g + 32768*b + 8421376) >> 16
			crVal := (32768*r - 27440*g - 5328*b + 8421376) >> 16

			lastY, lastCb, lastCr = clampSample(yy), clampSample(cbVal), clampSample(crVal)
			y[index+col] = lastY
			cb[index+col] = lastCb
			cr[index+col] = lastCr
		}
		// Replicate the right edge so the padded block has no stripes.
		for col := enc.width; col < stride; col++ {
			y[index+col] = lastY
			cb[index+col] = lastCb
			cr[index+col] = lastCr
		}
	}

	// Replicate the bottom rows.
	if enc.height < height {
		lastRow := (enc.height - 1) * stride
		for row := enc.height; row < height; row++ {
			dst := row * stride
			copy(y[dst:dst+stride], y[lastRow:lastRow+stride])
			copy(cb[dst:dst+stride], cb[lastRow:lastRow+stride])
			copy(cr[dst:dst+stride], cr[lastRow:lastRow+stride])
		}
	}

	return &YCbCrData{Y: y, Cb: cb, Cr: cr}
}

// clampSample rounds a clamped 8-bit sample.
func clampSample(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

// encodeBlock encodes one block of quantized coefficients.
func (enc *Encoder) encodeBlock(huffEnc *standard.HuffmanEncoder, coef []int32, tableIdx int, dcPred *int) error {
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

// prepareQuantTable precomputes the quantization divisor and its reciprocal.
func (enc *Encoder) prepareQuantTable(tableIdx int) {
	for i := 0; i < 64; i++ {
		divisor := uint32(enc.qtables[tableIdx][i] * 8)
		enc.qdivisors[tableIdx][i] = divisor
		// ceil(2^32/divisor): rounding a magnitude by (2x+divisor)*magic>>33
		// equals (x + divisor/2) / divisor exactly for the DCT range.
		enc.qmagic[tableIdx][i] = (uint64(1)<<32 + uint64(divisor) - 1) / uint64(divisor)
	}
}

func (enc *Encoder) quantizeBlock(dst []int32, data []byte, blockX, blockY, stride, tableIdx int) {
	// Extract the 8x8 block, clamping only at the right and bottom edges.
	var block [64]byte
	dataHeight := len(data) / stride
	startX := blockX * 8
	startY := blockY * 8
	if startY+8 <= dataHeight && startX+8 <= stride {
		for y := 0; y < 8; y++ {
			row := data[(startY+y)*stride+startX:]
			copy(block[y*8:y*8+8], row[:8])
		}
	} else {
		for y := 0; y < 8; y++ {
			srcY := min(startY+y, dataHeight-1)
			for x := 0; x < 8; x++ {
				srcX := min(startX+x, stride-1)
				block[y*8+x] = data[srcY*stride+srcX]
			}
		}
	}

	// IJG's integer DCT retains an eightfold scale that its quantizer removes.
	var coef [64]int32
	standard.DCTISlow(block[:], 8, coef[:])

	// Quantize with a reciprocal multiply; the magic number is exact for the
	// 16-bit coefficient range the integer DCT produces.
	if enc.qmagic[tableIdx][0] == 0 {
		enc.prepareQuantTable(tableIdx)
	}
	divisors := &enc.qdivisors[tableIdx]
	magic := &enc.qmagic[tableIdx]
	for i := 0; i < 64; i++ {
		c := coef[i]
		divisor := divisors[i]
		var magnitude uint32
		if c < 0 {
			magnitude = uint32(-c)
		} else {
			magnitude = uint32(c)
		}
		scaled := 2*uint64(magnitude) + uint64(divisor)
		q := int32((scaled * magic[i]) >> 33)
		if c < 0 {
			q = -q
		}
		dst[i] = q
	}
}

type huffmanFrequencies struct {
	dc [2][256]uint64
	ac [2][256]uint64
}

func (enc *Encoder) optimizeHuffmanTables() error {
	frequencies := &enc.freq
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

func countCoefficients(frequencies *huffmanFrequencies, coef []int32, tableIdx int, dcPred *int) {
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
	return bits.Len(uint(value))
}
