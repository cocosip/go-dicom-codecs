package t2

import "sort"

type positionKey struct {
	X int
	Y int
}

type positionMaps struct {
	byCompRes map[int]map[int]map[positionKey]int
	byRes     map[int][]positionKey
	byComp    map[int][]positionKey
	all       []positionKey
}

type positionInputs struct {
	numComponents     int
	numResolutions    int
	precinctIndices   func(comp, res int) []int
	componentBounds   func(comp int) componentBounds
	componentSampling func(comp int) (dx, dy int)
	precinctSize      func(res int) (pw, ph int)
}

func buildPositionMaps(in positionInputs) *positionMaps {
	numLevels := in.numResolutions - 1
	if numLevels < 0 {
		numLevels = 0
	}

	byCompRes := make(map[int]map[int]map[positionKey]int)
	resSets := make([]map[positionKey]struct{}, in.numResolutions)
	compSets := make([]map[positionKey]struct{}, in.numComponents)
	allSet := make(map[positionKey]struct{})

	for comp := 0; comp < in.numComponents; comp++ {
		bounds := in.componentBounds(comp)
		dx, dy := in.componentSampling(comp)
		if dx <= 0 {
			dx = 1
		}
		if dy <= 0 {
			dy = 1
		}
		for res := 0; res < in.numResolutions; res++ {
			indices := in.precinctIndices(comp, res)
			if len(indices) == 0 {
				continue
			}
			pw, ph := in.precinctSize(res)
			for _, idx := range indices {
				pos, ok := precinctPositionKey(bounds, dx, dy, numLevels, res, pw, ph, idx)
				if !ok {
					continue
				}
				if byCompRes[comp] == nil {
					byCompRes[comp] = make(map[int]map[positionKey]int)
				}
				if byCompRes[comp][res] == nil {
					byCompRes[comp][res] = make(map[positionKey]int)
				}
				byCompRes[comp][res][pos] = idx

				if resSets[res] == nil {
					resSets[res] = make(map[positionKey]struct{})
				}
				resSets[res][pos] = struct{}{}

				if compSets[comp] == nil {
					compSets[comp] = make(map[positionKey]struct{})
				}
				compSets[comp][pos] = struct{}{}

				allSet[pos] = struct{}{}
			}
		}
	}

	byRes := make(map[int][]positionKey)
	for res, set := range resSets {
		if len(set) == 0 {
			continue
		}
		byRes[res] = sortedPositions(set)
	}

	byComp := make(map[int][]positionKey)
	for comp, set := range compSets {
		if len(set) == 0 {
			continue
		}
		byComp[comp] = sortedPositions(set)
	}

	return &positionMaps{
		byCompRes: byCompRes,
		byRes:     byRes,
		byComp:    byComp,
		all:       sortedPositions(allSet),
	}
}

func sortedPositions(set map[positionKey]struct{}) []positionKey {
	keys := make([]positionKey, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Y != keys[j].Y {
			return keys[i].Y < keys[j].Y
		}
		return keys[i].X < keys[j].X
	})
	return keys
}

func precinctPositionKey(bounds componentBounds, dx, dy, numLevels, res, pw, ph, precinctIdx int) (positionKey, bool) {
	if pw <= 0 || ph <= 0 {
		return positionKey{}, false
	}
	width := bounds.x1 - bounds.x0
	height := bounds.y1 - bounds.y0
	if width <= 0 || height <= 0 {
		return positionKey{}, false
	}

	resW, resH, resX0, resY0 := resolutionDimsWithOrigin(width, height, bounds.x0, bounds.y0, numLevels, res)
	startX := floorDiv(resX0, pw) * pw
	startY := floorDiv(resY0, ph) * ph
	endX := ceilDiv(resX0+resW, pw) * pw
	endY := ceilDiv(resY0+resH, ph) * ph
	numPrecinctX := (endX - startX) / pw
	numPrecinctY := (endY - startY) / ph
	if numPrecinctX < 1 {
		numPrecinctX = 1
	}
	if numPrecinctY < 1 {
		numPrecinctY = 1
	}

	if precinctIdx < 0 || precinctIdx >= numPrecinctX*numPrecinctY {
		return positionKey{}, false
	}

	px := precinctIdx % numPrecinctX
	py := precinctIdx / numPrecinctX
	originResX := startX + px*pw
	originResY := startY + py*ph

	levelno := numLevels - res
	if levelno < 0 {
		levelno = 0
	}
	scaleX := dx << levelno
	scaleY := dy << levelno
	return positionKey{X: originResX * scaleX, Y: originResY * scaleY}, true
}
