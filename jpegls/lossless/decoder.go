package lossless

import (
	"bytes"
	"fmt"
	"io"

	"github.com/cocosip/go-dicom-codecs/jpeg/standard"
	"github.com/cocosip/go-dicom-codecs/jpegls/runmode"
)

// Decoder represents a JPEG-LS lossless decoder
type Decoder struct {
	width      int
	height     int
	components int
	bitDepth   int
	maxVal     int
	interleave int
	traits     Traits

	contextTable   *ContextTable
	quantizer      *GradientQuantizer
	runModeScanner *RunModeScanner
}

// NewDecoder creates a new JPEG-LS decoder
func NewDecoder() *Decoder {
	return &Decoder{}
}

// computeErrorValue mirrors encoder compute_error_value: for Near=0 use unsigned wrap.
func (dec *Decoder) computeErrorValue(delta int) int {
	if dec.bitDepth <= 8 {
		return int(int8(delta))
	}
	return int(int16(delta))
}

// Decode decodes JPEG-LS compressed data
// Returns: pixelData, width, height, components, bitDepth, error
func Decode(jpegLSData []byte) ([]byte, int, int, int, int, error) {
	decoder := NewDecoder()
	return decoder.decode(jpegLSData)
}

// decode performs the actual decoding
func (dec *Decoder) decode(jpegLSData []byte) ([]byte, int, int, int, int, error) {
	reader := standard.NewReader(jpegLSData)

	// Read SOI marker
	marker, err := reader.ReadMarker()
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	if marker != standard.MarkerSOI {
		return nil, 0, 0, 0, 0, standard.ErrInvalidSOI
	}

	// Parse segments
	for {
		marker, err := reader.ReadMarker()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, 0, 0, 0, 0, err
		}

		switch marker {
		case 0xFFF7: // SOF55 (JPEG-LS)
			if err := dec.parseSOF55(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}

		case 0xFFF8: // LSE (JPEG-LS parameters)
			if err := dec.parseLSE(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}

		case standard.MarkerSOS:
			if err := dec.parseSOS(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}

			// Decode scan data
			pixelData, err := dec.decodeScan(reader)
			if err != nil {
				return nil, 0, 0, 0, 0, err
			}

			return pixelData, dec.width, dec.height, dec.components, dec.bitDepth, nil

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

	return nil, 0, 0, 0, 0, fmt.Errorf("incomplete JPEG-LS data")
}

// parseSOF55 parses the SOF55 segment
func (dec *Decoder) parseSOF55(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 6 {
		return standard.ErrInvalidSOF
	}

	dec.bitDepth = int(data[0])
	dec.height = int(data[1])<<8 | int(data[2])
	dec.width = int(data[3])<<8 | int(data[4])
	dec.components = int(data[5])

	if dec.width <= 0 || dec.height <= 0 {
		return standard.ErrInvalidDimensions
	}

	if dec.components != 1 && dec.components != 3 {
		return standard.ErrInvalidComponents
	}

	dec.maxVal = (1 << uint(dec.bitDepth)) - 1
	dec.traits = NewTraits(dec.maxVal, 0, 64)
	dec.initCodingParameters(0, 0, 0)

	return nil
}

// initCodingParameters recomputes derived parameters and contexts (legacy/bitDepth-based).
func (dec *Decoder) initCodingParameters(t1, t2, t3 int) {
	params := ComputeCodingParameters(dec.maxVal, 0, dec.traits.Reset)
	if t1 == 0 || t2 == 0 || t3 == 0 {
		t1, t2, t3 = params.T1, params.T2, params.T3
	}

	dec.traits = NewTraits(dec.maxVal, 0, params.Reset)
	dec.quantizer = NewGradientQuantizer(t1, t2, t3, dec.traits.Near)
	dec.contextTable = NewContextTable(dec.maxVal, 0, dec.traits.Reset)
	dec.runModeScanner = NewRunModeScanner(dec.traits)
}

// parseLSE parses the LSE segment (JPEG-LS parameters)
func (dec *Decoder) parseLSE(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 1 {
		return fmt.Errorf("invalid LSE segment")
	}

	// ID byte
	id := data[0]

	if id == 1 {
		// Preset parameters
		// Expected: 11 bytes of data (ID + MAXVAL + T1 + T2 + T3 + RESET)
		if len(data) < 11 {
			return fmt.Errorf("invalid LSE preset parameters")
		}

		// Read MAXVAL
		maxVal := int(data[1])<<8 | int(data[2])
		t1 := int(data[3])<<8 | int(data[4])
		t2 := int(data[5])<<8 | int(data[6])
		t3 := int(data[7])<<8 | int(data[8])
		reset := int(data[9])<<8 | int(data[10])

		if maxVal <= 0 {
			maxVal = dec.maxVal
		}
		dec.maxVal = maxVal
		if reset == 0 {
			reset = 64
		}
		dec.traits.Reset = reset
		dec.initCodingParameters(t1, t2, t3)
	}

	return nil
}

// parseSOS parses the SOS segment
func (dec *Decoder) parseSOS(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 4 {
		return standard.ErrInvalidSOS
	}

	numComponents := int(data[0])
	if numComponents != dec.components {
		return fmt.Errorf("SOS component count mismatch")
	}
	dec.interleave = int(data[len(data)-2])
	if dec.components == 1 && dec.interleave != 0 {
		return fmt.Errorf("invalid JPEG-LS interleave mode %d for grayscale scan", dec.interleave)
	}
	if dec.components > 1 && dec.interleave != 2 {
		return fmt.Errorf("unsupported JPEG-LS interleave mode %d for RGB scan", dec.interleave)
	}

	return nil
}

// decodeScan decodes the scan data
func (dec *Decoder) decodeScan(reader *standard.Reader) ([]byte, error) {
	// Read all remaining data until EOI
	var scanData bytes.Buffer
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if b == 0xFF {
			b2, err := reader.ReadByte()
			if err != nil {
				if err == io.EOF {
					scanData.WriteByte(b)
					break
				}
				return nil, err
			}

			// JPEG-LS byte stuffing: after 0xFF, next byte has high bit = 0 (stuffed bit)
			// If b2 < 0x80: scan data (write both bytes as-is, GolombReader handles bit stuffing)
			// If b2 >= 0x80: marker
			if b2 < 0x80 {
				// Scan data with stuffed bit
				scanData.WriteByte(b)
				scanData.WriteByte(b2)
				continue
			}
			if b2 == 0xD9 {
				break
			}
			// restart markers: ignore marker, continue decoding data after it
			continue
		}
		scanData.WriteByte(b)
	}

	// Create Golomb reader
	gr := NewGolombReader(bytes.NewReader(scanData.Bytes()))

	// Allocate pixel array
	totalPixels := dec.width * dec.height * dec.components
	pixels := make([]int, totalPixels)

	if dec.components > 1 {
		if err := dec.decodeSampleInterleaved(gr, pixels); err != nil {
			return nil, err
		}
	} else {
		if err := dec.decodeComponent(gr, pixels, 0); err != nil {
			return nil, err
		}
	}

	// Convert integers to bytes
	return dec.integersToPixels(pixels), nil
}

