package t2

import "testing"

func TestResolutionDimsWithOriginParity(t *testing.T) {
	width := 5
	height := 4
	numLevels := 1

	resW, resH, resX0, resY0 := resolutionDimsWithOrigin(width, height, 0, 0, numLevels, 0)
	if resW != 3 || resH != 2 {
		t.Fatalf("even origin: got %dx%d, want 3x2", resW, resH)
	}
	if resX0 != 0 || resY0 != 0 {
		t.Fatalf("even origin coords: got (%d,%d), want (0,0)", resX0, resY0)
	}

	resW, resH, resX0, resY0 = resolutionDimsWithOrigin(width, height, 1, 0, numLevels, 0)
	if resW != 2 || resH != 2 {
		t.Fatalf("odd origin: got %dx%d, want 2x2", resW, resH)
	}
	if resX0 != 1 || resY0 != 0 {
		t.Fatalf("odd origin coords: got (%d,%d), want (1,0)", resX0, resY0)
	}

	resW, resH, resX0, resY0 = resolutionDimsWithOrigin(width, height, 1, 0, numLevels, 1)
	if resW != width || resH != height {
		t.Fatalf("full res: got %dx%d, want %dx%d", resW, resH, width, height)
	}
	if resX0 != 1 || resY0 != 0 {
		t.Fatalf("full res coords: got (%d,%d), want (1,0)", resX0, resY0)
	}
}

func TestBandInfosForResolution(t *testing.T) {
	width := 5
	height := 4
	numLevels := 1

	_, _, _, _, bands := bandInfosForResolution(width, height, 1, 0, numLevels, 1)
	if len(bands) != 3 {
		t.Fatalf("bands: got %d, want 3", len(bands))
	}

	if bands[0].band != 1 || bands[0].width != 3 || bands[0].height != 2 || bands[0].offsetX != 2 || bands[0].offsetY != 0 {
		t.Fatalf("HL band: got band=%d size=%dx%d offset=%d,%d", bands[0].band, bands[0].width, bands[0].height, bands[0].offsetX, bands[0].offsetY)
	}
	if bands[1].band != 2 || bands[1].width != 2 || bands[1].height != 2 || bands[1].offsetX != 0 || bands[1].offsetY != 2 {
		t.Fatalf("LH band: got band=%d size=%dx%d offset=%d,%d", bands[1].band, bands[1].width, bands[1].height, bands[1].offsetX, bands[1].offsetY)
	}
	if bands[2].band != 3 || bands[2].width != 3 || bands[2].height != 2 || bands[2].offsetX != 2 || bands[2].offsetY != 2 {
		t.Fatalf("HH band: got band=%d size=%dx%d offset=%d,%d", bands[2].band, bands[2].width, bands[2].height, bands[2].offsetX, bands[2].offsetY)
	}
}
