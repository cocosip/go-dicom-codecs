package nearlossless

import (
	"bytes"
	"fmt"
	"io"

	"github.com/cocosip/go-dicom-codecs/jpeg/standard"
	"github.com/cocosip/go-dicom-codecs/jpegls/lossless"
	"github.com/cocosip/go-dicom-codecs/jpegls/runmode"
)

// Decoder represents a JPEG-LS near-lossless decoder
type Decoder struct {
	width      int
	height     int
	components int
	bitDepth   int
	maxVal     int
	near       int // NEAR parameter
	interleave int
	t1         int
	t2         int
	t3         int

	traits         lossless.Traits
	contextTable   *lossless.ContextTable
	quantizer      *lossless.GradientQuantizer
	runModeScanner *lossless.RunModeScanner
}

// NewDecoder creates a new JPEG-LS near-lossless decoder
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Decode decodes JPEG-LS near-lossless compressed data
func Decode(jpegLSData []byte) ([]byte, int, int, int, int, int, error) {
	decoder := NewDecoder()
	return decoder.decode(jpegLSData)
}

// decode performs the actual decoding
func (dec *Decoder) decode(jpegLSData []byte) ([]byte, int, int, int, int, int, error) {
	reader := standard.NewReader(jpegLSData)

	// Read SOI marker
	marker, err := reader.ReadMarker()
	if err != nil {
		return nil, 0, 0, 0, 0, 0, err
	}
	if marker != standard.MarkerSOI {
		return nil, 0, 0, 0, 0, 0, standard.ErrInvalidSOI
	}

	// Parse segments
	for {
		marker, err := reader.ReadMarker()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, 0, 0, 0, 0, 0, err
		}

		switch marker {
		case 0xFFF7: // SOF55 (JPEG-LS)
			if err := dec.parseSOF55(reader); err != nil {
				return nil, 0, 0, 0, 0, 0, err
			}

		case 0xFFF8: // LSE (JPEG-LS parameters)
			if err := dec.parseLSE(reader); err != nil {
				return nil, 0, 0, 0, 0, 0, err
			}

		case standard.MarkerSOS:
			if err := dec.parseSOS(reader); err != nil {
				return nil, 0, 0, 0, 0, 0, err
			}

			// Decode scan data
			pixelData, err := dec.decodeScan(reader)
			if err != nil {
				return nil, 0, 0, 0, 0, 0, err
			}

			return pixelData, dec.width, dec.height, dec.components, dec.bitDepth, dec.near, nil

		case standard.MarkerEOI:
			return nil, 0, 0, 0, 0, 0, fmt.Errorf("unexpected EOI before scan data")

		default:
			if standard.HasLength(marker) {
				_, err := reader.ReadSegment()
				if err != nil {
					return nil, 0, 0, 0, 0, 0, err
				}
			}
		}
	}

	return nil, 0, 0, 0, 0, 0, fmt.Errorf("incomplete JPEG-LS data")
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
	dec.traits.Reset = 64

	return nil
}

// parseLSE parses the LSE segment
func (dec *Decoder) parseLSE(reader *standard.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 1 {
		return fmt.Errorf("invalid LSE segment")
	}

	id := data[0]
	if id == 1 && len(data) >= 11 { // CharLS format: 11 bytes = ID + MAXVAL + T1 + T2 + T3 + RESET
		// Read MAXVAL
		maxVal := int(data[1])<<8 | int(data[2])
		if maxVal > 0 {
			dec.maxVal = maxVal
		}

		dec.t1 = int(data[3])<<8 | int(data[4])
		dec.t2 = int(data[5])<<8 | int(data[6])
		dec.t3 = int(data[7])<<8 | int(data[8])

		// RESET interval
		reset := int(data[9])<<8 | int(data[10])
		if reset == 0 {
			reset = 64
		}
		dec.traits.Reset = reset
	}

	return nil
}

