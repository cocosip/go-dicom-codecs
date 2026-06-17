package jpeg2000

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

type fingerprintBlockEncoder struct{}

func (fingerprintBlockEncoder) Encode(coeffs []int32, _ int, _ int) ([]byte, error) {
	var sum uint32
	for i, coeff := range coeffs {
		sum += uint32(i+1) * uint32(coeff)
	}
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, sum)
	return out, nil
}

func TestPacketCodeBlockOrderMatchesDecoderGeometry(t *testing.T) {
	params := DefaultEncodeParams(128, 128, 1, 8, false)
	params.ProgressionOrder = 2
	params.BlockEncoderFactory = func(_, _ int) BlockEncoder {
		return fingerprintBlockEncoder{}
	}

	enc := NewEncoder(params)
	coeffs := make([][]int32, 1)
	coeffs[0] = make([]int32, params.Width*params.Height)
	for i := range coeffs[0] {
		coeffs[0][i] = int32(i + 1)
	}

	packetEnc, allBlocks := enc.buildTilePacketEncoder(coeffs, params.Width, params.Height)
	owners := make(map[string]int)
	for _, block := range allBlocks {
		owners[string(block.Data)] = block.Index
	}

	packetEnc.ResetState()
	packets, err := packetEnc.EncodePackets()
	if err != nil {
		t.Fatalf("EncodePackets failed: %v", err)
	}

	got := make(map[string][]int)
	for _, packet := range packets {
		key := fmt.Sprintf("%d:%d", packet.ResolutionLevel, packet.PrecinctIndex)
		for _, incl := range packet.CodeBlockIncls {
			if !incl.Included {
				continue
			}
			got[key] = append(got[key], owners[string(incl.Data)])
		}
	}

	want := decoderGeometryOrder(params)
	for key, wantOrder := range want {
		if !reflect.DeepEqual(got[key], wantOrder) {
			t.Fatalf("order mismatch for packet %s:\n got %v\nwant %v", key, got[key], wantOrder)
		}
	}
}

func TestHTJ2KPacketCodeBlocksUseCleanupPassOnly(t *testing.T) {
	params := DefaultEncodeParams(128, 128, 1, 8, false)
	params.ProgressionOrder = 2
	params.HTJ2KMode = true
	params.BlockEncoderFactory = func(_, _ int) BlockEncoder {
		return fingerprintBlockEncoder{}
	}

	enc := NewEncoder(params)
	coeffs := make([][]int32, 1)
	coeffs[0] = make([]int32, params.Width*params.Height)
	for i := range coeffs[0] {
		coeffs[0][i] = int32(i + 1)
	}

	_, blocks := enc.buildTilePacketEncoder(coeffs, params.Width, params.Height)
	if len(blocks) == 0 {
		t.Fatal("expected HTJ2K code-blocks")
	}
	wantMissingMSBs := expectedHTJ2KMissingMSBs(params, enc)
	if len(blocks) != len(wantMissingMSBs) {
		t.Fatalf("block count=%d, expected missing-MSB count=%d", len(blocks), len(wantMissingMSBs))
	}
	for i, block := range blocks {
		if block.NumPassesTotal != 1 {
			t.Fatalf("HTJ2K block %d NumPassesTotal=%d, want cleanup-only pass count 1",
				block.Index, block.NumPassesTotal)
		}
		if block.ZeroBitPlanes != wantMissingMSBs[i] {
			t.Fatalf("HTJ2K block %d missing MSBs=%d, want OpenJPH Kmax-1=%d",
				block.Index, block.ZeroBitPlanes, wantMissingMSBs[i])
		}
	}
}

func expectedHTJ2KMissingMSBs(params *EncodeParams, enc *Encoder) []int {
	var expected []int
	for res := 0; res <= params.NumLevels; res++ {
		for _, band := range jpeg2000BandInfosForResolution(params.Width, params.Height, 0, 0, params.NumLevels, res) {
			if band.width <= 0 || band.height <= 0 {
				continue
			}
			numCBX := (band.width + params.CodeBlockWidth - 1) / params.CodeBlockWidth
			numCBY := (band.height + params.CodeBlockHeight - 1) / params.CodeBlockHeight
			missing := enc.bandNumbps(res, band.band) - 1
			if missing < 0 {
				missing = 0
			}
			for i := 0; i < numCBX*numCBY; i++ {
				expected = append(expected, missing)
			}
		}
	}
	return expected
}

func decoderGeometryOrder(params *EncodeParams) map[string][]int {
	order := make(map[string][]int)
	globalCBIdx := 0
	for res := 0; res <= params.NumLevels; res++ {
		bands := jpeg2000BandInfosForResolution(params.Width, params.Height, 0, 0, params.NumLevels, res)
		precinctBands := make(map[int]map[int][]int)
		for _, band := range bands {
			if band.width <= 0 || band.height <= 0 {
				continue
			}
			numCBX := (band.width + params.CodeBlockWidth - 1) / params.CodeBlockWidth
			numCBY := (band.height + params.CodeBlockHeight - 1) / params.CodeBlockHeight
			for cby := 0; cby < numCBY; cby++ {
				for cbx := 0; cbx < numCBX; cbx++ {
					if precinctBands[0] == nil {
						precinctBands[0] = make(map[int][]int)
					}
					sortKey := cby*100000 + cbx
					precinctBands[0][band.band] = append(precinctBands[0][band.band], sortKey*1000000+globalCBIdx)
					globalCBIdx++
				}
			}
		}
		bandOrder := []int{0}
		if res > 0 {
			bandOrder = []int{1, 2, 3}
		}
		for precinctIdx, bandMap := range precinctBands {
			key := fmt.Sprintf("%d:%d", res, precinctIdx)
			for _, band := range bandOrder {
				entries := append([]int(nil), bandMap[band]...)
				sort.Ints(entries)
				for _, entry := range entries {
					order[key] = append(order[key], entry%1000000)
				}
			}
		}
	}
	return order
}

func jpeg2000BandInfosForResolution(width, height, x0, y0, numLevels, res int) []bandInfo {
	return bandInfosForResolution(width, height, x0, y0, numLevels, res)
}
