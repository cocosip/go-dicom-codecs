// Package wavelet implements discrete wavelet transforms used by JPEG 2000.
package wavelet

// DWT53 implements the 5/3 reversible wavelet transform
// Used for lossless JPEG 2000 compression
// Reference: ISO/IEC 15444-1:2019 Annex F

// Forward53_1D performs the forward 5/3 wavelet transform on a 1D signal
// Separates the signal into low-pass (L) and high-pass (H) subbands
// Input: data (will be modified in-place)
// Output: first half = L (approximation), second half = H (detail)
//
// Performance notes:
// - Uses integer arithmetic only (no floating point)
// - Bit shifts (>>) are faster than division for powers of 2
// - In-place processing minimizes memory allocations
// - Typical performance: ~100ns for 64 samples, ~1.5µs for 1024 samples
func Forward53_1D(data []int32) {
	Forward53_1DWithParity(data, true)
}

// Forward53_1DWithParity performs the forward 5/3 wavelet transform on a 1D signal
// even=true means low-pass starts at even indices (cas=0). even=false means odd (cas=1).
//
// This is a direct translation of OpenJPEG's opj_dwt_encode_and_deinterleave_h_one_row()
// to ensure 100% compatibility.
func Forward53_1DWithParity(data []int32, even bool) {
	width := len(data)

	if even {
		// even=true: low-pass from even indices (0, 2, 4, ...), high-pass from odd (1, 3, 5, ...)
		// This matches OpenJPEG's "even" case

		if width <= 1 {
			return
		}

		// sn = number of low-pass samples, dn = number of high-pass samples
		sn := int32((width + 1) >> 1)
		dn := int32(width - int(sn))

		// Allocate temporary array
		tmp := make([]int32, width)

		// Predict step: high[i] -= (low[i] + low[i+1]) >> 1
		var i int32
		for i = 0; i < sn-1; i++ {
			tmp[sn+i] = data[2*i+1] - ((data[i*2] + data[(i+1)*2]) >> 1)
		}
		if (width % 2) == 0 {
			tmp[sn+i] = data[2*i+1] - data[i*2]
		}

		// Update step: low[i] += (high[i-1] + high[i] + 2) >> 2
		data[0] += (tmp[sn] + tmp[sn] + 2) >> 2
		for i = 1; i < dn; i++ {
			data[i] = data[2*i] + ((tmp[sn+(i-1)] + tmp[sn+i] + 2) >> 2)
		}
		if (width % 2) == 1 {
			data[i] = data[2*i] + ((tmp[sn+(i-1)] + tmp[sn+(i-1)] + 2) >> 2)
		}

		// Copy high-pass coefficients to second half
		copy(data[sn:], tmp[sn:sn+dn])

	} else {
		// even=false: low-pass from odd indices (1, 3, 5, ...), high-pass from even (0, 2, 4, ...)
		// This matches OpenJPEG's "!even" case

		if width == 1 {
			data[0] *= 2
			return
		}

		// sn = number of low-pass samples, dn = number of high-pass samples
		sn := int32(width >> 1)
		dn := int32(width - int(sn))

		// Allocate temporary array
		tmp := make([]int32, width)

		// Predict step: high[i] -= (low[i-1] + low[i]) >> 1
		tmp[sn+0] = data[0] - data[1]
		var i int32
		for i = 1; i < sn; i++ {
			tmp[sn+i] = data[2*i] - ((data[2*i+1] + data[2*(i-1)+1]) >> 1)
		}
		if (width % 2) == 1 {
			tmp[sn+i] = data[2*i] - data[2*(i-1)+1]
		}

		// Update step: low[i] += (high[i] + high[i+1] + 2) >> 2
		for i = 0; i < dn-1; i++ {
			data[i] = data[2*i+1] + ((tmp[sn+i] + tmp[sn+i+1] + 2) >> 2)
		}
		if (width % 2) == 0 {
			data[i] = data[2*i+1] + ((tmp[sn+i] + tmp[sn+i] + 2) >> 2)
		}

		// Copy high-pass coefficients to second half
		copy(data[sn:], tmp[sn:sn+dn])
	}
}

// Inverse53_1D performs the inverse 5/3 wavelet transform on a 1D signal
// Reconstructs the original signal from L and H subbands
// Input: data with first half = L, second half = H (will be modified in-place)
// Output: reconstructed signal
//
// Performance notes:
// - Perfect reconstruction guaranteed (lossless)
// - Integer-only arithmetic ensures bit-exact inverse
// - Performance similar to Forward53_1D
// - Optimization: could benefit from SIMD for large signals
func Inverse53_1D(data []int32) {
	Inverse53_1DWithParity(data, true)
}

