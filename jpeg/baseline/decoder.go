package baseline

import (
	"bytes"
	"fmt"
	"io"

	"github.com/cocosip/go-dicom-codec/jpeg/standard"
)

// Component represents a color component in the image
type Component struct {
	ID              byte   // Component identifier
	H               int    // Horizontal sampling factor
	V               int    // Vertical sampling factor
	Tq              int    // Quantization table selector
	width           int    // Component width in blocks
	height          int    // Component height in blocks
	dcTableSelector int    // DC Huffman table selector
	acTableSelector int    // AC Huffman table selector
	dcPred          int    // DC prediction value
	data            []byte // Decoded component data
}

// Decoder represents a JPEG Baseline decoder
type Decoder struct {
	width      int          // Image width
	height     int          // Image height
	components []*Component // Color components
	qtables    [4][64]int32 // Quantization tables
	dcTables   [4]*standard.HuffmanTable
	acTables   [4]*standard.HuffmanTable
	mcuWidth   int // MCU width in blocks
	mcuHeight  int // MCU height in blocks
	restartInt int // Restart interval
	precision  int // Sample precision (bits)
}

// Decode decodes JPEG Baseline data
func Decode(jpegData []byte) (pixelData []byte, width, height, components int, err error) {
	r := bytes.NewReader(jpegData)
	reader := standard.NewReader(r)

	decoder := &Decoder{}

	// Read SOI marker
	marker, err := reader.ReadMarker()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if marker != standard.MarkerSOI {
		return nil, 0, 0, 0, standard.ErrInvalidSOI
	}

	// Parse JPEG segments
	for {
		marker, err := reader.ReadMarker()
		if err != nil {
			return nil, 0, 0, 0, err
		}

		switch marker {
		case standard.MarkerSOF0:
			if err := decoder.parseSOF(reader); err != nil {
				return nil, 0, 0, 0, err
			}

		case standard.MarkerDQT:
			if err := decoder.parseDQT(reader); err != nil {
				return nil, 0, 0, 0, err
			}

		case standard.MarkerDHT:
			if err := decoder.parseDHT(reader); err != nil {
				return nil, 0, 0, 0, err
			}

		case standard.MarkerDRI:
			if err := decoder.parseDRI(reader); err != nil {
				return nil, 0, 0, 0, err
			}

		case standard.MarkerSOS:
			if err := decoder.parseSOS(reader); err != nil {
				return nil, 0, 0, 0, err
			}
			// Decode scan data
			if err := decoder.decodeScan(reader); err != nil {
				return nil, 0, 0, 0, err
			}
			// After decoding scan, we're done (baseline JPEG has only one scan)
			// Convert to output format
			pixelData = decoder.convertToPixels()
			return pixelData, decoder.width, decoder.height, len(decoder.components), nil

		case standard.MarkerEOI:
			// End of image, convert to output format
			pixelData = decoder.convertToPixels()
			return pixelData, decoder.width, decoder.height, len(decoder.components), nil

		default:
			// Skip unknown markers
			if standard.HasLength(marker) {
				_, err := reader.ReadSegment()
				if err != nil {
					return nil, 0, 0, 0, err
				}
			}
		}
	}
}

// parseSOF parses Start of Frame marker
func (d *Decoder) parseSOF(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 6 {
		return standard.ErrInvalidSOF
	}

	d.precision = int(data[0])
	if d.precision != 8 {
		return fmt.Errorf("unsupported precision: %d (only 8-bit supported for baseline)", d.precision)
	}

	d.height = int(data[1])<<8 | int(data[2])
	d.width = int(data[3])<<8 | int(data[4])
	numComponents := int(data[5])

	if d.width <= 0 || d.height <= 0 {
		return standard.ErrInvalidDimensions
	}

	if numComponents != 1 && numComponents != 3 {
		return standard.ErrInvalidComponents
	}

	if len(data) < 6+numComponents*3 {
		return standard.ErrInvalidSOF
	}

	// Parse component specifications
	maxH, maxV := 1, 1
	d.components = make([]*Component, numComponents)

	for i := 0; i < numComponents; i++ {
		offset := 6 + i*3
		comp := &Component{
			ID: data[offset],
			H:  int(data[offset+1] >> 4),
			V:  int(data[offset+1] & 0x0F),
			Tq: int(data[offset+2]),
		}

		if comp.H <= 0 || comp.H > 4 || comp.V <= 0 || comp.V > 4 {
			return standard.ErrInvalidSOF
		}

		if comp.H > maxH {
			maxH = comp.H
		}
		if comp.V > maxV {
			maxV = comp.V
		}

		d.components[i] = comp
	}

	// Calculate component dimensions and MCU size
	d.mcuWidth = maxH * 8
	d.mcuHeight = maxV * 8

	mcuCols := standard.DivCeil(d.width, d.mcuWidth)
	mcuRows := standard.DivCeil(d.height, d.mcuHeight)

	for _, comp := range d.components {
		comp.width = standard.DivCeil(d.width*comp.H, maxH*8)
		comp.height = standard.DivCeil(d.height*comp.V, maxV*8)
		comp.data = make([]byte, comp.width*comp.height*64)
	}

	_ = mcuCols
	_ = mcuRows

	return nil
}

