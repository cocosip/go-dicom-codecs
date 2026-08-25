package openjph

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k/openjph/t2"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

func TestAcceptedRangePacketAndTilePartStructureMatchesOpenJPH(t *testing.T) {
	root := acceptedRangeBundleRoot()
	manifest := readAcceptedRangeManifest(t, root)
	assertAcceptedRangeFixtureMatrix(t, manifest)

	for _, fixture := range manifest.Fixtures {
		fixture := fixture
		for _, syntax := range fixture.Syntaxes {
			syntax := syntax
			t.Run(fmt.Sprintf("%s/%s", fixture.Image.Name, syntax.Name), func(t *testing.T) {
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

						goStructure := parseAcceptedRangeRawStructure(t, got)
						nativeStructure := parseAcceptedRangeRawStructure(t, native)
						t.Run("tlm", func(t *testing.T) {
							assertAcceptedRangeTLMEntries(t, goStructure.TLMEntries, nativeStructure.TLMEntries)
						})
						t.Run("tile-parts", func(t *testing.T) {
							assertAcceptedRangeTileParts(t, goStructure.TileParts, nativeStructure.TileParts)
						})
						t.Run("packets", func(t *testing.T) {
							assertAcceptedRangePackets(t, got, native)
						})
						t.Run("codestream", func(t *testing.T) {
							if bytes.Equal(got, native) {
								return
							}
							first := firstAcceptedRangeByteDiff(got, native)
							t.Fatalf(
								"codestream differs at offset %d: Go length=%d Native length=%d Go=% X Native=% X",
								first, len(got), len(native), prefixAt(got, first, 16), prefixAt(native, first, 16),
							)
						})
					})
				}
			})
		}
	}
}

func acceptedRangeBundleRoot() string {
	return "../../../test-data/htj2k/interop-v1"
}

type acceptedRangeTLMEntry struct {
	SegmentIndex byte
	EntryIndex   int
	TileIndex    int
	Length       uint32
}

type acceptedRangeTilePart struct {
	Offset    int
	TileIndex uint16
	Length    uint32
	PartIndex byte
	PartCount byte
	Header    []byte
	SODOffset int
	Data      []byte
}

type acceptedRangeRawStructure struct {
	TLMEntries []acceptedRangeTLMEntry
	TileParts  []acceptedRangeTilePart
}

func parseAcceptedRangeRawStructure(t *testing.T, data []byte) acceptedRangeRawStructure {
	t.Helper()
	if len(data) < 2 || binary.BigEndian.Uint16(data) != codestream.MarkerSOC {
		t.Fatal("accepted-range codestream does not start with SOC")
	}

	structure := acceptedRangeRawStructure{}
	offset := 2
	for {
		if offset+2 > len(data) {
			t.Fatal("accepted-range main header is truncated before SOT")
		}
		marker := binary.BigEndian.Uint16(data[offset:])
		if marker == codestream.MarkerSOT {
			break
		}
		segmentEnd := acceptedRangeMarkerSegmentEnd(t, data, offset)
		if marker == codestream.MarkerTLM {
			structure.TLMEntries = append(structure.TLMEntries, parseAcceptedRangeTLMSegment(t, data[offset:segmentEnd])...)
		}
		offset = segmentEnd
	}

	for offset < len(data) {
		if offset+2 <= len(data) && binary.BigEndian.Uint16(data[offset:]) == codestream.MarkerEOC {
			if offset+2 != len(data) {
				t.Fatalf("%d trailing bytes after EOC", len(data)-(offset+2))
			}
			return structure
		}
		part := parseAcceptedRangeTilePart(t, data, offset)
		structure.TileParts = append(structure.TileParts, part)
		offset += int(part.Length)
	}

	t.Fatal("accepted-range codestream is missing EOC")
	return acceptedRangeRawStructure{}
}

func acceptedRangeMarkerSegmentEnd(t *testing.T, data []byte, offset int) int {
	t.Helper()
	if offset+4 > len(data) {
		t.Fatalf("marker at offset %d has a truncated length", offset)
	}
	length := int(binary.BigEndian.Uint16(data[offset+2:]))
	end := offset + 2 + length
	if length < 2 || end > len(data) {
		t.Fatalf("marker at offset %d has invalid length %d", offset, length)
	}
	return end
}

