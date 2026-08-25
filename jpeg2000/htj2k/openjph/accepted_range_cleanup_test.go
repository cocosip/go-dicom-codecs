// Package openjph tests the OpenJPH-aligned HTJ2K engine.
package openjph

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k/openjph/cleanup"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k/openjph/t2"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k/openjph/wavelet"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

func TestAcceptedRangeCleanupBytesMatchOpenJPH(t *testing.T) {
	root := filepath.Join("..", "..", "..", "test-data", "htj2k", "interop-v1")
	manifest := readAcceptedRangeManifest(t, root)
	assertAcceptedRangeFixtureMatrix(t, manifest)

	for _, fixture := range manifest.Fixtures {
		fixture := fixture
		for _, syntax := range fixture.Syntaxes {
			syntax := syntax
			t.Run(fmt.Sprintf("%s/%s", fixture.Image.Name, syntax.Name), func(t *testing.T) {
				if len(fixture.Source.Frames) != fixture.Image.FrameCount {
					t.Fatalf("source frame count = %d, want metadata %d", len(fixture.Source.Frames), fixture.Image.FrameCount)
				}
				if len(syntax.EncodedFrames) != fixture.Image.FrameCount {
					t.Fatalf("encoded frame count = %d, want metadata %d", len(syntax.EncodedFrames), fixture.Image.FrameCount)
				}
				params := DefaultEncodeParams(
					fixture.Image.Width,
					fixture.Image.Height,
					fixture.Image.SamplesPerPixel,
					fixture.Image.BitsAllocated,
					fixture.Image.Signed,
				)
				params.Lossless = syntax.Lossless
				params.Quality = 80
				params.ProgressionOrder = uint8(t2.ProgressionRPCL)
				encoder := NewEncoder(params)

				for frame := 0; frame < fixture.Image.FrameCount; frame++ {
					frame := frame
					t.Run(fmt.Sprintf("frame-%04d", frame), func(t *testing.T) {
						raw := readAcceptedRangeArtifactPath(t, root, fixture.Source.EncoderInputFrames[frame])
						native := readAcceptedRangeArtifactPath(t, root, syntax.EncodedFrames[frame])
						got, err := encoder.Encode(raw)
						if err != nil {
							t.Fatalf("encode Go HTJ2K reference: %v", err)
						}
						assertAcceptedRangeMainHeaderMarkers(t, got, native)
						assertAcceptedRangeCleanupBytes(t, encoder, got, native)
					})
				}
			})
		}
	}
}

type acceptedRangeManifest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Fixtures      []acceptedRangeFixture `json:"fixtures"`
}

type acceptedRangeFixture struct {
	Image    acceptedRangeImage    `json:"image"`
	Source   acceptedRangeSource   `json:"source"`
	Syntaxes []acceptedRangeSyntax `json:"syntaxes"`
}

type acceptedRangeImage struct {
	Name                      string `json:"name"`
	Width                     int    `json:"width"`
	Height                    int    `json:"height"`
	SamplesPerPixel           int    `json:"samplesPerPixel"`
	BitsAllocated             int    `json:"bitsAllocated"`
	BitsStored                int    `json:"bitsStored"`
	Signed                    bool   `json:"signed"`
	PhotometricInterpretation string `json:"photometricInterpretation"`
	FrameCount                int    `json:"frameCount"`
}

type acceptedRangeSource struct {
	Frames             []string `json:"frames"`
	EncoderInputFrames []string `json:"encoderInputFrames"`
}

type acceptedRangeSyntax struct {
	Name              string   `json:"name"`
	TransferSyntaxUID string   `json:"transferSyntaxUid"`
	Lossless          bool     `json:"lossless"`
	GoEncodedDicom    string   `json:"goEncodedDicom"`
	EncodedFrames     []string `json:"encodedFrames"`
	DecodedFrames     []string `json:"decodedFrames"`
	GoEncodedFrames   []string `json:"goEncodedFrames"`
	GoFromGoFrames    []string `json:"goFromGoFrames"`
	GoFromFoFrames    []string `json:"goFromFoFrames"`
	FoFromGoFrames    []string `json:"foFromGoFrames"`
}

func readAcceptedRangeManifest(t *testing.T, root string) acceptedRangeManifest {
	t.Helper()
	data := readAcceptedRangeArtifact(t, root, "manifest.json")
	var manifest acceptedRangeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode accepted-range manifest: %v", err)
	}
	if manifest.SchemaVersion != 2 {
		t.Fatalf("accepted-range schemaVersion = %d, want 2", manifest.SchemaVersion)
	}
	return manifest
}

