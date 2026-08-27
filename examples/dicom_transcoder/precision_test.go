package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
	"github.com/cocosip/go-dicom/pkg/imaging"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func TestTranscode12BitPixelsWritesStoredPrecisionToHTJ2K(t *testing.T) {
	const width, height = 64, 64
	pixels := make([]byte, width*height*2)
	values := []uint16{0x000, 0x001, 0x7ff, 0x800, 0xfff}
	for index := 0; index < width*height; index++ {
		value := values[index%len(values)]
		pixels[index*2] = byte(value)
		pixels[index*2+1] = byte(value >> 8)
	}

	ds := dataset.NewWithTransferSyntax(transfer.ExplicitVRLittleEndian)
	addTestElement(t, ds, element.NewString(tag.SOPClassUID, vr.UI, []string{"1.2.840.10008.5.1.4.1.1.7"}))
	addTestElement(t, ds, element.NewString(tag.SOPInstanceUID, vr.UI, []string{"1.2.826.0.1.3680043.10.543.1"}))
	addTestElement(t, ds, element.NewUnsignedShort(tag.Rows, []uint16{height}))
	addTestElement(t, ds, element.NewUnsignedShort(tag.Columns, []uint16{width}))
	addTestElement(t, ds, element.NewUnsignedShort(tag.BitsAllocated, []uint16{16}))
	addTestElement(t, ds, element.NewUnsignedShort(tag.BitsStored, []uint16{12}))
	addTestElement(t, ds, element.NewUnsignedShort(tag.HighBit, []uint16{11}))
	addTestElement(t, ds, element.NewUnsignedShort(tag.SamplesPerPixel, []uint16{1}))
	addTestElement(t, ds, element.NewUnsignedShort(tag.PixelRepresentation, []uint16{0}))
	addTestElement(t, ds, element.NewString(tag.PhotometricInterpretation, vr.CS, []string{"MONOCHROME2"}))
	addTestElement(t, ds, element.NewOtherWord(tag.PixelData, pixels))

	outputPath := filepath.Join(t.TempDir(), "stored-precision-12-htj2k.dcm")
	registry := codec.GetGlobalRegistry()
	if err := transcodeDICOMFile(ds, outputPath, transfer.ExplicitVRLittleEndian, transfer.HTJ2KLossless, registry); err != nil {
		t.Fatalf("transcode native DICOM to HTJ2K: %v", err)
	}

	parsed, err := parser.ParseFile(outputPath, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		t.Fatalf("parse transcoded HTJ2K DICOM: %v", err)
	}
	if got := parsed.Dataset.TryGetUInt16(tag.BitsAllocated, 0); got != 16 {
		t.Fatalf("BitsAllocated = %d, want 16", got)
	}
	if got := parsed.Dataset.TryGetUInt16(tag.BitsStored, 0); got != 12 {
		t.Fatalf("BitsStored = %d, want 12", got)
	}
	if got := parsed.Dataset.TryGetUInt16(tag.HighBit, 0); got != 11 {
		t.Fatalf("HighBit = %d, want 11", got)
	}

	encodedPixelData, err := imaging.CreatePixelData(parsed.Dataset)
	if err != nil {
		t.Fatalf("create encapsulated pixel data: %v", err)
	}
	encodedFrame, err := encodedPixelData.GetFrame(0)
	if err != nil {
		t.Fatalf("read encapsulated frame: %v", err)
	}
	ssiz, err := firstComponentSsiz(encodedFrame)
	if err != nil {
		t.Fatalf("read HTJ2K SIZ precision: %v", err)
	}
	if ssiz != 0x0b {
		t.Fatalf("HTJ2K Ssiz = 0x%02X, want unsigned 12-bit 0x0B", ssiz)
	}

	decoder := codec.NewTranscoder(parsed.TransferSyntax, transfer.ExplicitVRLittleEndian, codec.WithCodecRegistry(registry))
	decodedDataset, err := decoder.Transcode(parsed.Dataset)
	if err != nil {
		t.Fatalf("transcode HTJ2K DICOM back to native: %v", err)
	}
	decodedPixelData, err := imaging.CreatePixelData(decodedDataset)
	if err != nil {
		t.Fatalf("create decoded pixel data: %v", err)
	}
	decodedFrame, err := decodedPixelData.GetFrame(0)
	if err != nil {
		t.Fatalf("read decoded frame: %v", err)
	}
	if !bytes.Equal(decodedFrame, pixels) {
		t.Fatalf("decoded pixels differ at byte %d", firstDifferentByte(decodedFrame, pixels))
	}
}

func addTestElement(t *testing.T, ds *dataset.Dataset, value element.Element) {
	t.Helper()
	if err := ds.Add(value); err != nil {
		t.Fatalf("add DICOM element %s: %v", value.Tag(), err)
	}
}

func firstDifferentByte(left, right []byte) int {
	limit := min(len(left), len(right))
	for index := 0; index < limit; index++ {
		if left[index] != right[index] {
			return index
		}
	}
	return limit
}

func firstComponentSsiz(frame []byte) (byte, error) {
	const (
		socMarker = 0xff4f
		sizMarker = 0xff51
	)
	if len(frame) < 43 {
		return 0, fmt.Errorf("codestream is too short: %d bytes", len(frame))
	}
	if marker := binary.BigEndian.Uint16(frame[0:2]); marker != socMarker {
		return 0, fmt.Errorf("missing SOC marker: got 0x%04X", marker)
	}
	if marker := binary.BigEndian.Uint16(frame[2:4]); marker != sizMarker {
		return 0, fmt.Errorf("missing SIZ marker after SOC: got 0x%04X", marker)
	}

	segmentLength := int(binary.BigEndian.Uint16(frame[4:6]))
	componentCount := int(binary.BigEndian.Uint16(frame[40:42]))
	if componentCount < 1 {
		return 0, fmt.Errorf("SIZ has no components")
	}
	expectedLength := 38 + 3*componentCount
	if segmentLength != expectedLength {
		return 0, fmt.Errorf("invalid SIZ length: got %d, want %d", segmentLength, expectedLength)
	}
	if len(frame) < 4+segmentLength {
		return 0, fmt.Errorf("truncated SIZ segment: got %d bytes, need %d", len(frame), 4+segmentLength)
	}
	return frame[42], nil
}
