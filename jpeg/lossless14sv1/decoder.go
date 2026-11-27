package lossless14sv1

import (
	"io"

	"github.com/cocosip/go-dicom-codecs/jpeg/standard"
)

// Component represents a color component
type Component struct {
	ID              byte
	H               int
	V               int
	width           int
	height          int
	dcTableSelector int
}

// Decoder represents a JPEG Lossless decoder
type Decoder struct {
	width      int
	height     int
	precision  int // Bit depth (2-16)
	components []*Component
	dcTables   [4]*standard.HuffmanTable
}

// Decode decodes JPEG Lossless First-Order Prediction data
func Decode(jpegData []byte) (pixelData []byte, width, height, components, bitDepth int, err error) {
	reader := standard.NewReader(jpegData)

	decoder := &Decoder{}

	// Read SOI marker
	marker, err := reader.ReadMarker()
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	if marker != standard.MarkerSOI {
		return nil, 0, 0, 0, 0, standard.ErrInvalidSOI
	}

	// Parse JPEG segments
	for {
		marker, err := reader.ReadMarker()
		if err != nil {
			return nil, 0, 0, 0, 0, err
		}

		switch marker {
		case standard.MarkerSOF3: // Lossless
			if err := decoder.parseSOF3(reader); err != nil {
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
			// Decode scan data
			pixelData, err := decoder.decodeScan(reader)
			if err != nil {
				return nil, 0, 0, 0, 0, err
			}
			return pixelData, decoder.width, decoder.height, len(decoder.components), decoder.precision, nil

		case standard.MarkerEOI:
			// A stream without a scan carries no samples.
			return nil, decoder.width, decoder.height, len(decoder.components), decoder.precision, nil

		default:
			// Skip unknown markers
			if standard.HasLength(marker) {
				_, err := reader.ReadSegment()
				if err != nil {
					return nil, 0, 0, 0, 0, err
				}
			}
		}
	}
}

// parseSOF3 parses Start of Frame (Lossless)
func (d *Decoder) parseSOF3(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 6 {
		return standard.ErrInvalidSOF
	}

	d.precision = int(data[0])
	if d.precision < 2 || d.precision > 16 {
		return standard.ErrInvalidBitDepth
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
	d.components = make([]*Component, numComponents)

	for i := 0; i < numComponents; i++ {
		offset := 6 + i*3
		comp := &Component{
			ID:     data[offset],
			H:      int(data[offset+1] >> 4),
			V:      int(data[offset+1] & 0x0F),
			width:  d.width,
			height: d.height,
		}

		// For lossless, sampling factors should be 1x1
		if comp.H != 1 || comp.V != 1 {
			return standard.ErrUnsupportedFormat
		}

		d.components[i] = comp
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
		tc := tcTh >> 4   // Table class (0=DC/Lossless)
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

		// For lossless, we only use DC tables
		if tc == 0 {
			d.dcTables[th] = table
		}
	}

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
		cs := data[1+i*2]   // Component selector
		td := data[1+i*2+1] // Table selector (only DC table for lossless)

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

		comp.dcTableSelector = int(td)
	}

	// Get predictor selection (Ss field)
	predictor := int(data[1+ns*2])
	if predictor != 1 {
		return standard.ErrInvalidPredictor
	}

	return nil
}

// decodeScan decodes the scan data straight into interleaved pixel bytes.
func (d *Decoder) decodeScan(reader *standard.Reader) ([]byte, error) {
	scan := make([]byte, 0, 64*1024)
	for {
		b, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if b == 0xFF {
			b2, err := reader.ReadByte()
			if err == io.EOF {
				scan = append(scan, b)
				break
			}
			if err != nil {
				return nil, err
			}

			if b2 == 0x00 {
				// Byte stuffing
				scan = append(scan, b, b2)
			} else if standard.IsRST(uint16(0xFF00) | uint16(b2)) {
				continue
			} else {
				break
			}
		} else {
			scan = append(scan, b)
		}
	}

	huffDec := standard.NewHuffmanDecoder(scan)

	comps := len(d.components)
	bytesPerSample := (d.precision + 7) / 8
	modulus := 1 << uint(d.precision)
	defaultVal := 1 << uint(d.precision-1)
	pixelData := make([]byte, d.width*d.height*comps*bytesPerSample)

	var tables [4]*standard.HuffmanTable
	for i, comp := range d.components {
		tables[i] = d.dcTables[comp.dcTableSelector]
		if tables[i] == nil {
			return nil, standard.ErrInvalidDHT
		}
	}

	prev := make([][]int32, comps)
	cur := make([][]int32, comps)
	for i := range prev {
		prev[i] = make([]int32, d.width)
		cur[i] = make([]int32, d.width)
	}

	rowStride := d.width * comps * bytesPerSample
	for row := 0; row < d.height; row++ {
		out := pixelData[row*rowStride : row*rowStride+rowStride]
		for col := 0; col < d.width; col++ {
			for i := 0; i < comps; i++ {
				diff, err := huffDec.DecodeLossless(tables[i])
				if err != nil {
					return nil, err
				}

				var predicted int
				switch {
				case col > 0:
					predicted = int(cur[i][col-1])
				case row > 0:
					predicted = int(prev[i][col])
				default:
					predicted = defaultVal
				}

				sample := predicted + diff
				if sample < 0 {
					sample += modulus
				} else if sample >= modulus {
					sample -= modulus
				}
				cur[i][col] = int32(sample)

				if bytesPerSample == 1 {
					out[col*comps+i] = byte(sample)
				} else {
					off := (col*comps + i) * 2
					out[off] = byte(sample)
					out[off+1] = byte(sample >> 8)
				}
			}
		}
		prev, cur = cur, prev
	}

	return pixelData, nil
}