func parseAcceptedRangeTLMSegment(t *testing.T, segment []byte) []acceptedRangeTLMEntry {
	t.Helper()
	if len(segment) < 6 || binary.BigEndian.Uint16(segment) != codestream.MarkerTLM {
		t.Fatal("invalid accepted-range TLM segment")
	}
	segmentIndex := segment[4]
	style := segment[5]
	tileIndexBytes := int((style >> 4) & 0x03)
	if tileIndexBytes == 3 {
		t.Fatalf("TLM segment %d uses reserved tile-index style 3", segmentIndex)
	}
	lengthBytes := 2
	if style&0x40 != 0 {
		lengthBytes = 4
	}
	entryBytes := tileIndexBytes + lengthBytes
	if entryBytes == 0 || (len(segment)-6)%entryBytes != 0 {
		t.Fatalf("TLM segment %d payload length %d is not divisible by entry size %d", segmentIndex, len(segment)-6, entryBytes)
	}

	entries := make([]acceptedRangeTLMEntry, 0, (len(segment)-6)/entryBytes)
	for offset, entryIndex := 6, 0; offset < len(segment); offset, entryIndex = offset+entryBytes, entryIndex+1 {
		entry := acceptedRangeTLMEntry{SegmentIndex: segmentIndex, EntryIndex: entryIndex, TileIndex: -1}
		switch tileIndexBytes {
		case 1:
			entry.TileIndex = int(segment[offset])
		case 2:
			entry.TileIndex = int(binary.BigEndian.Uint16(segment[offset:]))
		}
		lengthOffset := offset + tileIndexBytes
		if lengthBytes == 2 {
			entry.Length = uint32(binary.BigEndian.Uint16(segment[lengthOffset:]))
		} else {
			entry.Length = binary.BigEndian.Uint32(segment[lengthOffset:])
		}
		entries = append(entries, entry)
	}
	return entries
}

func parseAcceptedRangeTilePart(t *testing.T, data []byte, offset int) acceptedRangeTilePart {
	t.Helper()
	if offset+12 > len(data) || binary.BigEndian.Uint16(data[offset:]) != codestream.MarkerSOT {
		t.Fatalf("expected SOT at offset %d", offset)
	}
	if length := binary.BigEndian.Uint16(data[offset+2:]); length != 10 {
		t.Fatalf("SOT at offset %d has length %d, want 10", offset, length)
	}
	part := acceptedRangeTilePart{
		Offset:    offset,
		TileIndex: binary.BigEndian.Uint16(data[offset+4:]),
		Length:    binary.BigEndian.Uint32(data[offset+6:]),
		PartIndex: data[offset+10],
		PartCount: data[offset+11],
	}
	if part.Length < 14 || uint64(offset)+uint64(part.Length) > uint64(len(data)) {
		t.Fatalf("tile-part T=%d P=%d at offset %d has invalid Psot %d", part.TileIndex, part.PartIndex, offset, part.Length)
	}

	partEnd := offset + int(part.Length)
	headerOffset := offset + 12
	markerOffset := headerOffset
	for {
		if markerOffset+2 > partEnd {
			t.Fatalf("tile-part T=%d P=%d is missing SOD", part.TileIndex, part.PartIndex)
		}
		marker := binary.BigEndian.Uint16(data[markerOffset:])
		if marker == codestream.MarkerSOD {
			break
		}
		markerOffset = acceptedRangeMarkerSegmentEnd(t, data[:partEnd], markerOffset)
	}
	part.Header = append([]byte(nil), data[headerOffset:markerOffset]...)
	part.SODOffset = markerOffset - offset
	part.Data = append([]byte(nil), data[markerOffset+2:partEnd]...)
	return part
}

func assertAcceptedRangeTLMEntries(t *testing.T, got, native []acceptedRangeTLMEntry) {
	t.Helper()
	if len(got) != len(native) {
		t.Fatalf("TLM entry count = %d, want Native %d", len(got), len(native))
	}
	for index := range native {
		if got[index] != native[index] {
			t.Errorf("TLM entry %d differs: Go=%+v Native=%+v", index, got[index], native[index])
		}
	}
}

