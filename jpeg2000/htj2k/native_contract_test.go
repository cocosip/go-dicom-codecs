package htj2k

import (
	"bytes"
	"encoding/binary"
	"testing"

	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/codestream"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

const (
	nativeHTJ2KLosslessCapability = uint16(0x000A)
	nativeHTJ2KLossyCapability    = uint16(0x002A)
)

func TestHTJ2KNativeHeaderContractUsesAllocatedPrecisionAndCapabilities(t *testing.T) {
	tests := []struct {
		name       string
		codec      *Codec
		capability uint16
	}{
		{name: "201 lossless", codec: NewLosslessCodec(), capability: nativeHTJ2KLosslessCapability},
		{name: "202 lossless rpcl", codec: NewLosslessRPCLCodec(), capability: nativeHTJ2KLosslessCapability},
		{name: "203 lossy", codec: NewCodec(80), capability: nativeHTJ2KLossyCapability},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := encodeNativeContractFrame(t, tt.codec)
			cs, err := codestream.NewParser(encoded).Parse()
			if err != nil {
				t.Fatalf("parse encoded codestream: %v", err)
			}
			if got := cs.SIZ.Components[0].BitDepth(); got != 16 {
				t.Fatalf("SIZ precision = %d, want BitsAllocated 16 used by fo-dicom Native", got)
			}
			if got := nativeCAPCapability(t, encoded); got != tt.capability {
				t.Fatalf("CAP capability = 0x%04X, want Native 0x%04X", got, tt.capability)
			}
			if got := nativeTLMEntryCount(t, encoded); got != 6 {
				t.Fatalf("TLM tile-part entries = %d, want 6 (one for each Native resolution)", got)
			}
		})
	}
}

func TestHTJ2KNativeQCDContract(t *testing.T) {
	tests := []struct {
		name  string
		codec *Codec
		want  []byte
	}{
		{
			name:  "201 lossless",
			codec: NewLosslessCodec(),
			want:  []byte{0x20, 0x88, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x88, 0x88, 0x88},
		},
		{
			name:  "202 lossless rpcl",
			codec: NewLosslessRPCLCodec(),
			want:  []byte{0x20, 0x88, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x88, 0x88, 0x88},
		},
		{
			name:  "203 lossy",
			codec: NewCodec(80),
			want:  []byte{0x22, 0xB7, 0x18, 0xB6, 0xEA, 0xB6, 0xEA, 0xB6, 0xBC, 0xAF, 0x00, 0xAF, 0x00, 0xAE, 0xE2, 0xA7, 0x4C, 0xA7, 0x4C, 0xA7, 0x64, 0x90, 0x03, 0x90, 0x03, 0x90, 0x46, 0x97, 0xD2, 0x97, 0xD2, 0x97, 0x61},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := markerPayload(t, encodeNativeContractFrame(t, tt.codec), 0x5C); !bytes.Equal(got, tt.want) {
				t.Fatalf("QCD payload = % X, want Native % X", got, tt.want)
			}
		})
	}
}

func TestHTJ2KNativeRCTQCDContract(t *testing.T) {
	encoded := encodeNativeContractRGBFrame(t, NewLosslessCodec())
	want := []byte{0x20, 0x90, 0x98, 0x98, 0x98, 0x98, 0x98, 0x98, 0x98, 0x98, 0x98, 0x98, 0x98, 0x98, 0x90, 0x90, 0x90}
	if got := markerPayload(t, encoded, 0x5C); !bytes.Equal(got, want) {
		t.Fatalf("RGB RCT QCD payload = % X, want Native % X", got, want)
	}
}

func TestHTJ2KNativeCOMContract(t *testing.T) {
	for _, htCodec := range []*Codec{NewLosslessCodec(), NewLosslessRPCLCodec(), NewCodec(80)} {
		payload := markerPayload(t, encodeNativeContractFrame(t, htCodec), 0x64)
		if got, want := string(payload), "\x00\x01OpenJPH Ver 0.21.2."; got != want {
			t.Fatalf("COM payload = %q, want Native %q", got, want)
		}
	}
}