// parseDQT parses Define Quantization Table marker
func (d *Decoder) parseDQT(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	offset := 0
	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		pqTq := data[offset]
		pq := pqTq >> 4   // Precision (0=8-bit, 1=16-bit)
		tq := pqTq & 0x0F // Table ID

		if tq > 3 {
			return standard.ErrInvalidDQT
		}

		offset++

		if pq == 0 {
			// 8-bit quantization table
			if offset+64 > len(data) {
				return standard.ErrInvalidDQT
			}
			for i := 0; i < 64; i++ {
				d.qtables[tq][standard.ZigZag[i]] = int32(data[offset+i])
			}
			offset += 64
		} else {
			// 16-bit quantization table
			if offset+128 > len(data) {
				return standard.ErrInvalidDQT
			}
			for i := 0; i < 64; i++ {
				d.qtables[tq][standard.ZigZag[i]] = int32(data[offset+i*2])<<8 | int32(data[offset+i*2+1])
			}
			offset += 128
		}
	}

	return nil
}

// parseDHT parses Define Huffman Table marker
func (d *Decoder) parseDHT(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	offset := 0
	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		tcTh := data[offset]
		tc := tcTh >> 4   // Table class (0=DC, 1=AC)
		th := tcTh & 0x0F // Table ID

		if th > 3 {
			return standard.ErrInvalidDHT
		}

		offset++

		// Read the number of codes for each length
		table := &standard.HuffmanTable{}
		totalCodes := 0
		for i := 0; i < 16; i++ {
			if offset >= len(data) {
				return standard.ErrInvalidDHT
			}
			table.Bits[i] = int(data[offset])
			totalCodes += table.Bits[i]
			offset++
		}

		// Read the symbol values
		if offset+totalCodes > len(data) {
			return standard.ErrInvalidDHT
		}
		table.Values = make([]byte, totalCodes)
		copy(table.Values, data[offset:offset+totalCodes])
		offset += totalCodes

		// Build the table
		if err := table.Build(); err != nil {
			return err
		}

		// Store the table
		if tc == 0 {
			d.dcTables[th] = table
		} else {
			d.acTables[th] = table
		}
	}

	return nil
}

// parseDRI parses Define Restart Interval marker
func (d *Decoder) parseDRI(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) != 2 {
		return standard.ErrInvalidData
	}

	d.restartInt = int(data[0])<<8 | int(data[1])
	return nil
}

// parseSOS parses Start of Scan marker
func (d *Decoder) parseSOS(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 1 {
		return standard.ErrInvalidSOS
	}

	ns := int(data[0]) // Number of components in scan
	if len(data) < 1+ns*2+3 {
		return standard.ErrInvalidSOS
	}

	// Parse component selectors
	for i := 0; i < ns; i++ {
		cs := data[1+i*2]      // Component selector
		tdTa := data[1+i*2+1]  // DC and AC table selectors
		td := int(tdTa >> 4)   // DC table
		ta := int(tdTa & 0x0F) // AC table

		// Find the component
		var comp *Component
		for _, c := range d.components {
			if c.ID == cs {
				comp = c
				break
			}
		}

		if comp == nil {
			return standard.ErrInvalidSOS
		}

		comp.dcTableSelector = td
		comp.acTableSelector = ta
	}

	// Skip spectral selection and successive approximation
	// (not used in baseline sequential)

	return nil
}

