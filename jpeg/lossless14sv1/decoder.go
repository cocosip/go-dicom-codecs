package lossless14sv1

import (
	"bytes"
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
	pred            int   // Prediction value for current row
	data            []int // Decoded component data (signed integers for lossless)
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
			if err := decoder.decodeScan(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}
			// Convert to output format
			pixelData = decoder.convertToPixels()
			return pixelData, decoder.width, decoder.height, len(decoder.components), decoder.precision, nil

		case standard.MarkerEOI:
			// Should not reach here normally
			pixelData = decoder.convertToPixels()
			return pixelData, decoder.width, decoder.height, len(decoder.components), decoder.precision, nil

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
			data:   make([]int, d.width*d.height),
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

// decodeScan decodes the scan data
func (d *Decoder) decodeScan(reader *standard.Reader) error {
	// Compute modulus for wrapping reconstructed samples to P-bit range
	modulus := 1 << uint(d.precision)

	// Collect scan data
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
			b2, err := reader.ReadByte()
			if err == io.EOF {
				scanData.WriteByte(b)
				break
			}
			if err != nil {
				return err
			}

			if b2 == 0x00 {
				// Byte stuffing
				scanData.WriteByte(b)
				scanData.WriteByte(b2)
			} else if standard.IsRST(uint16(0xFF00) | uint16(b2)) {
				// Restart marker, reset predictors
				for _, comp := range d.components {
					comp.pred = 0
				}
				continue
			} else {
				// Found a marker, done
				break
			}
		} else {
			scanData.WriteByte(b)
		}
	}

	huffDec := standard.NewHuffmanDecoder(scanData.Bytes())

	// Decode samples (lossless uses line-by-line interleaved)
	for row := 0; row < d.height; row++ {
		for col := 0; col < d.width; col++ {
			for _, comp := range d.components {
				// Decode difference
				table := d.dcTables[comp.dcTableSelector]
				if table == nil {
					return standard.ErrInvalidDHT
				}

				ssss, err := huffDec.Decode(table)
				if err != nil {
					return err
				}

				diff, err := huffDec.ReceiveLosslessDifference(int(ssss))
				if err != nil {
					return err
				}

				// First-order prediction: use left pixel (Predictor 1)
				// Special case: first pixel of each row
				var predicted int
				if col == 0 {
					if row == 0 {
						// First pixel of first row: use 2^(P-1) per JPEG spec
						predicted = 1 << uint(d.precision-1)
					} else {
						// First pixel of other rows: use pixel from row above
						predicted = comp.data[(row-1)*d.width+col]
					}
				} else {
					// Other pixels: use left pixel
					predicted = comp.data[row*d.width+col-1]
				}

				// Reconstruct sample with wrapping to unsigned P-bit range
				sample := predicted + diff
				// Wrap to range [0, 2^P-1]
				if sample < 0 {
					sample += modulus
				} else if sample >= modulus {
					sample -= modulus
				}

				// Store
				comp.data[row*d.width+col] = sample
			}
		}
	}

	return nil
}

// convertToPixels converts component data to byte array
func (d *Decoder) convertToPixels() []byte {
	numComponents := len(d.components)
	bytesPerSample := (d.precision + 7) / 8
	pixelData := make([]byte, d.width*d.height*numComponents*bytesPerSample)

	if d.precision <= 8 {
		// 8-bit or less: one byte per sample
		for y := 0; y < d.height; y++ {
			for x := 0; x < d.width; x++ {
				for i, comp := range d.components {
					val := comp.data[y*d.width+x]
					pixelData[(y*d.width+x)*numComponents+i] = byte(val)
				}
			}
		}
	} else {
		// 9-16 bit: two bytes per sample (little-endian)
		offset := 0
		for y := 0; y < d.height; y++ {
			for x := 0; x < d.width; x++ {
				for _, comp := range d.components {
					val := comp.data[y*d.width+x]
					// Little-endian
					pixelData[offset] = byte(val & 0xFF)
					pixelData[offset+1] = byte((val >> 8) & 0xFF)
					offset += 2
				}
			}
		}
	}

	return pixelData
}