func encodeNativeContractFrame(t *testing.T, htCodec *Codec) []byte {
	t.Helper()
	frameInfo := &imagetypes.FrameInfo{
		Width:                     288,
		Height:                    288,
		BitsAllocated:             16,
		BitsStored:                12,
		HighBit:                   11,
		SamplesPerPixel:           1,
		PhotometricInterpretation: photometricMonochrome2,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(make([]byte, int(frameInfo.Width)*int(frameInfo.Height)*2)); err != nil {
		t.Fatalf("add source frame: %v", err)
	}
	dst := codecHelpers.NewTestPixelData(frameInfo)
	if err := htCodec.Encode(src, dst, nil); err != nil {
		t.Fatalf("encode frame: %v", err)
	}
	encoded, err := dst.GetFrame(0)
	if err != nil {
		t.Fatalf("read encoded frame: %v", err)
	}
	return encoded
}

func encodeNativeContractRGBFrame(t *testing.T, htCodec *Codec) []byte {
	t.Helper()
	frameInfo := &imagetypes.FrameInfo{
		Width:                     288,
		Height:                    288,
		BitsAllocated:             16,
		BitsStored:                12,
		HighBit:                   11,
		SamplesPerPixel:           3,
		PhotometricInterpretation: photometricRGB,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(make([]byte, int(frameInfo.Width)*int(frameInfo.Height)*int(frameInfo.SamplesPerPixel)*2)); err != nil {
		t.Fatalf("add source frame: %v", err)
	}
	dst := codecHelpers.NewTestPixelData(frameInfo)
	if err := htCodec.Encode(src, dst, nil); err != nil {
		t.Fatalf("encode frame: %v", err)
	}
	encoded, err := dst.GetFrame(0)
	if err != nil {
		t.Fatalf("read encoded frame: %v", err)
	}
	return encoded
}

func nativeCAPCapability(t *testing.T, encoded []byte) uint16 {
	t.Helper()
	for index := 0; index+9 < len(encoded); index++ {
		if encoded[index] == 0xFF && encoded[index+1] == 0x50 {
			if got := binary.BigEndian.Uint16(encoded[index+2 : index+4]); got != 8 {
				t.Fatalf("CAP length = %d, want 8", got)
			}
			return binary.BigEndian.Uint16(encoded[index+8 : index+10])
		}
	}
	t.Fatal("missing CAP marker")
	return 0
}

func nativeTLMEntryCount(t *testing.T, encoded []byte) int {
	t.Helper()
	for index := 0; index+5 < len(encoded); index++ {
		if encoded[index] != 0xFF || encoded[index+1] != 0x55 {
			continue
		}
		length := int(binary.BigEndian.Uint16(encoded[index+2 : index+4]))
		if length < 4 || index+2+length > len(encoded) {
			t.Fatalf("invalid TLM length %d", length)
		}
		if encoded[index+5] != 0x60 {
			t.Fatalf("TLM Stlm = 0x%02X, want 0x60 for 16-bit tile indexes and 32-bit lengths", encoded[index+5])
		}
		payloadLength := length - 4
		if payloadLength%6 != 0 {
			t.Fatalf("TLM entry bytes = %d, want a multiple of 6", payloadLength)
		}
		return payloadLength / 6
	}
	t.Fatal("missing TLM marker required by fo-dicom Native/OpenJPH output")
	return 0
}

func markerPayload(t *testing.T, encoded []byte, marker byte) []byte {
	t.Helper()
	for index := 0; index+3 < len(encoded); index++ {
		if encoded[index] != 0xFF || encoded[index+1] != marker {
			continue
		}
		length := int(binary.BigEndian.Uint16(encoded[index+2 : index+4]))
		if length < 2 || index+2+length > len(encoded) {
			t.Fatalf("invalid FF%02X length %d", marker, length)
		}
		return encoded[index+4 : index+2+length]
	}
	t.Fatalf("missing FF%02X marker", marker)
	return nil
}
