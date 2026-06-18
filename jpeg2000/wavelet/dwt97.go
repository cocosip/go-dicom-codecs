// Package wavelet implements discrete wavelet transforms used by JPEG 2000.
package wavelet

// DWT97 implements the 9/7 irreversible wavelet transform
// Used for lossy JPEG 2000 compression
// Reference: ISO/IEC 15444-1:2019 Annex F
// Direct translation from OpenJPEG to ensure 100% compatibility

// 9/7 filter coefficients (Cohen-Daubechies-Feauveau)
// Direct translation from OpenJPEG's dwt.c to ensure 100% compatibility
const (
	// From OpenJPEG table F.4 from the standard
	alpha97 = -1.586134342 // opj_dwt_alpha
	beta97  = -0.052980118 // opj_dwt_beta
	gamma97 = 0.882911075  // opj_dwt_gamma
	delta97 = 0.443506852  // opj_dwt_delta

	// Normalization factors
	K97       = 1.230174105 // opj_K
	invK97    = 0.812893066 // opj_invK = 1.0 / 1.230174105
	twoInvK97 = 1.625732422 // OpenJPEG BUG_WEIRD_TWO_INVK decode scaling
)

// Forward97_1D performs the forward 9/7 wavelet transform on a 1D signal
// Separates the signal into low-pass (L) and high-pass (H) subbands
// Input: data (will be modified in-place)
// Output: first half = L (approximation), second half = H (detail)
//
// Note: Uses floating-point arithmetic (irreversible/lossy)
func Forward97_1D(data []float64) {
	Forward97_1DWithParity(data, true)
}

// Forward97_1DWithParity performs the forward 9/7 wavelet transform on a 1D signal.
// even=true means low-pass starts at even indices (cas=0). even=false means odd (cas=1).
//
// This is a direct translation of OpenJPEG's opj_dwt_encode_1_real() to ensure 100% compatibility.
func Forward97_1DWithParity(data []float64, even bool) {
	floatData := ConvertFloat64ToFloat32(data)
	Forward97_1DFloat32WithParity(floatData, even)
	for i, v := range floatData {
		data[i] = float64(v)
	}
}

// Forward97_1DFloat32WithParity performs OpenJPEG's forward 9/7 transform on float32 data.
func Forward97_1DFloat32WithParity(data []float32, even bool) {
	width := len(data)
	if width <= 1 {
		return
	}

	// Calculate sn (low-pass count) and dn (high-pass count)
	var sn, dn int32
	if even {
		sn = int32((width + 1) >> 1)
		dn = int32(width) - sn
	} else {
		sn = int32(width >> 1)
		dn = int32(width) - sn
	}

	// Work directly on interleaved data, matching OpenJPEG's approach
	var a, b int32
	if even {
		a = 0 // Low-pass at even indices
		b = 1 // High-pass at odd indices
	} else {
		a = 1 // Low-pass at odd indices
		b = 0 // High-pass at even indices
	}

	// Apply lifting steps directly on interleaved data
	// Step 1: alpha (predict 1)
	encodeStep2_97Float32(data, a, b+1, dn, min32(dn, sn-b), alpha97)

	// Step 2: beta (update 1)
	encodeStep2_97Float32(data, b, a+1, sn, min32(sn, dn-a), beta97)

	// Step 3: gamma (predict 2)
	encodeStep2_97Float32(data, a, b+1, dn, min32(dn, sn-b), gamma97)

	// Step 4: delta (update 2)
	encodeStep2_97Float32(data, b, a+1, sn, min32(sn, dn-a), delta97)

	// Normalization (scale)
	if a == 0 {
		encodeStep1Combined97Float32(data, sn, dn, invK97, K97)
	} else {
		encodeStep1Combined97Float32(data, dn, sn, K97, invK97)
	}

	// Deinterleave to [L | H] format
	deinterleaveH97Float32(data, dn, sn, even)
}