func (dec *Decoder) decodeSampleInterleaved(gr *GolombReader, pixels []int) error {
	var previousLineFirst, previousPreviousLineFirst [3]int
	for y := 0; y < dec.height; y++ {
		x := 0
		for x < dec.width {
			var ra, rb, rc, rd [3]int
			var qs [3]int
			for comp := 0; comp < dec.components; comp++ {
				ra[comp], rb[comp], rc[comp], rd[comp] = dec.sampleNeighbors(pixels, x, y, comp, previousLineFirst[comp], previousPreviousLineFirst[comp])
				q1, q2, q3 := dec.quantizer.ComputeContext(ra[comp], rb[comp], rc[comp], rd[comp])
				qs[comp] = ComputeContextID(q1, q2, q3)
			}

			if qs[0] == 0 && qs[1] == 0 && qs[2] == 0 {
				processed, err := dec.decodeSampleRunMode(gr, pixels, x, y, previousLineFirst, previousPreviousLineFirst)
				if err != nil {
					return err
				}
				x += processed
				continue
			}

			for comp := 0; comp < dec.components; comp++ {
				if err := dec.decodeRegularSample(gr, pixels, x, y, comp, qs[comp], ra[comp], rb[comp], rc[comp]); err != nil {
					return err
				}
			}
			x++
		}
		for comp := 0; comp < dec.components; comp++ {
			previousPreviousLineFirst[comp] = previousLineFirst[comp]
			previousLineFirst[comp] = pixels[(y*dec.width)*dec.components+comp]
		}
	}
	return nil
}

