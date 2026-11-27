package extended

import (
	"bytes"
	"fmt"
	"io"
	"math/bits"

	"github.com/cocosip/go-dicom-codecs/jpeg/standard"
)

const sequential12Precision = 12

func encodeSequential12(pixelData []byte, width, height, components, quality int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, standard.ErrInvalidDimensions
	}
	if components != 1 {
		return nil, fmt.Errorf("12-bit JPEG Extended supports only one monochrome component")
	}
	if quality < 1 || quality > 100 {
		return nil, standard.ErrInvalidQuality
	}
	if len(pixelData) < width*height*2 {
		return nil, standard.ErrBufferTooSmall
	}

	encoder := sequential12Encoder{
		width:  width,
		height: height,
		pixels: pixelData,
		qtable: standard.ScaleQuantTable(standard.DefaultLuminanceQuantTable, quality),
	}
	encoder.prepareQuantTable()
	encoder.computeCoefficients()
	if err := encoder.buildHuffmanTables(); err != nil {
		return nil, err
	}

	var output bytes.Buffer
	writer := standard.NewWriter(&output)
	if err := writer.WriteMarker(standard.MarkerSOI); err != nil {
		return nil, err
	}
	if err := writer.WriteJFIFAPP0(); err != nil {
		return nil, err
	}
	if err := encoder.writeDQT(writer); err != nil {
		return nil, err
	}
	if err := encoder.writeSOF1(writer); err != nil {
		return nil, err
	}
	if err := standard.WriteHuffmanTable(writer, 0, 0, encoder.dcTable); err != nil {
		return nil, err
	}
	if err := standard.WriteHuffmanTable(writer, 1, 0, encoder.acTable); err != nil {
		return nil, err
	}
	if err := encoder.writeSOS(writer); err != nil {
		return nil, err
	}
	if err := writer.WriteMarker(standard.MarkerEOI); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

type sequential12Encoder struct {
	width, height int
	pixels        []byte
	qtable        [64]int32
	dcTable       *standard.HuffmanTable
	acTable       *standard.HuffmanTable
	dcCodes       []standard.HuffmanCode
	acCodes       []standard.HuffmanCode
	// coefs caches the quantized coefficients so the DCT runs once.
	coefs []int32
	// qdiv and qmagic turn the quantization step into a reciprocal multiply.
	qdiv   [64]uint32
	qmagic [64]uint64
}

// prepareQuantTable precomputes the quantization divisors and reciprocals.
func (e *sequential12Encoder) prepareQuantTable() {
	for i := 0; i < 64; i++ {
		divisor := uint32(e.qtable[i] << 3)
		e.qdiv[i] = divisor
		e.qmagic[i] = (uint64(1)<<32 + uint64(divisor) - 1) / uint64(divisor)
	}
}

func (e *sequential12Encoder) writeDQT(writer *standard.Writer) error {
	data := make([]byte, 65)
	for i := 0; i < 64; i++ {
		data[i+1] = byte(e.qtable[standard.ZigZag[i]])
	}
	return writer.WriteSegment(standard.MarkerDQT, data)
}

func (e *sequential12Encoder) writeSOF1(writer *standard.Writer) error {
	data := []byte{
		sequential12Precision,
		byte(e.height >> 8), byte(e.height),
		byte(e.width >> 8), byte(e.width),
		1,
		1, 0x11, 0,
	}
	return writer.WriteSegment(standard.MarkerSOF1, data)
}

func (e *sequential12Encoder) writeSOS(writer *standard.Writer) error {
	if err := writer.WriteSegment(standard.MarkerSOS, []byte{1, 1, 0, 0, 63, 0}); err != nil {
		return err
	}

	var scan bytes.Buffer
	encoder := standard.NewHuffmanEncoder(&scan)
	dcPredictor := 0
	for block := 0; block < len(e.coefs)/64; block++ {
		if err := e.encodeBlock(encoder, block, &dcPredictor); err != nil {
			return err
		}
	}
	if err := encoder.Flush(); err != nil {
		return err
	}
	return writer.WriteBytes(scan.Bytes())
}

