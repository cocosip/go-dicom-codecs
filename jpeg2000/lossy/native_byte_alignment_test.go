package lossy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transcode"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	imagingcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/pixeldata"
)

func TestNativeJpeg2000LossyByteAlignmentFor70(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "test-data", "jpeg2000", "native-alignment")
	sourcePath := filepath.Join(fixtureRoot, "70.dcm")
	nativePath := filepath.Join(fixtureRoot, "70_j2k_lossy.dcm")

	source, err := parser.ParseFile(sourcePath, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	sourcePixelData, err := pixeldata.FromDataset(source.Dataset)
	if err != nil {
		t.Fatalf("read source PixelData: %v", err)
	}
	info := sourcePixelData.FrameInfo()
	t.Logf("Source frame: %dx%d samples=%d bitsAllocated=%d bitsStored=%d pixelRepresentation=%d photometric=%s", info.Width, info.Height, info.SamplesPerPixel, info.BitDepth.BitsAllocated, info.BitDepth.BitsStored, info.PixelRepresentation, info.PhotometricInterpretation.Value)
	manager, err := transcode.NewManager(imagingcodec.GlobalRegistry())
	if err != nil {
		t.Fatalf("create transcode manager: %v", err)
	}
	transcoder, err := manager.NewTranscoder(
		source.TransferSyntax,
		transfer.JPEG2000,
		transcode.WithStrictDICOMVR(false),
	)
	if err != nil {
		t.Fatalf("create transcoder: %v", err)
	}
	sourceDataset := source.Dataset.Clone()
	sourceDataset.SetAutoValidate(false)
	encodedDataset, err := transcoder.Transcode(context.Background(), sourceDataset)
	if err != nil {
		t.Fatalf("encode source with JPEG 2000 lossy: %v", err)
	}
	if !encodedDataset.Contains(tag.PatientAge) {
		t.Fatal("encoded dataset does not preserve Patient Age")
	}
	encodedPixelData, err := pixeldata.FromDataset(encodedDataset)
	if err != nil {
		t.Fatalf("read Go encoded PixelData: %v", err)
	}
	actual, err := encodedPixelData.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("read Go encoded frame: %v", err)
	}

	native, err := parser.ParseFile(nativePath, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		t.Fatalf("parse Native baseline: %v", err)
	}
	nativePixelData, err := pixeldata.FromDataset(native.Dataset)
	if err != nil {
		t.Fatalf("read Native PixelData: %v", err)
	}
	expected, err := nativePixelData.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("read Native encoded frame: %v", err)
	}

	if bytes.Equal(expected, actual) {
		return
	}
	const packetDataOffset = 155 // First byte immediately after the SOD marker.
	if len(expected) >= packetDataOffset+16 && len(actual) >= packetDataOffset+16 {
		t.Logf("First packet bytes: native=% x go=% x", expected[packetDataOffset:packetDataOffset+16], actual[packetDataOffset:packetDataOffset+16])
		t.Logf("First packet header fields: native=%s go=%s", describeSingleCodeBlockPacketHeader(expected[packetDataOffset:]), describeSingleCodeBlockPacketHeader(actual[packetDataOffset:]))
	}
	t.Logf("Native markers: %s", describeMainHeaderMarkers(expected))
	t.Logf("Go markers: %s", describeMainHeaderMarkers(actual))
	t.Logf("First tile payload mismatch: %s", firstByteMismatchAfter(expected, actual, 155))
	t.Fatalf(
		"JPEG 2000 lossy frame differs from Native: native=%d sha256=%x, go=%d sha256=%x, firstMismatch=%s",
		len(expected), sha256.Sum256(expected), len(actual), sha256.Sum256(actual), firstByteMismatch(expected, actual),
	)
}

func describeMainHeaderMarkers(data []byte) string {
	markers := make([]string, 0, 8)
	for offset := 0; offset+1 < len(data); {
		if data[offset] != 0xff {
			return strings.Join(markers, ",") + fmt.Sprintf(",invalid@%d", offset)
		}
		marker := data[offset+1]
		markers = append(markers, fmt.Sprintf("FF%02X@%d", marker, offset))
		if marker == 0x4f {
			offset += 2
			continue
		}
		if marker == 0x90 || marker == 0x93 || marker == 0xd9 {
			break
		}
		if offset+3 >= len(data) {
			return strings.Join(markers, ",") + ",truncated"
		}
		length := int(data[offset+2])<<8 | int(data[offset+3])
		if length < 2 || offset+2+length > len(data) {
			return strings.Join(markers, ",") + fmt.Sprintf(",invalidLength=%d", length)
		}
		offset += 2 + length
	}
	return strings.Join(markers, ",")
}

func firstByteMismatch(expected, actual []byte) string {
	return firstByteMismatchAfter(expected, actual, 0)
}

func firstByteMismatchAfter(expected, actual []byte, start int) string {
	limit := len(expected)
	if len(actual) < limit {
		limit = len(actual)
	}
	for index := start; index < limit; index++ {
		if expected[index] != actual[index] {
			return fmt.Sprintf("offset=%d native=0x%02x go=0x%02x", index, expected[index], actual[index])
		}
	}
	return fmt.Sprintf("length native=%d go=%d", len(expected), len(actual))
}

// describeSingleCodeBlockPacketHeader decodes the first packet for this fixture's
// lowest-resolution precinct, which contains exactly one code-block.
func describeSingleCodeBlockPacketHeader(data []byte) string {
	reader := &packetHeaderBitReader{data: data}
	present := reader.readBits(1)
	included := reader.readBits(1)
	zbp := 0
	for reader.readBits(1) == 0 {
		zbp++
	}
	passes := readPacketHeaderPasses(reader)
	increment := 0
	for reader.readBits(1) == 1 {
		increment++
	}
	lengthBits := 3 + increment + packetFloorLog2(passes)
	length := reader.readBits(lengthBits)
	return fmt.Sprintf("present=%d included=%d zbp=%d passes=%d increment=%d length=%d headerBits=%d", present, included, zbp, passes, increment, length, reader.bitOffset)
}

type packetHeaderBitReader struct {
	data      []byte
	bitOffset int
}

func (r *packetHeaderBitReader) readBits(count int) int {
	value := 0
	for i := 0; i < count; i++ {
		byteOffset := r.bitOffset / 8
		bit := 0
		if byteOffset < len(r.data) {
			bit = int((r.data[byteOffset] >> (7 - (r.bitOffset % 8))) & 1)
		}
		value = (value << 1) | bit
		r.bitOffset++
	}
	return value
}

func readPacketHeaderPasses(reader *packetHeaderBitReader) int {
	if reader.readBits(1) == 0 {
		return 1
	}
	if reader.readBits(1) == 0 {
		return 2
	}
	if value := reader.readBits(2); value != 3 {
		return 3 + value
	}
	if value := reader.readBits(5); value != 31 {
		return 6 + value
	}
	return 37 + reader.readBits(7)
}

func packetFloorLog2(value int) int {
	result := 0
	for value > 1 {
		value >>= 1
		result++
	}
	return result
}