func (dec *Decoder) decodeRegularSample(gr *GolombReader, pixels []int, x, y, comp, qs, ra, rb, rc int) error {
	sign := BitwiseSign(qs)
	contextIdx := ApplySign(qs, sign)
	if contextIdx < 0 || contextIdx >= len(dec.contextTable.contexts) {
		return fmt.Errorf("context index %d out of bounds [0, %d)", contextIdx, len(dec.contextTable.contexts))
	}
	ctx := dec.contextTable.contexts[contextIdx]
	k := ctx.ComputeGolombParameter()
	predictedValue := dec.traits.CorrectPrediction(Predict(ra, rb, rc) + ApplySign(ctx.C, sign))
	mappedError, err := gr.DecodeValue(k, dec.traits.Limit, dec.traits.Qbpp)
	if err != nil {
		return fmt.Errorf("decode regular at x=%d y=%d comp=%d (bits=%d): %w", x, y, comp, gr.bitsRead, err)
	}
	errorValue := UnmapErrorValue(mappedError)
	if k == 0 {
		errorValue ^= ctx.GetErrorCorrection(k, 0)
	}
	ctx.UpdateContext(errorValue, 0, dec.traits.Reset)
	idx := (y*dec.width+x)*dec.components + comp
	pixels[idx] = dec.traits.ComputeReconstructedSample(predictedValue, ApplySign(errorValue, sign))
	return nil
}

func (dec *Decoder) decodeSampleRunMode(gr *GolombReader, pixels []int, x, y int, previousLineFirst, previousPreviousLineFirst [3]int) (int, error) {
	remaining := dec.width - x
	runLength, err := dec.runModeScanner.DecodeRunLength(gr, remaining)
	if err != nil {
		return 0, err
	}
	for i := 0; i < runLength; i++ {
		for comp := 0; comp < dec.components; comp++ {
			left, _, _, _ := dec.sampleNeighbors(pixels, x, y, comp, previousLineFirst[comp], previousPreviousLineFirst[comp])
			pixels[((y*dec.width)+(x+i))*dec.components+comp] = left
		}
	}
	if runLength == remaining {
		return runLength, nil
	}

	idx := ((y * dec.width) + (x + runLength)) * dec.components
	for comp := 0; comp < dec.components; comp++ {
		left, above, _, _ := dec.sampleNeighbors(pixels, x+runLength, y, comp, previousLineFirst[comp], previousPreviousLineFirst[comp])
		errorValue, err := dec.runModeScanner.DecodeRunInterruption(gr, dec.runModeScanner.RunModeContexts[0])
		if err != nil {
			return 0, err
		}
		pixels[idx+comp] = dec.traits.ComputeReconstructedSample(above, errorValue*signInt(above-left))
	}
	dec.runModeScanner.DecRunIndex()
	return runLength + 1, nil
}

func (dec *Decoder) sampleNeighbors(pixels []int, x, y, comp, previousLineFirst, previousPreviousLineFirst int) (left, above, aboveLeft, aboveRight int) {
	if x == 0 {
		left = previousLineFirst
		if y > 0 {
			above = previousLineFirst
		}
		aboveLeft = previousPreviousLineFirst
		if y > 0 && dec.width > 1 {
			aboveRight = pixels[((y-1)*dec.width+1)*dec.components+comp]
		} else {
			aboveRight = above
		}
		return
	}

	if y > 0 {
		above = pixels[((y-1)*dec.width+x)*dec.components+comp]
		aboveLeft = pixels[((y-1)*dec.width+x-1)*dec.components+comp]
		aboveRight = pixels[((y-1)*dec.width+min(x+1, dec.width-1))*dec.components+comp]
	}
	left = pixels[(y*dec.width+x-1)*dec.components+comp]
	if y == 0 {
		aboveRight = above
	}
	return
}

