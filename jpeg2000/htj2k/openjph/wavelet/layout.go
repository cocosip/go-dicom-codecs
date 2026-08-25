package wavelet

// LLDimensions returns the low-low (LL) subband dimensions after a multilevel
// decomposition with origin (0,0).
func LLDimensions(width, height, levels int) (llWidth, llHeight int) {
	return LLDimensionsWithParity(width, height, levels, 0, 0)
}

// LLDimensionsWithParity returns the LL subband dimensions after a multilevel
// decomposition for an arbitrary image origin (x0,y0).
func LLDimensionsWithParity(width, height, levels int, x0, y0 int) (llWidth, llHeight int) {
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	if levels <= 0 {
		return width, height
	}

	curWidth := width
	curHeight := height
	curX0 := x0
	curY0 := y0

	for level := 0; level < levels; level++ {
		if curWidth <= 1 && curHeight <= 1 {
			break
		}

		curWidth, curHeight, curX0, curY0 = nextLowpassWindow(curWidth, curHeight, curX0, curY0)
	}

	return curWidth, curHeight
}

func nextLowpassWindow(width, height, x0, y0 int) (nextWidth, nextHeight, nextX0, nextY0 int) {
	evenRow := isEven(x0)
	evenCol := isEven(y0)

	nextWidth = splitLengths(width, evenRow)
	nextHeight = splitLengths(height, evenCol)
	nextX0 = nextCoord(x0)
	nextY0 = nextCoord(y0)
	return
}