func assertAcceptedRangeFixtureMatrix(t *testing.T, manifest acceptedRangeManifest) {
	t.Helper()
	coverage := map[string]bool{}
	for _, fixture := range manifest.Fixtures {
		image := fixture.Image
		switch {
		case image.SamplesPerPixel == 1 && image.BitsAllocated == 8 && !image.Signed:
			coverage["mono-u8"] = true
		case image.SamplesPerPixel == 1 && image.BitsAllocated == 16 && !image.Signed:
			coverage["mono-u16"] = true
		case image.SamplesPerPixel == 1 && image.BitsAllocated == 16 && image.Signed:
			coverage["mono-s16"] = true
		case image.PhotometricInterpretation == "RGB" && image.BitsAllocated == 8:
			coverage["rgb-u8"] = true
		case image.PhotometricInterpretation == "RGB" && image.BitsAllocated == 16:
			coverage["rgb-u16"] = true
		}
		if image.PhotometricInterpretation == "YBR_FULL" {
			coverage["ybr-full"] = true
		}
		if image.PhotometricInterpretation == "YBR_FULL_422" {
			coverage["ybr-full-422"] = true
		}
		if image.Width%2 != 0 || image.Height%2 != 0 {
			coverage["odd-dimension"] = true
		}
		if image.FrameCount > 1 {
			coverage["multiframe"] = true
		}
		if len(fixture.Syntaxes) != 3 {
			t.Fatalf("fixture %s syntax count = %d, want 3", image.Name, len(fixture.Syntaxes))
		}
		if len(fixture.Source.EncoderInputFrames) != image.FrameCount {
			t.Fatalf("fixture %s encoder-input frame count = %d, want %d", image.Name, len(fixture.Source.EncoderInputFrames), image.FrameCount)
		}
	}
	for _, requirement := range []string{
		"mono-u8", "mono-u16", "mono-s16", "rgb-u8", "rgb-u16",
		"ybr-full", "ybr-full-422", "odd-dimension", "multiframe",
	} {
		if !coverage[requirement] {
			t.Errorf("accepted-range fixture matrix missing %s", requirement)
		}
	}
}

func assertAcceptedRangeMainHeaderMarkers(t *testing.T, got, native []byte) {
	t.Helper()
	gotSegments := acceptedRangeMainHeaderSegments(t, got)
	nativeSegments := acceptedRangeMainHeaderSegments(t, native)
	gotOrder := make([]uint16, len(gotSegments))
	nativeOrder := make([]uint16, len(nativeSegments))
	for index := range gotSegments {
		gotOrder[index] = binary.BigEndian.Uint16(gotSegments[index])
	}
	for index := range nativeSegments {
		nativeOrder[index] = binary.BigEndian.Uint16(nativeSegments[index])
	}
	if fmt.Sprint(gotOrder) != fmt.Sprint(nativeOrder) {
		t.Fatalf("main-header marker order = %v, want Native %v", gotOrder, nativeOrder)
	}

	for _, marker := range []uint16{
		codestream.MarkerSIZ,
		codestream.MarkerCAP,
		codestream.MarkerCOD,
		codestream.MarkerQCD,
		codestream.MarkerCOM,
	} {
		gotSegment := acceptedRangeFindMarkerSegment(t, gotSegments, marker)
		nativeSegment := acceptedRangeFindMarkerSegment(t, nativeSegments, marker)
		if !bytes.Equal(gotSegment, nativeSegment) {
			t.Fatalf("%s marker differs: Go=% X Native=% X", codestream.MarkerName(marker), gotSegment, nativeSegment)
		}
	}
}

func acceptedRangeMainHeaderSegments(t *testing.T, data []byte) [][]byte {
	t.Helper()
	if len(data) < 2 || binary.BigEndian.Uint16(data) != codestream.MarkerSOC {
		t.Fatal("accepted-range codestream does not start with SOC")
	}
	segments := [][]byte{data[:2]}
	for offset := 2; ; {
		if offset+2 > len(data) {
			t.Fatal("accepted-range main header is truncated")
		}
		marker := binary.BigEndian.Uint16(data[offset:])
		if marker == codestream.MarkerSOT {
			return segments
		}
		if offset+4 > len(data) {
			t.Fatalf("%s marker length is truncated", codestream.MarkerName(marker))
		}
		length := int(binary.BigEndian.Uint16(data[offset+2:]))
		end := offset + 2 + length
		if length < 2 || end > len(data) {
			t.Fatalf("%s marker length %d exceeds codestream", codestream.MarkerName(marker), length)
		}
		segments = append(segments, data[offset:end])
		offset = end
	}
}