// decodeComponent decodes a single component with RUN mode support
func (dec *Decoder) decodeComponent(gr *GolombReader, pixels []int, comp int) error {
	stride := dec.components
	offset := comp

	prevFirstPrev := 0 // first pixel of previous line (y-1)
	prevNeg1 := 0      // stored previous_line[-1] value (line y-2 first pixel)

	// Process line by line
	for y := 0; y < dec.height; y++ {
		// Reset run index at start of each line (JPEG-LS standard)
		dec.runModeScanner.ResetLine()

		x := 0
		for x < dec.width {
			idx := (y*dec.width+x)*stride + offset

			// Get neighboring pixels (ra=left, rb=top, rc=top-left, rd=top-right)
			var ra, rb, rc, rd int
			if x == 0 {
				ra = prevFirstPrev
				rb = 0
				if y > 0 {
					rb = prevFirstPrev
				}
				rc = prevNeg1
				if y > 0 && dec.width > 1 {
					rdIdx := ((y-1)*dec.width+(x+1))*stride + offset
					if rdIdx < len(pixels) {
						rd = pixels[rdIdx]
					} else {
						rd = rb
					}
				} else {
					rd = rb
				}
			} else {
				ra, rb, rc, rd = dec.getNeighbors(pixels, x, y, comp)
			}

			// Compute prediction
			predicted := Predict(ra, rb, rc)

			// Compute context ID with sign symmetry
			q1, q2, q3 := dec.quantizer.ComputeContext(ra, rb, rc, rd)
			qs := ComputeContextID(q1, q2, q3)

			// Check if we should use RUN mode (qs == 0 means flat region)
			if qs != 0 {
				// Regular mode - matches CharLS do_regular for decoder

				// Extract sign and apply sign symmetry
				sign := BitwiseSign(qs)
				contextIdx := ApplySign(qs, sign)

				// Get context with bounds check
				if contextIdx < 0 || contextIdx >= len(dec.contextTable.contexts) {
					return fmt.Errorf("context index %d out of bounds [0, %d)", contextIdx, len(dec.contextTable.contexts))
				}
				ctx := dec.contextTable.contexts[contextIdx]

				// Get Golomb parameter
				k := ctx.ComputeGolombParameter()

				// Apply prediction correction (CharLS: traits_.correct_prediction(predicted + apply_sign(context.c(), sign)))
				correctionC := ApplySign(ctx.C, sign)
				predictedValue := dec.traits.CorrectPrediction(predicted + correctionC)

				// In REGULAR MODE, use limit directly (J[RunIndex] is only for RUN MODE)
				mappedError, err := gr.DecodeValue(k, dec.traits.Limit, dec.traits.Qbpp)
				if err != nil {
					return fmt.Errorf("decode regular at x=%d y=%d comp=%d (bits=%d): %w", x, y, comp, gr.bitsRead, err)
				}

				// Unmap error (before sign)
				errorValue := UnmapErrorValue(mappedError)
				if k == 0 {
					errorValue ^= ctx.GetErrorCorrection(k, 0)
				}

				// Update context using unsigned error (before sign)
				ctx.UpdateContext(errorValue, 0, dec.traits.Reset)

				// Apply sign to error and reconstruct
				signedError := ApplySign(errorValue, sign)
				reconstructed := dec.traits.ComputeReconstructedSample(predictedValue, signedError)

				pixels[idx] = reconstructed
				x++
			} else {
				// RUN mode (qs == 0, flat region)
				pixelsProcessed, err := dec.doRunMode(gr, pixels, x, y, comp, ra)
				if err != nil {
					return fmt.Errorf("decode run at x=%d y=%d comp=%d (bits=%d): %w", x, y, comp, gr.bitsRead, err)
				}
				x += pixelsProcessed
			}
		}

		firstIdx := (y*dec.width+0)*stride + offset
		if firstIdx < len(pixels) {
			nextFirst := pixels[firstIdx]
			prevNeg1 = prevFirstPrev
			prevFirstPrev = nextFirst
		}
	}

	return nil
}

// decodeRunInterruptionPixel decodes the pixel that interrupts a run (CharLS: decode_run_interruption_pixel)
func (dec *Decoder) decodeRunInterruptionPixel(gr *GolombReader, ra, rb int) (int, error) {
	if runmode.Abs(ra-rb) <= dec.traits.Near {
		// Use run mode context 1
		errorValue, err := dec.runModeScanner.DecodeRunInterruption(gr, dec.runModeScanner.RunModeContexts[1])
		if err != nil {
			return 0, err
		}
		errorValue = dec.computeErrorValue(errorValue)
		return dec.traits.ComputeReconstructedSample(ra, errorValue), nil
	}

	// Use run mode context 0
	errorValue, err := dec.runModeScanner.DecodeRunInterruption(gr, dec.runModeScanner.RunModeContexts[0])
	if err != nil {
		return 0, err
	}
	errorValue = dec.computeErrorValue(errorValue * signInt(rb-ra))
	return dec.traits.ComputeReconstructedSample(rb, errorValue), nil
}