func (e *sequential12Encoder) buildHuffmanTables() error {
	var dcFrequency, acFrequency [256]uint64
	dcPredictor := 0
	for block := 0; block < len(e.coefs)/64; block++ {
		{
			coefficients := e.coefs[block*64 : block*64+64]
			difference := int(coefficients[0]) - dcPredictor
			dcPredictor = int(coefficients[0])
			dcFrequency[sequential12Category(difference)]++

			zeroRun := 0
			for k := 1; k < 64; k++ {
				value := int(coefficients[standard.ZigZag[k]])
				if value == 0 {
					zeroRun++
					continue
				}
				for zeroRun >= 16 {
					acFrequency[0xf0]++
					zeroRun -= 16
				}
				category := sequential12Category(value)
				if category > 15 {
					return fmt.Errorf("12-bit JPEG Extended coefficient category %d is unsupported", category)
				}
				acFrequency[(zeroRun<<4)|category]++
				zeroRun = 0
			}
			if zeroRun > 0 {
				acFrequency[0]++
			}
		}
	}

	e.dcTable = standard.BuildOptimalHuffmanTable(dcFrequency)
	e.acTable = standard.BuildOptimalHuffmanTable(acFrequency)
	e.dcCodes = standard.BuildHuffmanCodes(e.dcTable)
	e.acCodes = standard.BuildHuffmanCodes(e.acTable)
	return nil
}

func (e *sequential12Encoder) encodeBlock(encoder *standard.HuffmanEncoder, block int, dcPredictor *int) error {
	coefficients := e.coefs[block*64 : block*64+64]
	difference := int(coefficients[0]) - *dcPredictor
	*dcPredictor = int(coefficients[0])
	if err := sequential12WriteValue(encoder, e.dcCodes, difference); err != nil {
		return err
	}

	zeroRun := 0
	for k := 1; k < 64; k++ {
		value := int(coefficients[standard.ZigZag[k]])
		if value == 0 {
			zeroRun++
			continue
		}
		for zeroRun >= 16 {
			code := e.acCodes[0xf0]
			if err := encoder.WriteBits(uint32(code.Code), code.Len); err != nil {
				return err
			}
			zeroRun -= 16
		}
		category := sequential12Category(value)
		if category > 15 {
			return fmt.Errorf("12-bit JPEG Extended coefficient category %d is unsupported", category)
		}
		code := e.acCodes[(zeroRun<<4)|category]
		if err := encoder.WriteBits(uint32(code.Code), code.Len); err != nil {
			return err
		}
		if err := sequential12WriteMagnitude(encoder, value, category); err != nil {
			return err
		}
		zeroRun = 0
	}
	if zeroRun > 0 {
		code := e.acCodes[0]
		return encoder.WriteBits(uint32(code.Code), code.Len)
	}
	return nil
}

// computeCoefficients runs the DCT and quantization once per block.
func (e *sequential12Encoder) computeCoefficients() {
	blocksWide := standard.DivCeil(e.width, 8)
	blocksHigh := standard.DivCeil(e.height, 8)
	e.coefs = make([]int32, blocksWide*blocksHigh*64)
	index := 0
	for blockY := 0; blockY < blocksHigh; blockY++ {
		for blockX := 0; blockX < blocksWide; blockX++ {
			e.quantizeBlock(e.coefs[index*64:index*64+64], blockX, blockY)
			index++
		}
	}
}

func (e *sequential12Encoder) quantizeBlock(dst []int32, blockX, blockY int) {
	// Extract the 8x8 block, clamping only at the right and bottom edges.
	var transformed [64]int32
	startX := blockX * 8
	startY := blockY * 8
	if startX+8 <= e.width && startY+8 <= e.height {
		for y := 0; y < 8; y++ {
			source := e.pixels[(startY+y)*e.width*2+startX*2:]
			for x := 0; x < 8; x++ {
				transformed[y*8+x] = int32(int(source[x*2])|int(source[x*2+1])<<8) - 2048
			}
		}
	} else {
		for y := 0; y < 8; y++ {
			sourceY := min(startY+y, e.height-1)
			for x := 0; x < 8; x++ {
				sourceX := min(startX+x, e.width-1)
				offset := (sourceY*e.width + sourceX) * 2
				value := int(e.pixels[offset]) | int(e.pixels[offset+1])<<8
				transformed[y*8+x] = int32(value - 2048)
			}
		}
	}
	sequential12DCTISlow(&transformed)

	for i, coefficient := range transformed {
		divisor := e.qdiv[i]
		var magnitude uint32
		if coefficient < 0 {
			magnitude = uint32(-coefficient)
		} else {
			magnitude = uint32(coefficient)
		}
		q := int32(((2*uint64(magnitude) + uint64(divisor)) * e.qmagic[i]) >> 33)
		if coefficient < 0 {
			q = -q
		}
		dst[i] = q
	}
}