func acceptedRangeFindMarkerSegment(t *testing.T, segments [][]byte, marker uint16) []byte {
	t.Helper()
	for _, segment := range segments {
		if len(segment) >= 2 && binary.BigEndian.Uint16(segment) == marker {
			return segment
		}
	}
	t.Fatalf("missing %s marker", codestream.MarkerName(marker))
	return nil
}

func assertAcceptedRangeCleanupBytes(t *testing.T, encoder *Encoder, got, native []byte) {
	t.Helper()
	nativeBlocks := acceptedRangeCleanupBlocks(t, native)
	goBlocks := acceptedRangeCleanupBlocks(t, got)
	if len(goBlocks) != len(nativeBlocks) {
		t.Fatalf("cleanup block count = %d, want Native %d", len(goBlocks), len(nativeBlocks))
	}
	for index := range nativeBlocks {
		if bytes.Equal(goBlocks[index].Data, nativeBlocks[index].Data) {
			continue
		}
		first := firstAcceptedRangeByteDiff(goBlocks[index].Data, nativeBlocks[index].Data)
		goScup := cleanupScup(goBlocks[index].Data)
		nativeScup := cleanupScup(nativeBlocks[index].Data)
		bandNumbps := encoder.bandNumbps(nativeBlocks[index].Resolution, nativeBlocks[index].Band)
		goCoefficients := decodeAcceptedRangeCleanupBlock(t, goBlocks[index], bandNumbps)
		nativeCoefficients := decodeAcceptedRangeCleanupBlock(t, nativeBlocks[index], bandNumbps)
		coefficientIndex := firstAcceptedRangeCoefficientDiff(goCoefficients, nativeCoefficients)
		coefficientX := coefficientIndex % nativeBlocks[index].Width
		coefficientY := coefficientIndex / nativeBlocks[index].Width
		globalCoefficientX := nativeBlocks[index].X0 + coefficientX
		globalCoefficientY := nativeBlocks[index].Y0 + coefficientY
		var waveletCoefficient, deltaInv, quantizedProduct float32
		if !encoder.params.Lossless {
			waveletCoefficient, deltaInv, quantizedProduct = acceptedRangeGoQuantizationTrace(
				t, encoder, nativeBlocks[index].Component,
				nativeBlocks[index].Resolution, nativeBlocks[index].Band,
				globalCoefficientX, globalCoefficientY,
			)
		}
		t.Fatalf(
			"cleanup block %d differs at byte %d (%d byte diffs): packet={L:%d R:%d C:%d P:%d incl:%d band:%d cb:(%d,%d) origin:(%d,%d) size:%dx%d kmax:%d zbp:%d passes:%d} coefficient={index:%d local:(%d,%d) global:(%d,%d) Go:%d Native:%d wavelet:%g/0x%08x deltaInv:%g/0x%08x product:%g/0x%08x} Go length=%d scup=%d magsgn=%d sha256=%x Native length=%d scup=%d magsgn=%d sha256=%x Go=% X Native=% X",
			index,
			first,
			acceptedRangeByteDiffCount(goBlocks[index].Data, nativeBlocks[index].Data),
			nativeBlocks[index].Layer,
			nativeBlocks[index].Resolution,
			nativeBlocks[index].Component,
			nativeBlocks[index].Precinct,
			nativeBlocks[index].InclusionIndex,
			nativeBlocks[index].Band,
			nativeBlocks[index].CBX,
			nativeBlocks[index].CBY,
			nativeBlocks[index].X0,
			nativeBlocks[index].Y0,
			nativeBlocks[index].Width,
			nativeBlocks[index].Height,
			bandNumbps,
			nativeBlocks[index].ZeroBitplanes,
			nativeBlocks[index].NumPasses,
			coefficientIndex,
			coefficientX,
			coefficientY,
			globalCoefficientX,
			globalCoefficientY,
			goCoefficients[coefficientIndex],
			nativeCoefficients[coefficientIndex],
			waveletCoefficient,
			math.Float32bits(waveletCoefficient),
			deltaInv,
			math.Float32bits(deltaInv),
			quantizedProduct,
			math.Float32bits(quantizedProduct),
			len(goBlocks[index].Data),
			goScup,
			len(goBlocks[index].Data)-goScup,
			sha256.Sum256(goBlocks[index].Data),
			len(nativeBlocks[index].Data),
			nativeScup,
			len(nativeBlocks[index].Data)-nativeScup,
			sha256.Sum256(nativeBlocks[index].Data),
			prefixAt(goBlocks[index].Data, first, 12),
			prefixAt(nativeBlocks[index].Data, first, 12),
		)
	}
}

