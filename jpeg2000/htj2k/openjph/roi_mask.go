package openjph

import "fmt"

// roiMask represents a per-component ROI mask (full-resolution boolean map).
type roiMask struct {
	width  int
	height int
	data   []bool

	// cache for downsampled blocks: key = "x0,y0,w,h,step"
	cache map[string][][]bool
}

func newROIMask(width, height int) *roiMask {
	return &roiMask{
		width:  width,
		height: height,
		data:   make([]bool, width*height),
		cache:  make(map[string][][]bool),
	}
}

func (m *roiMask) setRect(x0, y0, x1, y1 int) {
	if m == nil {
		return
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > m.width {
		x1 = m.width
	}
	if y1 > m.height {
		y1 = m.height
	}
	for y := y0; y < y1; y++ {
		row := y * m.width
		for x := x0; x < x1; x++ {
			m.data[row+x] = true
		}
	}
}

func (m *roiMask) get(x, y int) bool {
	if m == nil {
		return false
	}
	if x < 0 || y < 0 || x >= m.width || y >= m.height {
		return false
	}
	return m.data[y*m.width+x]
}

// downsample returns a mask cropped to [x0,x1)×[y0,y1), optionally downsampling by step.
// step should be >=1; if step>1, mask is OR-reduced over the block.
func (m *roiMask) downsample(x0, y0, x1, y1, step int) [][]bool {
	if m == nil || step <= 0 {
		return nil
	}
	if step == 0 {
		step = 1
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > m.width {
		x1 = m.width
	}
	if y1 > m.height {
		y1 = m.height
	}

	key := fmt.Sprintf("%d,%d,%d,%d,%d", x0, y0, x1, y1, step)
	if cached, ok := m.cache[key]; ok {
		return cached
	}

	outW := (x1 - x0 + step - 1) / step
	outH := (y1 - y0 + step - 1) / step
	out := make([][]bool, outH)
	for j := 0; j < outH; j++ {
		out[j] = make([]bool, outW)
		for i := 0; i < outW; i++ {
			blockX0 := x0 + i*step
			blockY0 := y0 + j*step
			blockX1 := min(blockX0+step, x1)
			blockY1 := min(blockY0+step, y1)
			val := false
			for yy := blockY0; yy < blockY1 && !val; yy++ {
				for xx := blockX0; xx < blockX1; xx++ {
					if m.get(xx, yy) {
						val = true
						break
					}
				}
			}
			out[j][i] = val
		}
	}
	m.cache[key] = out
	return out
}

// buildRectMasks constructs per-component masks from RoiRects.
func buildRectMasks(width, height int, rects [][]RoiRect) []*roiMask {
	if len(rects) == 0 {
		return nil
	}
	masks := make([]*roiMask, len(rects))
	for comp, rs := range rects {
		mask := newROIMask(width, height)
		for _, r := range rs {
			mask.setRect(r.x0, r.y0, r.x1, r.y1)
		}
		masks[comp] = mask
	}
	return masks
}

// maskFromBitmap builds a mask from a boolean bitmap with given dimensions.
func maskFromBitmap(width, height int, mw, mh int, data []bool) *roiMask {
	if mw <= 0 || mh <= 0 || len(data) != mw*mh {
		return nil
	}
	if mw != width || mh != height {
		return nil
	}
	m := newROIMask(width, height)
	copy(m.data, data)
	return m
}

// boundingRectFromMask computes the bounding box of true pixels in a bitmap.
// Returns rect and a boolean indicating whether any pixel was set.
func boundingRectFromMask(mw, mh int, data []bool) (RoiRect, bool) {
	if mw <= 0 || mh <= 0 || len(data) != mw*mh {
		return RoiRect{}, false
	}
	x0, y0 := mw, mh
	x1, y1 := 0, 0
	found := false
	for y := 0; y < mh; y++ {
		row := y * mw
		for x := 0; x < mw; x++ {
			if data[row+x] {
				if !found {
					x0, y0, x1, y1 = x, y, x+1, y+1
					found = true
				} else {
					if x < x0 {
						x0 = x
					}
					if y < y0 {
						y0 = y
					}
					if x+1 > x1 {
						x1 = x + 1
					}
					if y+1 > y1 {
						y1 = y + 1
					}
				}
			}
		}
	}
	if !found {
		return RoiRect{}, false
	}
	return RoiRect{x0: x0, y0: y0, x1: x1, y1: y1}, true
}

// rasterizePolygon creates a mask from polygon points using even-odd rule.
func rasterizePolygon(width, height int, pts []Point) *roiMask {
	if len(pts) < 3 {
		return nil
	}
	m := newROIMask(width, height)

	for y := 0; y < height; y++ {
		// find intersections with scanline y+0.5
		var xs []int
		scanY := float64(y) + 0.5
		for i := 0; i < len(pts); i++ {
			j := (i + 1) % len(pts)
			y0 := pts[i].Y
			y1 := pts[j].Y
			if y0 == y1 {
				continue
			}
			minY := y0
			maxY := y1
			if minY > maxY {
				minY, maxY = maxY, minY
			}
			if float64(minY) <= scanY && scanY < float64(maxY) {
				x0 := float64(pts[i].X)
				x1 := float64(pts[j].X)
				t := (scanY - float64(y0)) / float64(y1-y0)
				x := x0 + t*(x1-x0)
				xs = append(xs, int(x))
			}
		}
		if len(xs) == 0 {
			continue
		}
		// sort intersections
		for i := 0; i < len(xs); i++ {
			for j := i + 1; j < len(xs); j++ {
				if xs[j] < xs[i] {
					xs[i], xs[j] = xs[j], xs[i]
				}
			}
		}
		for i := 0; i+1 < len(xs); i += 2 {
			xStart := xs[i]
			xEnd := xs[i+1]
			if xStart < 0 {
				xStart = 0
			}
			if xEnd > width {
				xEnd = width
			}
			for x := xStart; x < xEnd; x++ {
				m.data[y*width+x] = true
			}
		}
	}
	return m
}

// buildMasksFromConfig builds per-component masks from ROIConfig regions (rectangles + optional polygon/mask).
func buildMasksFromConfig(width, height, components int, rects [][]RoiRect, cfg *ROIConfig) []*roiMask {
	if cfg == nil || len(cfg.ROIs) == 0 {
		return buildRectMasks(width, height, rects)
	}

	masks := make([]*roiMask, components)
	for comp := 0; comp < components; comp++ {
		masks[comp] = newROIMask(width, height)
	}

	for i := range cfg.ROIs {
		roi := cfg.ROIs[i]
		targetComponents := roi.Components
		if len(targetComponents) == 0 {
			targetComponents = make([]int, components)
			for c := 0; c < components; c++ {
				targetComponents[c] = c
			}
		}

		hasPolygon := len(roi.Polygon) >= 3
		hasMask := len(roi.MaskData) > 0 && roi.MaskWidth > 0 && roi.MaskHeight > 0

		var polyMask *roiMask
		if hasPolygon {
			polyMask = rasterizePolygon(width, height, roi.Polygon)
		}
		var bitmapMask *roiMask
		if hasMask {
			bitmapMask = maskFromBitmap(width, height, roi.MaskWidth, roi.MaskHeight, roi.MaskData)
		}

		for _, comp := range targetComponents {
			if comp < 0 || comp >= components {
				continue
			}
			if masks[comp] == nil {
				masks[comp] = newROIMask(width, height)
			}
			switch {
			case hasPolygon && polyMask != nil:
				for idx, v := range polyMask.data {
					if v {
						masks[comp].data[idx] = true
					}
				}
			case hasMask && bitmapMask != nil:
				for idx, v := range bitmapMask.data {
					if v {
						masks[comp].data[idx] = true
					}
				}
			case roi.Rect != nil:
				masks[comp].setRect(roi.Rect.X0, roi.Rect.Y0, roi.Rect.X0+roi.Rect.Width, roi.Rect.Y0+roi.Rect.Height)
			}
		}
	}
	return masks
}