// sequential12DCTISlow is a direct port of libjpeg-turbo's 12-bit
// _jpeg_fdct_islow path. Its output retains libjpeg's factor-of-eight scale.
func sequential12DCTISlow(data *[64]int32) {
	const (
		constBits = 13
		pass1Bits = 1

		fix0298631336 = 2446
		fix0390180644 = 3196
		fix0541196100 = 4433
		fix0765366865 = 6270
		fix0899976223 = 7373
		fix1175875602 = 9633
		fix1501321110 = 12299
		fix1847759065 = 15137
		fix1961570560 = 16069
		fix2053119869 = 16819
		fix2562915447 = 20995
		fix3072711026 = 25172
	)

	for y := 0; y < 8; y++ {
		row := y * 8
		tmp0 := data[row] + data[row+7]
		tmp7 := data[row] - data[row+7]
		tmp1 := data[row+1] + data[row+6]
		tmp6 := data[row+1] - data[row+6]
		tmp2 := data[row+2] + data[row+5]
		tmp5 := data[row+2] - data[row+5]
		tmp3 := data[row+3] + data[row+4]
		tmp4 := data[row+3] - data[row+4]

		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2

		data[row] = (tmp10 + tmp11) << pass1Bits
		data[row+4] = (tmp10 - tmp11) << pass1Bits

		z1 := (tmp12 + tmp13) * fix0541196100
		data[row+2] = sequential12Descale(z1+tmp13*fix0765366865, constBits-pass1Bits)
		data[row+6] = sequential12Descale(z1-tmp12*fix1847759065, constBits-pass1Bits)

		z1 = tmp4 + tmp7
		z2 := tmp5 + tmp6
		z3 := tmp4 + tmp6
		z4 := tmp5 + tmp7
		z5 := (z3 + z4) * fix1175875602
		tmp4 *= fix0298631336
		tmp5 *= fix2053119869
		tmp6 *= fix3072711026
		tmp7 *= fix1501321110
		z1 *= -fix0899976223
		z2 *= -fix2562915447
		z3 *= -fix1961570560
		z4 *= -fix0390180644
		z3 += z5
		z4 += z5

		data[row+7] = sequential12Descale(tmp4+z1+z3, constBits-pass1Bits)
		data[row+5] = sequential12Descale(tmp5+z2+z4, constBits-pass1Bits)
		data[row+3] = sequential12Descale(tmp6+z2+z3, constBits-pass1Bits)
		data[row+1] = sequential12Descale(tmp7+z1+z4, constBits-pass1Bits)
	}

	for x := 0; x < 8; x++ {
		tmp0 := data[x] + data[56+x]
		tmp7 := data[x] - data[56+x]
		tmp1 := data[8+x] + data[48+x]
		tmp6 := data[8+x] - data[48+x]
		tmp2 := data[16+x] + data[40+x]
		tmp5 := data[16+x] - data[40+x]
		tmp3 := data[24+x] + data[32+x]
		tmp4 := data[24+x] - data[32+x]

		tmp10 := tmp0 + tmp3
		tmp13 := tmp0 - tmp3
		tmp11 := tmp1 + tmp2
		tmp12 := tmp1 - tmp2

		data[x] = sequential12Descale(tmp10+tmp11, pass1Bits)
		data[32+x] = sequential12Descale(tmp10-tmp11, pass1Bits)

		z1 := (tmp12 + tmp13) * fix0541196100
		data[16+x] = sequential12Descale(z1+tmp13*fix0765366865, constBits+pass1Bits)
		data[48+x] = sequential12Descale(z1-tmp12*fix1847759065, constBits+pass1Bits)

		z1 = tmp4 + tmp7
		z2 := tmp5 + tmp6
		z3 := tmp4 + tmp6
		z4 := tmp5 + tmp7
		z5 := (z3 + z4) * fix1175875602
		tmp4 *= fix0298631336
		tmp5 *= fix2053119869
		tmp6 *= fix3072711026
		tmp7 *= fix1501321110
		z1 *= -fix0899976223
		z2 *= -fix2562915447
		z3 *= -fix1961570560
		z4 *= -fix0390180644
		z3 += z5
		z4 += z5

		data[56+x] = sequential12Descale(tmp4+z1+z3, constBits+pass1Bits)
		data[40+x] = sequential12Descale(tmp5+z2+z4, constBits+pass1Bits)
		data[24+x] = sequential12Descale(tmp6+z2+z3, constBits+pass1Bits)
		data[8+x] = sequential12Descale(tmp7+z1+z4, constBits+pass1Bits)
	}
}

