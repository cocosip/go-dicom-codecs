package jpeg2000

import (
	"math"
	"math/bits"
)

// OpenJPEG 9-7 wavelet norms (opj_dwt_norms_real).
// These values are used to derive per-subband quantization step sizes.
var dwtNorms53 = [4][10]float64{
	{1.000, 1.500, 2.750, 5.375, 10.68, 21.34, 42.67, 85.33, 170.7, 341.3},
	{1.038, 1.592, 2.919, 5.703, 11.33, 22.64, 45.25, 90.48, 180.9, 0.0},
	{1.038, 1.592, 2.919, 5.703, 11.33, 22.64, 45.25, 90.48, 180.9, 0.0},
	{.7186, .9218, 1.586, 3.043, 6.019, 12.01, 24.00, 47.97, 95.93, 0.0},
}

var dwtNorms97 = [4][10]float64{
	{1.000, 1.965, 4.177, 8.403, 16.90, 33.84, 67.69, 135.3, 270.6, 540.9},
	{2.022, 3.989, 8.355, 17.04, 34.27, 68.63, 137.3, 274.6, 549.0, 0.0},
	{2.022, 3.989, 8.355, 17.04, 34.27, 68.63, 137.3, 274.6, 549.0, 0.0},
	{2.080, 3.865, 8.307, 17.18, 34.71, 69.59, 139.3, 278.6, 557.2, 0.0},
}

func dwtNorm53(level, orient int) float64 {
	if level < 0 {
		level = 0
	}
	if orient == 0 && level >= 10 {
		level = 9
	} else if orient > 0 && level >= 9 {
		level = 8
	}
	if orient < 0 || orient > 3 {
		return 1.0
	}
	return dwtNorms53[orient][level]
}

func dwtNorm97(level, orient int) float64 {
	if level < 0 {
		level = 0
	}
	if orient == 0 && level >= 10 {
		level = 9
	} else if orient > 0 && level >= 9 {
		level = 8
	}
	if orient < 0 || orient > 3 {
		return 1.0
	}
	return dwtNorms97[orient][level]
}

func qualityScale(quality int) float64 {
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}
	scale := math.Pow(2.0, (100.0-float64(quality))/12.5)
	if scale < 0.01 {
		scale = 0.01
	}
	return scale * 0.05
}

func subbandParams(idx, numLevels int) (orient, level int) {
	resno := 0
	if idx == 0 {
		resno = 0
		orient = 0
	} else {
		resno = (idx-1)/3 + 1
		orient = (idx-1)%3 + 1
	}
	level = numLevels - resno
	if level < 0 {
		level = 0
	}
	return orient, level
}

func calcOpenJPEGStepSizes97(numLevels int, scale float64) []float64 {
	if numLevels <= 0 {
		return []float64{scale}
	}
	numSubbands := 3*numLevels + 1
	steps := make([]float64, numSubbands)
	for idx := 0; idx < numSubbands; idx++ {
		orient, level := subbandParams(idx, numLevels)
		norm := dwtNorm97(level, orient)
		if norm <= 0 {
			steps[idx] = scale
		} else {
			steps[idx] = scale / norm
		}
	}
	return steps
}

func encodeQuantizationStep(stepSize float64, numbps int) uint16 {
	if stepSize <= 0 {
		return 0
	}
	fixed := int32(math.Floor(stepSize * 8192.0))
	if fixed <= 0 {
		fixed = 1
	}
	log2 := bits.Len32(uint32(fixed)) - 1
	p := log2 - 13
	n := 11 - log2
	mant := int32(0)
	if n < 0 {
		mant = fixed >> -n
	} else {
		mant = fixed << n
	}
	mant &= 0x7ff
	expn := numbps - p
	if expn < 0 {
		expn = 0
	}
	if expn > 0x1f {
		expn = 0x1f
	}
	return uint16((expn << 11) | int(mant))
}

func decodeQuantizationStepWithGain(encoded uint16, bitDepth, log2Gain int) float64 {
	expn := int((encoded >> 11) & 0x1f)
	mant := float64(encoded & 0x7ff)
	rb := bitDepth + log2Gain
	return math.Ldexp(1.0+mant/2048.0, rb-expn)
}

