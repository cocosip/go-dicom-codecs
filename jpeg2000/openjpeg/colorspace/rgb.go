package colorspace

// RGBToYCbCr converts RGB to JPEG 2000 ICT components (no 128 offset).
// Input is expected to be level-shifted/signed, matching OpenJPEG's MCT.
func RGBToYCbCr(r, g, b int32) (y, cb, cr int32) {
	return ICTForward(r, g, b)
}

// YCbCrToRGB converts JPEG 2000 ICT components back to RGB (no 128 offset).
func YCbCrToRGB(y, cb, cr int32) (r, g, b int32) {
	return ICTInverse(y, cb, cr)
}

// ConvertRGBToYCbCr converts an RGB image to YCbCr components
// Input: interleaved RGB data [R0,G0,B0,R1,G1,B1,...]
// Output: three separate component arrays [Y], [Cb], [Cr]
func ConvertRGBToYCbCr(rgb []int32, width, height int) (y, cb, cr []int32) {
	numPixels := width * height
	y = make([]int32, numPixels)
	cb = make([]int32, numPixels)
	cr = make([]int32, numPixels)

	for i := 0; i < numPixels; i++ {
		r := rgb[i*3]
		g := rgb[i*3+1]
		b := rgb[i*3+2]

		y[i], cb[i], cr[i] = RGBToYCbCr(r, g, b)
	}

	return
}

// ConvertYCbCrToRGB converts YCbCr components to RGB image
// Input: three separate component arrays [Y], [Cb], [Cr]
// Output: interleaved RGB data [R0,G0,B0,R1,G1,B1,...]
func ConvertYCbCrToRGB(y, cb, cr []int32, width, height int) []int32 {
	numPixels := width * height
	rgb := make([]int32, numPixels*3)

	for i := 0; i < numPixels; i++ {
		r, g, b := YCbCrToRGB(y[i], cb[i], cr[i])
		rgb[i*3] = r
		rgb[i*3+1] = g
		rgb[i*3+2] = b
	}

	return rgb
}

// InterleaveComponents interleaves separate component arrays
// Input: components [[C0_p0, C0_p1, ...], [C1_p0, C1_p1, ...], ...]
// Output: interleaved [C0_p0, C1_p0, ..., C0_p1, C1_p1, ...]
func InterleaveComponents(components [][]int32) []int32 {
	if len(components) == 0 {
		return nil
	}

	numComponents := len(components)
	numPixels := len(components[0])

	result := make([]int32, numPixels*numComponents)

	for p := 0; p < numPixels; p++ {
		for c := 0; c < numComponents; c++ {
			result[p*numComponents+c] = components[c][p]
		}
	}

	return result
}

// DeinterleaveComponents separates interleaved data into component arrays
// Input: interleaved [C0_p0, C1_p0, ..., C0_p1, C1_p1, ...]
// Output: components [[C0_p0, C0_p1, ...], [C1_p0, C1_p1, ...], ...]
func DeinterleaveComponents(data []int32, numComponents int) [][]int32 {
	if len(data) == 0 || numComponents == 0 {
		return nil
	}

	numPixels := len(data) / numComponents
	components := make([][]int32, numComponents)

	for c := 0; c < numComponents; c++ {
		components[c] = make([]int32, numPixels)
	}

	for p := 0; p < numPixels; p++ {
		for c := 0; c < numComponents; c++ {
			components[c][p] = data[p*numComponents+c]
		}
	}

	return components
}

// ConvertComponentsRGBToYCbCr converts separate R,G,B slices to Y,Cb,Cr using ICT.
// params: r,g,b - input component slices
// returns: y,cb,cr slices
func ConvertComponentsRGBToYCbCr(r, g, b []int32) (y, cb, cr []int32) {
    n := len(r)
    y = make([]int32, n)
    cb = make([]int32, n)
    cr = make([]int32, n)
    for i := 0; i < n; i++ {
        y[i], cb[i], cr[i] = RGBToYCbCr(r[i], g[i], b[i])
    }
    return
}

// ConvertComponentsYCbCrToRGB converts Y,Cb,Cr slices back to R,G,B using ICT inverse.
// params: y,cb,cr - transformed component slices
// returns: r,g,b slices
func ConvertComponentsYCbCrToRGB(y, cb, cr []int32) (r, g, b []int32) {
    n := len(y)
    r = make([]int32, n)
    g = make([]int32, n)
    b = make([]int32, n)
    for i := 0; i < n; i++ {
        r[i], g[i], b[i] = YCbCrToRGB(y[i], cb[i], cr[i])
    }
    return
}