// encodeStep2_97 implements OpenJPEG's opj_dwt_encode_step2 for 9/7 wavelet
// Direct translation to match exact behavior
func encodeStep2_97(data []float64, flStart, fwStart int32, end, m int32, c float64) {
	imax := min32(end, m)

	if imax > 0 {
		// First iteration (special case)
		fw := fwStart
		fl := flStart
		data[fw-1] += (data[fl] + data[fw]) * c
		fw += 2

		// Main loop
		for i := int32(1); i < imax; i++ {
			data[fw-1] += (data[fw-2] + data[fw]) * c
			fw += 2
		}
	}

	// Boundary case
	if m < end {
		fw := fwStart + 2*m
		data[fw-1] += (2 * data[fw-2]) * c
	}
}

func encodeStep2_97Float32(data []float32, flStart, fwStart int32, end, m int32, c float64) {
	imax := min32(end, m)
	c32 := float32(c)

	if imax > 0 {
		fw := fwStart
		fl := flStart
		data[fw-1] += (data[fl] + data[fw]) * c32
		fw += 2

		for i := int32(1); i < imax; i++ {
			data[fw-1] += (data[fw-2] + data[fw]) * c32
			fw += 2
		}
	}

	if m < end {
		fw := fwStart + 2*m
		data[fw-1] += (2 * data[fw-2]) * c32
	}
}

// encodeStep1Combined97 implements OpenJPEG's opj_dwt_encode_step1_combined
// Applies normalization factors to interleaved data
func encodeStep1Combined97(data []float64, itersC1, itersC2 int32, c1, c2 float64) {
	itersCommon := min32(itersC1, itersC2)

	var i int32
	fw := int32(0)
	for i = 0; i < itersCommon; i++ {
		data[fw] *= c1
		data[fw+1] *= c2
		fw += 2
	}

	if i < itersC1 {
		data[fw] *= c1
	} else if i < itersC2 {
		data[fw+1] *= c2
	}
}

func encodeStep1Combined97Float32(data []float32, itersC1, itersC2 int32, c1, c2 float64) {
	itersCommon := min32(itersC1, itersC2)
	c1f := float32(c1)
	c2f := float32(c2)

	var i int32
	fw := int32(0)
	for i = 0; i < itersCommon; i++ {
		data[fw] *= c1f
		data[fw+1] *= c2f
		fw += 2
	}

	if i < itersC1 {
		data[fw] *= c1f
	} else if i < itersC2 {
		data[fw+1] *= c2f
	}
}

// deinterleaveH97 separates interleaved data into [low | high] format
func deinterleaveH97(data []float64, dn, sn int32, even bool) {
	width := int(dn + sn)
	tmp := make([]float64, width)

	if even {
		// cas == 0: even indices are low-pass
		for i := int32(0); i < sn; i++ {
			tmp[i] = data[2*i]
		}
		for i := int32(0); i < dn; i++ {
			tmp[sn+i] = data[2*i+1]
		}
	} else {
		// cas == 1: odd indices are low-pass
		for i := int32(0); i < sn; i++ {
			tmp[i] = data[2*i+1]
		}
		for i := int32(0); i < dn; i++ {
			tmp[sn+i] = data[2*i]
		}
	}

	copy(data, tmp)
}

