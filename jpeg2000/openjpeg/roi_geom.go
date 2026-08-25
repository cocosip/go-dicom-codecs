package openjpeg

// Point represents an integer point.
type Point struct {
	X int
	Y int
}

// boundingRect computes axis-aligned bounding box for polygon.
func boundingRect(pts []Point) RoiRect {
	if len(pts) == 0 {
		return RoiRect{}
	}
	x0, y0 := pts[0].X, pts[0].Y
	x1, y1 := pts[0].X, pts[0].Y
	for _, p := range pts[1:] {
		if p.X < x0 {
			x0 = p.X
		}
		if p.Y < y0 {
			y0 = p.Y
		}
		if p.X > x1 {
			x1 = p.X
		}
		if p.Y > y1 {
			y1 = p.Y
		}
	}
	return RoiRect{x0: x0, y0: y0, x1: x1 + 1, y1: y1 + 1}
}