func assertAcceptedRangeTileParts(t *testing.T, got, native []acceptedRangeTilePart) {
	t.Helper()
	if len(got) != len(native) {
		t.Fatalf("tile-part count = %d, want Native %d", len(got), len(native))
	}
	for index := range native {
		goPart := got[index]
		nativePart := native[index]
		coordinate := fmt.Sprintf("T=%d P=%d order=%d", nativePart.TileIndex, nativePart.PartIndex, index)
		if goPart.TileIndex != nativePart.TileIndex || goPart.PartIndex != nativePart.PartIndex || goPart.PartCount != nativePart.PartCount {
			t.Errorf("tile-part %s identity differs: Go={T:%d P:%d N:%d} Native={T:%d P:%d N:%d}", coordinate, goPart.TileIndex, goPart.PartIndex, goPart.PartCount, nativePart.TileIndex, nativePart.PartIndex, nativePart.PartCount)
		}
		if goPart.Length != nativePart.Length {
			t.Errorf("tile-part %s Psot = %d, want Native %d", coordinate, goPart.Length, nativePart.Length)
		}
		if goPart.SODOffset != nativePart.SODOffset {
			t.Errorf("tile-part %s SOD relative offset = %d, want Native %d", coordinate, goPart.SODOffset, nativePart.SODOffset)
		}
		if !bytes.Equal(goPart.Header, nativePart.Header) {
			first := firstAcceptedRangeByteDiff(goPart.Header, nativePart.Header)
			t.Errorf("tile-part %s header differs at byte %d: Go=% X Native=% X", coordinate, first, prefixAt(goPart.Header, first, 12), prefixAt(nativePart.Header, first, 12))
		}
		if !bytes.Equal(goPart.Data, nativePart.Data) {
			first := firstAcceptedRangeByteDiff(goPart.Data, nativePart.Data)
			t.Errorf("tile-part %s data differs at byte %d: Go length=%d Native length=%d Go=% X Native=% X", coordinate, first, len(goPart.Data), len(nativePart.Data), prefixAt(goPart.Data, first, 12), prefixAt(nativePart.Data, first, 12))
		}
	}
}

func assertAcceptedRangePackets(t *testing.T, got, native []byte) {
	t.Helper()
	goPackets := decodeAcceptedRangePackets(t, got)
	nativePackets := decodeAcceptedRangePackets(t, native)
	if len(goPackets) != len(nativePackets) {
		t.Fatalf("packet count = %d, want Native %d", len(goPackets), len(nativePackets))
	}
	for index := range nativePackets {
		goPacket := goPackets[index]
		nativePacket := nativePackets[index]
		coordinate := fmt.Sprintf("L=%d R=%d C=%d P=%d order=%d", nativePacket.LayerIndex, nativePacket.ResolutionLevel, nativePacket.ComponentIndex, nativePacket.PrecinctIndex, index)
		if goPacket.LayerIndex != nativePacket.LayerIndex || goPacket.ResolutionLevel != nativePacket.ResolutionLevel || goPacket.ComponentIndex != nativePacket.ComponentIndex || goPacket.PrecinctIndex != nativePacket.PrecinctIndex {
			t.Fatalf("packet %s coordinate differs: Go={L:%d R:%d C:%d P:%d}", coordinate, goPacket.LayerIndex, goPacket.ResolutionLevel, goPacket.ComponentIndex, goPacket.PrecinctIndex)
		}
		if goPacket.HeaderPresent != nativePacket.HeaderPresent {
			t.Errorf("packet %s header-present = %t, want Native %t", coordinate, goPacket.HeaderPresent, nativePacket.HeaderPresent)
		}
		if !bytes.Equal(goPacket.Header, nativePacket.Header) {
			first := firstAcceptedRangeByteDiff(goPacket.Header, nativePacket.Header)
			t.Errorf("packet %s header differs at byte %d: Go=% X Native=% X", coordinate, first, prefixAt(goPacket.Header, first, 12), prefixAt(nativePacket.Header, first, 12))
		}
		assertAcceptedRangePacketInclusions(t, coordinate, goPacket.CodeBlockIncls, nativePacket.CodeBlockIncls)
		if !bytes.Equal(goPacket.Body, nativePacket.Body) {
			first := firstAcceptedRangeByteDiff(goPacket.Body, nativePacket.Body)
			t.Errorf("packet %s body differs at byte %d: Go length=%d Native length=%d Go=% X Native=% X", coordinate, first, len(goPacket.Body), len(nativePacket.Body), prefixAt(goPacket.Body, first, 12), prefixAt(nativePacket.Body, first, 12))
		}
	}
}