// OpenJPEGRuntimeQuantizationSteps mirrors tcd.c band->stepsize initialization.
// OpenJPEG writes QCD as mantissa/exponent, then decodes those encoded values
// back into OPJ_FLOAT32 band steps before T1 quantization.
func OpenJPEGRuntimeQuantizationSteps(encoded []uint16, numLevels, bitDepth int) []float64 {
	steps := make([]float64, len(encoded))
	for idx, step := range encoded {
		orient, _ := subbandParams(idx, numLevels)
		log2Gain := 0
		switch orient {
		case 3:
			log2Gain = 2
		case 1, 2:
			log2Gain = 1
		}
		steps[idx] = float64(float32(decodeQuantizationStepWithGain(step, bitDepth, log2Gain)))
	}
	return steps
}

// QuantizationParams holds quantization parameters for all subbands.
type QuantizationParams struct {
	// Quantization style
	// 0 = no quantization (lossless)
	// 1 = scalar derived (single base step size)
	// 2 = scalar expounded (explicit step size for each subband)
	Style int

	// Guard bits (0-7)
	GuardBits int

	// Step sizes for each subband.
	// Index order: LL, HL1, LH1, HH1, HL2, LH2, HH2, ..., HLn, LHn, HHn
	StepSizes []float64

	// Encoded step sizes (exponent + mantissa for each subband).
	// Format: bits 0-10 = mantissa (11 bits), bits 11-15 = exponent (5 bits)
	EncodedSteps []uint16
}

// CalculateQuantizationParams calculates quantization parameters based on quality.
// quality: 1-100 (1 = maximum compression, 100 = minimal quantization/near-lossless)
// numLevels: number of wavelet decomposition levels
// bitDepth: original bit depth of the image
func CalculateQuantizationParams(quality, numLevels, bitDepth int) *QuantizationParams {
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}

	// Calculate number of subbands: LL + 3 * numLevels (HL, LH, HH per level).
	numSubbands := 3*numLevels + 1

	params := &QuantizationParams{
		Style:        2, // Scalar expounded (explicit step size per subband)
		GuardBits:    2, // Standard guard bits
		StepSizes:    make([]float64, numSubbands),
		EncodedSteps: make([]uint16, numSubbands),
	}

	// Convert quality to a global scale and apply OpenJPEG norms for 9/7.
	scale := qualityScale(quality)
	params.StepSizes = calcOpenJPEGStepSizes97(numLevels, scale)

	// Encode step sizes using OpenJPEG's stepsize encoding.
	for i, stepSize := range params.StepSizes {
		params.EncodedSteps[i] = encodeQuantizationStep(stepSize, bitDepth)
	}

	return params
}

// CalculateOpenJPEGQuantizationParams mirrors OpenJPEG's opj_dwt_calc_explicit_stepsizes
// for the default irreversible 9/7 encoder path used by fo-dicom.Codecs.
func CalculateOpenJPEGQuantizationParams(numLevels, bitDepth int) *QuantizationParams {
	if numLevels < 0 {
		numLevels = 0
	}
	numSubbands := 3*numLevels + 1
	params := &QuantizationParams{
		Style:        2,
		GuardBits:    2,
		StepSizes:    make([]float64, numSubbands),
		EncodedSteps: make([]uint16, numSubbands),
	}

	for bandno := 0; bandno < numSubbands; bandno++ {
		orient, level := subbandParams(bandno, numLevels)
		norm := dwtNorm97(level, orient)
		stepsize := 1.0
		if norm > 0 {
			stepsize = 1.0 / norm
		}
		params.StepSizes[bandno] = stepsize
		params.EncodedSteps[bandno] = encodeQuantizationStep(stepsize, bitDepth)
	}

	return params
}

var openJPH97LowGain = []float64{1, 1.4021, 2.0304, 2.9012, 4.1153, 5.8245, 8.2388}
var openJPH97HighGain = []float64{1.4425, 1.9669, 2.8839, 4.1475, 5.8946, 8.3472}
var openJPH53LowBIBO = []float64{1, 1.5, 1.625, 1.6875, 1.6963, 1.7067, 1.7116}
var openJPH53HighBIBO = []float64{2, 2.5, 2.75, 2.8047, 2.8198, 2.8410}

