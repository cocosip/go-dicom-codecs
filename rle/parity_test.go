package rle

import (
	"bytes"
	"context"
	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
	"testing"
)

func TestRLECodecRoundTripPixelLayouts(t *testing.T) {
	tests := []struct {
		name string
		info *dicomcodec.FrameInfo
		data []byte
	}{
		{
			name: "8-bit monochrome",
			info: &dicomcodec.FrameInfo{
				Width: 10, Height: 10,
				SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
			},
			data: patternedBytes(100, 1),
		},
		{
			name: "16-bit monochrome",
			info: &dicomcodec.FrameInfo{
				Width: 8, Height: 8,
				SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 16, BitsStored: 16, HighBit: 15, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
			},
			data: patternedBytes(128, 3),
		},
		{
			name: "8-bit RGB interleaved",
			info: &dicomcodec.FrameInfo{
				Width: 8, Height: 8,
				SamplesPerPixel: 3, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
			},
			data: patternedBytes(192, 5),
		},
		{
			name: "8-bit RGB planar",
			info: &dicomcodec.FrameInfo{
				Width: 8, Height: 8,
				SamplesPerPixel: 3, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(1), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
			},
			data: patternedBytes(192, 7),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded := encodeFrame(t, NewRLECodec(), test.info, test.data)
			assertDecodedFrame(t, NewRLECodec(), test.info, encoded, test.data)
		})
	}
}

func patternedBytes(length, factor int) []byte {
	result := make([]byte, length)
	for i := range result {
		result[i] = byte((i * factor) % 251)
	}
	return result
}

func encodeFrame(t *testing.T, codec *Codec, info *dicomcodec.FrameInfo, frame []byte) []byte {
	t.Helper()
	source := newTestPixelData(info)
	_ = source.AddFrame(context.Background(), frame)
	destination := newTestPixelData(info)
	if err := codec.Encode(context.Background(), source, destination, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	encoded, err := destination.Frame(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func assertDecodedFrame(t *testing.T, codec *Codec, info *dicomcodec.FrameInfo, encoded, want []byte) {
	t.Helper()
	source := newTestPixelData(info)
	_ = source.AddFrame(context.Background(), encoded)
	destination := newTestPixelData(info)
	if err := codec.Decode(context.Background(), source, destination, nil); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	got, err := destination.Frame(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[:len(want)], want) {
		t.Fatal("decoded pixels differ from source frame")
	}
}
