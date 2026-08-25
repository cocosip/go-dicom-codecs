package openjpeg

import (
	"testing"
)

// TestUnderstandSubbandCoordinates helps understand the subband coordinate system
func TestUnderstandSubbandCoordinates(t *testing.T) {
	// For a 64x64 image with 1 decomposition level:
	// - Resolution 0: LL subband (32x32)
	// - Resolution 1: HL, LH, HH subbands (each 32x32)
	//
	// In the wavelet transform output, these are arranged as:
	// +-------+-------+
	// |  LL   |  HL   |  (each 32x32)
	// +-------+-------+
	// |  LH   |  HH   |
	// +-------+-------+
	//
	// But when we extract them as separate subbands, each one's coordinates
	// start from (0,0) within that subband.
	//
	// The question is: for a precinct grid defined on the resolution level,
	// how should we map subband coordinates to precinct indices?

	width, height := 64, 64
	numLevels := 1

	// Resolution 0: 32x32 LL subband
	res0Width := width >> numLevels   // 32
	res0Height := height >> numLevels // 32

	// Resolution 1: Each of HL/LH/HH is 32x32
	// At resolution r, subbands correspond to decomposition level (numLevels - r)
	// For res=1, level=0, so dimensions are width >> level = width >> 0 = full width?
	// No! Let me recalculate...
	//
	// After 1 decomposition level:
	// - Level 1 (highest): HL1, LH1, HH1 are each 32x32
	// - Level 0 (lowest): LL is 32x32
	//
	// Resolution 0 corresponds to LL (32x32)
	// Resolution 1 corresponds to adding HL1/LH1/HH1 (each 32x32)

	// Resolution 1 dimensions (the full resolution at this level)
	// This is the reference grid that all subbands at this resolution use
	divisor := numLevels - 1        // For resolution 1 with numLevels=1: divisor = 0
	res1Width := width >> divisor   // 64 >> 0 = 64
	res1Height := height >> divisor // 64 >> 0 = 64

	// Each subband at resolution 1 (HL, LH, HH) has dimensions equal to the resolution
	// divided by 2 (because they're from one level of decomposition)
	// But for precinct calculation, we use the resolution-level dimensions
	res1SubbandWidth := width >> (numLevels)   // 64 >> 1 = 32
	res1SubbandHeight := height >> (numLevels) // 64 >> 1 = 32

	t.Logf("Image: %dx%d, NumLevels: %d", width, height, numLevels)
	t.Logf("Resolution 0 (LL): %dx%d", res0Width, res0Height)
	t.Logf("Resolution 1 full: %dx%d", res1Width, res1Height)
	t.Logf("Resolution 1 subbands (HL/LH/HH): each %dx%d", res1SubbandWidth, res1SubbandHeight)

	// If we want to partition Resolution 1 into precincts:
	// The precinct grid is defined on the resolution level (64x64 in this case)
	// But each subband is only 32x32
	//
	// Answer from JPEG 2000 standard: Each subband at the same resolution
	// shares the SAME precinct partition. A precinct at position (px, py)
	// contains code-blocks from all three subbands (HL, LH, HH) at the
	// same relative position within their respective subbands.
	//
	// The key: Use resolution dimensions for precinct grid calculation,
	// but each subband maps to that grid using its own coordinates.

	precinctWidth := 32  // Same as subband width
	precinctHeight := 32 // Same as subband height

	// Calculate precinct grid based on resolution dimensions
	numPrecinctX := (res1Width + precinctWidth - 1) / precinctWidth    // (64 + 31) / 32 = 2
	numPrecinctY := (res1Height + precinctHeight - 1) / precinctHeight // (64 + 31) / 32 = 2

	t.Logf("With %dx%d precincts: grid is %dx%d (total %d precincts)",
		precinctWidth, precinctHeight, numPrecinctX, numPrecinctY, numPrecinctX*numPrecinctY)

	// For a code-block at position (cbX, cbY) in subband HL:
	// It should map to precinct index: (cbY / precinctHeight) * numPrecinctX + (cbX / precinctWidth)
	//
	// The SAME formula applies for LH and HH subbands because they all have
	// the same dimensions and start from (0,0) within their respective spaces.

	t.Logf("\nConclusion: All subbands at the same resolution use the SAME precinct formula!")
	t.Logf("Each subband's (cbX, cbY) maps directly to: py*numPrecinctX + px")
	t.Logf("where px = cbX / precinctWidth, py = cbY / precinctHeight")
}
