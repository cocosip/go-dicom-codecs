// Package openjpeg tests the classic OpenJPEG-aligned engine.
package openjpeg

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

func TestCOMMarkerEncodesROIConfig(t *testing.T) {
	width, height := 8, 8
	params := DefaultEncodeParams(width, height, 2, 8, false)
	params.NumLevels = 1
	params.ROIConfig = &ROIConfig{
		DefaultShift: 3,
		ROIs: []ROIRegion{
			{Rect: &ROIParams{X0: 1, Y0: 1, Width: 3, Height: 3}, Components: []int{0}},
			{Rect: &ROIParams{X0: 0, Y0: 0, Width: 4, Height: 2}},
		},
	}

	numPixels := width * height
	componentData := [][]int32{make([]int32, numPixels), make([]int32, numPixels)}
	for i := 0; i < numPixels; i++ {
		componentData[0][i] = int32(i % 5)
		componentData[1][i] = int32((i * 2) % 7)
	}

	stream, err := NewEncoder(params).EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	cs, err := codestream.NewParser(stream).Parse()
	if err != nil {
		t.Fatalf("parse codestream failed: %v", err)
	}

	var roiCOM *codestream.COMSegment
	for i := range cs.COM {
		seg := cs.COM[i]
		if seg.Rcom != 0 || len(seg.Data) < 7 {
			continue
		}
		if string(seg.Data[:6]) == "JP2ROI" {
			roiCOM = &cs.COM[i]
			break
		}
	}
	if roiCOM == nil {
		t.Fatalf("missing ROI COM segment, total COM segments=%d", len(cs.COM))
	}
	if version := roiCOM.Data[6]; version != 1 {
		t.Fatalf("unexpected COM version: got %d", version)
	}

	cfg, err := parseROIFromCOMData(roiCOM.Data[7:])
	if err != nil {
		t.Fatalf("parseROIFromCOMData failed: %v", err)
	}
	if len(cfg.ROIs) != len(params.ROIConfig.ROIs) {
		t.Fatalf("expected %d ROI entries, got %d", len(params.ROIConfig.ROIs), len(cfg.ROIs))
	}

	first := cfg.ROIs[0]
	if first.Rect == nil || first.Rect.X0 != 1 || first.Rect.Y0 != 1 || first.Rect.Width != 3 || first.Rect.Height != 3 {
		t.Fatalf("unexpected first ROI rect: %+v", first.Rect)
	}
	if len(first.Components) != 1 || first.Components[0] != 0 {
		t.Fatalf("unexpected first ROI components: %+v", first.Components)
	}

	second := cfg.ROIs[1]
	if second.Rect == nil || second.Rect.X0 != 0 || second.Rect.Y0 != 0 || second.Rect.Width != 4 || second.Rect.Height != 2 {
		t.Fatalf("unexpected second ROI rect: %+v", second.Rect)
	}
	if len(second.Components) != params.Components {
		t.Fatalf("expected second ROI to target all %d components, got %d", params.Components, len(second.Components))
	}
	for c := 0; c < params.Components; c++ {
		if second.Components[c] != c {
			t.Fatalf("unexpected component %d in second ROI: %+v", c, second.Components)
		}
	}
}

func TestTilePartRGNWrittenPerTile(t *testing.T) {
	width, height := 16, 16
	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = 1
	params.TileWidth = 8
	params.TileHeight = 8
	params.ROI = &ROIParams{X0: 0, Y0: 0, Width: 8, Height: 8, Shift: 2}

	numPixels := width * height
	componentData := [][]int32{make([]int32, numPixels)}
	for i := 0; i < numPixels; i++ {
		componentData[0][i] = int32(i % 251)
	}

	stream, err := NewEncoder(params).EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	cs, err := codestream.NewParser(stream).Parse()
	if err != nil {
		t.Fatalf("parse codestream failed: %v", err)
	}

	expectedTiles := ((width + params.TileWidth - 1) / params.TileWidth) * ((height + params.TileHeight - 1) / params.TileHeight)
	if len(cs.Tiles) != expectedTiles {
		t.Fatalf("expected %d tiles, got %d", expectedTiles, len(cs.Tiles))
	}

	for idx, tile := range cs.Tiles {
		if len(tile.RGN) != params.Components {
			t.Fatalf("tile %d: expected %d RGN segments, got %d", idx, params.Components, len(tile.RGN))
		}
		for _, rgn := range tile.RGN {
			if rgn.Crgn != 0 {
				t.Fatalf("tile %d: unexpected Crgn=%d", idx, rgn.Crgn)
			}
			if rgn.Srgn != 0 {
				t.Fatalf("tile %d: unexpected Srgn=%d (want MaxShift)", idx, rgn.Srgn)
			}
			if int(rgn.SPrgn) != params.ROI.Shift {
				t.Fatalf("tile %d: unexpected SPrgn=%d, want %d", idx, rgn.SPrgn, params.ROI.Shift)
			}
		}
	}
}