func acceptedRangeGoQuantizationTrace(t *testing.T, encoder *Encoder, component, resolution, band, x, y int) (float32, float32, float32) {
	t.Helper()
	if component < 0 || component >= len(encoder.irreversibleData) {
		t.Fatalf("irreversible component index = %d, component count %d", component, len(encoder.irreversibleData))
	}
	coefficients := append([]float32(nil), encoder.irreversibleData[component]...)
	wavelet.ForwardMultilevel97Float32WithParity(
		coefficients, encoder.params.Width, encoder.params.Height,
		encoder.params.NumLevels, 0, 0,
	)
	quantization := encoder.quantizationInfo()
	stepIndex := subbandIndex(encoder.params.NumLevels, resolution, band)
	state := openJPHIrreversibleSubbandState(quantization.steps[stepIndex], quantization.guardBits, band)
	coefficient := coefficients[y*encoder.params.Width+x]
	product := coefficient * state.deltaInv
	return coefficient, state.deltaInv, product
}

type acceptedRangeCleanupBlock struct {
	Data           []byte
	Layer          int
	Resolution     int
	Component      int
	Precinct       int
	InclusionIndex int
	ZeroBitplanes  int
	NumPasses      int
	Band           int
	CBX            int
	CBY            int
	X0             int
	Y0             int
	Width          int
	Height         int
}

type acceptedRangeBlockGeometry struct {
	Band   int
	CBX    int
	CBY    int
	X0     int
	Y0     int
	Width  int
	Height int
}

func cleanupScup(data []byte) int {
	if len(data) < 2 {
		return 0
	}
	return int(data[len(data)-1])<<4 | int(data[len(data)-2]&0x0f)
}

func acceptedRangeByteDiffCount(left, right []byte) int {
	count := 0
	for index := 0; index < min(len(left), len(right)); index++ {
		if left[index] != right[index] {
			count++
		}
	}
	if len(left) > len(right) {
		count += len(left) - len(right)
	} else {
		count += len(right) - len(left)
	}
	return count
}

func acceptedRangeCleanupBlocks(t *testing.T, data []byte) []acceptedRangeCleanupBlock {
	t.Helper()
	cs, err := codestream.NewParser(data).Parse()
	if err != nil {
		t.Fatalf("parse codestream: %v", err)
	}
	if len(cs.Tiles) != 1 {
		t.Fatalf("tile count = %d, want 1", len(cs.Tiles))
	}
	cod := cs.TileCOD(cs.Tiles[0])
	if cod == nil {
		t.Fatal("missing COD")
	}
	pd := t2.NewPacketDecoder(
		cs.Tiles[0].Data,
		int(cs.SIZ.Csiz),
		int(cod.NumberOfLayers),
		int(cod.NumberOfDecompositionLevels)+1,
		t2.ProgressionOrder(cod.ProgressionOrder),
		cod.CodeBlockStyle,
	)
	cbWidth, cbHeight := cod.CodeBlockSize()
	width := int(cs.SIZ.Xsiz - cs.SIZ.XOsiz)
	height := int(cs.SIZ.Ysiz - cs.SIZ.YOsiz)
	pd.SetImageDimensions(width, height, cbWidth, cbHeight)
	for component := 0; component < int(cs.SIZ.Csiz); component++ {
		pd.SetComponentBounds(component, 0, 0, width, height)
		pd.SetComponentSampling(
			component,
			int(cs.SIZ.Components[component].XRsiz),
			int(cs.SIZ.Components[component].YRsiz),
		)
	}
	if len(cod.PrecinctSizes) > 0 {
		widths := make([]int, len(cod.PrecinctSizes))
		heights := make([]int, len(cod.PrecinctSizes))
		for index, precinct := range cod.PrecinctSizes {
			widths[index] = 1 << precinct.PPx
			heights[index] = 1 << precinct.PPy
		}
		pd.SetPrecinctSizes(widths, heights)
	}
	packets, err := pd.DecodePackets()
	if err != nil {
		t.Fatalf("parse packets: %v", err)
	}
	blocks := make([]acceptedRangeCleanupBlock, 0)
	for _, packet := range packets {
		if packet.PrecinctIndex != 0 {
			t.Fatalf("accepted-range diagnostic only supports precinct 0, got %d", packet.PrecinctIndex)
		}
		geometries := acceptedRangeBlockGeometries(width, height, int(cod.NumberOfDecompositionLevels), cbWidth, cbHeight, packet.ResolutionLevel)
		if len(packet.CodeBlockIncls) != 0 && len(packet.CodeBlockIncls) != len(geometries) {
			t.Fatalf("packet R=%d inclusion count = %d, want geometry count %d", packet.ResolutionLevel, len(packet.CodeBlockIncls), len(geometries))
		}
		for inclusionIndex, block := range packet.CodeBlockIncls {
			if block.Included && len(block.Data) > 0 {
				geometry := geometries[inclusionIndex]
				blocks = append(blocks, acceptedRangeCleanupBlock{
					Data:           append([]byte(nil), block.Data...),
					Layer:          packet.LayerIndex,
					Resolution:     packet.ResolutionLevel,
					Component:      packet.ComponentIndex,
					Precinct:       packet.PrecinctIndex,
					InclusionIndex: inclusionIndex,
					ZeroBitplanes:  block.ZeroBitplanes,
					NumPasses:      block.NumPasses,
					Band:           geometry.Band,
					CBX:            geometry.CBX,
					CBY:            geometry.CBY,
					X0:             geometry.X0,
					Y0:             geometry.Y0,
					Width:          geometry.Width,
					Height:         geometry.Height,
				})
			}
		}
	}
	return blocks
}