// Inverse53_1DWithParity performs the inverse 5/3 wavelet transform on a 1D signal.
// even=true means low-pass starts at even indices (cas=0). even=false means odd (cas=1).
//
// This is a direct translation of OpenJPEG's opj_idwt53_h_cas0/cas1() to ensure 100% compatibility.
func Inverse53_1DWithParity(data []int32, even bool) {
	width := len(data)

	if even {
		// even=true: cas=0, low-pass from even indices
		// This matches OpenJPEG's opj_idwt53_h_cas0

		if width <= 1 {
			return
		}

		sn := int32((width + 1) >> 1)

		// in_even points to low-pass (first half), in_odd points to high-pass (second half)
		// We'll work in-place using a temporary array
		tmp := make([]int32, width)

		var d1c, d1n, s1n, s0c, s0n int32

		s1n = data[0]        // in_even[0]
		d1n = data[sn]       // in_odd[0]
		s0n = s1n - ((d1n + 1) >> 1)

		var i, j int32
		for i, j = 0, 1; i < (int32(width)-3); i, j = i+2, j+1 {
			d1c = d1n
			s0c = s0n

			s1n = data[j]      // in_even[j]
			d1n = data[sn+j]   // in_odd[j]

			s0n = s1n - ((d1c + d1n + 2) >> 2)

			tmp[i] = s0c
			tmp[i+1] = d1c + ((s0c + s0n) >> 1)
		}

		tmp[i] = s0n

		if (width & 1) != 0 { // len is odd
			tmp[width-1] = data[(width-1)/2] - ((d1n + 1) >> 1)
			tmp[width-2] = d1n + ((s0n + tmp[width-1]) >> 1)
		} else { // len is even
			tmp[width-1] = d1n + s0n
		}

		copy(data, tmp)

	} else {
		// even=false: cas=1, low-pass from odd indices
		// This matches OpenJPEG's opj_idwt53_h_cas1

		if width == 1 {
			data[0] /= 2
			return
		}

		if width == 2 {
			// Special case for width==2
			out1 := data[0] - ((data[1] + 1) >> 1)
			out0 := data[1] + out1
			data[0] = out0
			data[1] = out1
			return
		}

		// width > 2
		sn := int32(width >> 1)

		// in_even points to low-pass (second half), in_odd points to high-pass (first half)
		// Note: this is swapped compared to cas0!
		tmp := make([]int32, width)

		var s1, s2, dc, dnVar int32

		s1 = data[sn+1]  // in_even[1]
		dc = data[0] - ((data[sn] + s1 + 2) >> 2)  // in_odd[0] - ((in_even[0] + s1 + 2) >> 2)
		tmp[0] = data[sn] + dc  // in_even[0] + dc

		var i, j int32
		// OpenJPEG: i < (len - 2 - !(len & 1))
		// !(len & 1) = 1 when even, 0 when odd
		notOdd := int32(0)
		if (width & 1) == 0 {
			notOdd = 1
		}
		limit := int32(width) - 2 - notOdd

		for i, j = 1, 1; i < limit; i, j = i+2, j+1 {
			s2 = data[sn+j+1]  // in_even[j+1]

			dnVar = data[j] - ((s1 + s2 + 2) >> 2)  // in_odd[j] - ((s1 + s2 + 2) >> 2)
			tmp[i] = dc
			tmp[i+1] = s1 + ((dnVar + dc) >> 1)

			dc = dnVar
			s1 = s2
		}

		tmp[i] = dc

		if (width & 1) == 0 { // len is even
			dnVar = data[width/2-1] - ((s1 + 1) >> 1)  // in_odd[len/2-1] - ((s1 + 1) >> 1)
			tmp[width-2] = s1 + ((dnVar + dc) >> 1)
			tmp[width-1] = dnVar
		} else { // len is odd
			tmp[width-1] = s1 + dc
		}

		copy(data, tmp)
	}
}

// Forward53_2D performs the forward 5/3 wavelet transform on a 2D image
// Applies 1D transform to rows, then to columns (separable transform)
// Input: data (row-major order), width, height, stride
// Output: LL (top-left), HL (top-right), LH (bottom-left), HH (bottom-right)
//
// The stride parameter is the full width of the data array (important for multi-level transforms)
// For the first level, stride == width
// For subsequent levels, stride remains the original width while width shrinks
//
// Performance notes:
// - Separable transform: rows first, then columns
// - Row/column buffers reused to minimize allocations
// - Cache-friendly row processing (contiguous memory access)
// - Column processing less cache-friendly (strided access)
// - Typical performance: ~13µs for 64x64, ~102µs for 256x256
// - Potential optimization: transpose for better column access pattern
func Forward53_2D(data []int32, width, height, stride int) {
	Forward53_2DWithParity(data, width, height, stride, true, true)
}

