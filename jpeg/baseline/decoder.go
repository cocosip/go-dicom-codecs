package baseline

import (
	"fmt"

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
	data            []byte // Decoded component plane, row stride width*8
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
		comp.data = make([]byte, comp.width*8*comp.height*8)
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
	scan, ok := reader.ReadEntropySegment()
	if !ok {
		scan = readEntropySegmentSlow(reader)
	}
	huffDec := standard.NewHuffmanDecoder(scan)

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
	dcTable := d.dcTables[comp.dcTableSelector]
	if dcTable == nil {
		return standard.ErrInvalidDHT
	}
	acTable := d.acTables[comp.acTableSelector]
	if acTable == nil {
		return standard.ErrInvalidDHT
	}

	var coef [64]int32
	dcPred, err := huffDec.DecodeBlock(dcTable, acTable, &d.qtables[comp.Tq], comp.dcPred, coef[:])
	if err != nil {
		return err
	}
	comp.dcPred = dcPred

	stride := comp.width * 8
	blockOffset := blockY*8*stride + blockX*8
	if blockOffset+7*stride+8 > len(comp.data) {
		// Block is outside the component data, skip it
		return nil
	}

	standard.IDCT8x8(coef[:], comp.data[blockOffset:], stride)

	return nil
}

// readEntropySegmentSlow copies the scan data when restart markers split it.
func readEntropySegmentSlow(reader *standard.Reader) []byte {
	var scan []byte
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return scan
		}
		if b != 0xFF {
			scan = append(scan, b)
			continue
		}
		b2, err := reader.ReadByte()
		if err != nil {
			scan = append(scan, b)
			return scan
		}
		switch {
		case b2 == 0x00:
			scan = append(scan, b, b2)
		case standard.IsRST(uint16(0xFF00) | uint16(b2)):
		default:
			return scan
		}
	}
}

// componentPlane returns the component's de-blockified plane.
func componentPlane(comp *Component) []byte {
	return comp.data
}

// convertToPixels converts component data to interleaved pixel data
func (d *Decoder) convertToPixels() []byte {
	numComponents := len(d.components)
	pixelData := make([]byte, d.width*d.height*numComponents)

	if numComponents == 1 {
		comp := d.components[0]
		// Without padding the plane is already the output buffer.
		if comp.width*8 == d.width && comp.height*8 == d.height {
			return comp.data
		}
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
