package rle

import (
	"bytes"
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func TestRLECodecRoundTripPixelLayouts(t *testing.T) {
	tests := []struct {
		name string
		info *imagetypes.FrameInfo
		data []byte
	}{
		{
			name: "8-bit monochrome",
			info: &imagetypes.FrameInfo{
				Width: 10, Height: 10, BitsAllocated: 8, BitsStored: 8, HighBit: 7,
				SamplesPerPixel: 1, PlanarConfiguration: 0, PhotometricInterpretation: photometricMonochrome2,
			},
			data: patternedBytes(100, 1),
		},
		{
			name: "16-bit monochrome",
			info: &imagetypes.FrameInfo{
				Width: 8, Height: 8, BitsAllocated: 16, BitsStored: 16, HighBit: 15,
				SamplesPerPixel: 1, PlanarConfiguration: 0, PhotometricInterpretation: photometricMonochrome2,
			},
			data: patternedBytes(128, 3),
		},
		{
			name: "8-bit RGB interleaved",
			info: &imagetypes.FrameInfo{
				Width: 8, Height: 8, BitsAllocated: 8, BitsStored: 8, HighBit: 7,
				SamplesPerPixel: 3, PlanarConfiguration: 0, PhotometricInterpretation: photometricRGB,
			},
			data: patternedBytes(192, 5),
		},
		{
			name: "8-bit RGB planar",
			info: &imagetypes.FrameInfo{
				Width: 8, Height: 8, BitsAllocated: 8, BitsStored: 8, HighBit: 7,
				SamplesPerPixel: 3, PlanarConfiguration: 1, PhotometricInterpretation: photometricRGB,
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

func encodeFrame(t *testing.T, codec *Codec, info *imagetypes.FrameInfo, frame []byte) []byte {
	t.Helper()
	source := newTestPixelData(info)
	_ = source.AddFrame(frame)
	destination := newTestPixelData(info)
	if err := codec.Encode(source, destination, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	encoded, err := destination.GetFrame(0)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func assertDecodedFrame(t *testing.T, codec *Codec, info *imagetypes.FrameInfo, encoded, want []byte) {
	t.Helper()
	source := newTestPixelData(info)
	_ = source.AddFrame(encoded)
	destination := newTestPixelData(info)
	if err := codec.Decode(source, destination, nil); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	got, err := destination.GetFrame(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[:len(want)], want) {
		t.Fatal("decoded pixels differ from source frame")
	}
}