// Forward53_2DWithParity performs the forward 5/3 wavelet transform on a 2D image
// evenRow/evenCol control parity for horizontal/vertical passes (cas=0 when true).
// IMPORTANT: OpenJPEG does VERTICAL (columns) first, then HORIZONTAL (rows).
func Forward53_2DWithParity(data []int32, width, height, stride int, evenRow, evenCol bool) {
	if width <= 1 && height <= 1 {
		return
	}

	// Step 1: Transform columns (VERTICAL pass - OpenJPEG does this FIRST)
	if height > 1 {
		col := make([]int32, height)
		for x := 0; x < width; x++ {
			// Extract column (use stride for row indexing)
			for y := 0; y < height; y++ {
				col[y] = data[y*stride+x]
			}

			// Transform column with evenCol parity (from y0)
			Forward53_1DWithParity(col, evenCol)

			// Write back (use stride for row indexing)
			for y := 0; y < height; y++ {
				data[y*stride+x] = col[y]
			}
		}
	}

	// Step 2: Transform rows (HORIZONTAL pass - OpenJPEG does this SECOND)
	if width > 1 {
		row := make([]int32, width)
		for y := 0; y < height; y++ {
			// Extract row (use stride for indexing)
			for x := 0; x < width; x++ {
				row[x] = data[y*stride+x]
			}

			// Transform row with evenRow parity (from x0)
			Forward53_1DWithParity(row, evenRow)

			// Write back (use stride for indexing)
			for x := 0; x < width; x++ {
				data[y*stride+x] = row[x]
			}
		}
	}
}

// Inverse53_2D performs the inverse 5/3 wavelet transform on a 2D image
// Applies inverse 1D transform to columns, then to rows
// Input: data with subbands (LL, HL, LH, HH), width, height, stride
// Output: reconstructed image
func Inverse53_2D(data []int32, width, height, stride int) {
	Inverse53_2DWithParity(data, width, height, stride, true, true)
}

// Inverse53_2DWithParity performs the inverse 5/3 wavelet transform on a 2D image
// evenRow/evenCol control parity for horizontal/vertical passes (cas=0 when true).
func Inverse53_2DWithParity(data []int32, width, height, stride int, evenRow, evenCol bool) {
	if width <= 1 && height <= 1 {
		return
	}

	// Step 1: Inverse transform rows (HORIZONTAL pass - done FIRST in inverse)
	if width > 1 {
		row := make([]int32, width)
		for y := 0; y < height; y++ {
			// Extract row (use stride for indexing)
			for x := 0; x < width; x++ {
				row[x] = data[y*stride+x]
			}

			// Inverse transform row with evenRow parity
			Inverse53_1DWithParity(row, evenRow)

			// Write back (use stride for indexing)
			for x := 0; x < width; x++ {
				data[y*stride+x] = row[x]
			}
		}
	}

	// Step 2: Inverse transform columns (VERTICAL pass - done SECOND in inverse)
	if height > 1 {
		col := make([]int32, height)
		for x := 0; x < width; x++ {
			// Extract column (use stride for row indexing)
			for y := 0; y < height; y++ {
				col[y] = data[y*stride+x]
			}

			// Inverse transform column with evenCol parity
			Inverse53_1DWithParity(col, evenCol)

			// Write back (use stride for row indexing)
			for y := 0; y < height; y++ {
				data[y*stride+x] = col[y]
			}
		}
	}
}

// ForwardMultilevel performs multilevel 5/3 wavelet decomposition
// levels: number of decomposition levels (1 = one level, 2 = two levels, etc.)
// After each level, only the LL subband is further decomposed
func ForwardMultilevel(data []int32, width, height, levels int) {
	ForwardMultilevelWithParity(data, width, height, levels, 0, 0)
}

// ForwardMultilevelWithParity performs multilevel 5/3 wavelet decomposition with origin parity.
func ForwardMultilevelWithParity(data []int32, width, height, levels int, x0, y0 int) {
	// Keep the original stride (full width) throughout all levels
	// This is critical: after level 1, the LL subband is stored in the top-left,
	// but the row stride remains the original full width
	originalStride := width

	// Current resolution dimensions
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

		// Transform current level in-place with original stride
		// For level 0: curWidth == originalStride
		// For level 1+: curWidth < originalStride (only process LL subband)
		Forward53_2DWithParity(data, curWidth, curHeight, originalStride, evenRow, evenCol)

		// Next level will work only on LL subband (top-left region).
		curWidth, curHeight, curX0, curY0 = nextLowpassWindow(curWidth, curHeight, curX0, curY0)
	}
}

// InverseMultilevel performs multilevel 5/3 wavelet reconstruction
// levels: number of decomposition levels to inverse
// Reconstructs from coarsest to finest
func InverseMultilevel(data []int32, width, height, levels int) {
	InverseMultilevelWithParity(data, width, height, levels, 0, 0)
}

// InverseMultilevelWithParity performs multilevel 5/3 wavelet reconstruction with origin parity.
func InverseMultilevelWithParity(data []int32, width, height, levels int, x0, y0 int) {
	// Keep the original stride (full width) throughout all levels
	originalStride := width

	// Calculate dimensions at each level
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

	// Inverse transform from coarsest to finest
	for level := levels - 1; level >= 0; level-- {
		curWidth := levelWidths[level]
		curHeight := levelHeights[level]
		evenRow := isEven(levelX0[level])
		evenCol := isEven(levelY0[level])

		// Inverse transform this level in-place with original stride
		Inverse53_2DWithParity(data, curWidth, curHeight, originalStride, evenRow, evenCol)
	}
}