func deinterleaveH97Float32(data []float32, dn, sn int32, even bool) {
	width := int(dn + sn)
	tmp := make([]float32, width)

	if even {
		for i := int32(0); i < sn; i++ {
			tmp[i] = data[2*i]
		}
		for i := int32(0); i < dn; i++ {
			tmp[sn+i] = data[2*i+1]
		}
	} else {
		for i := int32(0); i < sn; i++ {
			tmp[i] = data[2*i+1]
		}
		for i := int32(0); i < dn; i++ {
			tmp[sn+i] = data[2*i]
		}
	}

	copy(data, tmp)
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

// Inverse97_1D performs the inverse 9/7 wavelet transform on a 1D signal
// Reconstructs the original signal from L and H subbands
// Input: data with first half = L, second half = H (will be modified in-place)
// Output: reconstructed signal
func Inverse97_1D(data []float64) {
	Inverse97_1DWithParity(data, true)
}

// Inverse97_1DWithParity performs the inverse 9/7 wavelet transform on a 1D signal.
// even=true means low-pass starts at even indices (cas=0). even=false means odd (cas=1).
//
// This reverses the OpenJPEG forward transform to ensure 100% compatibility.
func Inverse97_1DWithParity(data []float64, even bool) {
	floatData := ConvertFloat64ToFloat32(data)
	Inverse97_1DOpenJPEGWithParity(floatData, even)
	for i, v := range floatData {
		data[i] = float64(v)
	}
}

// Inverse97_1DOpenJPEGWithParity performs OpenJPEG's irreversible 9/7 inverse
// decode path. It uses float32 arithmetic, opj_K for low-pass samples, and
// OpenJPEG's historic two_invK high-pass scaling.
func Inverse97_1DOpenJPEGWithParity(data []float32, even bool) {
	width := len(data)
	if width <= 1 {
		return
	}

	var sn, dn int32
	if even {
		sn = int32((width + 1) >> 1)
		dn = int32(width) - sn
	} else {
		sn = int32(width >> 1)
		dn = int32(width) - sn
	}

	var a, b int32
	if even {
		a = 0
		b = 1
	} else {
		a = 1
		b = 0
	}

	interleaveH97Float32(data, dn, sn, even)
	decodeStep1OpenJPEG97Float32(data, a, sn, float32(K97))
	decodeStep1OpenJPEG97Float32(data, b, dn, float32(twoInvK97))
	decodeStep2OpenJPEG97Float32(data, b, a+1, sn, min32(sn, dn-a), float32(-delta97))
	decodeStep2OpenJPEG97Float32(data, a, b+1, dn, min32(dn, sn-b), float32(-gamma97))
	decodeStep2OpenJPEG97Float32(data, b, a+1, sn, min32(sn, dn-a), float32(-beta97))
	decodeStep2OpenJPEG97Float32(data, a, b+1, dn, min32(dn, sn-b), float32(-alpha97))
}

// interleaveH97 converts [low | high] format back to interleaved
func interleaveH97(data []float64, dn, sn int32, even bool) {
	width := int(dn + sn)
	tmp := make([]float64, width)

	if even {
		// cas == 0: even indices are low-pass, odd are high-pass
		for i := int32(0); i < sn; i++ {
			tmp[2*i] = data[i]
		}
		for i := int32(0); i < dn; i++ {
			tmp[2*i+1] = data[sn+i]
		}
	} else {
		// cas == 1: odd indices are low-pass, even are high-pass
		for i := int32(0); i < sn; i++ {
			tmp[2*i+1] = data[i]
		}
		for i := int32(0); i < dn; i++ {
			tmp[2*i] = data[sn+i]
		}
	}

	copy(data, tmp)
}

func interleaveH97Float32(data []float32, dn, sn int32, even bool) {
	width := int(dn + sn)
	tmp := make([]float32, width)

	if even {
		for i := int32(0); i < sn; i++ {
			tmp[2*i] = data[i]
		}
		for i := int32(0); i < dn; i++ {
			tmp[2*i+1] = data[sn+i]
		}
	} else {
		for i := int32(0); i < sn; i++ {
			tmp[2*i+1] = data[i]
		}
		for i := int32(0); i < dn; i++ {
			tmp[2*i] = data[sn+i]
		}
	}

	copy(data, tmp)
}

// Forward97_2D performs the forward 9/7 wavelet transform on a 2D image
func Forward97_2D(data []float64, width, height, stride int) {
	Forward97_2DWithParity(data, width, height, stride, true, true)
}

// Forward97_2DWithParity performs the forward 9/7 wavelet transform on a 2D image
// IMPORTANT: OpenJPEG does VERTICAL (columns) first, then HORIZONTAL (rows).
func Forward97_2DWithParity(data []float64, width, height, stride int, evenRow, evenCol bool) {
	floatData := ConvertFloat64ToFloat32(data)
	Forward97_2DFloat32WithParity(floatData, width, height, stride, evenRow, evenCol)
	for i, v := range floatData {
		data[i] = float64(v)
	}
}

func decodeStep1OpenJPEG97Float32(data []float32, start, end int32, c float32) {
	for i := int32(0); i < end; i++ {
		data[start+2*i] *= c
	}
}

func decodeStep2OpenJPEG97Float32(data []float32, flStart, fwStart, end, m int32, c float32) {
	imax := min32(end, m)
	if imax > 0 {
		fw := fwStart
		fl := flStart
		data[fw-1] += (data[fl] + data[fw]) * c
		fw += 2

		for i := int32(1); i < imax; i++ {
			data[fw-1] += (data[fw-2] + data[fw]) * c
			fw += 2
		}
	}

	if m < end {
		fw := fwStart + 2*m
		data[fw-1] += (2 * data[fw-2]) * c
	}
}

// Forward97_2DFloat32WithParity performs OpenJPEG's forward 9/7 transform on float32 data.
func Forward97_2DFloat32WithParity(data []float32, width, height, stride int, evenRow, evenCol bool) {
	if width <= 1 && height <= 1 {
		return
	}

	// Transform columns (VERTICAL pass - OpenJPEG does this FIRST)
	if height > 1 {
		col := make([]float32, height)
		for x := 0; x < width; x++ {
			for y := 0; y < height; y++ {
				col[y] = data[y*stride+x]
			}
			Forward97_1DFloat32WithParity(col, evenCol)
			for y := 0; y < height; y++ {
				data[y*stride+x] = col[y]
			}
		}
	}

	// Transform rows (HORIZONTAL pass - OpenJPEG does this SECOND)
	if width > 1 {
		row := make([]float32, width)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				row[x] = data[y*stride+x]
			}
			Forward97_1DFloat32WithParity(row, evenRow)
			for x := 0; x < width; x++ {
				data[y*stride+x] = row[x]
			}
		}
	}
}