func acceptedRangeBlockGeometries(width, height, numLevels, cbWidth, cbHeight, resolution int) []acceptedRangeBlockGeometry {
	geometries := make([]acceptedRangeBlockGeometry, 0)
	for _, band := range bandInfosForResolution(width, height, 0, 0, numLevels, resolution) {
		for cby, y0 := 0, 0; y0 < band.height; cby, y0 = cby+1, y0+cbHeight {
			for cbx, x0 := 0, 0; x0 < band.width; cbx, x0 = cbx+1, x0+cbWidth {
				geometries = append(geometries, acceptedRangeBlockGeometry{
					Band: band.band, CBX: cbx, CBY: cby,
					X0: band.offsetX + x0, Y0: band.offsetY + y0,
					Width: min(cbWidth, band.width-x0), Height: min(cbHeight, band.height-y0),
				})
			}
		}
	}
	return geometries
}

func decodeAcceptedRangeCleanupBlock(t *testing.T, block acceptedRangeCleanupBlock, bandNumbps int) []int32 {
	t.Helper()
	decoder := cleanup.NewDecoder(block.Width, block.Height)
	decoder.SetCodingContext(bandNumbps, block.ZeroBitplanes)
	coefficients, err := decoder.Decode(block.Data, block.NumPasses)
	if err != nil {
		t.Fatalf("decode accepted-range cleanup block: %v", err)
	}
	return coefficients
}

func firstAcceptedRangeCoefficientDiff(left, right []int32) int {
	for index := 0; index < min(len(left), len(right)); index++ {
		if left[index] != right[index] {
			return index
		}
	}
	return min(len(left), len(right))
}

func readAcceptedRangeArtifact(t *testing.T, elements ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(elements...))
	if err != nil {
		t.Fatalf("read accepted-range artifact: %v", err)
	}
	return data
}

func readAcceptedRangeArtifactPath(t *testing.T, root, relativePath string) []byte {
	t.Helper()
	if filepath.IsAbs(relativePath) || strings.Contains(filepath.ToSlash(relativePath), "../") {
		t.Fatalf("accepted-range artifact path escapes bundle: %q", relativePath)
	}
	return readAcceptedRangeArtifact(t, root, filepath.FromSlash(relativePath))
}

func firstAcceptedRangeByteDiff(left, right []byte) int {
	limit := min(len(left), len(right))
	for index := 0; index < limit; index++ {
		if left[index] != right[index] {
			return index
		}
	}
	return limit
}

func prefixAt(data []byte, offset, length int) []byte {
	if offset > len(data) {
		offset = len(data)
	}
	end := min(len(data), offset+length)
	return data[offset:end]
}
