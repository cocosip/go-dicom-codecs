package rle

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
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

// TestRLECodecStridedRunBoundaries exercises the strided decode paths with
// run lengths on both sides of the 128-byte PackBits chunk boundary.
func TestRLECodecStridedRunBoundaries(t *testing.T) {
	runs := []int{1, 2, 3, 4, 127, 128, 129, 130, 255, 256, 257}
	total := 0
	for _, n := range runs {
		total += n
	}

	t.Run("16-bit mono", func(t *testing.T) {
		data := make([]byte, total*2)
		index := 0
		for value, n := range runs {
			for i := 0; i < n; i++ {
				binary.LittleEndian.PutUint16(data[index*2:], uint16(value*37))
				index++
			}
		}
		info := &dicomcodec.FrameInfo{
			Width: uint16(total), Height: 1,
			SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 16, BitsStored: 16, HighBit: 15},
			PixelRepresentation: pixel.UnsignedPixels, PlanarConfiguration: pixel.InterleavedPlanar,
			PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
		}
		encoded := encodeFrame(t, NewRLECodec(), info, data)
		assertDecodedFrame(t, NewRLECodec(), info, encoded, data)
	})

	t.Run("8-bit RGB", func(t *testing.T) {
		data := make([]byte, total*3)
		index := 0
		for value, n := range runs {
			for i := 0; i < n; i++ {
				data[index*3] = byte(value * 37)
				data[index*3+1] = byte(value*37 + 1)
				data[index*3+2] = byte(value*37 + 2)
				index++
			}
		}
		info := &dicomcodec.FrameInfo{
			Width: uint16(total), Height: 1,
			SamplesPerPixel: 3, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7},
			PixelRepresentation: pixel.UnsignedPixels, PlanarConfiguration: pixel.InterleavedPlanar,
			PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
		}
		encoded := encodeFrame(t, NewRLECodec(), info, data)
		assertDecodedFrame(t, NewRLECodec(), info, encoded, data)
	})
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