// Inverse97_2D performs the inverse 9/7 wavelet transform on a 2D image
func Inverse97_2D(data []float64, width, height, stride int) {
	Inverse97_2DWithParity(data, width, height, stride, true, true)
}

// Inverse97_2DWithParity performs the inverse 9/7 wavelet transform on a 2D image
// IMPORTANT: Inverse order - HORIZONTAL (rows) first, then VERTICAL (columns).
func Inverse97_2DWithParity(data []float64, width, height, stride int, evenRow, evenCol bool) {
	floatData := ConvertFloat64ToFloat32(data)
	Inverse97_2DOpenJPEGWithParity(floatData, width, height, stride, evenRow, evenCol)
	for i, v := range floatData {
		data[i] = float64(v)
	}
}

// ForwardMultilevel97 performs multilevel 9/7 wavelet decomposition
func ForwardMultilevel97(data []float64, width, height, levels int) {
	ForwardMultilevel97WithParity(data, width, height, levels, 0, 0)
}

// ForwardMultilevel97WithParity performs multilevel 9/7 wavelet decomposition with origin parity
func ForwardMultilevel97WithParity(data []float64, width, height, levels int, x0, y0 int) {
	floatData := ConvertFloat64ToFloat32(data)
	ForwardMultilevel97Float32WithParity(floatData, width, height, levels, x0, y0)
	for i, v := range floatData {
		data[i] = float64(v)
	}
}

// Inverse97_2DOpenJPEGWithParity matches OpenJPEG's opj_dwt_decode_tile_97
// order: horizontal rows first, then vertical columns.
func Inverse97_2DOpenJPEGWithParity(data []float32, width, height, stride int, evenRow, evenCol bool) {
	if width <= 1 && height <= 1 {
		return
	}

	if width > 1 {
		row := make([]float32, width)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				row[x] = data[y*stride+x]
			}
			Inverse97_1DOpenJPEGWithParity(row, evenRow)
			for x := 0; x < width; x++ {
				data[y*stride+x] = row[x]
			}
		}
	}

	if height > 1 {
		col := make([]float32, height)
		for x := 0; x < width; x++ {
			for y := 0; y < height; y++ {
				col[y] = data[y*stride+x]
			}
			Inverse97_1DOpenJPEGWithParity(col, evenCol)
			for y := 0; y < height; y++ {
				data[y*stride+x] = col[y]
			}
		}
	}
}

// ForwardMultilevel97Float32WithParity performs OpenJPEG's multilevel 9/7 decomposition.
func ForwardMultilevel97Float32WithParity(data []float32, width, height, levels int, x0, y0 int) {
	originalStride := width
	curWidth := width
	curHeight := height
	curX0 := x0
	curY0 := y0

	for level := 0; level < levels; level++ {
		if curWidth <= 1 && curHeight <= 1 {
			break
		}

		evenRow := isEven(curX0)
		evenCol := isEven(curY0)

		Forward97_2DFloat32WithParity(data, curWidth, curHeight, originalStride, evenRow, evenCol)

		curWidth, curHeight, curX0, curY0 = nextLowpassWindow(curWidth, curHeight, curX0, curY0)
	}
}

