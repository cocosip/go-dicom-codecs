package baseline

import (
	"bytes"
	"fmt"
	"io"

	"github.com/cocosip/go-dicom-codecs/jpeg/standard"
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
	reader := standard.NewReader(jpegData)

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

	huffDec := standard.NewHuffmanDecoder(scanData.Bytes())

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

	for i := 0; i < 64; i++ {
		coef[i] *= qtable[i]
	}
	standard.IDCTFast(coef[:], comp.data[blockOffset:], 8)

	return nil
}

// componentPlane de-blockifies one component into a row-major plane.
func componentPlane(comp *Component) []byte {
	planeWidth := comp.width * 8
	plane := make([]byte, planeWidth*comp.height*8)
	for by := 0; by < comp.height; by++ {
		for bx := 0; bx < comp.width; bx++ {
			src := comp.data[(by*comp.width+bx)*64:]
			for r := 0; r < 8; r++ {
				dst := (by*8+r)*planeWidth + bx*8
				copy(plane[dst:dst+8], src[r*8:r*8+8])
			}
		}
	}
	return plane
}

// convertToPixels converts component data to interleaved pixel data
func (d *Decoder) convertToPixels() []byte {
	numComponents := len(d.components)
	pixelData := make([]byte, d.width*d.height*numComponents)

	if numComponents == 1 {
		comp := d.components[0]
		plane := componentPlane(comp)
		planeWidth := comp.width * 8
		for y := 0; y < d.height; y++ {
			copy(pixelData[y*d.width:(y+1)*d.width], plane[y*planeWidth:y*planeWidth+d.width])
		}
		return pixelData
	}

	planes := make([][]byte, numComponents)
	widths := make([]int, numComponents)
	xmap := make([][]int, numComponents)
	maxH, maxV := 1, 1
	for i, comp := range d.components {
		if comp.H > maxH {
			maxH = comp.H
		}
		if comp.V > maxV {
			maxV = comp.V
		}
		planes[i] = componentPlane(comp)
		widths[i] = comp.width * 8
		xmap[i] = make([]int, d.width)
		for x := 0; x < d.width; x++ {
			xmap[i][x] = (x * comp.H) / maxH
		}
	}

	yPlane, cbPlane, crPlane := planes[0], planes[1], planes[2]
	yWidth, cbWidth, crWidth := widths[0], widths[1], widths[2]
	yMap, cbMap, crMap := xmap[0], xmap[1], xmap[2]
	identity := maxH == 1 && maxV == 1
	for y := 0; y < d.height; y++ {
		yRow := yPlane[(y*d.components[0].V/maxV)*yWidth:]
		cbRow := cbPlane[(y*d.components[1].V/maxV)*cbWidth:]
		crRow := crPlane[(y*d.components[2].V/maxV)*crWidth:]
		out := pixelData[y*d.width*3:]
		if identity {
			for x := 0; x < d.width; x++ {
				yy := int(yRow[x])
				cb, cr := cbRow[x], crRow[x]
				r := yy + int(ycbcrCrR[cr])
				g := yy + int(ycbcrCbG[cb]) + int(ycbcrCrG[cr])
				b := yy + int(ycbcrCbB[cb])
				if r < 0 {
					r = 0
				} else if r > 255 {
					r = 255
				}
				if g < 0 {
					g = 0
				} else if g > 255 {
					g = 255
				}
				if b < 0 {
					b = 0
				} else if b > 255 {
					b = 255
				}
				out[x*3] = byte(r)
				out[x*3+1] = byte(g)
				out[x*3+2] = byte(b)
			}
			continue
		}
		for x := 0; x < d.width; x++ {
			yy := int(yRow[yMap[x]])
			cb, cr := cbRow[cbMap[x]], crRow[crMap[x]]
			r := yy + int(ycbcrCrR[cr])
			g := yy + int(ycbcrCbG[cb]) + int(ycbcrCrG[cr])
			b := yy + int(ycbcrCbB[cb])
			if r < 0 {
				r = 0
			} else if r > 255 {
				r = 255
			}
			if g < 0 {
				g = 0
			} else if g > 255 {
				g = 255
			}
			if b < 0 {
				b = 0
			} else if b > 255 {
				b = 255
			}
			out[x*3] = byte(r)
			out[x*3+1] = byte(g)
			out[x*3+2] = byte(b)
		}
	}

	return pixelData
}

var (
	ycbcrCrR [256]int32
	ycbcrCbB [256]int32
	ycbcrCrG [256]int32
	ycbcrCbG [256]int32
)

func init() {
	for i := 0; i < 256; i++ {
		v := int32(i - 128)
		ycbcrCrR[i] = (91881 * v) >> 16
		ycbcrCbB[i] = (116130 * v) >> 16
		ycbcrCrG[i] = (-46802 * v) >> 16
		ycbcrCbG[i] = (-22554 * v) >> 16
	}
}
