package openjpeg

import "testing"

// Verify ROI shift usage for MaxShift (ROI upshift) vs General Scaling (explicit scaling).
func TestROIShiftDirectionByStyle(t *testing.T) {
	rect := &ROIParams{X0: 0, Y0: 0, Width: 4, Height: 4, Shift: 2}

	// MaxShift: ROI blocks carry shift, background stays 0.
	maxParams := DefaultEncodeParams(8, 8, 1, 8, false)
	maxParams.ROIConfig = &ROIConfig{
		DefaultStyle: ROIStyleMaxShift,
		ROIs: []ROIRegion{
			{Rect: rect},
		},
	}
	maxEnc := NewEncoder(maxParams)
	if err := maxEnc.resolveROI(); err != nil {
		t.Fatalf("resolveROI (MaxShift) failed: %v", err)
	}
	roiBlock := codeBlockInfo{compIdx: 0, globalX0: 1, globalY0: 1, width: 2, height: 2}
	bgBlock := codeBlockInfo{compIdx: 0, globalX0: 5, globalY0: 5, width: 2, height: 2}
	if shift := maxEnc.roiShiftForCodeBlock(roiBlock); shift != 2 {
		t.Fatalf("MaxShift ROI block should have shift=2, got %d", shift)
	}
	if shift := maxEnc.roiShiftForCodeBlock(bgBlock); shift != 0 {
		t.Fatalf("MaxShift background block should have shift=0, got %d", shift)
	}

	// General Scaling: roishift is not used (scaling is explicit).
	gsParams := DefaultEncodeParams(8, 8, 1, 8, false)
	gsParams.ROIConfig = &ROIConfig{
		DefaultStyle: ROIStyleGeneralScaling,
		ROIs: []ROIRegion{
			{Rect: rect},
		},
	}
	gsEnc := NewEncoder(gsParams)
	if err := gsEnc.resolveROI(); err != nil {
		t.Fatalf("resolveROI (GeneralScaling) failed: %v", err)
	}
	if shift := gsEnc.roiShiftForCodeBlock(roiBlock); shift != 0 {
		t.Fatalf("GeneralScaling ROI block should have shift=0, got %d", shift)
	}
	if shift := gsEnc.roiShiftForCodeBlock(bgBlock); shift != 0 {
		t.Fatalf("GeneralScaling background block should have shift=0, got %d", shift)
	}
}