// InverseMultilevel97 performs multilevel 9/7 wavelet reconstruction
func InverseMultilevel97(data []float64, width, height, levels int) {
	InverseMultilevel97WithParity(data, width, height, levels, 0, 0)
}

// InverseMultilevel97WithParity performs multilevel 9/7 wavelet reconstruction with origin parity
func InverseMultilevel97WithParity(data []float64, width, height, levels int, x0, y0 int) {
	floatData := ConvertFloat64ToFloat32(data)
	InverseMultilevel97OpenJPEGWithParity(floatData, width, height, levels, x0, y0)
	for i, v := range floatData {
		data[i] = float64(v)
	}
}

// InverseMultilevel97OpenJPEGWithParity reconstructs OpenJPEG irreversible
// decoder samples from dequantized float32 subbands.
func InverseMultilevel97OpenJPEGWithParity(data []float32, width, height, levels int, x0, y0 int) {
	originalStride := width

	levelWidths := make([]int, levels+1)
	levelHeights := make([]int, levels+1)
	levelX0 := make([]int, levels+1)
	levelY0 := make([]int, levels+1)
	levelWidths[0] = width
	levelHeights[0] = height
	levelX0[0] = x0
	levelY0[0] = y0

	for i := 1; i <= levels; i++ {
		levelWidths[i], levelHeights[i], levelX0[i], levelY0[i] = nextLowpassWindow(
			levelWidths[i-1], levelHeights[i-1], levelX0[i-1], levelY0[i-1],
		)
	}

	for level := levels - 1; level >= 0; level-- {
		curWidth := levelWidths[level]
		curHeight := levelHeights[level]
		evenRow := isEven(levelX0[level])
		evenCol := isEven(levelY0[level])

		Inverse97_2DOpenJPEGWithParity(data, curWidth, curHeight, originalStride, evenRow, evenCol)
	}
}

// ConvertInt32ToFloat64 converts a slice of int32 to float64 values.
func ConvertInt32ToFloat64(data []int32) []float64 {
	result := make([]float64, len(data))
	for i, v := range data {
		result[i] = float64(v)
	}
	return result
}

// ConvertInt32ToFloat32 converts a slice of int32 to OpenJPEG's irreversible sample type.
func ConvertInt32ToFloat32(data []int32) []float32 {
	result := make([]float32, len(data))
	for i, v := range data {
		result[i] = float32(v)
	}
	return result
}

// ConvertFloat32ToInt32OpenJPEG rounds like OpenJPEG's opj_lrintf-based
// irreversible decode finalization.
func ConvertFloat32ToInt32OpenJPEG(data []float32) []int32 {
	result := make([]int32, len(data))
	for i, v := range data {
		result[i] = int32(roundFloat32ToNearestEven(v))
	}
	return result
}

func roundFloat32ToNearestEven(v float32) int64 {
	i := int64(v)
	frac := float64(v - float32(i))
	if frac < 0 {
		frac = -frac
	}
	if frac > 0.5 {
		if v >= 0 {
			return i + 1
		}
		return i - 1
	}
	if frac < 0.5 {
		return i
	}
	if i%2 == 0 {
		return i
	}
	if v >= 0 {
		return i + 1
	}
	return i - 1
}

// ConvertFloat64ToFloat32 converts a float64 slice to OpenJPEG's irreversible sample type.
func ConvertFloat64ToFloat32(data []float64) []float32 {
	result := make([]float32, len(data))
	for i, v := range data {
		result[i] = float32(v)
	}
	return result
}

// ConvertFloat64ToInt32 converts a slice of float64 to int32 with rounding.
func ConvertFloat64ToInt32(data []float64) []int32 {
	result := make([]int32, len(data))
	for i, v := range data {
		// Round to nearest integer
		if v >= 0 {
			result[i] = int32(v + 0.5)
		} else {
			result[i] = int32(v - 0.5)
		}
	}
	return result
}
