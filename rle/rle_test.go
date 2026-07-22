package rle

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

const (
	photometricMonochrome2 = "MONOCHROME2"
	photometricRGB         = "RGB"
)

var _ = (*Codec)(nil)

func TestRLECodec_Name(t *testing.T) {
	codec := NewRLECodec()
	if codec.Name() != "RLE Lossless" {
		t.Errorf("Name() = %q, want %q", codec.Name(), "RLE Lossless")
	}
}

func TestRLECodec_EncodeDecodeSimple(t *testing.T) {
	codec := NewRLECodec()

	width := uint16(10)
	height := uint16(10)
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricMonochrome2,
	}

	src := newTestPixelData(frameInfo)
	_ = src.AddFrame(pixelData)
	encoded := newTestPixelData(frameInfo)
	if err := codec.Encode(src, encoded, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	encodedData, _ := encoded.GetFrame(0)
	if len(encodedData) == 0 {
		t.Fatal("Encode() produced no data")
	}

	decoded := newTestPixelData(frameInfo)
	if err := codec.Decode(encoded, decoded, nil); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	decodedData, _ := decoded.GetFrame(0)
	if !bytes.Equal(pixelData, decodedData[:len(pixelData)]) {
		t.Error("Decoded data does not match original")
	}
}

func TestRLECodec_EncodeDecodeRepeating(t *testing.T) {
	codec := NewRLECodec()

	width := uint16(20)
	height := uint16(20)
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i/10) % 16
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricMonochrome2,
	}

	src := newTestPixelData(frameInfo)
	_ = src.AddFrame(pixelData)
	encoded := newTestPixelData(frameInfo)
	if err := codec.Encode(src, encoded, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded := newTestPixelData(frameInfo)
	if err := codec.Decode(encoded, decoded, nil); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	decodedData, _ := decoded.GetFrame(0)
	if !bytes.Equal(pixelData, decodedData[:len(pixelData)]) {
		t.Error("Decoded data does not match original")
	}
}

func TestRLECodec_EncodeDecodeRGB(t *testing.T) {
	codec := NewRLECodec()

	width := uint16(10)
	height := uint16(10)
	samplesPerPixel := uint16(3)
	pixelData := make([]byte, int(width)*int(height)*int(samplesPerPixel))
	for i := 0; i < len(pixelData); i += 3 {
		pixelData[i] = byte((i / 3) % 256)
		pixelData[i+1] = byte((i/3 + 50) % 256)
		pixelData[i+2] = byte((i/3 + 100) % 256)
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           samplesPerPixel,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricRGB,
	}

	src := newTestPixelData(frameInfo)
	_ = src.AddFrame(pixelData)
	encoded := newTestPixelData(frameInfo)
	if err := codec.Encode(src, encoded, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded := newTestPixelData(frameInfo)
	if err := codec.Decode(encoded, decoded, nil); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	decodedData, _ := decoded.GetFrame(0)
	if !bytes.Equal(pixelData, decodedData[:len(pixelData)]) {
		t.Error("Decoded RGB data does not match original")
	}
}

func TestRLECodec_Encode16Bit(t *testing.T) {
	codec := NewRLECodec()

	width := uint16(8)
	height := uint16(8)
	pixelData := make([]byte, int(width)*int(height)*2)
	for i := 0; i < len(pixelData); i += 2 {
		value := uint16((i / 2) * 257)
		pixelData[i] = byte(value & 0xFF)
		pixelData[i+1] = byte((value >> 8) & 0xFF)
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             16,
		BitsStored:                16,
		HighBit:                   15,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricMonochrome2,
	}

	src := newTestPixelData(frameInfo)
	_ = src.AddFrame(pixelData)
	encoded := newTestPixelData(frameInfo)
	if err := codec.Encode(src, encoded, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded := newTestPixelData(frameInfo)
	if err := codec.Decode(encoded, decoded, nil); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	decodedData, _ := decoded.GetFrame(0)
	if !bytes.Equal(pixelData, decodedData[:len(pixelData)]) {
		t.Error("Decoded 16-bit data does not match original")
	}
}

func TestRLEEncoder_AllSameValue(t *testing.T) {
	encoder := newRLEEncoder()
	encoder.NextSegment()
	for i := 0; i < 200; i++ {
		encoder.Encode(0xAA)
	}
	encoder.Flush()

	data := encoder.GetBuffer()
	if len(data) == 0 {
		t.Fatal("Encoder produced no data")
	}
	if len(data) > 100 {
		t.Errorf("RLE compression inefficient for repeating data: %d bytes", len(data))
	}
}

func TestRLEEncoder_NoRepetition(t *testing.T) {
	encoder := newRLEEncoder()
	encoder.NextSegment()
	for i := 0; i < 100; i++ {
		encoder.Encode(byte(i))
	}
	encoder.Flush()

	data := encoder.GetBuffer()
	if len(data) == 0 {
		t.Fatal("Encoder produced no data")
	}
}

func TestRLECodec_TransferSyntax(t *testing.T) {
	codec := NewRLECodec()
	ts := codec.TransferSyntax()
	if ts == nil {
		t.Fatal("TransferSyntax() returned nil")
	}
	if ts.UID().UID() != "1.2.840.10008.1.2.5" {
		t.Errorf("UID = %s, want %s", ts.UID().UID(), "1.2.840.10008.1.2.5")
	}
	if !ts.IsExplicitVR() {
		t.Error("RLE should use Explicit VR")
	}
	if !ts.IsEncapsulated() {
		t.Error("RLE should be encapsulated")
	}
}

func TestRLEDecoderRepeatRunReturnsErrorBeforeOverflow(t *testing.T) {
	data := make([]byte, 64)
	binary.LittleEndian.PutUint32(data[0:4], 1)
	binary.LittleEndian.PutUint32(data[4:8], 64)
	data = append(data, byte(0x81), 0x7F)

	decoder, err := newRLEDecoder(data)
	if err != nil {
		t.Fatalf("newRLEDecoder() error = %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DecodeSegment panicked instead of returning an error: %v", r)
		}
	}()

	err = decoder.DecodeSegment(0, make([]byte, 127), 0, 1)
	if err == nil {
		t.Fatal("DecodeSegment() error = nil, want output overflow error")
	}
}