// decodeScan decodes the scan data
func (d *Decoder) decodeScan(reader *standard.Reader) error {
	// Create a Huffman decoder
	// We'll read the rest of the data until we hit a marker
	var scanData bytes.Buffer
	for {
		b, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if b == 0xFF {
			// Peek at next byte
			b2, err := reader.ReadByte()
			if err == io.EOF {
				scanData.WriteByte(b)
				break
			}
			if err != nil {
				return err
			}

			if b2 == 0x00 {
				// Byte stuffing, keep both 0xFF and 0x00
				scanData.WriteByte(b)
				scanData.WriteByte(b2)
			} else if standard.IsRST(uint16(0xFF00) | uint16(b2)) {
				// Restart marker, ignore
				continue
			} else {
				// Found a marker, we're done with scan data
				// We've read too far, but that's okay for now
				break
			}
		} else {
			scanData.WriteByte(b)
		}
	}

	huffDec := standard.NewHuffmanDecoder(bytes.NewReader(scanData.Bytes()))

	// Decode MCUs
	mcuCols := standard.DivCeil(d.width, d.mcuWidth)
	mcuRows := standard.DivCeil(d.height, d.mcuHeight)

	for mcuY := 0; mcuY < mcuRows; mcuY++ {
		for mcuX := 0; mcuX < mcuCols; mcuX++ {
			// Decode each component in the MCU
			for _, comp := range d.components {
				for v := 0; v < comp.V; v++ {
					for h := 0; h < comp.H; h++ {
						if err := d.decodeBlock(huffDec, comp, mcuX*comp.H+h, mcuY*comp.V+v); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// decodeBlock decodes a single 8x8 block
func (d *Decoder) decodeBlock(huffDec *standard.HuffmanDecoder, comp *Component, blockX, blockY int) error {
	var coef [64]int32

	// Decode DC coefficient
	dcTable := d.dcTables[comp.dcTableSelector]
	if dcTable == nil {
		return standard.ErrInvalidDHT
	}

	s, err := huffDec.Decode(dcTable)
	if err != nil {
		return err
	}

	diff, err := huffDec.ReceiveExtend(int(s))
	if err != nil {
		return err
	}

	comp.dcPred += diff
	coef[0] = int32(comp.dcPred)

	// Decode AC coefficients
	acTable := d.acTables[comp.acTableSelector]
	if acTable == nil {
		return standard.ErrInvalidDHT
	}

	k := 1
	for k < 64 {
		rs, err := huffDec.Decode(acTable)
		if err != nil {
			return err
		}

		r := int(rs >> 4)   // Run length of zeros
		s := int(rs & 0x0F) // Coefficient size

		if s == 0 {
			if r == 15 {
				// ZRL: skip 16 zeros
				k += 16
			} else {
				// EOB: end of block
				break
			}
		} else {
			k += r

			if k >= 64 {
				return standard.ErrInvalidData
			}

			val, err := huffDec.ReceiveExtend(s)
			if err != nil {
				return err
			}

			coef[standard.ZigZag[k]] = int32(val)
			k++
		}
	}

	qtable := &d.qtables[comp.Tq]
	blockOffset := (blockY*comp.width + blockX) * 64
	if blockOffset+63 >= len(comp.data) {
		// Block is outside the component data, skip it
		return nil
	}

	standard.IDCTISlow(coef[:], *qtable, comp.data[blockOffset:], 8)

	return nil
}

// convertToPixels converts component data to interleaved pixel data
func (d *Decoder) convertToPixels() []byte {
	numComponents := len(d.components)
	pixelData := make([]byte, d.width*d.height*numComponents)

	switch numComponents {
	case 1:
		// Grayscale
		comp := d.components[0]
		for y := 0; y < d.height; y++ {
			for x := 0; x < d.width; x++ {
				blockX := x / 8
				blockY := y / 8
				inBlockX := x % 8
				inBlockY := y % 8

				if blockX < comp.width && blockY < comp.height {
					blockOffset := (blockY*comp.width + blockX) * 64
					val := comp.data[blockOffset+inBlockY*8+inBlockX]
					pixelData[y*d.width+x] = val
				}
			}
		}
	case 3:
		// YCbCr to RGB conversion
		for y := 0; y < d.height; y++ {
			for x := 0; x < d.width; x++ {
				// Sample each component
				var yy, cb, cr byte

				// Get maximum sampling factors
				maxH := d.components[0].H
				maxV := d.components[0].V

				for i, comp := range d.components {
					// Scale coordinates based on sampling factors
					// For 4:2:0, Y has H=2,V=2, Cb/Cr have H=1,V=1
					// So Cb/Cr coordinates are half of Y coordinates
					sx := (x * comp.H) / maxH
					sy := (y * comp.V) / maxV

					blockX := sx / 8
					blockY := sy / 8
					inBlockX := sx % 8
					inBlockY := sy % 8

					if blockX < comp.width && blockY < comp.height {
						blockOffset := (blockY*comp.width + blockX) * 64
						val := comp.data[blockOffset+inBlockY*8+inBlockX]

						switch i {
						case 0:
							yy = val
						case 1:
							cb = val
						case 2:
							cr = val
						}
					}
				}

				// YCbCr to RGB conversion
				r, g, b := ycbcrToRGB(yy, cb, cr)

				offset := (y*d.width + x) * 3
				pixelData[offset+0] = r
				pixelData[offset+1] = g
				pixelData[offset+2] = b
			}
		}
	}

	return pixelData
}

// ycbcrToRGB converts YCbCr to RGB
func ycbcrToRGB(yy, cb, cr byte) (byte, byte, byte) {
	y := int(yy)
	cbVal := int(cb) - 128
	crVal := int(cr) - 128

	r := y + (91881*crVal)>>16
	g := y - ((22554*cbVal + 46802*crVal) >> 16)
	b := y + (116130*cbVal)>>16

	return byte(standard.Clamp(r, 0, 255)),
		byte(standard.Clamp(g, 0, 255)),
		byte(standard.Clamp(b, 0, 255))
}