func assertAcceptedRangePacketInclusions(t *testing.T, packetCoordinate string, got, native []t2.CodeBlockIncl) {
	t.Helper()
	if len(got) != len(native) {
		t.Errorf("packet %s inclusion count = %d, want Native %d", packetCoordinate, len(got), len(native))
		return
	}
	for index := range native {
		goIncl := got[index]
		nativeIncl := native[index]
		coordinate := fmt.Sprintf("%s CB=%d", packetCoordinate, index)
		if goIncl.Included != nativeIncl.Included || goIncl.FirstInclusion != nativeIncl.FirstInclusion || goIncl.ZeroBitplanes != nativeIncl.ZeroBitplanes || goIncl.NumPasses != nativeIncl.NumPasses || goIncl.DataLength != nativeIncl.DataLength || goIncl.UseTERMALL != nativeIncl.UseTERMALL {
			t.Errorf("packet inclusion %s differs: Go={included:%t first:%t zbp:%d passes:%d length:%d termall:%t} Native={included:%t first:%t zbp:%d passes:%d length:%d termall:%t}", coordinate, goIncl.Included, goIncl.FirstInclusion, goIncl.ZeroBitplanes, goIncl.NumPasses, goIncl.DataLength, goIncl.UseTERMALL, nativeIncl.Included, nativeIncl.FirstInclusion, nativeIncl.ZeroBitplanes, nativeIncl.NumPasses, nativeIncl.DataLength, nativeIncl.UseTERMALL)
		}
		if !reflect.DeepEqual(goIncl.PassLengths, nativeIncl.PassLengths) {
			t.Errorf("packet inclusion %s pass lengths = %v, want Native %v", coordinate, goIncl.PassLengths, nativeIncl.PassLengths)
		}
		if !bytes.Equal(goIncl.Data, nativeIncl.Data) {
			first := firstAcceptedRangeByteDiff(goIncl.Data, nativeIncl.Data)
			t.Errorf("packet inclusion %s data differs at byte %d: Go=% X Native=% X", coordinate, first, prefixAt(goIncl.Data, first, 12), prefixAt(nativeIncl.Data, first, 12))
		}
	}
}

func decodeAcceptedRangePackets(t *testing.T, data []byte) []t2.Packet {
	t.Helper()
	cs, err := codestream.NewParser(data).Parse()
	if err != nil {
		t.Fatalf("parse codestream for packets: %v", err)
	}
	if len(cs.Tiles) != 1 {
		t.Fatalf("tile count = %d, want 1", len(cs.Tiles))
	}
	cod := cs.TileCOD(cs.Tiles[0])
	if cod == nil {
		t.Fatal("missing COD")
	}
	pd := t2.NewPacketDecoder(cs.Tiles[0].Data, int(cs.SIZ.Csiz), int(cod.NumberOfLayers), int(cod.NumberOfDecompositionLevels)+1, t2.ProgressionOrder(cod.ProgressionOrder), cod.CodeBlockStyle)
	cbWidth, cbHeight := cod.CodeBlockSize()
	width := int(cs.SIZ.Xsiz - cs.SIZ.XOsiz)
	height := int(cs.SIZ.Ysiz - cs.SIZ.YOsiz)
	pd.SetImageDimensions(width, height, cbWidth, cbHeight)
	for component := 0; component < int(cs.SIZ.Csiz); component++ {
		pd.SetComponentBounds(component, 0, 0, width, height)
		pd.SetComponentSampling(component, int(cs.SIZ.Components[component].XRsiz), int(cs.SIZ.Components[component].YRsiz))
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
		t.Fatalf("decode packets: %v", err)
	}
	return packets
}
