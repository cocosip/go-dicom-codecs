package openjpeg

import (
	"testing"
)

// Integration: mask ROI with General Scaling should scale ROI block and leave background unscaled.
func TestIntegrationMaskGeneralScaling(t *testing.T) {
	width, height := 8, 8
	maskData := make([]bool, width*height)
	// Mark center 4x4 as ROI
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			maskData[y*width+x] = true
		}
	}

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = 0 // keep mask and coefficient grids aligned
	params.ROIConfig = &ROIConfig{
		DefaultStyle: ROIStyleGeneralScaling,
		DefaultShift: 2,
		ROIs: []ROIRegion{
			{
				MaskWidth:  width,
				MaskHeight: height,
				MaskData:   maskData,
			},
		},
	}

	// Input: constant background 10, ROI 1 => expect ROI scaled (higher) after encode/decode inverse
	numPixels := width * height
	componentData := [][]int32{make([]int32, numPixels)}
	for i := 0; i < numPixels; i++ {
		componentData[0][i] = 10
	}
	// raise ROI pixels
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			componentData[0][y*width+x] = 20
		}
	}

	enc := NewEncoder(params)
	stream, err := enc.EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	dec := NewDecoder()
	dec.SetROIConfig(params.ROIConfig)
	if err := dec.Decode(stream); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	decoded, _ := dec.GetComponentData(0)

	var roiMin int32 = 1 << 30
	var bgMax int32 = -1
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := decoded[y*width+x]
			if x >= 2 && x < 6 && y >= 2 && y < 6 {
				if val < roiMin {
					roiMin = val
				}
			} else {
				if val > bgMax {
					bgMax = val
				}
			}
		}
	}
	if roiMin <= bgMax {
		t.Fatalf("ROI not higher than background: roiMin=%d bgMax=%d", roiMin, bgMax)
	}
}