// applyCodingParameters recomputes derived parameters once NEAR is known.
func (dec *Decoder) applyCodingParameters() {
	reset := 64
	if dec.traits.Reset > 0 {
		reset = dec.traits.Reset
	}
	params := lossless.ComputeCodingParameters(dec.maxVal, dec.near, reset)
	if dec.t1 > 0 {
		params.T1 = dec.t1
	}
	if dec.t2 > 0 {
		params.T2 = dec.t2
	}
	if dec.t3 > 0 {
		params.T3 = dec.t3
	}

	dec.traits = lossless.NewTraits(dec.maxVal, dec.near, params.Reset)
	dec.quantizer = lossless.NewGradientQuantizer(params.T1, params.T2, params.T3, dec.near)
	dec.contextTable = lossless.NewContextTable(dec.maxVal, dec.near, params.Reset)
	dec.runModeScanner = lossless.NewRunModeScanner(dec.traits)
}

// parseSOS parses the SOS segment and extracts NEAR parameter
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

	// Extract NEAR parameter (at position len-3)
	dec.near = int(data[len(data)-3])
	dec.interleave = int(data[len(data)-2])
	if dec.components == 1 && dec.interleave != 0 {
		return fmt.Errorf("invalid JPEG-LS interleave mode %d for grayscale scan", dec.interleave)
	}
	if dec.components > 1 && dec.interleave != 2 {
		return fmt.Errorf("unsupported JPEG-LS interleave mode %d for RGB scan", dec.interleave)
	}

	// Compute quantization parameters and contexts using NEAR + LSE thresholds
	dec.applyCodingParameters()

	return nil
}

// decodeScan decodes the scan data
func (dec *Decoder) decodeScan(reader *standard.Reader) ([]byte, error) {
	// Read scan data
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
			} else if b2 == 0xD9 {
				// EOI - end of image
				break
			} else {
				// Other markers
				break
			}
		} else {
			scanData.WriteByte(b)
		}
	}

	// Create Golomb reader
	gr := lossless.NewGolombReader(bytes.NewReader(scanData.Bytes()))

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

	return dec.integersToPixels(pixels), nil
}