func sequential12Descale(value int32, shift uint) int32 {
	return (value + (1 << (shift - 1))) >> shift
}

func sequential12Category(value int) int {
	if value < 0 {
		value = -value
	}
	return bits.Len(uint(value))
}

func sequential12WriteValue(encoder *standard.HuffmanEncoder, codes []standard.HuffmanCode, value int) error {
	category := sequential12Category(value)
	if category >= len(codes) {
		return fmt.Errorf("12-bit JPEG Extended DC category %d is unsupported", category)
	}
	code := codes[category]
	if err := encoder.WriteBits(uint32(code.Code), code.Len); err != nil {
		return err
	}
	return sequential12WriteMagnitude(encoder, value, category)
}

func sequential12WriteMagnitude(encoder *standard.HuffmanEncoder, value, category int) error {
	if category == 0 {
		return nil
	}
	bits := value
	if bits < 0 {
		bits = (1 << category) - 1 + bits
	}
	return encoder.WriteBits(uint32(bits), category)
}

func decodeSequential12(jpegData []byte) ([]byte, int, int, int, int, error) {
	reader := standard.NewReader(jpegData)
	marker, err := reader.ReadMarker()
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	if marker != standard.MarkerSOI {
		return nil, 0, 0, 0, 0, standard.ErrInvalidSOI
	}

	decoder := &sequential12Decoder{}
	for {
		marker, err = reader.ReadMarker()
		if err != nil {
			return nil, 0, 0, 0, 0, err
		}
		switch marker {
		case standard.MarkerDQT:
			if err := decoder.parseDQT(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}
		case standard.MarkerSOF1:
			if err := decoder.parseSOF1(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}
		case standard.MarkerDHT:
			if err := decoder.parseDHT(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}
		case standard.MarkerSOS:
			if err := decoder.parseSOS(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}
			if err := decoder.decodeScan(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}
			return decoder.pixels, decoder.width, decoder.height, 1, sequential12Precision, nil
		case standard.MarkerEOI:
			return decoder.pixels, decoder.width, decoder.height, 1, sequential12Precision, nil
		default:
			if standard.HasLength(marker) {
				if _, err := reader.ReadSegment(); err != nil {
					return nil, 0, 0, 0, 0, err
				}
			}
		}
	}
}

type sequential12Decoder struct {
	width, height int
	qtable        [4][64]int32
	dcTable       *standard.HuffmanTable
	acTable       *standard.HuffmanTable
	pixels        []byte
	dcPredictor   int
}

func (d *sequential12Decoder) parseSOF1(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}
	if len(data) != 9 || data[0] != sequential12Precision || data[5] != 1 || (data[6] != 0 && data[6] != 1) || data[7] != 0x11 || data[8] != 0 {
		return fmt.Errorf("unsupported JPEG Extended SOF1 layout")
	}
	d.height = int(data[1])<<8 | int(data[2])
	d.width = int(data[3])<<8 | int(data[4])
	if d.width <= 0 || d.height <= 0 {
		return standard.ErrInvalidDimensions
	}
	d.pixels = make([]byte, d.width*d.height*2)
	return nil
}

