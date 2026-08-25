package openjph

// ROIParams defines a rectangular Region of Interest using image coordinates.
// Coordinates are zero-based, width/height are positive, and rectangle is [X0, X0+Width) × [Y0, Y0+Height).
type ROIParams struct {
	X0     int
	Y0     int
	Width  int
	Height int
	Shift  int // MaxShift bit-plane shift for ROI upscaling
}

// IsValid returns true if ROI rectangle and shift are valid.
func (r *ROIParams) IsValid(imgWidth, imgHeight int) bool {
	if r == nil {
		return false
	}
	if r.Width <= 0 || r.Height <= 0 || r.Shift <= 0 {
		return false
	}
	if r.X0 < 0 || r.Y0 < 0 {
		return false
	}
	if r.X0+r.Width > imgWidth || r.Y0+r.Height > imgHeight {
		return false
	}
	return true
}

// Intersects returns true if ROI rectangle intersects the given block rectangle.
// Block coordinates are [x0, x1) x [y0, y1).
func (r *ROIParams) Intersects(x0, y0, x1, y1 int) bool {
	if r == nil {
		return false
	}
	return r.X0 < x1 && x0 < r.X0+r.Width &&
		r.Y0 < y1 && y0 < r.Y0+r.Height
}
