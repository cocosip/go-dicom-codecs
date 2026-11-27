package lossless

import (
	"fmt"

	"github.com/cocosip/go-dicom-codecs/jpeg/standard"
)

// Decoder represents a JPEG Lossless decoder
type Decoder struct {
	width      int
	height     int
	components int
	precision  int // Bit depth (2-16)
	predictor  int // Predictor selection (1-7)

	dcTables         [2]*standard.HuffmanTable
	dcTableSelectors [3]int
}

// Decode decodes JPEG Lossless data
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
		case standard.MarkerSOF3:
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

			return pixelData, decoder.width, decoder.height, decoder.components, decoder.precision, nil

		case standard.MarkerEOI:
			return nil, 0, 0, 0, 0, fmt.Errorf("unexpected EOI before scan data")

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
		return fmt.Errorf("invalid precision: %d (must be 2-16)", d.precision)
	}

	d.height = int(data[1])<<8 | int(data[2])
	d.width = int(data[3])<<8 | int(data[4])
	d.components = int(data[5])

	if d.width <= 0 || d.height <= 0 {
		return standard.ErrInvalidDimensions
	}

	if d.components != 1 && d.components != 3 {
		return standard.ErrInvalidComponents
	}

	return nil
}

// parseDHT parses Define Huffman Table
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
		offset++

		tc := (tcTh >> 4) & 0x0F // Table class (0 = DC)
		th := tcTh & 0x0F        // Table ID

		if th >= 2 {
			return fmt.Errorf("invalid Huffman table ID: %d", th)
		}

		// Read bit counts
		var bits [16]int
		totalSymbols := 0
		for i := 0; i < 16; i++ {
			if offset >= len(data) {
				return standard.ErrInvalidDHT
			}
			bits[i] = int(data[offset])
			totalSymbols += bits[i]
			offset++
		}

		// Read symbol values
		if offset+totalSymbols > len(data) {
			return standard.ErrInvalidDHT
		}
		values := make([]byte, totalSymbols)
		copy(values, data[offset:offset+totalSymbols])
		offset += totalSymbols

		// Build Huffman table
		var bitsArray [16]int
		copy(bitsArray[:], bits[:])
		table := standard.BuildStandardHuffmanTable(bitsArray, values)

		if tc == 0 {
			// DC table (used for lossless)
			d.dcTables[th] = table
		}
	}

	return nil
}

// parseSOS parses Start of Scan
func (d *Decoder) parseSOS(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 1+d.components*2+3 {
		return standard.ErrInvalidSOS
	}

	numComponents := int(data[0])
	if numComponents != d.components {
		return fmt.Errorf("SOS component count %d does not match SOF %d", numComponents, d.components)
	}

	// Parse spectral selection (contains predictor)
	d.predictor = int(data[1+d.components*2]) // Ss field contains predictor

	if d.predictor < 1 || d.predictor > 7 {
		return fmt.Errorf("invalid predictor: %d (must be 1-7)", d.predictor)
	}

	for component := 0; component < d.components; component++ {
		selector := int(data[2+component*2] >> 4)
		if selector >= len(d.dcTables) {
			return fmt.Errorf("invalid DC Huffman table selector: %d", selector)
		}
		d.dcTableSelectors[component] = selector
	}

	return nil
}

// decodeScan decodes the scan data straight into interleaved pixel bytes.
func (d *Decoder) decodeScan(reader *standard.Reader) ([]byte, error) {
	scan, err := losslessEntropySegment(reader)
	if err != nil {
		return nil, err
	}
	huffDec := standard.NewHuffmanDecoder(scan)

	var tables [3]*standard.HuffmanTable
	for i := 0; i < d.components; i++ {
		tables[i] = d.dcTables[d.dcTableSelectors[i]]
		if tables[i] == nil {
			return nil, fmt.Errorf("huffman table %d not defined", d.dcTableSelectors[i])
		}
	}

	bytesPerSample := (d.precision + 7) / 8
	width := d.width
	comps := d.components
	modulus := 1 << uint(d.precision)
	defaultVal := modulus >> 1
	predictor := d.predictor
	pixelData := make([]byte, width*d.height*comps*bytesPerSample)

	// Only the current and previous sample rows are needed for prediction.
	// Samples of all components are kept in one flat row buffer.
	prev := make([]int32, comps*width)
	cur := make([]int32, comps*width)
	// The first row predicts against the neutral value, so seed prev with it.
	for i := range prev {
		prev[i] = int32(defaultVal)
	}

	rowStride := width * comps * bytesPerSample
	for row := 0; row < d.height; row++ {
		out := pixelData[row*rowStride : row*rowStride+rowStride]

		// The first sample of a row reads its left neighbour from above.
		for comp := 0; comp < comps; comp++ {
			diff, err := huffDec.DecodeLossless(tables[comp])
			if err != nil {
				return nil, err
			}
			above := int(prev[comp])
			left := defaultVal
			if predictor == 1 {
				left = above
			}
			sample := Predictor(predictor, left, above, defaultVal) + diff
			if sample < 0 {
				sample += modulus
			} else if sample >= modulus {
				sample -= modulus
			}
			cur[comp] = int32(sample)
			if bytesPerSample == 1 {
				out[comp] = byte(sample)
			} else {
				out[comp*2] = byte(sample)
				out[comp*2+1] = byte(sample >> 8)
			}
		}

		for col := 1; col < width; col++ {
			base := col * comps
			for comp := 0; comp < comps; comp++ {
				diff, err := huffDec.DecodeLossless(tables[comp])
				if err != nil {
					return nil, err
				}
				id := base + comp
				sample := Predictor(predictor, int(cur[id-comps]), int(prev[id]), int(prev[id-comps])) + diff
				if sample < 0 {
					sample += modulus
				} else if sample >= modulus {
					sample -= modulus
				}
				cur[id] = int32(sample)
				if bytesPerSample == 1 {
					out[id] = byte(sample)
				} else {
					out[id*2] = byte(sample)
					out[id*2+1] = byte(sample >> 8)
				}
			}
		}
		prev, cur = cur, prev
	}

	return pixelData, nil
}

// losslessEntropySegment returns the scan bytes, copying only when a restart
// marker splits the segment.
func losslessEntropySegment(reader *standard.Reader) ([]byte, error) {
	if segment, ok := reader.ReadEntropySegment(); ok {
		// A private copy keeps the decoder off the large file buffer.
		out := make([]byte, len(segment))
		copy(out, segment)
		return out, nil
	}
	return readEntropySegment(reader)
}

// readEntropySegment copies the entropy-coded bytes up to the next marker,
// keeping byte stuffing so the Huffman decoder can remove it.
func readEntropySegment(reader *standard.Reader) ([]byte, error) {
	segment := make([]byte, 0, 64*1024)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		if b != 0xFF {
			segment = append(segment, b)
			continue
		}
		b2, err := reader.ReadByte()
		if err != nil {
			segment = append(segment, b)
			break
		}
		switch {
		case b2 == 0x00:
			segment = append(segment, b, b2)
		case standard.IsRST(uint16(0xFF00) | uint16(b2)):
			// Restart markers are not part of the bit stream.
		default:
			return segment, nil
		}
	}
	return segment, nil
}