func (d *sequential12Decoder) parseDQT(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}
	for offset := 0; offset < len(data); {
		if offset >= len(data) {
			return standard.ErrInvalidDQT
		}
		precisionAndID := data[offset]
		offset++
		tableID := precisionAndID & 0x0f
		if tableID > 3 {
			return standard.ErrInvalidDQT
		}
		bytesPerValue := 64
		if precisionAndID>>4 == 1 {
			bytesPerValue = 128
		} else if precisionAndID>>4 != 0 {
			return standard.ErrInvalidDQT
		}
		if offset+bytesPerValue > len(data) {
			return standard.ErrInvalidDQT
		}
		for i := 0; i < 64; i++ {
			if bytesPerValue == 64 {
				d.qtable[tableID][standard.ZigZag[i]] = int32(data[offset+i])
			} else {
				d.qtable[tableID][standard.ZigZag[i]] = int32(data[offset+i*2])<<8 | int32(data[offset+i*2+1])
			}
		}
		offset += bytesPerValue
	}
	return nil
}

func (d *sequential12Decoder) parseDHT(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}
	for offset := 0; offset < len(data); {
		if offset+17 > len(data) {
			return standard.ErrInvalidDHT
		}
		classAndID := data[offset]
		offset++
		if classAndID&0x0f != 0 || classAndID>>4 > 1 {
			return standard.ErrInvalidDHT
		}
		table := &standard.HuffmanTable{}
		count := 0
		for i := 0; i < 16; i++ {
			table.Bits[i] = int(data[offset+i])
			count += table.Bits[i]
		}
		offset += 16
		if offset+count > len(data) {
			return standard.ErrInvalidDHT
		}
		table.Values = append([]byte(nil), data[offset:offset+count]...)
		offset += count
		if err := table.Build(); err != nil {
			return err
		}
		if classAndID>>4 == 0 {
			d.dcTable = table
		} else {
			d.acTable = table
		}
	}
	return nil
}

func (d *sequential12Decoder) parseSOS(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}
	if len(data) != 6 || data[0] != 1 || (data[1] != 0 && data[1] != 1) || data[2] != 0 || data[3] != 0 || data[4] != 63 || data[5] != 0 {
		return fmt.Errorf("unsupported JPEG Extended SOS layout")
	}
	if d.dcTable == nil || d.acTable == nil {
		return standard.ErrInvalidDHT
	}
	return nil
}

func (d *sequential12Decoder) decodeScan(reader *standard.Reader) error {
	scan := make([]byte, 0, 64*1024)
	for {
		value, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if value != 0xff {
			scan = append(scan, value)
			continue
		}
		next, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if next == 0 {
			scan = append(scan, value, next)
			continue
		}
		if standard.IsRST(uint16(0xff00) | uint16(next)) {
			continue
		}
		break
	}

	decoder := standard.NewHuffmanDecoder(scan)
	for blockY := 0; blockY < standard.DivCeil(d.height, 8); blockY++ {
		for blockX := 0; blockX < standard.DivCeil(d.width, 8); blockX++ {
			if err := d.decodeBlock(decoder, blockX, blockY); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *sequential12Decoder) decodeBlock(decoder *standard.HuffmanDecoder, blockX, blockY int) error {
	var coefficients [64]int32
	dcPred, err := decoder.DecodeBlock(d.dcTable, d.acTable, &d.qtable[0], d.dcPredictor, coefficients[:])
	if err != nil {
		return err
	}
	d.dcPredictor = dcPred

	var transformed [64]int32
	idct12Block(coefficients[:], &transformed)

	maxX := d.width - blockX*8
	if maxX > 8 {
		maxX = 8
	}
	maxY := d.height - blockY*8
	if maxY > 8 {
		maxY = 8
	}
	rowOffset := (blockY*8*d.width + blockX*8) * 2
	rowStride := d.width * 2
	for y := 0; y < maxY; y++ {
		row := d.pixels[rowOffset+y*rowStride:]
		for x := 0; x < maxX; x++ {
			value := int(transformed[y*8+x])
			if value < 0 {
				value = 0
			} else if value > 4095 {
				value = 4095
			}
			row[x*2] = byte(value)
			row[x*2+1] = byte(value >> 8)
		}
	}
	return nil
}