// doRunMode handles decoding in run mode (when qs == 0) - CharLS: do_run_mode for decoder
func (dec *Decoder) doRunMode(gr *GolombReader, pixels []int, x, y, comp int, ra int) (int, error) {
	stride := dec.components
	offset := comp

	startIdx := y*dec.width + x
	remainingInLine := dec.width - x

	// Decode run length using RunModeScanner - CharLS: decode_run_pixels(ra, current_line_ + start_index, width_ - start_index)
	runLength, err := dec.runModeScanner.DecodeRunLength(gr, remainingInLine)
	if err != nil {
		return 0, fmt.Errorf("runLength (bits=%d): %w", gr.bitsRead, err)
	}

	// Fill run with ra value
	for i := 0; i < runLength; i++ {
		idx := (startIdx+i)*stride + offset
		if idx < len(pixels) {
			pixels[idx] = ra
		}
	}

	// Check if run reaches end of line
	if runLength >= remainingInLine {
		return runLength, nil
	}

	// Handle run interruption - CharLS: current_line_[end_index] = decode_run_interruption_pixel(ra, rb)
	interruptIdx := (startIdx+runLength)*stride + offset

	// Get rb (top pixel at interruption point)
	rbIdx := ((y-1)*dec.width+(x+runLength))*stride + offset
	rb := 0
	if rbIdx >= 0 && rbIdx < len(pixels) {
		rb = pixels[rbIdx]
	}

	// Decode interruption pixel
	reconstructed, err := dec.decodeRunInterruptionPixel(gr, ra, rb)
	if err != nil {
		return 0, fmt.Errorf("runInterruption (bits=%d): %w", gr.bitsRead, err)
	}

	// Store reconstructed value
	if interruptIdx < len(pixels) {
		pixels[interruptIdx] = reconstructed
	}

	// Decrement run index - CharLS: decrement_run_index()
	dec.runModeScanner.DecRunIndex()

	return runLength + 1, nil
}

// getNeighbors gets neighboring pixels for prediction
func (dec *Decoder) getNeighbors(pixels []int, x, y, comp int) (int, int, int, int) {
	stride := dec.components
	offset := comp

	a, b, c, d := 0, 0, 0, 0

	// a = left pixel (West)
	// CharLS: current_line_[-1] is initialized to previous_line_[0] for first pixel of row
	if x > 0 {
		idx := (y*dec.width+(x-1))*stride + offset
		a = pixels[idx]
	} else if y > 0 {
		// Special case: x=0, y>0 - CharLS uses previous_line_[0] as current_line_[-1]
		idx := ((y-1)*dec.width+0)*stride + offset
		a = pixels[idx]
	}

	if y > 0 {
		idx := ((y-1)*dec.width+x)*stride + offset
		b = pixels[idx]
	}

	// c = top-left pixel (North-West)
	// CharLS: when x=0, rc = previous_line_[-1], which gets updated at END of each row to current_line_[0]
	// So for row y at x=0: previous_line_[-1] = current_line_[0] from row y-1 = pixels[y-1, 0]
	if x > 0 && y > 0 {
		idx := ((y-1)*dec.width+(x-1))*stride + offset
		c = pixels[idx]
	} else if x == 0 && y > 0 {
		// Special case: x=0, y>0 - CharLS keeps previous_line_[-1] at 0
		c = 0
	}

	// d = top-right pixel (North-East)
	// CharLS: previous_line_[width_] = previous_line_[width_ - 1]
	if y > 0 {
		if x < dec.width-1 {
			idx := ((y-1)*dec.width+(x+1))*stride + offset
			d = pixels[idx]
		} else {
			// Right edge: d = b (rightmost top pixel)
			d = b
		}
	}

	return a, b, c, d
}

// integersToPixels converts integer array to pixel bytes
func (dec *Decoder) integersToPixels(pixels []int) []byte {
	if dec.bitDepth <= 8 {
		// 8-bit: one byte per sample
		pixelData := make([]byte, len(pixels))
		for i, val := range pixels {
			if val < 0 {
				val = 0
			} else if val > dec.maxVal {
				val = dec.maxVal
			}
			pixelData[i] = byte(val)
		}
		return pixelData
	}

	// 9-16 bit: two bytes per sample (little-endian)
	pixelData := make([]byte, len(pixels)*2)
	for i, val := range pixels {
		if val < 0 {
			val = 0
		} else if val > dec.maxVal {
			val = dec.maxVal
		}
		idx := i * 2
		pixelData[idx] = byte(val & 0xFF)
		pixelData[idx+1] = byte((val >> 8) & 0xFF)
	}
	return pixelData
}