func (dec *Decoder) decodeSampleInterleaved(gr *lossless.GolombReader, pixels []int) error {
	var previousLineFirst, previousPreviousLineFirst [3]int
	for y := 0; y < dec.height; y++ {
		x := 0
		for x < dec.width {
			var ra, rb, rc, rd [3]int
			var q1, q2, q3 [3]int
			var qs [3]int
			for comp := 0; comp < dec.components; comp++ {
				ra[comp], rb[comp], rc[comp], rd[comp] = dec.sampleNeighbors(pixels, x, y, comp, previousLineFirst[comp], previousPreviousLineFirst[comp])
				q1[comp], q2[comp], q3[comp] = dec.quantizer.ComputeContext(ra[comp], rb[comp], rc[comp], rd[comp])
				qs[comp] = lossless.ComputeContextID(q1[comp], q2[comp], q3[comp])
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
				if err := dec.decodeRegularSample(gr, pixels, x, y, comp, qs[comp], q1[comp], q2[comp], q3[comp], ra[comp], rb[comp], rc[comp]); err != nil {
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

func (dec *Decoder) decodeRegularSample(gr *lossless.GolombReader, pixels []int, x, y, comp, qs, q1, q2, q3, ra, rb, rc int) error {
	sign := lossless.BitwiseSign(qs)
	ctx := dec.contextTable.GetContext(q1, q2, q3)
	k := ctx.ComputeGolombParameter()
	predictedValue := dec.correctPrediction(lossless.Predict(ra, rb, rc) + lossless.ApplySign(ctx.GetPredictionCorrection(), sign))
	mappedError, err := gr.DecodeValue(k, dec.traits.Limit, dec.traits.Qbpp)
	if err != nil {
		return fmt.Errorf("decode regular mode error at x=%d y=%d comp=%d: %w", x, y, comp, err)
	}
	errorValue := lossless.UnmapErrorValue(mappedError)
	if k == 0 {
		errorValue ^= ctx.GetErrorCorrection(k, dec.near)
	}
	ctx.UpdateContext(errorValue, dec.near, dec.traits.Reset)
	idx := (y*dec.width+x)*dec.components + comp
	pixels[idx] = dec.traits.ComputeReconstructedSample(predictedValue, lossless.ApplySign(errorValue, sign))
	return nil
}

func (dec *Decoder) decodeSampleRunMode(gr *lossless.GolombReader, pixels []int, x, y int, previousLineFirst, previousPreviousLineFirst [3]int) (int, error) {
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
		pixels[idx+comp] = dec.traits.ComputeReconstructedSample(above, errorValue*runmode.Sign(above-left))
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
		aboveX := x + 1
		if aboveX >= dec.width {
			aboveX = dec.width - 1
		}
		aboveRight = pixels[((y-1)*dec.width+aboveX)*dec.components+comp]
	}
	left = pixels[(y*dec.width+x-1)*dec.components+comp]
	if y == 0 {
		aboveRight = above
	}
	return
}

// decodeComponent decodes a single component
func (dec *Decoder) decodeComponent(gr *lossless.GolombReader, pixels []int, comp int) error {
	stride := dec.components
	offset := comp

	prevFirstPrev := 0 // previous line first pixel
	prevNeg1 := 0      // previous_line[-1]

	for y := 0; y < dec.height; y++ {
		// Reset run index at start of each line (JPEG-LS standard)
		dec.runModeScanner.ResetLine()

		x := 0
		for x < dec.width {
			idx := (y*dec.width+x)*stride + offset

			// Get neighbors
			var a, b, c, d int
			if x == 0 {
				a = prevFirstPrev
				b = 0
				if y > 0 {
					b = prevFirstPrev
				}
				c = prevNeg1
				if y > 0 && dec.width > 1 {
					rdIdx := ((y-1)*dec.width+(x+1))*stride + offset
					if rdIdx < len(pixels) {
						d = pixels[rdIdx]
					} else {
						d = b
					}
				} else {
					d = b
				}
			} else {
				a, b, c, d = dec.getNeighbors(pixels, x, y, comp)
			}

			// Compute context on ORIGINAL values (before quantization)
			// This ensures thresholds work correctly
			q1, q2, q3 := dec.quantizer.ComputeContext(a, b, c, d)

			// Compute context ID to check for RUN mode
			qs := lossless.ComputeContextID(q1, q2, q3)

			// Check if we should use RUN mode (qs == 0 means flat region)
			if qs != 0 {
				// Regular mode - EXACTLY following CharLS do_regular (decoder)
				// Reference: CharLS scan.h line 305-334

				// const int32_t sign{bit_wise_sign(qs)};
				sign := lossless.BitwiseSign(qs)

				// context_regular_mode& context{contexts_[apply_sign(qs, sign)]};
				ctx := dec.contextTable.GetContext(q1, q2, q3)

				// const int32_t k{context.get_golomb_coding_parameter()};
				k := ctx.ComputeGolombParameter()

				// predicted = get_predicted_value(ra, rb, rc)
				prediction := lossless.Predict(a, b, c)

				// const int32_t predicted_value{traits_.correct_prediction(predicted + apply_sign(context.c(), sign))};
				predictedValue := dec.correctPrediction(prediction + lossless.ApplySign(ctx.GetPredictionCorrection(), sign))

				// Decode mapped error
				mappedError, err := gr.DecodeValue(k, dec.traits.Limit, dec.traits.Qbpp)
				if err != nil {
					return fmt.Errorf("decode regular mode error at x=%d y=%d comp=%d: %w", x, y, comp, err)
				}

				// int32_t error_value = unmap_error_value(decode_value(...));
				errorValue := lossless.UnmapErrorValue(mappedError)

				// if (k == 0) error_value = error_value ^ context.get_error_correction(traits_.near_lossless);
				if k == 0 {
					errorValue ^= ctx.GetErrorCorrection(k, dec.near)
				}

				// context.update_variables_and_bias(error_value, traits_.near_lossless, traits_.reset_threshold);
				ctx.UpdateContext(errorValue, dec.near, dec.traits.Reset)

				// return traits_.compute_reconstructed_sample(predicted_value, apply_sign(error_value, sign));
				sample := dec.traits.ComputeReconstructedSample(predictedValue, lossless.ApplySign(errorValue, sign))

				pixels[idx] = sample

				x++
			} else {
				// RUN mode (qs == 0, flat region)
				pixelsProcessed, err := dec.doRunMode(gr, pixels, x, y, comp, a)
				if err != nil {
					return fmt.Errorf("decode run mode error at x=%d y=%d comp=%d: %w", x, y, comp, err)
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

// correctPrediction delegates to Traits.CorrectPrediction
// CharLS: default_traits.h correct_prediction() line 83-89
func (dec *Decoder) correctPrediction(predicted int) int {
	return dec.traits.CorrectPrediction(predicted)
}

// getNeighbors gets neighboring pixels following CharLS edge handling
// CharLS initializes edge pixels as:
//
//	current_line_[-1] = previous_line_[0]  (left edge = pixel above)
//	previous_line_[width_] = previous_line_[width_ - 1] (right padding = rightmost top pixel)
func (dec *Decoder) getNeighbors(pixels []int, x, y, comp int) (int, int, int, int) {
	stride := dec.components
	offset := comp

	a, b, c, d := 0, 0, 0, 0

	// b: pixel above current position
	if y > 0 {
		idx := ((y-1)*dec.width+x)*stride + offset
		b = pixels[idx]
	}

	// a: pixel to the left
	// CharLS: when x=0, current_line_[-1] = previous_line_[0], so a = b
	if x > 0 {
		idx := (y*dec.width+(x-1))*stride + offset
		a = pixels[idx]
	} else if y > 0 {
		// Left edge: a = b (pixel above)
		a = b
	}

	// c: pixel diagonally above-left
	if x > 0 && y > 0 {
		idx := ((y-1)*dec.width+(x-1))*stride + offset
		c = pixels[idx]
	}

	// d: pixel above-right
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

// doRunMode handles decoding in run mode (when qs == 0) for near-lossless
func (dec *Decoder) doRunMode(gr *lossless.GolombReader, pixels []int, x, y, comp int, ra int) (int, error) {
	stride := dec.components
	offset := comp

	startIdx := y*dec.width + x
	remainingInLine := dec.width - x

	// Decode run length using RunModeScanner
	runLength, err := dec.runModeScanner.DecodeRunLength(gr, remainingInLine)
	if err != nil {
		return 0, err
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

	// Handle run interruption
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
		return 0, err
	}

	// Store reconstructed value
	if interruptIdx < len(pixels) {
		pixels[interruptIdx] = reconstructed
	}

	dec.runModeScanner.DecRunIndex()

	return runLength + 1, nil
}

// decodeRunInterruptionPixel decodes the pixel that interrupts a run
func (dec *Decoder) decodeRunInterruptionPixel(gr *lossless.GolombReader, ra, rb int) (int, error) {
	if runmode.Abs(ra-rb) <= dec.near {
		// Use run mode context 1
		quantizedError, err := dec.runModeScanner.DecodeRunInterruption(gr, dec.runModeScanner.RunModeContexts[1])
		if err != nil {
			return 0, err
		}

		// ComputeReconstructedSample will dequantize internally
		reconstructed := dec.traits.ComputeReconstructedSample(ra, quantizedError)
		return reconstructed, nil
	}

	// Use run mode context 0
	quantizedError, err := dec.runModeScanner.DecodeRunInterruption(gr, dec.runModeScanner.RunModeContexts[0])
	if err != nil {
		return 0, err
	}

	// ComputeReconstructedSample will dequantize internally
	reconstructed := dec.traits.ComputeReconstructedSample(rb, quantizedError*runmode.Sign(rb-ra))
	return reconstructed, nil
}
