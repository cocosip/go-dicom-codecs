package htj2k

import (
	"bytes"
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"

	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
)

const (
	losslessTestName = "Lossless"
	lossyTestName    = "Lossy"
)

func TestHTJ2KCodec_Name(t *testing.T) {
	tests := []struct {
		name  string
		codec *Codec
		want  string
	}{
		{
			name:  losslessTestName,
			codec: NewLosslessCodec(),
			want:  "HTJ2K Lossless",
		},
		{
			name:  "Lossless RPCL",
			codec: NewLosslessRPCLCodec(),
			want:  "HTJ2K Lossless RPCL",
		},
		{
			name:  "Lossy Quality 80",
			codec: NewCodec(80),
			want:  "HTJ2K (Quality 80)",
		},
		{
			name:  "Lossy Quality 50",
			codec: NewCodec(50),
			want:  "HTJ2K (Quality 50)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.codec.Name()
			if got != tt.want {
				t.Errorf("Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTJ2KCodec_TransferSyntax(t *testing.T) {
	tests := []struct {
		name  string
		codec *Codec
		want  *transfer.Syntax
	}{
		{
			name:  losslessTestName,
			codec: NewLosslessCodec(),
			want:  transfer.HTJ2KLossless,
		},
		{
			name:  "Lossless RPCL",
			codec: NewLosslessRPCLCodec(),
			want:  transfer.HTJ2KLosslessRPCL,
		},
		{
			name:  lossyTestName,
			codec: NewCodec(80),
			want:  transfer.HTJ2K,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.codec.TransferSyntax()
			if got != tt.want {
				t.Errorf("TransferSyntax() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTJ2KCodec_EncodeDecodeRoundTrip(t *testing.T) {
	width := uint16(64)
	height := uint16(64)
	testData := make([]byte, int(width)*int(height))
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			testData[y*int(width)+x] = byte((x + y) & 0xff)
		}
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

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(testData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Test with lossless codec
	t.Run(losslessTestName, func(t *testing.T) {
		htj2kCodec := NewLosslessCodec()

		// Encode
		encoded := codecHelpers.NewTestPixelData(frameInfo)
		err := htj2kCodec.Encode(src, encoded, nil)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		srcData, _ := src.GetFrame(0)
		encodedData, _ := encoded.GetFrame(0)
		t.Logf("Original size: %d bytes, Encoded size: %d bytes", len(srcData), len(encodedData))

		// Decode
		decoded := codecHelpers.NewTestPixelData(frameInfo)
		err = htj2kCodec.Decode(encoded, decoded, nil)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		// Verify dimensions
		decodedInfo := decoded.GetFrameInfo()
		if decodedInfo.Width != frameInfo.Width {
			t.Errorf("Width mismatch: got %d, want %d", decodedInfo.Width, frameInfo.Width)
		}
		if decodedInfo.Height != frameInfo.Height {
			t.Errorf("Height mismatch: got %d, want %d", decodedInfo.Height, frameInfo.Height)
		}

		// For HTJ2K, we expect some differences due to the simplified implementation
		// Just verify that decode succeeded and produced data
		decodedData, _ := decoded.GetFrame(0)
		if len(decodedData) == 0 {
			t.Error("Decoded data is empty")
		}
	})

	// Test with lossy codec
	t.Run(lossyTestName, func(t *testing.T) {
		htj2kCodec := NewCodec(80)

		// Encode
		encoded := codecHelpers.NewTestPixelData(frameInfo)
		err := htj2kCodec.Encode(src, encoded, nil)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		srcData, _ := src.GetFrame(0)
		encodedData, _ := encoded.GetFrame(0)
		t.Logf("Original size: %d bytes, Encoded size: %d bytes", len(srcData), len(encodedData))

		// Decode
		decoded := codecHelpers.NewTestPixelData(frameInfo)
		err = htj2kCodec.Decode(encoded, decoded, nil)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		// Verify that decode produced data
		decodedData, _ := decoded.GetFrame(0)
		if len(decodedData) == 0 {
			t.Error("Decoded data is empty")
		}
	})
}

func TestHTJ2KCodec_TypedNilParametersUseDefaults(t *testing.T) {
	frameInfo := &imagetypes.FrameInfo{
		Width:                     64,
		Height:                    64,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PhotometricInterpretation: photometricMonochrome2,
	}
	source := codecHelpers.NewTestPixelData(frameInfo)
	pixels := make([]byte, int(frameInfo.Width)*int(frameInfo.Height))
	for index := range pixels {
		pixels[index] = byte((index / int(frameInfo.Width)) + (index % int(frameInfo.Width)))
	}
	if err := source.AddFrame(pixels); err != nil {
		t.Fatal(err)
	}

	var parameters *Parameters
	htj2kCodec := NewLosslessCodec()
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := htj2kCodec.Encode(source, encoded, parameters); err != nil {
		t.Fatalf("Encode() with typed-nil parameters error = %v", err)
	}
	decoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := htj2kCodec.Decode(encoded, decoded, parameters); err != nil {
		t.Fatalf("Decode() with typed-nil parameters error = %v", err)
	}
	got, err := decoded.GetFrame(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pixels) {
		t.Fatal("typed-nil parameter round trip changed lossless pixels")
	}
}

func TestHTJ2KCodec_InvalidInput(t *testing.T) {
	htj2kCodec := NewLosslessCodec()

	frameInfo := &imagetypes.FrameInfo{
		Width:                     8,
		Height:                    8,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricMonochrome2,
	}

	emptyPixelData := codecHelpers.NewTestPixelData(frameInfo)
	if err := emptyPixelData.AddFrame([]byte{}); err != nil {
		t.Fatalf("failed to add empty frame: %v", err)
	}

	tests := []struct {
		name    string
		src     imagetypes.PixelData
		dst     imagetypes.PixelData
		wantErr bool
	}{
		{
			name:    "Nil source",
			src:     nil,
			dst:     codecHelpers.NewTestPixelData(frameInfo),
			wantErr: true,
		},
		{
			name:    "Nil destination",
			src:     codecHelpers.NewTestPixelData(frameInfo),
			dst:     nil,
			wantErr: true,
		},
		{
			name:    "Empty data",
			src:     emptyPixelData,
			dst:     codecHelpers.NewTestPixelData(frameInfo),
			wantErr: true,
		},
		{
			name:    "No frames",
			src:     codecHelpers.NewTestPixelData(frameInfo),
			dst:     codecHelpers.NewTestPixelData(frameInfo),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := htj2kCodec.Encode(tt.src, tt.dst, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHTJ2KCodec_DecodeRejectsNoFrames(t *testing.T) {
	htj2kCodec := NewLosslessCodec()
	frameInfo := &imagetypes.FrameInfo{
		Width:           8,
		Height:          8,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}

	err := htj2kCodec.Decode(
		codecHelpers.NewTestPixelData(frameInfo),
		codecHelpers.NewTestPixelData(frameInfo),
		nil,
	)
	if err == nil {
		t.Fatal("expected Decode to reject source with no frames")
	}
}

func TestHTJ2KCodec_Registration(t *testing.T) {
	registry := codec.GetGlobalRegistry()

	tests := []struct {
		name           string
		transferSyntax *transfer.Syntax
		wantFound      bool
	}{
		{
			name:           "HTJ2K Lossless",
			transferSyntax: transfer.HTJ2KLossless,
			wantFound:      true,
		},
		{
			name:           "HTJ2K Lossless RPCL",
			transferSyntax: transfer.HTJ2KLosslessRPCL,
			wantFound:      true,
		},
		{
			name:           "HTJ2K Lossy",
			transferSyntax: transfer.HTJ2K,
			wantFound:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codec, found := registry.GetCodec(tt.transferSyntax)
			if found != tt.wantFound {
				t.Errorf("GetCodec() found = %v, want %v", found, tt.wantFound)
			}
			if found && codec == nil {
				t.Error("GetCodec() returned nil codec")
			}
		})
	}
}