// CalculateOpenJPHQuantizationParams mirrors OpenJPH param_qcd for HTJ2K.
func CalculateOpenJPHQuantizationParams(numLevels, bitDepth int, lossless bool) *QuantizationParams {
	return calculateOpenJPHQuantizationParams(numLevels, bitDepth, lossless, false)
}

// calculateOpenJPHQuantizationParams mirrors OpenJPH param_qcd for HTJ2K.
// OpenJPH reserves one extra magnitude bit when COD enables RCT.
func calculateOpenJPHQuantizationParams(numLevels, bitDepth int, lossless, usesRCT bool) *QuantizationParams {
	if numLevels < 0 {
		numLevels = 0
	}
	if numLevels > 6 {
		numLevels = 6
	}
	if lossless {
		precision := bitDepth
		if usesRCT {
			precision++
		}
		values := make([]float64, 0, 3*numLevels+1)
		appendExponent := func(v float64) { values = append(values, float64(precision)+math.Ceil(math.Log2(v*v))-1) }
		appendExponent(openJPH53LowBIBO[numLevels])
		for d := numLevels; d > 0; d-- {
			appendExponent(math.Sqrt(openJPH53LowBIBO[d] * openJPH53HighBIBO[d-1]))
			appendExponent(math.Sqrt(openJPH53LowBIBO[d] * openJPH53HighBIBO[d-1]))
			appendExponent(openJPH53HighBIBO[d-1])
		}
		encoded := make([]uint16, len(values))
		for i, v := range values {
			encoded[i] = uint16(int(v) << 3)
		}
		return &QuantizationParams{Style: 0, GuardBits: 1, EncodedSteps: encoded}
	}
	base := math.Ldexp(1, -min(16, bitDepth))
	steps := make([]uint16, 0, 3*numLevels+1)
	appendStep := func(delta float64) {
		exp := 0
		for delta < 1 {
			exp++
			delta *= 2
		}
		mant := int(math.Round(delta*2048)) - 2048
		if mant >= 2048 {
			mant = 2047
		}
		steps = append(steps, uint16(exp<<11|mant))
	}
	appendStep(base / (openJPH97LowGain[numLevels] * openJPH97LowGain[numLevels]))
	for d := numLevels; d > 0; d-- {
		appendStep(base / (openJPH97LowGain[d] * openJPH97HighGain[d-1]))
		appendStep(base / (openJPH97LowGain[d] * openJPH97HighGain[d-1]))
		appendStep(base / (openJPH97HighGain[d-1] * openJPH97HighGain[d-1]))
	}
	return &QuantizationParams{Style: 2, GuardBits: 1, EncodedSteps: steps}
}

// DecodeQuantizationStep decodes a JPEG 2000 quantization step from 16-bit encoded format.
// encoded: 16-bit value with bits 11-15 = exponent, bits 0-10 = mantissa
// bitDepth: original bit depth of the image
func DecodeQuantizationStep(encoded uint16, bitDepth int) float64 {
	return decodeQuantizationStepWithGain(encoded, bitDepth, 0)
}

// QuantizeCoefficients applies quantization to wavelet coefficients.
// coefficients: input wavelet coefficients
// stepSize: quantization step size
// Returns: quantized coefficients
func QuantizeCoefficients(coefficients []int32, stepSize float64) []int32 {
	if stepSize <= 0 {
		// No quantization
		return coefficients
	}

	quantized := make([]int32, len(coefficients))
	for i, coeff := range coefficients {
		// Quantize: round-to-even(coeff / stepSize) to match OpenJPEG's lrintf.
		quantized[i] = int32(math.RoundToEven(float64(coeff) / stepSize))
	}
	return quantized
}

// DequantizeCoefficients applies dequantization to coefficients.
// coefficients: quantized coefficients
// stepSize: quantization step size
// Returns: dequantized coefficients
func DequantizeCoefficients(coefficients []int32, stepSize float64) []int32 {
	if stepSize <= 0 {
		// No dequantization needed
		return coefficients
	}

	dequantized := make([]int32, len(coefficients))
	for i, coeff := range coefficients {
		// Dequantize: coeff * stepSize (matches quantization scaling after T1 inverse).
		dequantized[i] = int32(math.RoundToEven(float64(coeff) * stepSize))
	}
	return dequantized
}
